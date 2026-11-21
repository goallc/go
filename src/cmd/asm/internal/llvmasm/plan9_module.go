// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

//go:build !compiler_bootstrap

package llvmasm

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goallc/go-llvm"
)

// EmitPlan9File materializes a saved IR/bitcode carrier without the original
// assembly files, parser state, or preprocessing environment.
func EmitPlan9File(path string) ([]byte, error) {
	context := llvm.NewContext()
	defer context.Dispose()
	header, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var module llvm.Module
	if len(header) >= 4 && string(header[:4]) == "BC\xc0\xde" {
		module, err = context.ParseBitcodeFile(path)
	} else {
		var buffer llvm.MemoryBuffer
		buffer, err = llvm.NewMemoryBufferFromFile(path)
		if err == nil {
			module, err = context.ParseIR(buffer)
		} // ParseIR takes ownership.
	}
	if err != nil {
		return nil, err
	}
	defer module.Dispose()
	return materializePlan9Module(module)
}

func materializePlan9Module(module llvm.Module) ([]byte, error) {
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllAsmPrinters()
	llvm.InitializeAllAsmParsers()
	relocation := llvm.RelocStatic
	for f := module.FirstFunction(); !f.IsNil(); f = llvm.NextFunction(f) {
		a := f.GetStringAttributeAtIndex(-1, "go-plan9-context")
		if a.C == nil {
			continue
		}
		var c plan9Context
		if err := json.Unmarshal([]byte(a.GetStringValue()), &c); err != nil {
			return nil, err
		}
		arch := "x86_64"
		if c.GOARCH == "arm64" {
			arch = "aarch64"
		}
		if !strings.HasPrefix(module.Target(), arch+"-") || !strings.HasSuffix(module.Target(), "-"+c.GOOS+"-goobj") {
			return nil, fmt.Errorf("Plan 9 function target does not match module triple")
		}
		if c.Shared {
			relocation = llvm.RelocPIC
		}
	}
	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		return nil, err
	}
	if err := llvm.ConfigureGoObjFromModule(module); err != nil {
		return nil, err
	}
	target, err := llvm.GetTargetFromTriple(module.Target())
	if err != nil {
		return nil, err
	}
	tm := target.CreateTargetMachine(module.Target(), "generic", "", llvm.CodeGenLevelDefault, relocation, llvm.CodeModelDefault)
	defer tm.Dispose()
	var data []byte
	err = llvm.WithPlan9Assembler(assemblePlan9, func() error {
		buffer, err := tm.EmitToMemoryBuffer(module, llvm.ObjectFile)
		if err != nil {
			return err
		}
		defer buffer.Dispose()
		data = append([]byte(nil), buffer.Bytes()...)
		return nil
	})
	return data, err
}
