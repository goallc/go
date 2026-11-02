//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

import (
	"cmd/internal/llvmbackend"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/goallc/go-llvm"
)

var initialize sync.Once

func emit(input, output string, pic bool) error {
	c := llvm.NewContext()
	defer c.Dispose()
	m, err := c.ParseBitcodeFile(input)
	if err != nil {
		return err
	}
	defer m.Dispose()
	initialize.Do(func() {
		arch := "amd64"
		if strings.HasPrefix(m.Target(), "aarch64") {
			arch = "arm64"
		}
		llvm.ParseCommandLineOptions(append([]string{"cmd/link LTO"}, llvmbackend.CodeGenOptions(arch)...), "GoALLC LTO")
		llvm.InitializeAllTargetInfos()
		llvm.InitializeAllTargets()
		llvm.InitializeAllTargetMCs()
		llvm.InitializeAllAsmPrinters()
		llvm.InitializeAllAsmParsers()
	})
	if err := llvm.ConfigureGoObjFromModule(m); err != nil {
		return err
	}
	target, err := llvm.GetTargetFromTriple(m.Target())
	if err != nil {
		return err
	}
	reloc := llvm.RelocDefault
	if pic {
		reloc = llvm.RelocPIC
	}
	tm := target.CreateTargetMachine(m.Target(), "", "", llvm.CodeGenLevelDefault, reloc, llvm.CodeModelDefault)
	defer tm.Dispose()
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return err
		}
	}
	if err := tm.RunPassPluginPreCodeGen(m, llvm.ObjectFile, plugin); err != nil {
		return fmt.Errorf("LTO pre-codegen: %w", err)
	}
	if err := llvm.VerifyModule(m, llvm.ReturnStatusAction); err != nil {
		return err
	}
	buffer, err := tm.EmitToMemoryBuffer(m, llvm.ObjectFile)
	if err != nil {
		return err
	}
	defer buffer.Dispose()
	return os.WriteFile(output, buffer.Bytes(), 0600)
}
