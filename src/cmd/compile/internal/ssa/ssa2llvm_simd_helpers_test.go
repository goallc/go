// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

// Masked loads have generic SSA operations but no current public intrinsic.
// Exercise their emitter directly, paired with stores; public stores also have
// the protected-page runtime regression in llvm_simd_masked_memory.
func TestLLVMSIMDMaskedMemory(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	for i, bits := range []int{8, 16, 32, 64} {
		for _, width := range []int{128, 256, 512} {
			t.Run(fmt.Sprintf("%dx%d", bits, width/bits), func(t *testing.T) {
				typ := map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[width]
				module := GlobalCtxt.NewModule("masked-memory")
				CurrentModule = module
				builder := GlobalCtxt.NewBuilder()
				defer module.Dispose()
				defer builder.Dispose()
				ptr := GlobalCtxt.PointerType(0)
				fn := llvm.AddFunction(module, "masked", llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{ptr, getLLVMType(typ)}, false))
				builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
				address := &Value{ID: 1, Op: OpArg, Type: types.NewPtr(typ)}
				mask := &Value{ID: 2, Op: OpArg, Type: typ}
				mem := &Value{ID: 3, Op: OpInitMem, Type: types.TypeMem}
				load := &Value{ID: 4, Op: []Op{OpLoadMasked8, OpLoadMasked16, OpLoadMasked32, OpLoadMasked64}[i], Type: typ, Args: []*Value{address, mask, mem}}
				store := &Value{ID: 5, Op: []Op{OpStoreMasked8, OpStoreMasked16, OpStoreMasked32, OpStoreMasked64}[i], Type: types.TypeMem, Aux: typ, Args: []*Value{address, mask, load, mem}}
				ctx := &LLVMFuncContext{F: &Func{Config: &Config{arch: "amd64"}}, CPUFeatures: &llvmCPUFeaturePlan{floor: goCPUProfileX86AVX512}, Vs: map[ID]llvm.Value{1: fn.Param(0), 2: fn.Param(1)}, b: builder}
				ctx.GenLV(store)
				builder.CreateRetVoid()
				if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
					t.Fatal(err)
				}
				ir := module.String()
				for _, name := range []string{"llvm.masked.load.", "llvm.masked.store.", "zeroinitializer"} {
					if !strings.Contains(ir, name) {
						t.Fatalf("missing %s:\n%s", name, ir)
					}
				}
				if strings.Contains(ir, "load <") || strings.Contains(ir, "store <") {
					t.Fatalf("unconditional memory access:\n%s", ir)
				}
			})
		}
	}
}
