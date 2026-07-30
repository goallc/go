// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	rawBytesPerLine = 32
	rawHalfLine     = rawBytesPerLine / 2
	rawHalfWidth    = rawHalfLine*3 - 1
	rawHexWidth     = rawHalfWidth*2 + 2
	rawLeftWidth    = 8 + 2 + rawHexWidth
)

// writeRawFile prints the exact bytes of each Go object block on the left and
// the canonical parser's interpretation of that block on the right.
func writeRawFile(w io.Writer, path string) error {
	doc, err := parseCanonicalFile(path)
	if err != nil {
		return err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "GOOBJRAW 2 archive=%t members=%d\n", doc.Archive, len(doc.Members))
	for i := range doc.Members {
		member := &doc.Members[i]
		fmt.Fprintf(&out, "\n== member[%d] name=%q kind=%s size=%d sha256=%s\n",
			i, member.Name, member.Kind, member.Size, member.SHA256)
		if member.ArchiveMetadata != nil {
			m := member.ArchiveMetadata
			fmt.Fprintf(&out, "   archive mtime=%d uid=%d gid=%d mode=%#o\n",
				m.Mtime, m.UID, m.GID, m.Mode)
		}
		if member.GoObject == nil {
			continue
		}
		fmt.Fprintf(&out, "   target goos=%q goarch=%q version=%q options=%s\n",
			member.GOOS, member.Arch, member.GoVersion, quoteStrings(member.BuildOptions))
		fmt.Fprintf(&out, "   text_header=%q\n", member.TextHeader)
		fmt.Fprintf(&out, "%-8s  %-*s | %s\n", "OFFSET", rawHexWidth, "HEX BYTES", "INTERPRETATION")
		if err := writeRawObject(&out, member.GoObject); err != nil {
			return fmt.Errorf("member[%d] %q: %w", i, member.Name, err)
		}
	}
	_, err = io.WriteString(w, out.String())
	return err
}

func writeRawObject(out *strings.Builder, object *canonicalGoObject) error {
	if len(object.Blocks) == 0 {
		return fmt.Errorf("Go object has no blocks")
	}
	headerEnd := int(object.Blocks[0].Offset)
	if headerEnd < 0 || headerEnd > len(object.rawData) {
		return fmt.Errorf("header end %#x is outside %d-byte object", headerEnd, len(object.rawData))
	}
	writeRawSection(out, "header", 0, object.rawData[:headerEnd], rawHeaderNotes(object))
	for i, block := range object.Blocks {
		start := uint64(block.Offset)
		end := start + uint64(block.Size)
		if end > uint64(len(object.rawData)) {
			return fmt.Errorf("block[%d] %s range [%#x,%#x) is outside %d-byte object",
				i, block.Name, start, end, len(object.rawData))
		}
		title := fmt.Sprintf("block[%d] %s sha256=%s", i, block.Name, block.SHA256)
		writeRawSection(out, title, block.Offset, object.rawData[start:end], rawBlockNotes(object, block.Name))
	}
	return nil
}

func writeRawSection(out *strings.Builder, title string, start uint32, data []byte, notes []string) {
	fmt.Fprintf(out, "-- %s offset=%#x size=%#x\n", title, start, len(data))
	hexLines := (len(data) + rawBytesPerLine - 1) / rawBytesPerLine
	lines := max(hexLines, len(notes))
	if lines == 0 {
		notes = []string{"(empty)"}
		lines = 1
	}
	for line := 0; line < lines; line++ {
		if line < hexLines {
			at := line * rawBytesPerLine
			end := min(at+rawBytesPerLine, len(data))
			fmt.Fprintf(out, "%08x  %s", uint64(start)+uint64(at), formatRawHex(data[at:end]))
		} else {
			fmt.Fprintf(out, "%*s", rawLeftWidth, "")
		}
		fmt.Fprint(out, " | ")
		if line < len(notes) {
			fmt.Fprint(out, notes[line])
		}
		fmt.Fprintln(out)
	}
}

