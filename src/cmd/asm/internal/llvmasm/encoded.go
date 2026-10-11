// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/goallc/go-llvm"
)

type reference struct {
	sym  *obj.LSym
	text bool
}

type function struct {
	sym  *obj.LSym
	body string
	refs []reference
}

type emitter struct {
	ctxt     *obj.Link
	options  Options
	fn       *obj.LSym
	refs     []reference
	refIndex map[reference]int
}

func (e *emitter) operand(s *obj.LSym, text bool) uint32 {
	if s == nil {
		panic("nil assembly symbol")
	}
	text = text || s.Type == objabi.STEXT || s.Type == objabi.STEXTFIPS || s.ABI() == obj.ABIInternal
	r := reference{s, text}
	i, ok := e.refIndex[r]
	if !ok {
		i = len(e.refs)
		e.refs = append(e.refs, r)
		e.refIndex[r] = i
	}
	return uint32(i)
}

func (e *emitter) ref(s *obj.LSym, text bool) string {
	return fmt.Sprintf("${%d:c}", e.operand(s, text))
}

func withOffset(s string, off int64) string {
	if off == 0 {
		return s
	}
	return fmt.Sprintf("%s%+d", s, off)
}

func (e *emitter) pcLabel(pc int64) string {
	prefix := ".L"
	if e.options.ObjectFormat == "macho" {
		prefix = "L"
	}
	return fmt.Sprintf("%sgoasm_${:uid}_pc_%d", prefix, pc)
}

