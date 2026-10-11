// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"fmt"
	"internal/buildcfg"
	"strconv"
	"strings"
)

// GoObj keeps its existing MC metadata adapter. Native formats instead carry
// the original PC tables in an LLVM constant global (native_metadata.go).
func (e *emitter) goobjMetadata(body *strings.Builder) error {
	s := e.fn
	// MC aliases preserve exact offsets without inserting labels into symbolic
	// instructions. Every alias is bounded by the fixed-layout assertion.
	pcs := map[int64]bool{0: true, s.Size: true}
	for p := s.Func().Text; p != nil; p = p.Link {
		pcs[p.Pc] = true
		if p.Spadj != 0 && p.Link != nil {
			pcs[p.Link.Pc] = true
		}
	}
	for _, r := range s.R {
		if r.Type == objabi.R_CALLIND {
			pcs[int64(r.Off)] = true
		}
	}
	for _, pc := range sortedPCs(pcs) {
		if pc != 0 {
			fmt.Fprintf(body, ".set %s, %s+%d\n", e.pcLabel(pc), e.pcLabel(0), pc)
		}
	}
	fn := e.ref(s, true)
	fmt.Fprintf(body, ".goobj.asmfunc %s, %d, %d, %d, %d, %d\n", fn, s.Func().Args, s.Func().Locals, s.Func().FuncID, s.Func().FuncFlag, s.Func().StartLine)
	quoteFile := func(file string) string { return strings.ReplaceAll(strconv.Quote(file), "$", "$$") }
	decl := e.ctxt.InnermostPos(s.Func().Text.Pos)
	dwarfVersion := 4
	if buildcfg.Experiment.Dwarf5 {
		dwarfVersion = 5
	}
	asInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	fmt.Fprintf(body, ".goobj.asmdebug %s, %d, %d, %d, %s\n", fn, dwarfVersion, asInt(s.Wrapper()), asInt(!s.Static()), quoteFile(decl.AbsFilename()))
	// Keep debugger rows separate from runtime PCLN events. Multiple Progs at
	// one PC and prologue/statement markers follow Go's DWARF row selection.
	prologue, wrotePrologue := false, false
	for p := s.Func().Text; p != nil; p = p.Link {
		prologue = prologue || p.Pos.Xlogue() == src.PosPrologueEnd
		if p.Pos.Line() == 0 || (p.Link != nil && p.Link.Pc == p.Pc) {
			continue
		}
		pos := e.ctxt.InnermostPos(p.Pos)
		flags := asInt(p.Pos.IsStmt() != src.PosNotStmt)
		if prologue && !wrotePrologue {
			flags |= 2
			wrotePrologue = true
		}
		fmt.Fprintf(body, ".goobj.asmline %s, %s, %d, %d, %s\n", fn, e.pcLabel(p.Pc), pos.RelLine(), flags, quoteFile(pos.AbsFilename()))
	}
	sp := int64(0)
	funcdata := map[int64]bool{}
	for p := s.Func().Text; p != nil; p = p.Link {
		if p.Pc < 0 || p.Pc > s.Size {
			return fmt.Errorf("metadata PC %d is not an instruction boundary", p.Pc)
		}
		label := e.pcLabel(p.Pc)
		if p.Spadj != 0 {
			after := s.Size
			if p.Link != nil {
				after = p.Link.Pc
			}
			sp += int64(p.Spadj)
			fmt.Fprintf(body, ".goobj.asmpc %s, %s, -3, %d\n", fn, e.pcLabel(after), sp)
		}
		switch p.As {
		case obj.APCDATA:
			if p.From.Offset < 0 || p.From.Offset > 65535 {
				return fmt.Errorf("invalid PCDATA index")
			}
			fmt.Fprintf(body, ".goobj.asmpc %s, %s, %d, %d\n", fn, label, p.From.Offset, p.To.Offset)
		case obj.AFUNCDATA:
			if p.From.Offset < 0 || p.From.Offset > 255 || p.To.Offset != 0 || funcdata[p.From.Offset] {
				return fmt.Errorf("invalid or duplicate FUNCDATA")
			}
			funcdata[p.From.Offset] = true
			fmt.Fprintf(body, ".goobj.asmfuncdata %s, %d, %s\n", fn, p.From.Offset, e.ref(p.To.Sym, false))
		}
		pos := e.ctxt.PosTable.Pos(p.Pos)
		if p.As != obj.ATEXT && p.As != obj.ANOP && pos.RelLine() != 0 {
			filename := strings.ReplaceAll(strconv.Quote(pos.AbsFilename()), "$", "$$")
			fmt.Fprintf(body, ".goobj.asmpc %s, %s, -2, %d, %s\n", fn, label, pos.RelLine(), filename)
		}
	}
	for _, r := range s.R {
		if r.Type == objabi.R_CALLIND {
			// Preserve the native Go marker, including offsets within a REX prefix.
			pc := int64(r.Off)
			if pc < 0 || pc >= s.Size {
				return fmt.Errorf("indirect call PC %d is outside the function", pc)
			}
			fmt.Fprintf(body, ".goobj.asmpc %s, %s, -5, 0\n", fn, e.pcLabel(pc))
		}
	}
	return nil
}
