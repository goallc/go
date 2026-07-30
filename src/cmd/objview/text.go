// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmd/internal/disasm"
	"cmd/internal/objfile"
	"cmd/internal/sys"
	"encoding/hex"
	"errors"
	"fmt"
	"internal/abi"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

func writeTextFile(w io.Writer, path string) error {
	doc, err := parseCanonicalFile(path)
	if err != nil {
		return err
	}

	var out strings.Builder
	printed := false
	for i := range doc.Members {
		member := &doc.Members[i]
		if member.GoObject == nil {
			continue
		}
		memberPrinted, err := writeTextMember(&out, member, doc.Archive, printed)
		if err != nil {
			return err
		}
		printed = printed || memberPrinted
	}
	if !printed {
		return errors.New("no Go text symbols found")
	}
	_, err = io.WriteString(w, out.String())
	return err
}

func writeTextMember(out *strings.Builder, member *canonicalMember, archive, printedBefore bool) (bool, error) {
	if len(member.rawEntry) == 0 {
		return false, fmt.Errorf("member %q: missing raw Go object data", member.Name)
	}
	temp, err := os.CreateTemp("", "objview-*.goobj")
	if err != nil {
		return false, fmt.Errorf("member %q: create disassembly input: %w", member.Name, err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(member.rawEntry); err != nil {
		temp.Close()
		return false, fmt.Errorf("member %q: write disassembly input: %w", member.Name, err)
	}
	if err := temp.Close(); err != nil {
		return false, fmt.Errorf("member %q: close disassembly input: %w", member.Name, err)
	}

	file, err := objfile.Open(tempName)
	if err != nil {
		return false, fmt.Errorf("member %q: open disassembly input: %w", member.Name, err)
	}
	defer file.Close()
	if len(file.Entries()) != 1 {
		return false, fmt.Errorf("member %q: extracted Go object has %d entries, want 1", member.Name, len(file.Entries()))
	}
	entry := file.Entries()[0]
	if got := entry.GOARCH(); got != member.Arch {
		return false, fmt.Errorf("member %q: extracted Go object architecture is %q, want %q", member.Name, got, member.Arch)
	}
	d, err := disasm.DisasmForFile(file)
	if err != nil {
		return false, fmt.Errorf("member %q: create %s disassembler: %w", member.Name, member.Arch, err)
	}
	arch := findTextArch(member.Arch)
	if arch == nil {
		return false, fmt.Errorf("member %q: unsupported architecture %q", member.Name, member.Arch)
	}
	syms, err := entry.Symbols()
	if err != nil {
		return false, fmt.Errorf("member %q: read symbols: %w", member.Name, err)
	}
	symsByAddr := make(map[uint64]objfile.Sym)
	for _, sym := range syms {
		if sym.Code == 'T' || sym.Code == 't' {
			symsByAddr[sym.Addr] = sym
		}
	}

	memberPrinted := false
	for i := range member.GoObject.Symbols {
		sym := &member.GoObject.Symbols[i]
		if sym.Function == nil {
			continue
		}
		objSym, ok := symsByAddr[sym.dataOffset]
		if !ok {
			return false, fmt.Errorf("member %q: function %q: no text symbol at PC %#x", member.Name, sym.Name, sym.dataOffset)
		}
		if memberPrinted {
			out.WriteByte('\n')
		} else if printedBefore {
			out.WriteByte('\n')
		}
		if archive && !memberPrinted {
			fmt.Fprintf(out, "GOOBJ %q %s/%s\n\n", member.Name, member.GOOS, member.Arch)
		}
		if err := writeTextFunction(out, d, arch, sym, objSym); err != nil {
			return false, fmt.Errorf("member %q: function %q: %w", member.Name, sym.Name, err)
		}
		memberPrinted = true
	}
	return memberPrinted, nil
}

func findTextArch(name string) *sys.Arch {
	for _, arch := range sys.Archs {
		if arch.Name == name {
			return arch
		}
	}
	return nil
}

type decodedInstruction struct {
	PC      uint64
	Size    uint64
	Bytes   []byte
	Text    string
	Unknown bool
}

func decodeTextInstructions(d *disasm.Disasm, sym *canonicalSymbol, relocs []objfile.Reloc) (instructions []decodedInstruction, err error) {
	start := sym.dataOffset
	end := start + uint64(len(sym.rawData))
	expected := start
	defer func() {
		if p := recover(); p != nil {
			instructions = nil
			err = fmt.Errorf("decoder panic at PC %#x: %v", expected, p)
		}
	}()
	d.Decode(start, end, relocs, false, func(pc, size uint64, _ string, _ int, text string) {
		if err != nil {
			return
		}
		if pc != expected {
			err = fmt.Errorf("decoder produced PC %#x after %#x", pc, expected)
			return
		}
		if size == 0 || size > end-pc {
			err = fmt.Errorf("decoder produced invalid %d-byte instruction at PC %#x for range ending %#x", size, pc, end)
			return
		}
		offset := pc - start
		bytes := sym.rawData[offset : offset+size]
		opcode := text
		if before, _, ok := strings.Cut(opcode, "\t"); ok {
			opcode = before
		}
		instructions = append(instructions, decodedInstruction{
			PC:      pc,
			Size:    size,
			Bytes:   bytes,
			Text:    text,
			Unknown: strings.TrimSpace(opcode) == "?",
		})
		expected = pc + size
	})
	if err != nil {
		return nil, err
	}
	if expected != end {
		return nil, fmt.Errorf("decoder stopped at PC %#x, want %#x", expected, end)
	}
	return instructions, nil
}

func writeTextFunction(w io.Writer, d *disasm.Disasm, arch *sys.Arch, sym *canonicalSymbol, objSym objfile.Sym) error {
	if err := validateTextFunction(sym); err != nil {
		return err
	}
	instructions, err := decodeTextInstructions(d, sym, objSym.Relocs)
	if err != nil {
		return fmt.Errorf("disassemble: %w", err)
	}
	fn := sym.Function
	source, _ := functionSourceAt(fn, 0)
	fmt.Fprintf(w, "TEXT %s(SB) %s\n", sym.Name, source)
	if fn.Info == nil {
		fmt.Fprintf(w, "  size=%d args=? locals=?\n", sym.Size)
	} else {
		fmt.Fprintf(w, "  size=%d args=%d locals=%d\n", sym.Size, fn.Info.Args, fn.Info.Locals)
	}

	argsMaps := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_ArgsPointerMaps)
	localsMaps := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_LocalsPointerMaps)
	if argsMaps != nil && argsMaps.StackMap != nil {
		fmt.Fprintf(w, "  FUNCDATA_ArgsPointerMaps %s\n", formatStackMap(argsMaps.StackMap))
	}
	if localsMaps != nil && localsMaps.StackMap != nil {
		fmt.Fprintf(w, "  FUNCDATA_LocalsPointerMaps %s\n", formatStackMap(localsMaps.StackMap))
	}
	if argsMaps != nil && argsMaps.StackMap != nil && localsMaps != nil && localsMaps.StackMap != nil {
		selection, err := formatStackMapSelection(0, argsMaps.StackMap, localsMaps.StackMap)
		if err != nil {
			return fmt.Errorf("entry stack map: %w", err)
		}
		fmt.Fprintf(w, "  entry safepoint %s\n", selection)
	}
	if stackObjects := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_StackObjects); stackObjects != nil && len(stackObjects.StackObjects) != 0 {
		for _, object := range stackObjects.StackObjects {
			gcdata := "<unresolved>"
			if object.GCData != nil {
				gcdata = object.GCData.Name
				if gcdata == "" {
					gcdata = object.GCData.PkgKind
				}
			}
			pointerWords := int(object.PtrBytes) / arch.PtrSize
			fmt.Fprintf(w, "  FUNCDATA_StackObjects independent object[%d]={offset=%d size=%d ptrbytes=%d pointers=%s gcdata=%s+%d}\n",
				object.Index, object.Offset, object.Size, object.PtrBytes,
				formatBitmap(pointerWords, object.gcBits), gcdata, object.gcDataAddend)
		}
	}

	argLive, err := decodeArgLive(fn)
	if err != nil {
		return err
	}
	if argLive != nil {
		fmt.Fprintf(w, "  FUNCDATA_ArgLiveInfo start=%d tracked=%d", argLive.startOffset, argLive.slotCount)
		for _, offset := range argLive.mapOffsets {
			bits, err := argLive.bits(offset)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, " bitmap@%d=%s", offset, formatBitmap(argLive.slotCount, bits))
		}
		fmt.Fprintln(w)
	}

	queries := make(map[uint64][]canonicalStackMapQuery)
	for _, query := range fn.StackMapQueries {
		queries[query.ReturnPC] = append(queries[query.ReturnPC], query)
	}

	tw := tabwriter.NewWriter(w, 0, 4, 1, ' ', 0)
	defer tw.Flush()
	var previousMetadata string
	var previousInstruction string
	haveInstruction := false
	matchedQueries := 0
	seenQueries := make(map[canonicalStackMapQuery]bool)
	instructionBoundaries := map[uint64]bool{0: true}
	for _, inst := range instructions {
		offset := inst.PC - sym.dataOffset
		if haveInstruction && isTerminalInstruction(previousInstruction) &&
			noRelocationAtOrAfter(sym.Relocations, offset) && allZero(sym.rawData[offset:]) {
			instructionBoundaries[uint64(len(sym.rawData))] = true
			file, line := functionSourceAt(fn, offset)
			metadata, err := formatPCMetadata(fn, offset, argLive, argsMaps, localsMaps)
			if err != nil {
				return fmt.Errorf("PC +%#x: %w", offset, err)
			}
			fmt.Fprintf(tw, "  %s:%d\t+0x%04x\t%#x\t%s\tpadding(%d)",
				textBase(file), line, offset, inst.PC, formatInstructionBytes(arch, sym.rawData[offset:]), len(sym.rawData)-int(offset))
			if metadata != previousMetadata {
				fmt.Fprintf(tw, "\t|\t%s", metadata)
			}
			fmt.Fprintln(tw)
			break
		}
		if inst.Unknown {
			return fmt.Errorf("PC +%#x: unknown instruction encoding %x", offset, inst.Bytes)
		}
		file, line := functionSourceAt(fn, offset)
		metadata, err := formatPCMetadata(fn, offset, argLive, argsMaps, localsMaps)
		if err != nil {
			return fmt.Errorf("PC +%#x: %w", offset, err)
		}
		fmt.Fprintf(tw, "  %s:%d\t+0x%04x\t%#x\t%s\t%s",
			textBase(file), line, offset, inst.PC, formatInstructionBytes(arch, inst.Bytes), strings.ReplaceAll(inst.Text, "\t", " "))
		if metadata != previousMetadata {
			fmt.Fprintf(tw, "\t|\t%s", metadata)
		}
		fmt.Fprintln(tw)
		previousMetadata = metadata
		previousInstruction = inst.Text
		haveInstruction = true
		instructionBoundaries[offset+inst.Size] = true

		for _, query := range queries[offset+inst.Size] {
			if query.CallOffset < int32(offset) || uint64(query.CallOffset) >= offset+inst.Size {
				return fmt.Errorf("PC +%#x call query relocation +%#x is outside instruction", offset, query.CallOffset)
			}
			if query.DecodeError != "" {
				return fmt.Errorf("PC +%#x call query: %s", offset, query.DecodeError)
			}
			matchedQueries++
			seenQueries[query] = true
			kind := "ordinary"
			index := query.StackMapIndex
			if strings.HasPrefix(query.Target.Name, "runtime.morestack") || strings.Contains(inst.Text, "runtime.morestack") {
				kind = "stack-growth"
				if index < 0 {
					index = 0
				}
			} else if index < 0 {
				index = 0
				kind += "-entry-fallback"
			}
			selection := ""
			if argsMaps != nil && argsMaps.StackMap != nil && localsMaps != nil && localsMaps.StackMap != nil {
				selection, err = formatStackMapSelection(index, argsMaps.StackMap, localsMaps.StackMap)
				if err != nil {
					return fmt.Errorf("PC +%#x %s safepoint: %w", offset, kind, err)
				}
			}
			fmt.Fprintf(tw, "\t\t\t\t\t|\t%s safepoint return=+0x%x lookup=+0x%x",
				kind, query.ReturnPC, query.LookupPC)
			if selection != "" {
				fmt.Fprintf(tw, " %s", selection)
			}
			fmt.Fprintln(tw)
		}
	}
	if matchedQueries != len(fn.StackMapQueries) {
		for _, query := range fn.StackMapQueries {
			if !seenQueries[query] {
				return fmt.Errorf("call relocation +%#x (%s) with return PC +%#x did not match an instruction",
					query.CallOffset, query.RelocationType, query.ReturnPC)
			}
		}
		return fmt.Errorf("matched %d of %d call relocations", matchedQueries, len(fn.StackMapQueries))
	}
	for _, table := range append(append([]canonicalPCTable(nil), fn.PCTables...), fn.PCData...) {
		for i, r := range table.Ranges {
			if !instructionBoundaries[r.Start] || !instructionBoundaries[r.End] {
				return fmt.Errorf("%s range %d [%#x,%#x) is not instruction-aligned",
					textPCTableName(table), i, r.Start, r.End)
			}
		}
	}
	return nil
}

