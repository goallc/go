// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"fmt"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMFunctionAllocationAlignment(t *testing.T) {
	for _, name := range []string{"mallocgc", "mallocgcTinySC2", "mallocgcSmallNoScanSC1"} {
		for _, size := range []int64{-1, 0, 1, 2, 3, 4, 6, 8, 12, 16, 17, 24, 25, 32, 48, 64, 80, 128, 512, 1024, 32768} {
			if name == "mallocgcTinySC2" && size >= 16 || name == "mallocgcSmallNoScanSC1" && size > 0 && size != 8 {
				continue
			}
			for _, boundary := range []uint64{2, 4, 8, 16, 32, 128, 8192} {
				t.Run(fmt.Sprintf("%s/size=%d/boundary=%d", name, size, boundary), func(t *testing.T) {
					old := CurrentModule
					m := GlobalCtxt.NewModule("allocation_alignment")
					CurrentModule = m
					defer func() { CurrentModule = old; m.Dispose() }()
					ptr, i64, i1 := GlobalCtxt.PointerType(0), GlobalCtxt.Int64Type(), GlobalCtxt.Int1Type()
					sig := llvmFuncSignature{Type: llvm.FunctionType(ptr, []llvm.Type{i64, ptr, i1}, false), ClosureContextIndex: -1}
					fn := llvm.AddFunction(m, "probe", llvm.FunctionType(i64, []llvm.Type{i64}, false))
					callee := getOrInsertLLVMFunction("runtime."+name, sig, goABIInternalCallConv)
					b := GlobalCtxt.NewBuilder()
					defer b.Dispose()
					b.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
					n := fn.Param(0)
					if size >= 0 {
						n = llvm.ConstInt(i64, uint64(size), false)
					}
					args := []llvm.Value{n, llvm.ConstNull(ptr), llvm.ConstInt(i1, 1, false)}
					call := b.CreateCall(sig.Type, callee, args, "p")
					call.SetInstructionCallConv(goABIInternalCallConv)
					llvmFunctions.bindCall(call, callee, args, goABIInternalCallConv, nil)
					alignment := uint64(1)
					// An explicit expected table protects against promising uniform word
					// alignment for tiny allocations, including non-power-of-two sizes.
					switch size {
					case 2, 6:
						alignment = 2
					case 4, 12:
						alignment = 4
					case 8, 17, 24:
						alignment = 8
					case 16, 48, 80:
						alignment = 16
					case 25, 32:
						alignment = 32
					case 64, 128, 512, 1024:
						alignment = uint64(size)
					case 32768:
						alignment = 8192
					}
					attr := call.GetCallSiteEnumAttribute(0, llvm.AttributeKindID("align"))
					if alignment == 1 {
						if attr.C != nil {
							t.Fatal("unexpected alignment")
						}
					} else if attr.C == nil || attr.GetEnumValue() != alignment {
						t.Fatalf("expected align %d\n%s", alignment, call.String())
					}
					address := b.CreatePtrToInt(call, i64, "address")
					b.CreateRet(b.CreateAnd(address, llvm.ConstInt(i64, boundary-1, false), "lowbits"))
					opts := llvm.NewPassBuilderOptions()
					defer opts.Dispose()
					opts.SetVerifyEach(true)
					if err := m.RunPasses("function(instcombine)", llvm.TargetMachine{}, opts); err != nil {
						t.Fatal(err)
					}
					if got := strings.Contains(fn.String(), "ret i64 0"); got != (alignment >= boundary) {
						t.Fatalf("folded=%v alignment=%d\n%s", got, alignment, m.String())
					}
				})
			}
		}
	}
}

func TestLLVMFunctionHeaderResultExtent(t *testing.T) {
	oldPtrSize := types.PtrSize
	defer func() { types.PtrSize = oldPtrSize }()
	for _, word := range []int{4, 8} {
		types.PtrSize = word
		for _, name := range []string{"convTstring", "convTslice", "convT16", "convT32", "convT64"} {
			for _, cc := range []llvm.CallConv{goABIInternalCallConv, goABI0CallConv} {
				old := CurrentModule
				m := GlobalCtxt.NewModule("header_extent")
				CurrentModule = m
				ptr, size := GlobalCtxt.PointerType(0), GlobalCtxt.IntType(word*8)
				param := llvm.StructType([]llvm.Type{ptr, size}, false)
				extent := uint64(2 * word)
				align := uint64(0)
				switch name {
				case "convTslice":
					param = llvm.StructType([]llvm.Type{ptr, size, size}, false)
					extent = uint64(3 * word)
				case "convT16":
					param = GlobalCtxt.Int16Type()
					extent = 2
					align = 2
				case "convT32":
					param = GlobalCtxt.Int32Type()
					extent = 4
					align = 4
				case "convT64":
					param = GlobalCtxt.Int64Type()
					extent = 8
					align = uint64(word)
				}
				result := ptr
				if cc == goABI0CallConv {
					param = ptr
					result = GlobalCtxt.VoidType()
					extent = 0
					align = 0
				}
				fn := getOrInsertLLVMFunction("runtime."+name, llvmFuncSignature{Type: llvm.FunctionType(result, []llvm.Type{param}, false), ClosureContextIndex: -1}, cc)
				for attrName, want := range map[string]uint64{"dereferenceable": extent, "align": align} {
					attr := fn.GetEnumAttributeAtIndex(0, llvm.AttributeKindID(attrName))
					got := uint64(0)
					if attr.C != nil {
						got = attr.GetEnumValue()
					}
					if got != want {
						t.Fatalf("%s word=%d cc=%d %s=%d want=%d", name, word, cc, attrName, got, want)
					}
				}
				if err := llvm.VerifyModule(m, llvm.ReturnStatusAction); err != nil {
					t.Fatal(err)
				}
				CurrentModule = old
				m.Dispose()
			}
		}
	}
}

func TestLLVMFunctionASanAllocationAlignment(t *testing.T) {
	old, word := base.Flag.Cfg.ASan, types.PtrSize
	defer func() { base.Flag.Cfg.ASan = old; types.PtrSize = word }()
	types.PtrSize = 8
	for _, tc := range []struct {
		size         uint64
		noscan       bool
		normal, asan uint64
	}{
		{32, true, 32, 16}, {128, true, 128, 64}, {512, true, 512, 128},
		{512, false, 512, 8}, {520, false, 8, 8}, {32768, true, 8192, 8192},
	} {
		base.Flag.Cfg.ASan = false
		if got := llvmHeapAlignment(tc.size, tc.noscan); got != tc.normal {
			t.Fatalf("normal size=%d got=%d want=%d", tc.size, got, tc.normal)
		}
		base.Flag.Cfg.ASan = true
		if got := llvmHeapAlignment(tc.size, tc.noscan); got != tc.asan {
			t.Fatalf("asan size=%d got=%d want=%d", tc.size, got, tc.asan)
		}
	}
}
