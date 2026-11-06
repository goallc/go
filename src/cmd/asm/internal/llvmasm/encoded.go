// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"fmt"
	"sort"
	"strconv"
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
	fixedPC  map[int64]bool
}

func (e *emitter) ref(s *obj.LSym, text bool) string {
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
	return fmt.Sprintf("${%d:c}", i)
}

func withOffset(s string, off int64) string {
	if off == 0 {
		return s
	}
	return fmt.Sprintf("%s%+d", s, off)
}

// label names refer to PCs in Go's encoded function, but resolve to the new
// positions after LLVM encoding and relaxation.
func pcLabel(pc int64) string { return fmt.Sprintf(".Lgoasm_${:uid}_pc_%d", pc) }

func lower(ctxt *obj.Link, options Options) ([]function, error) {
	if options.GOARCH != "amd64" && options.GOARCH != "arm64" {
		return nil, fmt.Errorf("LLVM assembly is not supported for %s", options.GOARCH)
	}
	if options.GOOS != "linux" && options.GOOS != "darwin" {
		return nil, fmt.Errorf("LLVM assembly is not supported for %s", options.GOOS)
	}
	if ctxt.Flag_dynlink || ctxt.Flag_linkshared {
		return nil, fmt.Errorf("LLVM assembly does not yet support Go shared-library symbol binding")
	}
	arch := "x86_64"
	if options.GOARCH == "arm64" {
		arch = "aarch64"
	}
	// Use GoObj's ELF-like instruction syntax on both host operating systems.
	decoder, err := llvm.NewMCDecoder(arch+"-unknown-"+options.GOOS+"-goobj", assemblyFeatures(options.GOARCH))
	if err != nil {
		return nil, err
	}
	defer decoder.Dispose()
	var result []function
	for _, s := range ctxt.Text {
		if s.P == nil {
			ctxt.Arch.Assemble(ctxt, s, ctxt.NewProg)
		}
		if ctxt.Errors != 0 {
			return nil, fmt.Errorf("Go instruction encoding failed for %s", s.Name)
		}
	}
	// Collect interior-address constraints before lowering any function. Such
	// references remain symbol+offset, guarded by a final MC layout assertion.
	fixed := map[*obj.LSym]map[int64]bool{}
	for _, s := range ctxt.Text {
		fixed[s] = map[int64]bool{}
	}
	for _, s := range append(append([]*obj.LSym(nil), ctxt.Text...), ctxt.Data...) {
		for _, r := range s.R {
			offsets, local := fixed[r.Sym]
			if r.Type == objabi.R_CALLIND {
				continue
			}
			addend := r.Add
			if options.GOARCH == "amd64" && r.Type == objabi.R_PCREL && (s.Type == objabi.STEXT || s.Type == objabi.STEXTFIPS) {
				for p := s.Func().Text; p != nil; p = p.Link {
					if p.Pc <= int64(r.Off) && int64(r.Off) < p.Pc+int64(p.Isize) {
						addend += p.Pc + int64(p.Isize) - int64(r.Off) - int64(r.Siz)
						break
					}
				}
			}
			if addend == 0 {
				continue
			}
			if local {
				offsets[addend] = true
			} else if r.Sym != nil && (r.Sym.ABI() == obj.ABIInternal || r.Sym.Type == objabi.STEXT || r.Type == objabi.R_CALL || r.Type == objabi.R_CALLARM64) {
				return nil, fmt.Errorf("cannot validate interior address of external function %s", r.Sym.Name)
			}
		}
	}
	for _, s := range ctxt.Text {
		e := emitter{ctxt: ctxt, options: options, fn: s, refIndex: make(map[reference]int), fixedPC: fixed[s]}
		f, err := e.encodedFunction(decoder)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name, err)
		}
		result = append(result, f)
	}
	return result, nil
}

type region struct {
	end   int64
	data  bool
	align *obj.Prog
}

