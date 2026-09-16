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

func TestLLVMFunctionMemoryModelAttributes(t *testing.T) {
	oldModule := CurrentModule
	module := GlobalCtxt.NewModule("memory_model_signatures")
	CurrentModule = module
	defer func() { CurrentModule = oldModule; module.Dispose() }()
	ptr := GlobalCtxt.PointerType(0)
	size := GlobalCtxt.IntType(int(types.PtrSize * 8))
	str := llvm.StructType([]llvm.Type{ptr, size}, false)
	for _, test := range []struct {
		name   string
		result llvm.Type
		params []llvm.Type
		access []string
		leaf   bool
		raw    bool
	}{
		{"runtime.memmove", GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr, size}, []string{"writeonly", "readonly"}, true, true},
		{"runtime.memequal", GlobalCtxt.Int1Type(), []llvm.Type{ptr, ptr, size}, []string{"readonly", "readonly"}, true, true},
		{"runtime.memclrNoHeapPointers", GlobalCtxt.VoidType(), []llvm.Type{ptr, size}, []string{"writeonly"}, true, true},
		{"runtime.memequal_varlen", GlobalCtxt.Int1Type(), []llvm.Type{ptr, ptr}, []string{"readonly", "readonly"}, true, true},
		{"runtime.cmpstring", size, []llvm.Type{str, str}, nil, true, true},
		{"runtime.memequal64", GlobalCtxt.Int1Type(), []llvm.Type{ptr, ptr}, []string{"readonly", "readonly"}, false, false},
		{"runtime.fint32to32", GlobalCtxt.Int32Type(), []llvm.Type{GlobalCtxt.Int32Type()}, nil, false, false},
		{"runtime.memhash", size, []llvm.Type{ptr, size, size}, []string{"readonly"}, false, false},
		{"runtime.f64hash", size, []llvm.Type{ptr, size}, []string{"readonly"}, false, false},
		{"runtime.chanlen", size, []llvm.Type{ptr}, []string{"readonly"}, false, false},
		{"runtime.selectsetpc", GlobalCtxt.VoidType(), []llvm.Type{ptr}, []string{"writeonly"}, false, false},
		{"runtime.typedmemmove", GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr, ptr}, nil, true, false},
		{"runtime.typedmemclr", GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr}, nil, true, false},
		{"runtime.cgoCheckPtrWrite", GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr}, nil, true, false},
	} {
		for _, cc := range []llvm.CallConv{goABIInternalCallConv, goABI0CallConv} {
			sig := llvmFuncSignature{
				Type: llvm.FunctionType(test.result, test.params, false), ClosureContextIndex: -1,
			}
			// Model the actual ABI0 stack carriers, including the result home.
			if cc == goABI0CallConv {
				params := make([]llvm.Type, len(test.params))
				for i, typ := range test.params {
					params[i] = ptr
					sig.Params = append(sig.Params, llvmParamSignature{ValueType: typ, Alignment: int(types.PtrSize), ByVal: true})
				}
				if test.result.TypeKind() != llvm.VoidTypeKind {
					sig.Results = []llvmResultSignature{{ValueType: test.result, Alignment: 1, InMemory: true, ParamIndex: len(params)}}
					params = append(params, ptr)
				}
				sig.Type = llvm.FunctionType(GlobalCtxt.VoidType(), params, false)
			}
			if test.name == "runtime.memequal_varlen" {
				sig.ReturnType = sig.Type.ReturnType()
				sig = sig.withClosureContext()
			}
			fn := getOrInsertLLVMFunction(test.name, sig, cc)
			want := cc == goABIInternalCallConv
			if got := fn.GetStringAttributeAtIndex(llvmAttributeFunctionIndex, goGCLeafFunctionAttr).C != nil; got != test.leaf {
				t.Errorf("%s: GC leaf = %v, want %v", fn.Name(), got, test.leaf)
			}
			for _, attr := range []string{"nofree", "nocallback", "nounwind"} {
				want := test.raw || attr == "nounwind"
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
			// Integer and aggregate arguments do not get pointer attributes.
			if len(test.access) < len(test.params) && fn.GetEnumAttributeAtIndex(len(test.params), llvm.AttributeKindID("captures")).C != nil {
				t.Errorf("%s: pointer attribute attached to non-pointer argument", fn.Name())
			}
			comparison := test.name == "runtime.memequal" || test.name == "runtime.memequal_varlen" || test.name == "runtime.cmpstring"
			for _, attr := range []string{"willreturn", "nosync"} {
				want := comparison || attr == "willreturn" && (test.name == "runtime.memequal64" || test.name == "runtime.memhash" || test.name == "runtime.fint32to32" || test.name == "runtime.chanlen")
				if got := fn.GetEnumFunctionAttribute(llvm.AttributeKindID(attr)).C != nil; got != want {
					t.Errorf("%s: %s=%v want=%v", fn.Name(), attr, got, want)
				}
			}
			wantRead := cc == goABIInternalCallConv && (comparison || test.name == "runtime.memequal64" || test.name == "runtime.memhash" || test.name == "runtime.chanlen")
			wantNone := cc == goABIInternalCallConv && test.name == "runtime.fint32to32"
			if wantNone && !strings.Contains(fn.String(), "memory(none)") {
				t.Errorf("%s: missing memory(none)", fn.Name())
			}
			if wantRead && !strings.Contains(fn.String(), "memory(read)") {
				t.Errorf("%s: incorrect memory encoding: %s", fn.Name(), fn.String())
			}
			if got := fn.GetEnumFunctionAttribute(llvm.AttributeKindID("memory")).C != nil; got != (wantRead || wantNone) {
				t.Errorf("%s: memory attribute = %v, want %v", fn.Name(), got, wantRead || wantNone)
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
	for _, helper := range []string{"runtime.memequal", "runtime.memmove", "runtime.memclrNoHeapPointers", "runtime.memequal64", "runtime.f64equal", "runtime.memhash"} {
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
					if helper == "runtime.memequal" || helper == "runtime.memequal64" || helper == "runtime.f64equal" {
						result = GlobalCtxt.Int8Type()
						if helper != "runtime.memequal" {
							params = []llvm.Type{ptr, ptr}
						}
					} else if helper == "runtime.memhash" {
						result, params = size, []llvm.Type{ptr, size, size}
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
					if helper == "runtime.memequal64" || helper == "runtime.f64equal" {
						args = args[:2]
					} else if helper == "runtime.memhash" {
						args = []llvm.Value{local, llvm.ConstInt(size, 0, false), llvm.ConstInt(size, 8, false)}
					} else if helper == "runtime.memclrNoHeapPointers" {
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
					wantForward := modeled && helper != "runtime.memclrNoHeapPointers" && (helper != "runtime.memmove" || !alias)
					if got := strings.Contains(caller.String(), "ret i64 7"); got != wantForward {
						t.Fatalf("load forwarded = %v, want %v\n%s", got, wantForward, module.String())
					}
				})
			}
		}
	}
}

// Check effects on calls themselves, not just pointer-argument load forwarding.
// A write to the compared bytes must prevent commoning the two comparisons.
func TestLLVMFunctionComparisonEffects(t *testing.T) {
	for _, helper := range []string{"runtime.memequal", "runtime.memhash", "runtime.fint32to32", "runtime.f64hash"} {
		for _, modeled := range []bool{false, true} {
			for _, mutate := range []bool{false, true} {
				for _, discard := range []bool{false, true} {
					oldModule := CurrentModule
					module := GlobalCtxt.NewModule("comparison_effects")
					CurrentModule = module
					func() {
						defer func() { CurrentModule = oldModule; module.Dispose() }()
						ptr, i8 := GlobalCtxt.PointerType(0), GlobalCtxt.Int8Type()
						size := GlobalCtxt.IntType(int(types.PtrSize * 8))
						result, params := i8, []llvm.Type{ptr, ptr, size}
						if helper == "runtime.memhash" {
							result, params = size, []llvm.Type{ptr, size, size}
						}
						if helper == "runtime.f64hash" {
							result, params = size, []llvm.Type{ptr, size}
						}
						if helper == "runtime.fint32to32" {
							result, params = GlobalCtxt.Int32Type(), []llvm.Type{GlobalCtxt.Int32Type()}
						}
						sig := llvmFuncSignature{Type: llvm.FunctionType(result, params, false)}
						var callee llvm.Value
						if modeled {
							callee = getOrInsertLLVMFunction(helper, sig, goABIInternalCallConv)
						} else {
							callee = llvm.AddFunction(module, helper, sig.Type)
							callee.SetFunctionCallConv(goABIInternalCallConv)
						}
						caller := llvm.AddFunction(module, "probe", llvm.FunctionType(result, []llvm.Type{ptr, ptr}, false))
						b := GlobalCtxt.NewBuilder()
						defer b.Dispose()
						b.SetInsertPointAtEnd(llvm.AddBasicBlock(caller, "entry"))
						args := []llvm.Value{caller.Param(0), caller.Param(1), llvm.ConstInt(size, 1, false)}
						if helper == "runtime.memhash" {
							args = []llvm.Value{caller.Param(0), llvm.ConstInt(size, 0, false), llvm.ConstInt(size, 8, false)}
						}
						if helper == "runtime.f64hash" {
							args = []llvm.Value{caller.Param(0), llvm.ConstInt(size, 0, false)}
						}
						if helper == "runtime.fint32to32" {
							args = []llvm.Value{llvm.ConstInt(result, 13, false)}
						}
						first := b.CreateCall(sig.Type, callee, args, "first")
						first.SetInstructionCallConv(goABIInternalCallConv)
						if mutate {
							b.CreateStore(llvm.ConstInt(i8, 42, false), caller.Param(0))
						}
						second := b.CreateCall(sig.Type, callee, args, "second")
						second.SetInstructionCallConv(goABIInternalCallConv)
						if discard {
							b.CreateRet(llvm.ConstInt(result, 0, false))
						} else {
							b.CreateRet(b.CreateAdd(first, second, "sum"))
						}
						opts := llvm.NewPassBuilderOptions()
						defer opts.Dispose()
						opts.SetVerifyEach(true)
						if err := module.RunPasses("function(instcombine,gvn,dce)", llvm.TargetMachine{}, opts); err != nil {
							t.Fatal(err)
						}
						want := 2
						if modeled && helper != "runtime.f64hash" && discard {
							want = 0
						} else if modeled && helper != "runtime.f64hash" && (!mutate || helper == "runtime.fint32to32") {
							want = 1
						}
						if got := strings.Count(caller.String(), "call goabiinternal "); got != want {
							t.Fatalf("helper=%s modeled=%v mutate=%v discard=%v: calls=%d want=%d\n%s", helper, modeled, mutate, discard, got, want, module.String())
						}
					}()
				}
			}
		}
	}

}
