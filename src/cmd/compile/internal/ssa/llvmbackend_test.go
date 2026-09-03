// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/internal/llvmbackend"
	"internal/buildcfg"
	"slices"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMCodeGenOptions(t *testing.T) {
	want := []string{
		"-trap-unreachable",
		"-disable-machine-cse",
		"-force-loop-cold-block",
		"-disable-lsr",
	}
	if buildcfg.GOARCH == "arm64" {
		want = append(want, "-aarch64-goobj-composite-relocations")
	}
	if got := llvmCodeGenOptions(); !slices.Equal(got, want) {
		t.Errorf("llvmCodeGenOptions() = %q, want %q", got, want)
	}
}

func TestLLVMEmissionReportsLateStatepointDiagnostic(t *testing.T) {
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllAsmPrinters()

	context := llvm.NewContext()
	defer context.Dispose()
	module := context.NewModule("late_statepoint_diagnostic")
	defer module.Dispose()
	builder := context.NewBuilder()
	defer builder.Dispose()

	triple := llvm.DefaultTargetTriple()
	module.SetTarget(triple)
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		t.Fatalf("resolve LLVM target %q: %v", triple, err)
	}
	tm := target.CreateTargetMachine(triple, "", "", llvm.CodeGenLevelDefault, llvm.RelocDefault, llvm.CodeModelDefault)
	defer tm.Dispose()
	td := tm.CreateTargetData()
	module.SetDataLayout(td.String())
	td.Dispose()

	voidFunctionType := llvm.FunctionType(context.VoidType(), nil, false)
	safepoint := llvm.AddFunction(module, "safepoint", voidFunctionType)
	safepoint.SetFunctionCallConv(goABIInternalCallConv)
	function := llvm.AddFunction(module, "late_failure", voidFunctionType)
	function.SetFunctionCallConv(goABIInternalCallConv)
	function.SetGC("goallc")
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
	builder.CreateAlloca(llvm.VectorType(context.PointerType(0), 2), "unsupported")
	call := builder.CreateCall(voidFunctionType, safepoint, nil, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	builder.CreateRetVoid()

	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tm.RunPassPluginPreCodeGen(module, llvm.ObjectFile, plugin); err != nil {
		t.Fatalf("prepare GoALLC pre-codegen plugin: %v", err)
	}
	buffer, err := tm.EmitToMemoryBuffer(module, llvm.ObjectFile)
	if err == nil {
		buffer.Dispose()
		t.Fatal("LLVM target emission succeeded after a late statepoint diagnostic")
	}
	if !strings.Contains(err.Error(), "LLVM target code generation reported an error") {
		t.Fatalf("LLVM target emission error = %q", err)
	}
}
