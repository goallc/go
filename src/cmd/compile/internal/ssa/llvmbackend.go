// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/internal/llvmbackend"
	"fmt"
	"internal/buildcfg"
	"os"
	"strings"

	"github.com/goallc/go-llvm"
)

// EmitLLVMObject returns a GoObj linker object, or pre-optimization bitcode
// when -llvm-lto defers optimization and code generation to cmd/link.
func EmitLLVMObject(outputFile string) ([]byte, error) {
	finalizeLLVMModule()
	if err := llvm.ConfigureGoObjFromModule(CurrentModule); err != nil {
		return nil, fmt.Errorf("configure LLVM GoObj writer: %w", err)
	}
	configureLLVMCodeGenOptions()

	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllAsmPrinters()
	llvm.InitializeAllAsmParsers()

	triple := CurrentModule.Target()
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		return nil, fmt.Errorf("resolve LLVM target %q: %w", triple, err)
	}
	level := llvm.CodeGenLevelDefault
	if base.Flag.N != 0 {
		level = llvm.CodeGenLevelNone
	}
	reloc := llvm.RelocDefault
	if *base.Flag.Shared || base.Ctxt.Flag_dynlink {
		// Honor the driver's position-independent code request for GoObj too.
		reloc = llvm.RelocPIC
	}
	tm := target.CreateTargetMachine(triple, "", "", level, reloc, llvm.CodeModelDefault)
	defer tm.Dispose()
	td := tm.CreateTargetData()
	CurrentModule.SetDataLayout(td.String())
	td.Dispose()

	if err := llvm.VerifyModule(CurrentModule, llvm.ReturnStatusAction); err != nil {
		return nil, fmt.Errorf("verify LLVM module before optimization: %w", err)
	}
	// Keep the frontend diagnostic snapshot separate from prepared IR.
	var rawIR string
	if base.Flag.LLVMKeepIR {
		rawIR = CurrentModule.String()
	}
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return nil, err
		}
	}
	if err := CurrentModule.RunPassPluginEarlyIR(plugin); err != nil {
		return nil, fmt.Errorf("run GoALLC early IR plugin: %w", err)
	}
	// The early IR plugin verifies after mutation and propagates failures.
	// Its no-op paths leave the module verified above unchanged.

	// Functional lowering is required for both object emission and LTO.
	// Publish IR only after preparation succeeds; optimization remains below.
	if base.Flag.LLVMKeepIR {
		if err := os.WriteFile(outputFile+".raw.ll", []byte(rawIR), 0666); err != nil {
			return nil, fmt.Errorf("write frontend LLVM IR: %w", err)
		}
		if err := llvm.LLVMPrintModuleToFile(CurrentModule, outputFile+".ll"); err != nil {
			return nil, fmt.Errorf("write pre-optimization LLVM IR: %w", err)
		}
	}

	if base.Flag.LLVMLTO {
		buffer := llvm.WriteBitcodeToMemoryBuffer(CurrentModule)
		defer buffer.Dispose()
		return append([]byte(nil), buffer.Bytes()...), nil
	}

	pipeline := strings.TrimSpace(base.Flag.LLVMOptPasses)
	if pipeline == "auto" {
		pipeline = "default<O2>"
		if base.Flag.N != 0 {
			pipeline = "default<O0>"
		}
	}
	if pipeline != "" && pipeline != "none" {
		options := llvm.NewPassBuilderOptions()
		defer options.Dispose()
		if err := CurrentModule.RunPasses(pipeline, tm, options); err != nil {
			return nil, fmt.Errorf("run LLVM optimization pipeline %q: %w", pipeline, err)
		}
	}
	if base.Flag.LLVMKeepIR {
		if err := llvm.LLVMPrintModuleToFile(CurrentModule, outputFile+".opt.ll"); err != nil {
			return nil, fmt.Errorf("write optimized LLVM IR: %w", err)
		}
	}

	if err := tm.RunPassPluginPreCodeGen(CurrentModule, llvm.ObjectFile, plugin); err != nil {
		return nil, fmt.Errorf("run GoALLC pre-codegen plugin: %w", err)
	}
	if err := llvm.VerifyModule(CurrentModule, llvm.ReturnStatusAction); err != nil {
		return nil, fmt.Errorf("verify LLVM module after pre-codegen: %w", err)
	}

	buffer, err := tm.EmitToMemoryBuffer(CurrentModule, llvm.ObjectFile)
	if err != nil {
		return nil, fmt.Errorf("emit LLVM GoObj: %w", err)
	}
	defer buffer.Dispose()
	return append([]byte(nil), buffer.Bytes()...), nil
}

func configureLLVMCodeGenOptions() {
	args := append([]string{"cmd/compile LLVM backend"}, llvmCodeGenOptions()...)
	llvm.ParseCommandLineOptions(args, "GoALLC in-process LLVM backend")
}

func llvmCodeGenOptions() []string { return llvmbackend.CodeGenOptions(buildcfg.GOARCH) }
