// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

// Result facts may simplify callers without erasing synchronization, panic,
// allocation or user callbacks. Check unmodeled and ABI0 controls as well.
func TestLLVMFunctionContainerResults(t *testing.T) {
	ptr, i32, i64 := GlobalCtxt.PointerType(0), GlobalCtxt.Int32Type(), GlobalCtxt.Int64Type()
	str := llvm.StructType([]llvm.Type{ptr, i64}, false)
	cases := []struct {
		name   string
		result llvm.Type
		params []llvm.Type
		known  bool
	}{
		{"assertE2I", ptr, []llvm.Type{ptr, ptr}, true},
		{"assertE2I2", ptr, []llvm.Type{ptr, ptr}, false},
		{"makechan", ptr, []llvm.Type{ptr, i64}, true},
		{"makechan64", ptr, []llvm.Type{ptr, i64}, true},
		{"makemap", ptr, []llvm.Type{ptr, i64, ptr}, true},
		{"makemap64", ptr, []llvm.Type{ptr, i64, ptr}, true},
		{"makemap_small", ptr, nil, true},
		{"mapaccess1", ptr, []llvm.Type{ptr, ptr, ptr}, true},
		{"mapaccess1_fast32", ptr, []llvm.Type{ptr, ptr, i32}, true},
		{"mapaccess1_fast64", ptr, []llvm.Type{ptr, ptr, i64}, true},
		{"mapaccess1_faststr", ptr, []llvm.Type{ptr, ptr, str}, true},
		{"mapassign", ptr, []llvm.Type{ptr, ptr, ptr}, true},
		{"mapassign_fast32", ptr, []llvm.Type{ptr, ptr, i32}, true},
		{"mapassign_fast32ptr", ptr, []llvm.Type{ptr, ptr, ptr}, true},
		{"mapassign_fast64", ptr, []llvm.Type{ptr, ptr, i64}, true},
		{"mapassign_fast64ptr", ptr, []llvm.Type{ptr, ptr, ptr}, true},
		{"mapassign_faststr", ptr, []llvm.Type{ptr, ptr, str}, true},
		// The fat helper returns its caller-supplied zero pointer on a miss.
		{"mapaccess1_fat", ptr, []llvm.Type{ptr, ptr, ptr, ptr}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, modeled := range []bool{false, true} {
				func() {
					old := CurrentModule
					m := GlobalCtxt.NewModule("container_results")
					CurrentModule = m
					defer func() { CurrentModule = old; m.Dispose() }()
					sig := llvmFuncSignature{Type: llvm.FunctionType(tc.result, tc.params, false), ClosureContextIndex: -1}
					name := "runtime." + tc.name
					var callee llvm.Value
					if modeled {
						callee = getOrInsertLLVMFunction(name, sig, goABIInternalCallConv)
					} else {
						callee = llvm.AddFunction(m, name, sig.Type)
						callee.SetFunctionCallConv(goABIInternalCallConv)
					}
					fn := llvm.AddFunction(m, "probe", llvm.FunctionType(GlobalCtxt.Int1Type(), tc.params, false))
					b := GlobalCtxt.NewBuilder()
					defer b.Dispose()
					b.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
					call := b.CreateCall(sig.Type, callee, fn.Params(), "value")
					call.SetInstructionCallConv(goABIInternalCallConv)
					bad := b.CreateICmp(llvm.IntEQ, call, llvm.ConstNull(ptr), "bad")
					b.CreateRet(bad)
					opts := llvm.NewPassBuilderOptions()
					defer opts.Dispose()
					opts.SetVerifyEach(true)
					if err := m.RunPasses("function(instcombine,simplifycfg)", llvm.TargetMachine{}, opts); err != nil {
						t.Fatal(err)
					}
					if got := strings.Contains(fn.String(), "ret i1 false"); got != (modeled && tc.known) {
						t.Fatalf("modeled=%v: unexpected predicate\n%s", modeled, m.String())
					}
					if !strings.Contains(fn.String(), "call goabiinternal") {
						t.Fatalf("effectful call removed: %s", fn.String())
					}
					if modeled {
						for _, attr := range []string{"memory", "willreturn", "nosync"} {
							if callee.GetEnumFunctionAttribute(llvm.AttributeKindID(attr)).C != nil {
								t.Fatalf("unexpected %s", attr)
							}
						}
						if callee.GetEnumAttributeAtIndex(0, llvm.AttributeKindID("noalias")).C != nil {
							t.Fatal("shared result marked noalias")
						}
						abi0 := getOrInsertLLVMFunction(name+".abi0", llvmFuncSignature{Type: llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{ptr, ptr, ptr, ptr}, false), ClosureContextIndex: -1}, goABI0CallConv)
						for _, attr := range []string{"nonnull", "range"} {
							if abi0.GetEnumAttributeAtIndex(0, llvm.AttributeKindID(attr)).C != nil {
								t.Fatalf("ABI0 has %s", attr)
							}
						}
					}
					if err := llvm.VerifyModule(m, llvm.ReturnStatusAction); err != nil {
						t.Fatal(err)
					}
				}()
			}
		})
	}
}