func validateTextFunction(sym *canonicalSymbol) error {
	fn := sym.Function
	if fn == nil {
		return errors.New("missing function metadata")
	}
	if len(sym.rawData) != int(sym.Size) {
		return fmt.Errorf("text data has %d bytes, symbol size is %d", len(sym.rawData), sym.Size)
	}
	if sym.Size == 0 {
		return errors.New("text symbol has no machine code")
	}
	if fn.PCQuantum <= 0 {
		return fmt.Errorf("invalid PC quantum %d", fn.PCQuantum)
	}
	if fn.Info == nil {
		return errors.New("missing or malformed FuncInfo")
	}
	for _, kind := range []string{"pcsp", "pcfile", "pcline"} {
		if findCanonicalPCTable(fn.PCTables, kind) == nil {
			return fmt.Errorf("missing %s table", kind)
		}
	}
	for _, aux := range sym.Aux {
		if aux.Type == "funcinfo" && aux.Target.PkgKind != "invalid" && fn.Info == nil {
			return errors.New("FUNCDATA FuncInfo is malformed")
		}
	}
	for _, table := range append(append([]canonicalPCTable(nil), fn.PCTables...), fn.PCData...) {
		name := textPCTableName(table)
		if table.Error != "" {
			return fmt.Errorf("%s: %s", name, table.Error)
		}
		if table.Symbol.PkgKind == "invalid" {
			return fmt.Errorf("%s has an invalid symbol reference", name)
		}
		if table.Symbol.PkgKind != "hashed" {
			return fmt.Errorf("%s uses %s carrier; standard Go object disassembly requires hashed PC metadata", name, table.Symbol.PkgKind)
		}
		if len(table.Ranges) == 0 {
			if table.Symbol.PkgKind == "hashed" &&
				(table.Index >= 0 || table.Kind == "pcinline") &&
				len(table.Raw) == 0 {
				// An empty native PCDATA or pcinline carrier represents the
				// initial value -1 for the whole function.
				continue
			}
			return fmt.Errorf("%s has no decoded ranges", name)
		}
		var previousEnd uint64
		for i, r := range table.Ranges {
			if r.Start != previousEnd {
				return fmt.Errorf("%s range %d starts at %#x after %#x", name, i, r.Start, previousEnd)
			}
			if r.End <= r.Start || r.End > uint64(sym.Size) {
				return fmt.Errorf("%s range [%#x,%#x) is outside %d-byte function", name, r.Start, r.End, sym.Size)
			}
			if table.Kind == "pcfile" && r.Value >= 0 && r.File == "" {
				return fmt.Errorf("pcfile index %d at [%#x,%#x) is unresolved", r.Value, r.Start, r.End)
			}
			if table.Index == abi.PCDATA_StackMapIndex && r.Value < -1 {
				return fmt.Errorf("PCDATA_StackMapIndex has invalid value %d at [%#x,%#x)", r.Value, r.Start, r.End)
			}
			previousEnd = r.End
		}
		if end := table.Ranges[len(table.Ranges)-1].End; end != uint64(sym.Size) {
			return fmt.Errorf("%s ends at %#x, want function size %#x", name, end, sym.Size)
		}
	}
	for _, data := range fn.FuncData {
		if data.DecodeError != "" {
			return fmt.Errorf("%s: %s", textFuncDataName(data.Index), data.DecodeError)
		}
		if data.Index == abi.FUNCDATA_StackObjects {
			for _, object := range data.StackObjects {
				if object.PtrBytes != 0 && !object.gcDataDecoded {
					return fmt.Errorf("FUNCDATA_StackObjects object %d has no local decoded GC bitmap", object.Index)
				}
			}
		}
	}

	args := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_ArgsPointerMaps)
	locals := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_LocalsPointerMaps)
	var selectedStackMap bool
	for _, table := range fn.PCData {
		if table.Index != abi.PCDATA_StackMapIndex {
			continue
		}
		for _, r := range table.Ranges {
			selectedStackMap = selectedStackMap || r.Value >= 0
		}
	}
	if (selectedStackMap || len(fn.StackMapQueries) != 0) &&
		(args == nil || args.StackMap == nil || locals == nil || locals.StackMap == nil) {
		return errors.New("safepoint requires paired FUNCDATA ArgsPointerMaps and LocalsPointerMaps")
	}
	if args != nil && args.StackMap != nil && locals != nil && locals.StackMap != nil {
		if args.StackMap.Count != locals.StackMap.Count {
			return fmt.Errorf("FUNCDATA ArgsPointerMaps count %d differs from LocalsPointerMaps count %d", args.StackMap.Count, locals.StackMap.Count)
		}
		for _, table := range fn.PCData {
			if table.Index != abi.PCDATA_StackMapIndex {
				continue
			}
			for _, r := range table.Ranges {
				if r.Value >= args.StackMap.Count {
					return fmt.Errorf("PCDATA_StackMapIndex value %d at [%#x,%#x) exceeds %d maps", r.Value, r.Start, r.End, args.StackMap.Count)
				}
			}
		}
	}
	for _, query := range fn.StackMapQueries {
		if query.DecodeError != "" {
			return fmt.Errorf("call at +%#x (%s): %s", query.CallOffset, query.RelocationType, query.DecodeError)
		}
		if query.CallOffset < 0 || uint64(query.CallOffset) >= uint64(sym.Size) {
			return fmt.Errorf("call relocation +%#x (%s) is outside %d-byte function", query.CallOffset, query.RelocationType, sym.Size)
		}
		if query.ReturnPC == 0 || query.ReturnPC > uint64(sym.Size) || query.LookupPC != query.ReturnPC-1 {
			return fmt.Errorf("call relocation +%#x (%s) has invalid return PC +%#x and lookup PC +%#x",
				query.CallOffset, query.RelocationType, query.ReturnPC, query.LookupPC)
		}
	}
	return nil
}