func (e *emitter) encodedFunction(decoder llvm.MCDecoder) (function, error) {
	s := e.fn
	regions := map[int64]region{}
	// Encoder-generated literal pools are WORD/DWORD Progs too. Every byte of
	// such a region is data, even if LLVM could decode it as an instruction.
	endCode := int64(0)
	for p := s.Func().Text; p != nil; p = p.Link {
		end := s.Size
		if p.Link != nil {
			end = p.Link.Pc
		}
		if end < p.Pc || p.Pc < 0 || end > s.Size {
			return function{}, fmt.Errorf("invalid encoded Prog boundary")
		}
		if p.Isize != 0 {
			end = p.Pc + int64(p.Isize)
		}
		name := p.As.String()
		switch name {
		case "BYTE", "WORD", "LONG", "QUAD", "DWORD", "LOCK", "REP", "REPN":
			regions[p.Pc] = region{end: end, data: true}
		}
		if p.As == obj.APCALIGN || p.As == obj.APCALIGNMAX {
			regions[p.Pc] = region{end: p.Pc + int64(obj.AlignmentPaddingLength(int32(p.Pc), p, e.ctxt)), align: p}
		}
		// AArch64's trailing zero padding is not a decoded instruction stream.
		if p.As != obj.ATEXT && p.As != obj.APCDATA && p.As != obj.AFUNCDATA && p.As != obj.ANOP {
			endCode = max(endCode, end)
		}
	}
	if endCode < s.Size {
		regions[endCode] = region{end: s.Size, data: true}
	}
	relocs, exprs, err := e.instructionRelocs()
	if err != nil {
		return function{}, err
	}
	var body strings.Builder
	boundaries := map[int64]bool{}
	instructionStarts := map[int64]int64{}
	targets := map[int64]bool{}

	for pc := int64(0); pc < s.Size; {
		boundaries[pc] = true
		fmt.Fprintf(&body, "%s:\n", pcLabel(pc))
		if r, ok := regions[pc]; ok {
			if r.align != nil {
				if r.align.As == obj.APCALIGN {
					fmt.Fprintf(&body, ".balign %d\n", r.align.From.Offset)
				} else if r.align.To.Offset > 0 {
					fmt.Fprintf(&body, ".balign %d, , %d\n", r.align.From.Offset, r.align.To.Offset)
				}
				delete(regions, pc)
				if r.end > pc {
					pc = r.end
					continue
				}
			} else if r.data {
				for pc < r.end {
					if rel, ok := relocs[pc]; ok {
						if rel.Size != 4 && rel.Size != 8 {
							return function{}, fmt.Errorf("unsupported data relocation width at PC %d", pc)
						}
						if pc+int64(rel.Size) > r.end {
							return function{}, fmt.Errorf("data relocation crosses region at PC %d", pc)
						}
						directive := ".long"
						if rel.Size == 8 {
							directive = ".quad"
						}
						fmt.Fprintf(&body, "%s %s\n", directive, withOffset(exprs[rel.Symbol], rel.Addend))
						delete(relocs, pc)
						pc += int64(rel.Size)
					} else {
						fmt.Fprintf(&body, ".byte %d\n", s.P[pc])
						pc++
					}
					if pc < r.end {
						boundaries[pc] = true
						fmt.Fprintf(&body, "%s:\n", pcLabel(pc))
					}
				}
				continue
			}
		}
		// Decode once to get the instruction extent; decode again with only its
		// relocations. No numeric text from the first pass is consumed.
		size := decoder.InstructionSize(s.P[pc:], uint64(pc))
		if size == 0 {
			return function{}, fmt.Errorf("LLVM could not decode instruction at PC %d", pc)
		}
		var rs []llvm.MCReloc
		for off := pc; off < pc+int64(size); off++ {
			instructionStarts[off] = pc
			if r, ok := relocs[off]; ok {
				rs = append(rs, r)
			}
		}
		text, n, target, err := decoder.Decode(s.P[pc:pc+int64(size)], uint64(pc), rs)
		if err != nil {
			return function{}, err
		}
		if n != size {
			return function{}, fmt.Errorf("inconsistent decoded size")
		}
		for _, r := range rs {
			delete(relocs, int64(r.Offset))
		}
		if target >= 0 {
			targets[target] = true
		}
		text = strings.ReplaceAll(text, "$", "$$")
		text = strings.ReplaceAll(text, ".Lgoasm_decode_", ".Lgoasm_${:uid}_pc_")
		for _, r := range rs {
			text = strings.ReplaceAll(text, r.Symbol, exprs[r.Symbol])
		}
		body.WriteString(text)
		body.WriteByte('\n')
		pc += int64(size)
	}
	boundaries[s.Size] = true
	fmt.Fprintf(&body, "%s:\n", pcLabel(s.Size))
	for target := range targets {
		if !boundaries[target] {
			return function{}, fmt.Errorf("PC-relative target %d is not an instruction/data boundary", target)
		}
	}
	if len(relocs) != 0 {
		return function{}, fmt.Errorf("unconsumed instruction relocations")
	}
	fn := e.ref(s, true)
	fmt.Fprintf(&body, ".goobj.asmfunc %s, %d, %d, %d, %d, %d\n", fn, s.Func().Args, s.Func().Locals, s.Func().FuncID, s.Func().FuncFlag, s.Func().StartLine)
	var fixedPCs []int64
	for pc := range e.fixedPC {
		fixedPCs = append(fixedPCs, pc)
	}
	sort.Slice(fixedPCs, func(i, j int) bool { return fixedPCs[i] < fixedPCs[j] })
	for _, pc := range fixedPCs {
		if !boundaries[pc] {
			return function{}, fmt.Errorf("interior address %d is not an instruction/data boundary", pc)
		}
		fmt.Fprintf(&body, ".goobj.asmpc %s, %s, -4, %d\n", fn, pcLabel(pc), pc)
	}
	sp := int64(0)
	funcdata := map[int64]bool{}
	for p := s.Func().Text; p != nil; p = p.Link {
		if !boundaries[p.Pc] {
			return function{}, fmt.Errorf("metadata PC %d is not an instruction boundary", p.Pc)
		}
		label := pcLabel(p.Pc)
		if p.Spadj != 0 {
			after := s.Size
			if p.Link != nil {
				after = p.Link.Pc
			}
			sp += int64(p.Spadj)
			fmt.Fprintf(&body, ".goobj.asmpc %s, %s, -3, %d\n", fn, pcLabel(after), sp)
		}
		switch p.As {
		case obj.APCDATA:
			if p.From.Offset < 0 || p.From.Offset > 65535 {
				return function{}, fmt.Errorf("invalid PCDATA index")
			}
			fmt.Fprintf(&body, ".goobj.asmpc %s, %s, %d, %d\n", fn, label, p.From.Offset, p.To.Offset)
		case obj.AFUNCDATA:
			if p.From.Offset < 0 || p.From.Offset > 255 || p.To.Offset != 0 || funcdata[p.From.Offset] {
				return function{}, fmt.Errorf("invalid or duplicate FUNCDATA")
			}
			funcdata[p.From.Offset] = true
			fmt.Fprintf(&body, ".goobj.asmfuncdata %s, %d, %s\n", fn, p.From.Offset, e.ref(p.To.Sym, false))
		}
		pos := e.ctxt.PosTable.Pos(p.Pos)
		if p.As != obj.ATEXT && p.As != obj.ANOP && pos.RelLine() != 0 {
			filename := strings.ReplaceAll(strconv.Quote(pos.AbsFilename()), "$", "$$")
			fmt.Fprintf(&body, ".goobj.asmpc %s, %s, -2, %d, %s\n", fn, label, pos.RelLine(), filename)
		}
	}
	for _, r := range s.R {
		if r.Type == objabi.R_CALLIND {
			// x86's native REX insertion adjusts even the zero-width CALLIND
			// marker into the instruction. Attach it to the decoded call's start.
			pc, ok := instructionStarts[int64(r.Off)]
			if !ok {
				return function{}, fmt.Errorf("indirect call PC %d is outside decoded instructions", r.Off)
			}
			fmt.Fprintf(&body, ".goobj.asmpc %s, %s, -5, 0\n", fn, pcLabel(pc))
		}
	}
	return function{s, body.String(), e.refs}, nil
}

