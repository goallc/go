// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"fmt"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMFunctionFreshResultEffects(t *testing.T) {
	for _, name := range []string{"makechan", "makechan64", "makemap_small", "convT", "convTnoptr"} {
		for _, n := range []int64{-1, 0, 8} {
			for _, modeled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/size=%d/model=%v", name, n, modeled), func(t *testing.T) {
					old := CurrentModule
					m := GlobalCtxt.NewModule("fresh_results")
					CurrentModule = m
					defer func() { CurrentModule = old; m.Dispose() }()
					ptr, i64 := GlobalCtxt.PointerType(0), GlobalCtxt.Int64Type()
					params := []llvm.Type{ptr, i64}
					boxed := name == "convT" || name == "convTnoptr"
					if boxed {
						params = []llvm.Type{ptr, ptr}
					} else if name == "makemap_small" {
						params = nil
					}
					sig := llvmFuncSignature{Type: llvm.FunctionType(ptr, params, false), ClosureContextIndex: -1}
					var callee llvm.Value
					if modeled {
						callee = getOrInsertLLVMFunction("runtime."+name, sig, goABIInternalCallConv)
					} else {
						callee = llvm.AddFunction(m, "runtime."+name, sig.Type)
						callee.SetFunctionCallConv(goABIInternalCallConv)
					}
					fn := llvm.AddFunction(m, "probe", llvm.FunctionType(i64, append([]llvm.Type{ptr}, params...), false))
					b := GlobalCtxt.NewBuilder()
					defer b.Dispose()
					b.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
					args := fn.Params()[1:]
					call := b.CreateCall(sig.Type, callee, args, "fresh")
					call.SetInstructionCallConv(goABIInternalCallConv)
					if modeled && boxed {
						descriptor := &Value{Op: OpArg}
						if n >= 0 {
							typ := types.NewArray(types.Types[types.TUINT8], n)
							types.CalcSize(typ)
							sym := &obj.LSym{Name: "type:alias-test"}
							sym.NewTypeInfo().Type = typ
							descriptor = &Value{Op: OpAddr, Aux: sym}
						}
						source := &Value{Args: []*Value{descriptor}}
						llvmFunctions.bindCall(call, callee, args, goABIInternalCallConv, source)
						if got := call.GetCallSiteEnumAttribute(0, llvm.AttributeKindID("noalias")).C != nil; got != (n > 0) {
							t.Fatalf("noalias=%v size=%d", got, n)
						}
						if callee.GetEnumAttributeAtIndex(0, llvm.AttributeKindID("noalias")).C != nil {
							t.Fatal("conditional noalias leaked to declaration")
						}
						if call.GetCallSiteEnumAttribute(llvmAttributeFunctionIndex, llvm.AttributeKindID("allockind")).C != nil {
							t.Fatal("copy modeled as allocation-only")
						}
					}
					// Only probe memory within known positive allocations. The old pointer
					// is written after the call so unrestricted call effects cannot obscure
					// the result-alias property being tested.
					if !boxed || n > 0 {
						b.CreateStore(llvm.ConstInt(i64, 17, false), fn.Param(0))
						b.CreateStore(llvm.ConstInt(i64, 29, false), call)
						b.CreateRet(b.CreateLoad(i64, fn.Param(0), "old"))
					} else {
						b.CreateRet(llvm.ConstInt(i64, 0, false))
					}
					opts := llvm.NewPassBuilderOptions()
					defer opts.Dispose()
					opts.SetVerifyEach(true)
					if err := m.RunPasses("function(instcombine,early-cse<memssa>,gvn)", llvm.TargetMachine{}, opts); err != nil {
						t.Fatal(err)
					}
					if !boxed || n > 0 {
						if got := strings.Contains(fn.String(), "ret i64 17"); got != modeled {
							t.Fatalf("forwarded=%v modeled=%v\n%s", got, modeled, m.String())
						}
					}
					if !strings.Contains(fn.String(), "call goabiinternal") {
						t.Fatal("effectful call removed")
					}
				})
			}
		}
	}
}
