// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"strings"
	"testing"

	"cmd/compile/internal/types"
	"cmd/internal/obj"

	"github.com/goallc/go-llvm"
)

func TestLLVMCurrentGRegister(t *testing.T) {
	for _, test := range []struct {
		name     string
		arch     string
		abi      obj.ABI
		register string
		ok       bool
	}{
		{"amd64 ABIInternal", "amd64", obj.ABIInternal, "r14", true},
		{"amd64 ABI0", "amd64", obj.ABI0, "", false},
		{"arm64 ABIInternal", "arm64", obj.ABIInternal, "x28", true},
		{"arm64 ABI0", "arm64", obj.ABI0, "x28", true},
		{"unsupported target", "riscv64", obj.ABIInternal, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			register, ok := llvmCurrentGRegister(test.arch, test.abi)
			if register != test.register || ok != test.ok {
				t.Fatalf("llvmCurrentGRegister(%q, %v) = (%q, %v), want (%q, %v)", test.arch, test.abi, register, ok, test.register, test.ok)
			}
		})
	}
}

func TestLLVMAMD64MapPackedByteLowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = map[*types.Type]llvm.Type{
		types.Types[types.TUINT8]:  GlobalCtxt.Int8Type(),
		types.Types[types.TUINT64]: GlobalCtxt.Int64Type(),
		types.TypeInt128:           llvmAMD64ByteVectorType(),
	}
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	newContext := func(t *testing.T, name string, parameterCount int) (llvm.Module, *LLVMFuncContext, []*Value) {
		t.Helper()
		module := GlobalCtxt.NewModule(name)
		CurrentModule = module
		parameters := make([]llvm.Type, parameterCount)
		for i := range parameters {
			parameters[i] = GlobalCtxt.Int64Type()
		}
		function := llvm.AddFunction(module, name, llvm.FunctionType(GlobalCtxt.Int64Type(), parameters, false))
		block := llvm.AddBasicBlock(function, "entry")
		builder := GlobalCtxt.NewBuilder()
		t.Cleanup(module.Dispose)
		t.Cleanup(builder.Dispose)
		builder.SetInsertPointAtEnd(block)
		context := &LLVMFuncContext{
			Vs: make(map[ID]llvm.Value),
			b:  builder,
		}
		arguments := make([]*Value, parameterCount)
		for i := range arguments {
			arguments[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: types.Types[types.TUINT64]}
			context.Vs[arguments[i].ID] = function.Param(i)
		}
		return module, context, arguments
	}

	t.Run("GOAMD64v1", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v1", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		unpacked := &Value{ID: 5, Op: OpAMD64PUNPCKLBW, Type: types.TypeInt128, Args: []*Value{h2, h2}}
		broadcast := &Value{ID: 6, Op: OpAMD64PSHUFLW, Type: types.TypeInt128, AuxInt: 0, Args: []*Value{unpacked}}
		equal := &Value{ID: 7, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		mask := &Value{ID: 8, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT8], Args: []*Value{equal}}
		result := &Value{ID: 9, Op: OpZeroExt8to64, Type: types.Types[types.TUINT64], Args: []*Value{mask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v1 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"bitcast i64 %0 to <8 x i8>",
			"shufflevector <8 x i8>",
			"shufflevector <16 x i8>",
			"bitcast <16 x i8>",
			"icmp eq <16 x i8>",
			"sext <16 x i1>",
			"call i32 @llvm.x86.sse2.pmovmskb.128(<16 x i8>",
			"trunc i32",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v1 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v2", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v2", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		broadcast := &Value{ID: 5, Op: OpAMD64PSHUFBbroadcast, Type: types.TypeInt128, Args: []*Value{h2}}
		equal := &Value{ID: 6, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		equalMask := &Value{ID: 7, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{equal}}
		signed := &Value{ID: 8, Op: OpAMD64PSIGNB, Type: types.TypeInt128, Args: []*Value{group, group}}
		signMask := &Value{ID: 9, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{signed}}
		result := &Value{ID: 10, Op: OpOr64, Type: types.Types[types.TUINT64], Args: []*Value{equalMask, signMask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v2 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"insertelement <16 x i8>",
			"icmp sgt <16 x i8>",
			"icmp slt <16 x i8>",
			"select <16 x i1>",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v2 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v4", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v4", 1)
		broadcast := &Value{ID: 2, Op: OpAMD64VPBROADCASTB, Type: types.TypeInt128, Args: []*Value{args[0]}}
		mask := &Value{ID: 3, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{broadcast}}
		context.b.CreateRet(context.GenLV(mask))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v4 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		if !strings.Contains(ir, "trunc i64 %0 to i8") || !strings.Contains(ir, "shufflevector <16 x i8>") {
			t.Errorf("v4 LLVM IR does not broadcast the low input byte\n%s", ir)
		}
	})
}
