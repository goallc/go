// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/internal/src"
	"path/filepath"
	"strings"

	"github.com/goallc/go-llvm"
)

var llvmNoWriteBarrierCheck func()

// SetLLVMNoWriteBarrierCheck defers the native checker until the final LLVM
// call graph and expanded write barriers are available.
func SetLLVMNoWriteBarrierCheck(check func()) { llvmNoWriteBarrierCheck = check }

func recordLLVMWriteBarrierInfo() {
	functions := make(map[llvm.Value]*ir.Func)
	for _, fn := range typecheck.Target.Funcs {
		if fn.LSym == nil {
			continue
		}
		name := llvmFunctionStorageName(fn.LSym.Name, llvmCallConv(fn.LSym.ABI()))
		if f := CurrentModule.NamedFunction(name); !f.IsNil() {
			functions[f] = fn
		}
		// SSA barriers may have disappeared, or LLVM inlining may have added
		// barriers to the caller. Recompute both facts from the final IR.
		fn.WBPos = src.NoXPos
		fn.NWBRCalls = nil
	}
	files := make(map[string]*src.PosBase)
	position := func(i llvm.Value, fallback src.XPos) src.XPos {
		loc := i.InstructionDebugLoc()
		if loc.IsNil() || loc.LocationLine() == 0 {
			return fallback
		}
		file := loc.LocationScope().ScopeFile()
		if file.IsNil() {
			return fallback
		}
		name := file.FileFilename()
		if !filepath.IsAbs(name) {
			name = filepath.Join(file.FileDirectory(), name)
		}
		b := files[name]
		if b == nil {
			b = src.NewFileBase(name, name)
			files[name] = b
		}
		return base.Ctxt.PosTable.XPos(src.MakePos(b, loc.LocationLine(), loc.LocationColumn()))
	}
	for f, fn := range functions {
		seen := make(map[llvm.Value]bool)
		work := []llvm.Value{f}
		for len(work) != 0 {
			body := work[len(work)-1]
			work = work[:len(work)-1]
			if seen[body] || body.BasicBlocksCount() == 0 {
				continue
			}
			seen[body] = true
			for _, bb := range body.BasicBlocks() {
				for i := bb.FirstInstruction(); !i.IsNil(); i = llvm.NextInstruction(i) {
					if i.InstructionOpcode() != llvm.Call && i.InstructionOpcode() != llvm.Invoke && i.InstructionOpcode() != llvm.CallBr {
						continue
					}
					callee := i.CalledValue()
					if callee.IsAFunction().IsNil() {
						continue
					}
					pos := position(i, fn.Pos())
					switch strings.TrimSuffix(callee.Name(), goABI0SymbolSuffix) {
					case goWriteBarrierIntrinsic, "runtime.wbMove", "runtime.wbZero", "runtime.typedmemmove":
						fn.SetWBPos(pos)
					}
					if target := functions[callee]; target != nil {
						if fn.NWBRCalls == nil {
							fn.NWBRCalls = new([]ir.SymAndPos)
						}
						*fn.NWBRCalls = append(*fn.NWBRCalls, ir.SymAndPos{Sym: target.LSym, Pos: pos})
					} else if callee.BasicBlocksCount() != 0 {
						// LLVM-created helpers (including CPU variants) have no
						// frontend node. Follow them until reaching a Go function.
						work = append(work, callee)
					}
				}
			}
		}
	}
}