// Only relocation families are translated here. Instruction semantics, opcode
// selection and register/operand syntax belong to Go's encoder and LLVM MC.
func (e *emitter) instructionRelocs() (map[int64]llvm.MCReloc, map[string]string, error) {
	rs := map[int64]llvm.MCReloc{}
	exprs := map[string]string{}
	add := func(off int64, size uint32, sym *obj.LSym, text bool, addend int64, modifier string, pcrel bool) error {
		if _, ok := rs[off]; ok {
			return fmt.Errorf("overlapping relocation at PC %d", off)
		}
		if sym == nil {
			return fmt.Errorf("missing relocation symbol at PC %d", off)
		}

		name := fmt.Sprintf("__goasm_operand_%d__", len(exprs))
		expr := e.ref(sym, text)
		if strings.HasPrefix(modifier, "@") {
			expr += modifier
		} else {
			expr = modifier + expr
		}
		exprs[name] = expr
		rs[off] = llvm.MCReloc{Offset: uint64(off), Size: size, Symbol: name, Addend: addend, PCRelative: pcrel}
		return nil
	}
	for _, r := range e.fn.R {
		off := int64(r.Off)
		sym := r.Sym
		var err error
		switch r.Type {
		case objabi.R_CALLIND:
			continue
		case objabi.R_ADDR:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", false)
		case objabi.R_CALL:
			err = add(off, uint32(r.Siz), sym, true, r.Add, "", true)
		case objabi.R_PCREL:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", true)
		case objabi.R_GOTPCREL:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "@GOTPCREL", false)
		case objabi.R_TLS_LE:
			if sym == nil {
				sym = e.ctxt.Lookup("runtime.tlsg")
			}
			err = add(off, uint32(r.Siz), sym, false, r.Add, "@TPOFF", false)
		case objabi.R_CALLARM64:
			err = add(off, 4, sym, true, r.Add, "", false)
		case objabi.R_ADDRARM64, objabi.R_ARM64_PCREL, objabi.R_ARM64_PCREL_LDST8, objabi.R_ARM64_PCREL_LDST16, objabi.R_ARM64_PCREL_LDST32, objabi.R_ARM64_PCREL_LDST64:
			err = add(off, 4, sym, false, r.Add, "", false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, ":lo12:", false)
			}
		case objabi.R_ARM64_TLS_LE:
			err = add(off, 4, sym, false, r.Add, ":tprel_g0:", false)
		case objabi.R_ARM64_TLS_IE:
			err = add(off, 4, sym, false, r.Add, ":gottprel:", false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, ":gottprel_lo12:", false)
			}
		case objabi.R_ARM64_GOTPCREL, objabi.R_ARM64_GOT:
			err = add(off, 4, sym, false, r.Add, ":got:", false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, ":got_lo12:", false)
			}
		default:
			return nil, nil, fmt.Errorf("unsupported encoded relocation %s at PC %d", r.Type, r.Off)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return rs, exprs, nil
}

// These features permit explicitly written optional instructions, just as the
// Go assembler does. They do not add CPU dispatch or change the compiled Go
// baseline; callers remain responsible for guarding optional ISA routines.
func assemblyFeatures(arch string) string {
	if arch == "arm64" {
		return "+fp-armv8,+neon,+aes,+sha2,+sha3,+sm4,+lse,+crc,+dotprod,+fullfp16,+fp16fml,+rcpc,+rand,+sb,+ssbs,+pauth,+bti"
	}
	return ""
}