func sortedPCs(pcs map[int64]bool) []int64 {
	result := make([]int64, 0, len(pcs))
	for pc := range pcs {
		result = append(result, pc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func lower(ctxt *obj.Link, options Options) ([]function, error) {
	triple, err := options.triple()
	if err != nil {
		return nil, err
	}
	if !options.isGoObj() && (ctxt.Flag_dynlink || ctxt.Flag_linkshared) {
		return nil, fmt.Errorf("LLVM assembly does not yet support Go shared-library symbol binding")
	}
	// Only relocation-bearing instructions are decoded. Ordinary instructions,
	// local branches, alignment padding and literal pools keep Go's layout.
	decoder, err := llvm.NewMCDecoder(triple, assemblyFeatures(options.GOARCH))
	if err != nil {
		return nil, err
	}
	defer decoder.Dispose()
	var result []function
	for _, s := range ctxt.Text {
		if s.P == nil {
			obj.AssembleText(ctxt, s, ctxt.NewProg)
		}
		if ctxt.Errors != 0 {
			return nil, fmt.Errorf("Go instruction encoding failed for %s", s.Name)
		}
		if ctxt.Debugasm > 1 {
			// Repeated -S also dumps Go's DWARF auxiliaries. Materialize those
			// diagnostic symbols with the ordinary writer's implementation.
			ctxt.PopulateDWARF(nil, s)
		}
	}
	for _, s := range ctxt.Text {
		e := emitter{ctxt: ctxt, options: options, fn: s, refIndex: make(map[reference]int)}
		f, err := e.encodedFunction(decoder)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name, err)
		}
		result = append(result, f)
	}
	return result, nil
}

// A replacement covers exactly one encoded instruction or address data field.
// Its size is checked by MC before any output object can be accepted.
type replacement struct {
	begin, end int64
	text       string
}

func isData(p *obj.Prog) bool {
	switch p.As.String() {
	case "BYTE", "WORD", "LONG", "QUAD", "DWORD", "LOCK", "REP", "REPN":
		return true
	}
	return p.As == obj.APCALIGN || p.As == obj.APCALIGNMAX
}

func (e *emitter) encodedFunction(decoder llvm.MCDecoder) (function, error) {
	s := e.fn
	if s.Size != int64(len(s.P)) {
		return function{}, fmt.Errorf("encoded size %d differs from byte count %d", s.Size, len(s.P))
	}
	// Go PCALIGN padding assumes that the beginning of the function satisfies
	// the requested alignment too. Preserve that assumption in LLVM's layout.
	code := make(map[int64]bool)
	for p := s.Func().Text; p != nil; p = p.Link {
		if p.Pc < 0 || p.Pc > s.Size || p.Pc+int64(p.Isize) > s.Size {
			return function{}, fmt.Errorf("invalid encoded Prog boundary at PC %d", p.Pc)
		}
		if !e.options.isGoObj() && p.As.String() == "ADRP" {
			// Unlike ADR and ordinary local branches, page-relative distances
			// can change when the whole function moves, even at fixed size.
			symbolic := false
			for _, r := range s.R {
				if int64(r.Off) == p.Pc && r.Sym != nil && r.Siz != 0 {
					symbolic = true
					break
				}
			}
			if !symbolic {
				return function{}, fmt.Errorf("ADRP without a symbolic relocation is not position independent")
			}
		}
		if p.As == obj.APCALIGN || p.As == obj.APCALIGNMAX {
			if p.From.Offset > int64(s.Align) {
				s.Align = int16(p.From.Offset)
			}
		}
		if !isData(p) {
			for pc := p.Pc; pc < p.Pc+int64(p.Isize); pc += 4 {
				code[pc] = true
			}
		}
	}
	relocs, err := e.instructionRelocs()
	if err != nil {
		return function{}, err
	}
	replacements, err := e.fixedNativeRelocs()
	if err != nil {
		return function{}, err
	}
	for _, off := range sortedRelocOffsets(relocs) {
		r, ok := relocs[off]
		if !ok { // Another relocation in the same instruction was consumed.
			continue
		}
		var prog *obj.Prog
		for p := s.Func().Text; p != nil; p = p.Link {
			if p.Pc <= off && off < p.Pc+int64(p.Isize) {
				prog = p
				break
			}
		}
		if prog == nil {
			return function{}, fmt.Errorf("relocation at PC %d has no encoded Prog", off)
		}
		if isData(prog) {
			if r.PCRelative || r.Prefix != "" || r.Suffix != "" || (r.Size != 4 && r.Size != 8) {
				return function{}, fmt.Errorf("unsupported data relocation at PC %d", off)
			}
			if off+int64(r.Size) > prog.Pc+int64(prog.Isize) {
				return function{}, fmt.Errorf("data relocation crosses Prog at PC %d", off)
			}
			directive := ".long"
			if r.Size == 8 {
				directive = ".quad"
			}
			replacements = append(replacements, replacement{off, off + int64(r.Size), directive + " " + withOffset(fmt.Sprintf("${%d:c}", r.Operand), r.Addend)})
			delete(relocs, off)
			continue
		}
		// A single Go Prog can expand into several machine instructions. Decode
		// boundaries only inside this relocation-bearing Prog; all other Progs
		// are copied even if LLVM has no decoder for their ISA extensions.
		pc := prog.Pc
		limit := pc + int64(prog.Isize)
		for pc <= off {
			size := int64(4)
			if e.options.GOARCH == "amd64" {
				size = int64(decoder.InstructionSize(s.P[pc:limit], uint64(pc)))
			}
			if size == 0 || pc+size > limit {
				return function{}, fmt.Errorf("cannot locate symbolic instruction at PC %d", off)
			}
			if off >= pc+size {
				pc += size
				continue
			}
			var rs []llvm.MCReloc
			for at := pc; at < pc+size; at++ {
				if rel, ok := relocs[at]; ok {
					if at+int64(rel.Size) > pc+size {
						return function{}, fmt.Errorf("relocation crosses instruction at PC %d", at)
					}
					rs = append(rs, rel)
				}
			}
			text, n, err := decoder.Decode(s.P[pc:pc+size], uint64(pc), rs)
			if err != nil {
				return function{}, err
			}
			if int64(n) != size {
				return function{}, fmt.Errorf("inconsistent symbolic instruction size at PC %d", pc)
			}
			for _, rel := range rs {
				delete(relocs, int64(rel.Offset))
			}
			replacements = append(replacements, replacement{pc, pc + size, text})
			break
		}
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].begin < replacements[j].begin })
	var body strings.Builder
	fmt.Fprintf(&body, "%s:\n", e.pcLabel(0))
	// No LLVM-generated prefix (BTI, frame setup, etc.) may shift Go's PCs.
	fmt.Fprintf(&body, ".if %s-%s != 0\n.error \"Go assembly entry offset changed\"\n.endif\n", e.pcLabel(0), e.ref(s, true))
	writeBytes := func(begin, end int64) {
		for pc := begin; pc < end; {
			if e.options.GOARCH == "arm64" && pc+4 <= end && code[pc] {
				fmt.Fprintf(&body, ".inst 0x%08x\n", binary.LittleEndian.Uint32(s.P[pc:pc+4]))
				pc += 4
				continue
			}
			body.WriteString(".byte ")
			for count := 0; pc < end && count < 16; count++ {
				if count != 0 {
					body.WriteByte(',')
				}
				fmt.Fprintf(&body, "%d", s.P[pc])
				pc++
				if e.options.GOARCH == "arm64" && code[pc] {
					break
				}
			}
			body.WriteByte('\n')
		}
	}
	pc := int64(0)
	for _, r := range replacements {
		if r.begin < pc {
			return function{}, fmt.Errorf("overlapping symbolic instructions at PC %d", r.begin)
		}
		writeBytes(pc, r.begin)
		body.WriteString(r.text)
		body.WriteByte('\n')
		fmt.Fprintf(&body, ".if .-%s != %d\n.error \"Go assembly instruction size changed\"\n.endif\n", e.pcLabel(0), r.end)
		pc = r.end
	}
	writeBytes(pc, s.Size)
	fmt.Fprintf(&body, ".if .-%s != %d\n.error \"Go assembly function size changed\"\n.endif\n", e.pcLabel(0), s.Size)
	if e.options.isGoObj() {
		if err := e.goobjMetadata(&body); err != nil {
			return function{}, err
		}
	}
	return function{s, body.String(), e.refs}, nil
}

func sortedRelocOffsets(relocs map[int64]llvm.MCReloc) []int64 {
	pcs := make(map[int64]bool, len(relocs))
	for pc := range relocs {
		pcs[pc] = true
	}
	return sortedPCs(pcs)
}

// These features are needed only by relocation-bearing instructions. Raw ISA
// instructions never acquire an LLVM feature requirement through decoding.
func assemblyFeatures(arch string) string {
	if arch == "arm64" {
		return "+fp-armv8,+neon,+aes,+sha2,+sha3,+sm4,+lse,+crc,+dotprod,+fullfp16,+fp16fml,+rcpc,+rand,+sb,+ssbs,+pauth,+bti"
	}
	return ""
}
