// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/llvmbackend"
	"internal/buildcfg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMEmissionRejectsInvalidIRBeforeOptimization(t *testing.T) {
	module := GlobalCtxt.NewModule("invalid_before_optimization")
	defer module.Dispose()
	module.SetTarget(goObjTargetTriple())
	oldModule, oldFinalized := CurrentModule, llvmModuleFinalized
	oldShared, oldKeepIR := base.Flag.Shared, base.Flag.LLVMKeepIR
	defer func() {
		CurrentModule, llvmModuleFinalized = oldModule, oldFinalized
		base.Flag.Shared, base.Flag.LLVMKeepIR = oldShared, oldKeepIR
	}()
	CurrentModule, llvmModuleFinalized = module, true
	shared := false
	base.Flag.Shared, base.Flag.LLVMKeepIR = &shared, true
	addGoObjConfigMetadata(types.NewPkg("invalid_before_optimization", "invalid_before_optimization"))
	function := llvm.AddFunction(module, "missing_terminator", llvm.FunctionType(GlobalCtxt.VoidType(), nil, false))
	llvm.AddBasicBlock(function, "entry")

	output := filepath.Join(t.TempDir(), "invalid.a")
	data, err := EmitLLVMGoObj(output)
	if err == nil || !strings.Contains(err.Error(), "verify LLVM module before optimization") {
		t.Fatalf("EmitLLVMGoObj error = %v, want pre-optimization verification failure", err)
	}
	if len(data) != 0 {
		t.Fatal("invalid module produced object data")
	}
	if _, err := os.Stat(output + ".ll"); !os.IsNotExist(err) {
		t.Fatalf("invalid module reached IR output: %v", err)
	}
}

func TestLLVMEarlyIRReportsVerifierFailure(t *testing.T) {
	context := llvm.NewContext()
	defer context.Dispose()
	module := context.NewModule("invalid_early_ir")
	defer module.Dispose()
	// Exercise the plugin's verifier through the production Go binding.
	// Both linked and dynamically loaded plugins must return its failure.
	module.AddNamedMetadataOperand("goallc.cpu.config", context.MDNode([]llvm.Metadata{
		context.MDString("goallc.cpu.v1"),
		context.MDString("amd64"),
		context.MDString("v1"),
	}))
	function := llvm.AddFunction(module, "missing_terminator", llvm.FunctionType(context.VoidType(), nil, false))
	llvm.AddBasicBlock(function, "entry")
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			t.Fatal(err)
		}
	}
	err := module.RunPassPluginEarlyIR(plugin)
	if err == nil {
		t.Fatal("early IR plugin accepted invalid IR")
	}
	if !strings.Contains(err.Error(), "GoALLC early IR pass reported an error") &&
		!strings.Contains(err.Error(), "GoALLC CPU multiversioning produced invalid IR") {
		t.Fatalf("unexpected early IR error: %v", err)
	}
}

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
	// Fixed vectors of default-address-space pointers are supported. Use
	// non-default-address-space pointers to retain an actual late error.
	builder.CreateAlloca(llvm.VectorType(context.PointerType(1), 2), "unsupported")
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