func formatRawHex(data []byte) string {
	parts := make([]string, rawBytesPerLine)
	for i := range parts {
		parts[i] = "  "
	}
	for i, b := range data {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	left := strings.Join(parts[:rawHalfLine], " ")
	right := strings.Join(parts[rawHalfLine:], " ")
	return fmt.Sprintf("%-*s  %-*s", rawHalfWidth, left, rawHalfWidth, right)
}

func rawHeaderNotes(object *canonicalGoObject) []string {
	h := object.Header
	notes := []string{
		fmt.Sprintf("magic=%q", h.Magic),
		"fingerprint=" + h.Fingerprint,
		fmt.Sprintf("flags=%#x names=%s", h.Flags, quoteStrings(h.FlagNames)),
	}
	for i, block := range object.Blocks {
		notes = append(notes, fmt.Sprintf("block[%d] %s offset=%#x size=%#x",
			i, block.Name, block.Offset, block.Size))
	}
	return notes
}

func rawBlockNotes(object *canonicalGoObject, name string) []string {
	switch name {
	case "autolib":
		var notes []string
		for i, lib := range object.Autolib {
			notes = append(notes, fmt.Sprintf("autolib[%d] package=%q fingerprint=%s",
				i, lib.Package, lib.Fingerprint))
		}
		return notes
	case "package_index":
		var notes []string
		for i, pkg := range object.Packages {
			notes = append(notes, fmt.Sprintf("package[%d]=%q", i, pkg))
		}
		return notes
	case "file":
		var notes []string
		for i, file := range object.Files {
			notes = append(notes, fmt.Sprintf("file[%d]=%q", i, file))
		}
		return notes
	case "symbol_def":
		return rawDefinitionNotes(object, "package")
	case "hashed64_def":
		return rawDefinitionNotes(object, "hashed64")
	case "hashed_def":
		return rawDefinitionNotes(object, "hashed")
	case "nonpackage_def":
		return rawDefinitionNotes(object, "nonpackage")
	case "nonpackage_ref":
		var notes []string
		for i := range object.References {
			notes = append(notes, rawSymbolNote("reference", &object.References[i]))
		}
		return notes
	case "reference_flags":
		var notes []string
		for i, flags := range object.RefFlags {
			notes = append(notes, fmt.Sprintf(
				"reference_flags[%d] target={%s} flags=%#x names=%s flags2=%#x names2=%s",
				i, rawReference(flags.Target), flags.Flags, quoteStrings(flags.FlagNames),
				flags.Flags2, quoteStrings(flags.Flag2Names)))
		}
		return notes
	case "hash64":
		return rawHashNotes(object, "hashed64")
	case "hash":
		return rawHashNotes(object, "hashed")
	case "relocation_index", "aux_index", "data_index":
		return rawIndexNotes(object, name)
	case "relocation":
		return rawRelocationNotes(object)
	case "aux":
		return rawAuxNotes(object)
	case "data":
		return rawDataNotes(object)
	case "reference_name":
		var notes []string
		for i, refName := range object.RefNames {
			notes = append(notes, fmt.Sprintf("reference_name[%d] target={%s} name=%q",
				i, rawReference(refName.Target), refName.Name))
		}
		return notes
	default:
		return nil
	}
}

func rawDefinitionNotes(object *canonicalGoObject, class string) []string {
	var notes []string
	for i := range object.Symbols {
		sym := &object.Symbols[i]
		if sym.Class == class {
			notes = append(notes, rawSymbolNote("symbol", sym))
		}
	}
	return notes
}

func rawSymbolNote(record string, sym *canonicalSymbol) string {
	note := fmt.Sprintf("%s[%d] class=%s class_index=%d name=%q abi=%d kind=%s(%d) size=%d align=%d flags=%#x names=%s flags2=%#x names2=%s",
		record, sym.Index, sym.Class, sym.ClassIndex, sym.Name, sym.ABI,
		sym.Kind, sym.KindValue, sym.Size, sym.Align, sym.Flags,
		quoteStrings(sym.FlagNames), sym.Flags2, quoteStrings(sym.Flag2Names))
	if sym.Hash != "" {
		note += " hash=" + sym.Hash
	}
	return note
}

func rawHashNotes(object *canonicalGoObject, class string) []string {
	var notes []string
	for i := range object.Symbols {
		sym := &object.Symbols[i]
		if sym.Class == class {
			notes = append(notes, fmt.Sprintf("hash[%d] symbol[%d]=%q value=%s",
				len(notes), sym.Index, sym.Name, sym.Hash))
		}
	}
	return notes
}

func rawIndexNotes(object *canonicalGoObject, name string) []string {
	var block *canonicalBlock
	for i := range object.Blocks {
		if object.Blocks[i].Name == name {
			block = &object.Blocks[i]
			break
		}
	}
	if block == nil {
		return nil
	}
	data := object.rawData[block.Offset : block.Offset+block.Size]
	notes := make([]string, 0, len(data)/4)
	for i := 0; i+4 <= len(data); i += 4 {
		notes = append(notes, fmt.Sprintf("%s[%d]=%d", name, i/4, binary.LittleEndian.Uint32(data[i:])))
	}
	return notes
}

func rawRelocationNotes(object *canonicalGoObject) []string {
	var notes []string
	for i := range object.Symbols {
		sym := &object.Symbols[i]
		for j, reloc := range sym.Relocations {
			notes = append(notes, fmt.Sprintf(
				"symbol[%d]=%q reloc[%d] offset=%d size=%d type=%s(%d) weak=%t addend=%d target={%s}",
				sym.Index, sym.Name, j, reloc.Offset, reloc.Size, reloc.Type,
				reloc.TypeValue, reloc.Weak, reloc.Addend, rawReference(reloc.Target)))
		}
	}
	return notes
}

func rawAuxNotes(object *canonicalGoObject) []string {
	var notes []string
	for i := range object.Symbols {
		sym := &object.Symbols[i]
		for _, aux := range sym.Aux {
			notes = append(notes, fmt.Sprintf("symbol[%d]=%q aux[%d] type=%s(%d) target={%s}",
				sym.Index, sym.Name, aux.Index, aux.Type, aux.TypeValue, rawReference(aux.Target)))
		}
	}
	return notes
}

func rawDataNotes(object *canonicalGoObject) []string {
	var notes []string
	for i := range object.Symbols {
		sym := &object.Symbols[i]
		if len(sym.rawData) == 0 {
			continue
		}
		notes = append(notes, fmt.Sprintf("symbol[%d]=%q offset=%#x size=%d kind=%s sha256=%s",
			sym.Index, sym.Name, sym.dataOffset, len(sym.rawData), sym.Kind, sym.DataSHA256))
		if sym.Function != nil {
			notes = append(notes, rawFunctionNotes(sym)...)
		}
	}
	return notes
}

func rawFunctionNotes(sym *canonicalSymbol) []string {
	fn := sym.Function
	var notes []string
	if fn.Info != nil {
		notes = append(notes, fmt.Sprintf("function=%q args=%d locals=%d func_id=%d flags=%#x start_line=%d",
			sym.Name, fn.Info.Args, fn.Info.Locals, fn.Info.FuncID, fn.Info.FuncFlags, fn.Info.StartLine))
	}
	for _, table := range fn.PCTables {
		notes = append(notes, fmt.Sprintf("function=%q pc_table kind=%s target={%s} ranges=%d",
			sym.Name, table.Kind, rawReference(table.Symbol), len(table.Ranges)))
	}
	for _, table := range fn.PCData {
		notes = append(notes, fmt.Sprintf("function=%q pcdata[%d]=%s target={%s} ranges=%d",
			sym.Name, table.Index, textPCDataName(table.Index), rawReference(table.Symbol), len(table.Ranges)))
	}
	for _, data := range fn.FuncData {
		note := fmt.Sprintf("function=%q funcdata[%d]=%s target={%s}",
			sym.Name, data.Index, data.Kind, rawReference(data.Symbol))
		if data.StackMap != nil {
			note += " " + formatStackMap(data.StackMap)
		}
		if len(data.StackObjects) != 0 {
			note += fmt.Sprintf(" stack_objects=%d", len(data.StackObjects))
		}
		notes = append(notes, note)
	}
	return notes
}

func rawReference(ref canonicalReference) string {
	var fields []string
	fields = append(fields,
		fmt.Sprintf("pkg_index=%d", ref.PkgIndex),
		"pkg_kind="+ref.PkgKind,
		fmt.Sprintf("sym_index=%d", ref.SymIndex),
	)
	if ref.Package != "" {
		fields = append(fields, "package="+strconv.Quote(ref.Package))
	}
	if ref.Name != "" {
		fields = append(fields, "name="+strconv.Quote(ref.Name))
	}
	if ref.ABI != nil {
		fields = append(fields, fmt.Sprintf("abi=%d", *ref.ABI))
	}
	return strings.Join(fields, " ")
}

func quoteStrings(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