func formatPCMetadata(fn *canonicalFunction, pc uint64, argLive *argLiveInfo, argsMaps, localsMaps *canonicalFuncData) (string, error) {
	var fields []string
	if table := findCanonicalPCTable(fn.PCTables, "pcsp"); table != nil && table.Symbol.PkgKind != "invalid" {
		value, ok := pcValueAt(table.Ranges, pc)
		if !ok {
			return "", errors.New("pcsp has no value")
		}
		fields = append(fields, fmt.Sprintf("pcsp=%d", value))
	} else {
		fields = append(fields, "pcsp=?")
	}
	for _, table := range fn.PCData {
		if table.Symbol.PkgKind == "invalid" {
			continue
		}
		value, ok := textPCValueAt(&table, pc)
		if !ok {
			return "", fmt.Errorf("%s has no value", textPCTableName(table))
		}
		field := fmt.Sprintf("%s=%d", textPCDataName(table.Index), value)
		if table.Index == abi.PCDATA_StackMapIndex && value >= 0 &&
			argsMaps != nil && argsMaps.StackMap != nil &&
			localsMaps != nil && localsMaps.StackMap != nil {
			selection, err := formatStackMapSelection(value, argsMaps.StackMap, localsMaps.StackMap)
			if err != nil {
				return "", fmt.Errorf("PCDATA_StackMapIndex=%d: %w", value, err)
			}
			field += "(" + strings.TrimPrefix(selection, fmt.Sprintf("map[%d] ", value)) + ")"
		}
		if table.Index == abi.PCDATA_ArgLiveIndex && argLive != nil {
			if value <= 0 {
				field += "(all)"
			} else {
				bits, err := argLive.bits(value)
				if err != nil {
					return "", fmt.Errorf("PCDATA_ArgLiveIndex=%d: %w", value, err)
				}
				field += fmt.Sprintf("(%s)", formatBitmap(argLive.slotCount, bits))
			}
		}
		fields = append(fields, field)
	}
	return strings.Join(fields, " "), nil
}

