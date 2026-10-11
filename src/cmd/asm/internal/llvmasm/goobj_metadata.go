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
	"math"

	"github.com/goallc/go-llvm"
)

const (
	goObjAssemblyVersion = 1
	goObjPCSP            = -3
	goObjPCLine          = -2
	goObjLineIsStmt      = 1
	goObjLinePrologueEnd = 2
)

// emitGoObjMetadata attaches the Go-owned frame and source information to the
// LLVM function. PCs are offsets in its fixed layout, not assembler text.
// FUNCDATA targets are LLVM references; the carrier also retains them as inputs
// so optimization cannot discard a definition used only by this metadata.
func emitGoObjMetadata(c llvm.Context, ctxt *obj.Link, fn llvm.Value, s *obj.LSym, value func(reference) llvm.Value) error {
	i32 := func(v int64) llvm.Metadata {
		return llvm.ConstInt(c.Int32Type(), uint64(uint32(v)), false).ConstantAsMetadata()
	}
	i64 := func(v int64) llvm.Metadata {
		return llvm.ConstInt(c.Int64Type(), uint64(v), false).ConstantAsMetadata()
	}
	boolean := func(v bool) llvm.Metadata {
		if v {
			return i32(1)
		}
		return i32(0)
	}
	node := func(v ...llvm.Metadata) llvm.Metadata { return c.MDNode(v) }
	var events, lines, data, calls []llvm.Metadata
	event := func(pc, kind, v int64, file string) {
		events = append(events, node(i64(pc), i32(kind), i32(v), c.MDString(file)))
	}
	sp := int64(0)
	prologue, wrotePrologue := false, false
	funcdata := map[int64]bool{}
	for p := s.Func().Text; p != nil; p = p.Link {
		if p.Pc < 0 || p.Pc > s.Size {
			return fmt.Errorf("metadata PC %d is outside the function", p.Pc)
		}
		if p.Spadj != 0 {
			after := s.Size
			if p.Link != nil {
				after = p.Link.Pc
			}
			sp += int64(p.Spadj)
			if sp < math.MinInt32 || sp > math.MaxInt32 {
				return fmt.Errorf("SP metadata value exceeds int32")
			}
			event(after, goObjPCSP, sp, "")
		}
		switch p.As {
		case obj.APCDATA:
			if p.From.Offset < 0 || p.From.Offset > 65535 {
				return fmt.Errorf("invalid PCDATA index")
			}
			if p.To.Offset < math.MinInt32 || p.To.Offset > math.MaxInt32 {
				return fmt.Errorf("PCDATA value exceeds int32")
			}
			event(p.Pc, p.From.Offset, p.To.Offset, "")
		case obj.AFUNCDATA:
			if p.From.Offset < 0 || p.From.Offset > 255 || p.To.Offset != 0 || p.To.Sym == nil || funcdata[p.From.Offset] {
				return fmt.Errorf("invalid or duplicate FUNCDATA")
			}
			funcdata[p.From.Offset] = true
			data = append(data, node(i32(p.From.Offset), value(reference{p.To.Sym, false}).ConstantAsMetadata()))
		}
		pos := ctxt.PosTable.Pos(p.Pos)
		if p.As != obj.ATEXT && p.As != obj.ANOP && pos.RelLine() != 0 {
			event(p.Pc, goObjPCLine, int64(pos.RelLine()), pos.AbsFilename())
		}
		// DWARF row selection differs from runtime PCLN and retains statement
		// and prologue markers even when several Progs share one PC.
		prologue = prologue || p.Pos.Xlogue() == src.PosPrologueEnd
		if p.Pos.Line() == 0 || (p.Link != nil && p.Link.Pc == p.Pc) {
			continue
		}
		pos = ctxt.InnermostPos(p.Pos)
		flags := int64(0)
		if p.Pos.IsStmt() != src.PosNotStmt {
			flags |= goObjLineIsStmt
		}
		if prologue && !wrotePrologue {
			flags |= goObjLinePrologueEnd
			wrotePrologue = true
		}
		lines = append(lines, node(i64(p.Pc), i32(int64(pos.RelLine())), i32(flags), c.MDString(pos.AbsFilename())))
	}
	for _, r := range s.R {
		if r.Type == objabi.R_CALLIND {
			pc := int64(r.Off)
			if pc < 0 || pc >= s.Size {
				return fmt.Errorf("indirect call PC %d is outside the function", pc)
			}
			calls = append(calls, i64(pc))
		}
	}
	version := int64(4)
	if buildcfg.Experiment.Dwarf5 {
		version = 5
	}
	decl := ctxt.InnermostPos(s.Func().Text.Pos)
	info := s.Func()
	frame := node(i32(int64(info.Args)), i32(int64(info.Locals)), i32(int64(info.FuncID)), i32(int64(info.FuncFlag)), i32(int64(info.StartLine)))
	debug := node(i32(version), boolean(s.Wrapper()), boolean(!s.Static()), c.MDString(decl.AbsFilename()), c.MDNode(lines))
	fn.SetGlobalMetadata(c.MDKindID("goobj.asm"), node(i32(goObjAssemblyVersion), i64(s.Size), frame, c.MDNode(events), c.MDNode(data), debug, c.MDNode(calls)))
	return nil
}
