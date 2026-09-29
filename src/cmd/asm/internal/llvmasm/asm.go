// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package llvmasm lowers prepared Plan 9 instructions to LLVM inline assembly.
// It does not inspect machine code or invoke an external assembler.
package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"fmt"
	"strconv"
	"strings"
)

// Options selects the LLVM artifact. Object output uses GoObj and the Go linker.
type Options struct {
	GOOS, GOARCH string
	Output       string // obj, bc, or ir
}

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
	labels   map[*obj.Prog]string
	refs     []reference
	refIndex map[reference]int
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

func (e *emitter) branch(a obj.Addr) (string, error) {
	if a.Sym != nil {
		return withOffset(e.ref(a.Sym, true), a.Offset), nil
	}
	if p := a.Target(); p != nil {
		if label, ok := e.labels[p]; ok {
			return label, nil
		}
	}
	return "", fmt.Errorf("branch has no local instruction or symbol target")
}

func withOffset(s string, off int64) string {
	if off == 0 {
		return s
	}
	return fmt.Sprintf("%s%+d", s, off)
}

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
	var result []function
	for _, s := range ctxt.Text {
		e := emitter{ctxt: ctxt, options: options, fn: s, labels: make(map[*obj.Prog]string), refIndex: make(map[reference]int)}
		var body strings.Builder
		// LLVM substitutes a different uid for each inline-asm expansion, including
		// independently compiled ThinLTO partitions.
		prefix := ".Lgoasm_"
		start := prefix + "${:uid}_start"
		end := prefix + "${:uid}_end"
		fmt.Fprintf(&body, "%s:\n", start)
		n := 0
		for p := s.Func().Text; p != nil; p = p.Link {
			e.labels[p] = fmt.Sprintf("%s${:uid}_%d", prefix, n)
			n++
		}
		var records strings.Builder
		fn := e.ref(s, true)
		fmt.Fprintf(&records, ".goobj.asmfunc %s, %d, %d, %d, %d, %d\n", fn, s.Func().Args, s.Func().Locals, s.Func().FuncID, s.Func().FuncFlag, s.Func().StartLine)
		sp := int64(0)
		funcdata := make(map[int64]bool)
		for p := s.Func().Text; p != nil; p = p.Link {
			label := e.labels[p]
			fmt.Fprintf(&body, "%s:\n", label)
			var instruction string
			var err error
			switch p.As {
			case obj.ATEXT, obj.AFUNCDATA, obj.APCDATA, obj.ANOP:
			case obj.APCALIGN:
				if p.From.Offset <= 0 || p.From.Offset&(p.From.Offset-1) != 0 {
					err = fmt.Errorf("invalid PCALIGN %d", p.From.Offset)
				} else {
					instruction = fmt.Sprintf(".balign %d", p.From.Offset)
				}
			default:
				if options.GOARCH == "amd64" {
					instruction, err = e.x86(p)
				} else {
					instruction, err = e.arm64(p)
				}
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %v", ctxt.PosTable.Pos(p.Pos), p, err)
			}
			if instruction != "" {
				body.WriteString(instruction)
				body.WriteByte('\n')
			}
			// MC resolves these labels after encoding and branch relaxation.
			// Spadj applies after the instruction; PCDATA applies at its PC.
			after := fmt.Sprintf("%s${:uid}_after_%d", prefix, n)
			n++
			fmt.Fprintf(&body, "%s:\n", after)
			if (p.As.String() == "CALL" || p.As.String() == "BL") && p.To.Sym == nil && (p.To.Type == obj.TYPE_REG || p.To.Type == obj.TYPE_MEM) {
				fmt.Fprintf(&records, ".goobj.asmpc %s, %s, -5, 0\n", fn, label)
			}
			if p.Spadj != 0 {
				sp += int64(p.Spadj)
				fmt.Fprintf(&records, ".goobj.asmpc %s, %s, -3, %d\n", fn, after, sp)
			}
			switch p.As {
			case obj.APCDATA:
				if p.From.Offset < 0 || p.From.Offset > 65535 {
					return nil, fmt.Errorf("invalid PCDATA index %d", p.From.Offset)
				}
				fmt.Fprintf(&records, ".goobj.asmpc %s, %s, %d, %d\n", fn, label, p.From.Offset, p.To.Offset)
			case obj.AFUNCDATA:
				if p.From.Offset < 0 || p.From.Offset > 255 || p.To.Offset != 0 || funcdata[p.From.Offset] {
					return nil, fmt.Errorf("invalid or duplicate FUNCDATA in %s: %s", s.Name, p)
				}
				funcdata[p.From.Offset] = true
				fmt.Fprintf(&records, ".goobj.asmfuncdata %s, %d, %s\n", fn, p.From.Offset, e.ref(p.To.Sym, false))
			}
			pos := ctxt.PosTable.Pos(p.Pos)
			if p.As != obj.ATEXT && p.As != obj.ANOP && pos.RelLine() != 0 {
				filename := strings.ReplaceAll(strconv.Quote(pos.AbsFilename()), "$", "$$")
				fmt.Fprintf(&records, ".goobj.asmpc %s, %s, -2, %d, %s\n", fn, label, pos.RelLine(), filename)
			}
		}
		fmt.Fprintf(&body, "%s:\n", end)
		body.WriteString(records.String())
		result = append(result, function{s, body.String(), e.refs})
	}
	return result, nil
}