func textPCValueAt(table *canonicalPCTable, pc uint64) (int32, bool) {
	if len(table.Ranges) == 0 && table.Symbol.PkgKind == "hashed" && len(table.Raw) == 0 {
		return -1, true
	}
	return pcValueAt(table.Ranges, pc)
}

func functionSourceAt(fn *canonicalFunction, pc uint64) (string, int32) {
	var file string
	var line int32
	if table := findCanonicalPCTable(fn.PCTables, "pcfile"); table != nil {
		for _, r := range table.Ranges {
			if r.Start <= pc && pc < r.End {
				file = r.File
				break
			}
		}
	}
	if table := findCanonicalPCTable(fn.PCTables, "pcline"); table != nil {
		if value, ok := pcValueAt(table.Ranges, pc); ok {
			line = value
		}
	}
	if file == "" {
		file = "?"
	}
	return file, line
}

func pcValueAt(ranges []canonicalPCRange, pc uint64) (int32, bool) {
	for _, r := range ranges {
		if r.Start <= pc && pc < r.End {
			return r.Value, true
		}
	}
	return 0, false
}

func findFuncDataIndex(data []canonicalFuncData, index int) *canonicalFuncData {
	for i := range data {
		if data[i].Index == index {
			return &data[i]
		}
	}
	return nil
}

