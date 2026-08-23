// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/internal/llvmbackend"
	"fmt"
	"strings"

	"github.com/goallc/go-llvm"
)

// EmitLLVMGoObj runs the complete LLVM optimization and code-generation
// pipeline in cmd/compile and returns the linker object as owned Go memory.
func EmitLLVMGoObj(outputFile string) ([]byte, error) {
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
	tm := target.CreateTargetMachine(triple, "", "", llvm.CodeGenLevelDefault, llvm.RelocDefault, llvm.CodeModelDefault)
	defer tm.Dispose()
	setLLVMModuleDataLayout(CurrentModule, tm)

	if err := llvm.VerifyModule(CurrentModule, llvm.ReturnStatusAction); err != nil {
		return nil, fmt.Errorf("verify LLVM module before optimization: %w", err)
	}
	if base.Flag.LLVMKeepIR {
		if err := llvm.LLVMPrintModuleToFile(CurrentModule, outputFile+".ll"); err != nil {
			return nil, fmt.Errorf("write pre-optimization LLVM IR: %w", err)
		}
	}

	pipeline := strings.TrimSpace(base.Flag.LLVMOptPasses)
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

	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return nil, err
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

func setLLVMModuleDataLayout(module llvm.Module, tm llvm.TargetMachine) {
	if module.DataLayout() != "" {
		return
	}
	td := tm.CreateTargetData()
	defer td.Dispose()
	module.SetDataLayout(td.String())
}

func configureLLVMCodeGenOptions() {
	args := append([]string{"cmd/compile LLVM backend"}, llvmCodeGenOptions()...)
	llvm.ParseCommandLineOptions(args, "GoALLC in-process LLVM backend")
}

func llvmCodeGenOptions() []string {
	return []string{"-trap-unreachable", "-disable-machine-cse", "-disable-lsr"}
}
