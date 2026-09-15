// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMFunctionMemoryModelSignatures(t *testing.T) {
	oldModule := CurrentModule
	module := GlobalCtxt.NewModule("memory_model_signatures")
	CurrentModule = module
	defer func() { CurrentModule = oldModule; module.Dispose() }()
	ptr := GlobalCtxt.PointerType(0)
	size := GlobalCtxt.IntType(int(types.PtrSize * 8))
	for _, test := range []struct {
		name   string
		result llvm.Type
		params []llvm.Type
		access []string
	}{
		{"runtime.memmove", GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr, size}, []string{"writeonly", "readonly"}},
		{"runtime.memequal", GlobalCtxt.Int8Type(), []llvm.Type{ptr, ptr, size}, []string{"readonly", "readonly"}},
		{"runtime.memclrNoHeapPointers", GlobalCtxt.VoidType(), []llvm.Type{ptr, size}, []string{"writeonly"}},
	} {
		for _, cc := range []llvm.CallConv{goABIInternalCallConv, goABI0CallConv} {
			// A provisional declaration must not acquire parameter contracts.
			sig := llvmFuncSignature{Type: llvm.FunctionType(GlobalCtxt.VoidType(), nil, false), ClosureContextIndex: -1}
			fn := getOrInsertLLVMFunction(test.name, sig, cc)
			if fn.GetEnumFunctionAttribute(llvm.AttributeKindID("nofree")).C != nil {
				t.Fatalf("%s: modeled a provisional signature", test.name)
			}
			sig.Type = llvm.FunctionType(test.result, test.params, false)
			fn = getOrInsertLLVMFunction(test.name, sig, cc)
			want := cc == goABIInternalCallConv
			for _, attr := range []string{"nofree", "nocallback", "nounwind"} {
				if got := fn.GetEnumFunctionAttribute(llvm.AttributeKindID(attr)).C != nil; got != want {
					t.Errorf("%s: %s = %v, want %v", fn.Name(), attr, got, want)
				}
			}
			for i, access := range test.access {
				for _, attr := range []string{"captures", access} {
					if got := fn.GetEnumAttributeAtIndex(i+1, llvm.AttributeKindID(attr)).C != nil; got != want {
						t.Errorf("%s parameter %d: %s = %v, want %v", fn.Name(), i, attr, got, want)
					}
				}
				if fn.GetEnumAttributeAtIndex(i+1, llvm.AttributeKindID("noalias")).C != nil {
					t.Errorf("%s: overlapping memory must remain valid", fn.Name())
				}
			}
			// Size is an integer; pointer-only attributes must never reach it.
			if fn.GetEnumAttributeAtIndex(len(test.params), llvm.AttributeKindID("captures")).C != nil {
				t.Errorf("%s: pointer attribute attached to size", fn.Name())
			}
			if fn.GetEnumFunctionAttribute(llvm.AttributeKindID("memory")).C != nil {
				t.Errorf("%s: global/runtime memory effects were restricted", fn.Name())
			}
		}
	}
	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("invalid modeled declarations: %v\n%s", err, module.String())
	}
}

// The source of a raw comparison or copy remains unchanged, but an alias of a
// copy's destination does not. Check the optimization rather than only attrs.
func TestLLVMFunctionMemoryModelLoadForwarding(t *testing.T) {
	for _, helper := range []string{"runtime.memequal", "runtime.memmove", "runtime.memclrNoHeapPointers"} {
		for _, modeled := range []bool{false, true} {
			for _, alias := range []bool{false, true} {
				t.Run(helper+"/"+map[bool]string{false: "baseline", true: "modeled"}[modeled]+"/"+map[bool]string{false: "distinct", true: "alias"}[alias], func(t *testing.T) {
					oldModule := CurrentModule
					module := GlobalCtxt.NewModule("memory_model_optimization")
					CurrentModule = module
					defer func() { CurrentModule = oldModule; module.Dispose() }()
					ptr, i64 := GlobalCtxt.PointerType(0), GlobalCtxt.Int64Type()
					size := GlobalCtxt.IntType(int(types.PtrSize * 8))
					result := GlobalCtxt.VoidType()
					params := []llvm.Type{ptr, ptr, size}
					if helper == "runtime.memequal" {
						result = GlobalCtxt.Int8Type()
					} else if helper == "runtime.memclrNoHeapPointers" {
						params = []llvm.Type{ptr, size}
					}
					sig := llvmFuncSignature{Type: llvm.FunctionType(result, params, false), ClosureContextIndex: -1}
					var callee llvm.Value
					if modeled {
						callee = getOrInsertLLVMFunction(helper, sig, goABIInternalCallConv)
					} else {
						callee = llvm.AddFunction(module, helper, sig.Type)
						callee.SetFunctionCallConv(goABIInternalCallConv)
					}
					caller := llvm.AddFunction(module, "probe", llvm.FunctionType(i64, []llvm.Type{ptr}, false))
					b := GlobalCtxt.NewBuilder()
					defer b.Dispose()
					b.SetInsertPointAtEnd(llvm.AddBasicBlock(caller, "entry"))
					local := b.CreateAlloca(i64, "local")
					b.CreateStore(llvm.ConstInt(i64, 7, false), local)
					other := caller.Param(0)
					if alias {
						other = local
					}
					args := []llvm.Value{other, local, llvm.ConstInt(size, 8, false)}
					if helper == "runtime.memclrNoHeapPointers" {
						args = []llvm.Value{local, llvm.ConstInt(size, 8, false)}
					}
					call := b.CreateCall(sig.Type, callee, args, "")
					call.SetInstructionCallConv(goABIInternalCallConv)
					b.CreateRet(b.CreateLoad(i64, local, "loaded"))
					options := llvm.NewPassBuilderOptions()
					defer options.Dispose()
					options.SetVerifyEach(true)
					if err := module.RunPasses("function(instcombine,gvn)", llvm.TargetMachine{}, options); err != nil {
						t.Fatal(err)
					}
					wantForward := modeled && (helper == "runtime.memequal" || helper == "runtime.memmove" && !alias)
					if got := strings.Contains(caller.String(), "ret i64 7"); got != wantForward {
						t.Fatalf("load forwarded = %v, want %v\n%s", got, wantForward, module.String())
					}
				})
			}
		}
	}
}