func formatStackMap(stackMap *canonicalStackMap) string {
	var out strings.Builder
	fmt.Fprintf(&out, "count=%d bits=%d", stackMap.Count, stackMap.NumBits)
	for _, bitmap := range stackMap.Bitmaps {
		fmt.Fprintf(&out, " map[%d]=%s", bitmap.Index, formatBitmap(int(stackMap.NumBits), bitmap.SetBits))
	}
	return out.String()
}

func formatStackMapSelection(index int32, args, locals *canonicalStackMap) (string, error) {
	if index < 0 || index >= args.Count || index >= locals.Count {
		return "", fmt.Errorf("map index %d is outside Args=%d Locals=%d", index, args.Count, locals.Count)
	}
	return fmt.Sprintf("map[%d] ArgsPointerMaps=%s LocalsPointerMaps=%s",
		index,
		formatBitmap(int(args.NumBits), args.Bitmaps[index].SetBits),
		formatBitmap(int(locals.NumBits), locals.Bitmaps[index].SetBits)), nil
}

// formatBitmap prints bit index 0 at the left, matching the order used by Go
// GC bitmap names such as runtime.gcbits.01. A dash represents a zero-width
// bitmap, which is distinct from an all-zero bitmap.
func formatBitmap(numBits int, setBits []int) string {
	if numBits == 0 {
		return "-"
	}
	out := strings.Repeat("0", numBits)
	bitmap := []byte(out)
	for _, bit := range setBits {
		bitmap[bit] = '1'
	}
	return string(bitmap)
}

