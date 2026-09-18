// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"testing"

	"github.com/goallc/go-llvm"
)

// The native checker must receive optimized facts: deleted barriers/calls no
// longer count, and LLVM-created helper bodies must not hide new barriers.
func TestLLVMNoWriteBarrierInfo(t *testing.T) {
	oldModule, oldTarget := CurrentModule, typecheck.Target
	m := GlobalCtxt.NewModule("nowb")
	CurrentModule, typecheck.Target = m, new(ir.Package)
	defer func() { CurrentModule, typecheck.Target = oldModule, oldTarget; m.Dispose() }()
	pos := base.Ctxt.PosTable.XPos(src.MakePos(src.NewFileBase("nowb.go", "nowb.go"), 10, 1))
	pkg := types.NewPkg("llvm/nowb", "nowb")
	sig := llvm.FunctionType(GlobalCtxt.VoidType(), nil, false)
	declare := func(name string) (*ir.Func, llvm.Value) {
		fn := ir.NewFunc(pos, pos, pkg.Lookup(name), nil)
		fn.LSym = &obj.LSym{Name: "llvm/nowb." + name}
		fn.LSym.SetABI(obj.ABIInternal)
		typecheck.Target.Funcs = append(typecheck.Target.Funcs, fn)
		return fn, llvm.AddFunction(m, fn.LSym.Name, sig)
	}
	dead, deadIR := declare("dead")
	live, liveIR := declare("live")
	callee, calleeIR := declare("callee") // assembly declaration, no body
	wb := llvm.AddFunction(m, "runtime.wbMove", sig)
	helper := llvm.AddFunction(m, "llvm_created_helper", sig)
	b := GlobalCtxt.NewBuilder()
	defer b.Dispose()
	entry, unused, done := llvm.AddBasicBlock(deadIR, "entry"), llvm.AddBasicBlock(deadIR, "unused"), llvm.AddBasicBlock(deadIR, "done")
	b.SetInsertPointAtEnd(entry)
	b.CreateCondBr(llvm.ConstInt(GlobalCtxt.Int1Type(), 0, false), unused, done)
	b.SetInsertPointAtEnd(unused)
	b.CreateCall(sig, wb, nil, "")
	b.CreateCall(sig, calleeIR, nil, "")
	b.CreateBr(done)
	b.SetInsertPointAtEnd(done)
	b.CreateRetVoid()
	b.SetInsertPointAtEnd(llvm.AddBasicBlock(liveIR, "entry"))
	b.CreateCall(sig, helper, nil, "")
	b.CreateCall(sig, calleeIR, nil, "")
	b.CreateRetVoid()
	b.SetInsertPointAtEnd(llvm.AddBasicBlock(helper, "entry"))
	b.CreateCall(sig, wb, nil, "")
	b.CreateRetVoid()
	dead.WBPos = pos
	dead.NWBRCalls = &[]ir.SymAndPos{{Sym: callee.LSym, Pos: pos}}
	options := llvm.NewPassBuilderOptions()
	defer options.Dispose()
	if err := m.RunPasses("function(simplifycfg)", llvm.TargetMachine{}, options); err != nil {
		t.Fatal(err)
	}
	recordLLVMWriteBarrierInfo()
	if dead.WBPos.IsKnown() || dead.NWBRCalls != nil {
		t.Fatal("retained facts from an optimized-away block")
	}
	if !live.WBPos.IsKnown() {
		t.Fatal("lost write barrier in LLVM-created helper")
	}
	if live.NWBRCalls == nil || len(*live.NWBRCalls) != 1 || (*live.NWBRCalls)[0].Sym != callee.LSym {
		t.Fatal("lost surviving direct call")
	}
}