func formatInstructionBytes(arch *sys.Arch, code []byte) string {
	if arch.Name == "386" || arch.Name == "amd64" || len(code)%4 != 0 {
		return hex.EncodeToString(code)
	}
	var out strings.Builder
	for len(code) != 0 {
		if out.Len() != 0 {
			out.WriteByte(' ')
		}
		fmt.Fprintf(&out, "%08x", arch.ByteOrder.Uint32(code))
		code = code[4:]
	}
	return out.String()
}

func noRelocationAtOrAfter(relocs []canonicalReloc, pc uint64) bool {
	for _, reloc := range relocs {
		if reloc.Offset >= 0 && uint64(reloc.Offset) >= pc {
			return false
		}
	}
	return true
}

func allZero(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

func isTerminalInstruction(text string) bool {
	op, _, _ := strings.Cut(text, " ")
	op, _, _ = strings.Cut(op, "\t")
	switch op {
	case "RET", "JMP":
		return true
	}
	return false
}

func textBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.LastIndex(path, `\`); i >= 0 {
		path = path[i+1:]
	}
	if path == "" {
		return "?"
	}
	return path
}

func textPCTableName(table canonicalPCTable) string {
	switch table.Kind {
	case "pcsp":
		return "pcsp"
	case "pcfile":
		return "pcfile"
	case "pcline":
		return "pcline"
	case "pcinline":
		return "pcinline"
	default:
		if table.Index >= 0 {
			return textPCDataName(table.Index)
		}
		return table.Kind
	}
}

func textPCDataName(index int) string {
	switch index {
	case abi.PCDATA_UnsafePoint:
		return "PCDATA_UnsafePoint"
	case abi.PCDATA_StackMapIndex:
		return "PCDATA_StackMapIndex"
	case abi.PCDATA_InlTreeIndex:
		return "PCDATA_InlTreeIndex"
	case abi.PCDATA_ArgLiveIndex:
		return "PCDATA_ArgLiveIndex"
	case abi.PCDATA_PanicBounds:
		return "PCDATA_PanicBounds"
	default:
		return fmt.Sprintf("PCDATA_%d", index)
	}
}

func textFuncDataName(index int) string {
	switch index {
	case abi.FUNCDATA_ArgsPointerMaps:
		return "FUNCDATA_ArgsPointerMaps"
	case abi.FUNCDATA_LocalsPointerMaps:
		return "FUNCDATA_LocalsPointerMaps"
	case abi.FUNCDATA_StackObjects:
		return "FUNCDATA_StackObjects"
	case abi.FUNCDATA_InlTree:
		return "FUNCDATA_InlTree"
	case abi.FUNCDATA_OpenCodedDeferInfo:
		return "FUNCDATA_OpenCodedDeferInfo"
	case abi.FUNCDATA_ArgInfo:
		return "FUNCDATA_ArgInfo"
	case abi.FUNCDATA_ArgLiveInfo:
		return "FUNCDATA_ArgLiveInfo"
	case abi.FUNCDATA_WrapInfo:
		return "FUNCDATA_WrapInfo"
	default:
		return fmt.Sprintf("FUNCDATA_%d", index)
	}
}

type argLiveInfo struct {
	startOffset uint8
	slotCount   int
	data        []byte
	mapOffsets  []int32
}

func decodeArgLive(fn *canonicalFunction) (*argLiveInfo, error) {
	info := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_ArgInfo)
	live := findFuncDataIndex(fn.FuncData, abi.FUNCDATA_ArgLiveInfo)
	if live == nil || live.Symbol.PkgKind == "invalid" {
		if table := findCanonicalPCTable(fn.PCData, "argument_live_index"); table != nil {
			for _, r := range table.Ranges {
				if r.Value > 0 {
					return nil, fmt.Errorf("PCDATA_ArgLiveIndex=%d at [%#x,%#x) has no FUNCDATA_ArgLiveInfo",
						r.Value, r.Start, r.End)
				}
			}
		}
		return nil, nil
	}
	if info == nil || info.Symbol.PkgKind == "invalid" {
		return nil, errors.New("FUNCDATA_ArgLiveInfo exists without FUNCDATA_ArgInfo")
	}
	if len(live.rawData) == 0 {
		return nil, errors.New("FUNCDATA_ArgLiveInfo is missing its start-offset byte")
	}
	start := live.rawData[0]
	slots, err := trackedArgSlots(info.rawData, start)
	if err != nil {
		return nil, fmt.Errorf("FUNCDATA_ArgInfo: %w", err)
	}
	bytesPerMap := (slots + 7) / 8
	if bytesPerMap == 0 {
		if len(live.rawData) != 1 {
			return nil, fmt.Errorf("FUNCDATA_ArgLiveInfo has %d unexpected bitmap bytes for no tracked slots", len(live.rawData)-1)
		}
	} else if (len(live.rawData)-1)%bytesPerMap != 0 {
		return nil, fmt.Errorf("FUNCDATA_ArgLiveInfo has %d bitmap bytes, not a multiple of %d", len(live.rawData)-1, bytesPerMap)
	}
	out := &argLiveInfo{startOffset: start, slotCount: slots, data: live.rawData}
	for offset := 1; offset < len(live.rawData); offset += bytesPerMap {
		if slots&7 != 0 {
			last := live.rawData[offset+bytesPerMap-1]
			valid := byte(1<<uint(slots&7)) - 1
			if last & ^valid != 0 {
				return nil, fmt.Errorf("FUNCDATA_ArgLiveInfo bitmap at offset %d has bits outside %d tracked slots", offset, slots)
			}
		}
		out.mapOffsets = append(out.mapOffsets, int32(offset))
	}
	if table := findCanonicalPCTable(fn.PCData, "argument_live_index"); table != nil {
		for _, r := range table.Ranges {
			if r.Value <= 0 {
				continue
			}
			if _, err := out.bits(r.Value); err != nil {
				return nil, fmt.Errorf("PCDATA_ArgLiveIndex=%d at [%#x,%#x): %w", r.Value, r.Start, r.End, err)
			}
		}
	}
	return out, nil
}

func trackedArgSlots(data []byte, startOffset uint8) (int, error) {
	count := 0
	depth := 0
	for i := 0; i < len(data); {
		offset := data[i]
		i++
		switch offset {
		case abi.TraceArgsEndSeq:
			if depth != 0 {
				return 0, fmt.Errorf("end marker has %d unclosed aggregates", depth)
			}
			if i != len(data) {
				return 0, errors.New("trailing bytes after end marker")
			}
			return count, nil
		case abi.TraceArgsStartAgg:
			depth++
			continue
		case abi.TraceArgsEndAgg:
			if depth == 0 {
				return 0, errors.New("aggregate end without aggregate start")
			}
			depth--
			continue
		case abi.TraceArgsDotdotdot, abi.TraceArgsOffsetTooLarge:
			continue
		default:
			if offset >= abi.TraceArgsSpecial {
				return 0, fmt.Errorf("unknown operator %#x", offset)
			}
			if i == len(data) {
				return 0, errors.New("missing size after argument offset")
			}
			size := data[i]
			i++
			if size == 0 {
				return 0, fmt.Errorf("argument at offset %d has invalid size %d", offset, size)
			}
			if offset >= startOffset {
				count++
			}
		}
	}
	return 0, errors.New("missing end marker")
}

func (a *argLiveInfo) bits(offset int32) ([]int, error) {
	if offset <= 0 {
		bits := make([]int, a.slotCount)
		for i := range bits {
			bits[i] = i
		}
		return bits, nil
	}
	bytesPerMap := (a.slotCount + 7) / 8
	if offset < 1 || uint64(offset)+uint64(bytesPerMap) > uint64(len(a.data)) {
		return nil, fmt.Errorf("bitmap offset %d needs %d bytes in %d-byte payload", offset, bytesPerMap, len(a.data))
	}
	bitmap := a.data[offset : int(offset)+bytesPerMap]
	var bits []int
	for bit := 0; bit < a.slotCount; bit++ {
		if bitmap[bit/8]&(1<<uint(bit&7)) != 0 {
			bits = append(bits, bit)
		}
	}
	return bits, nil
}
