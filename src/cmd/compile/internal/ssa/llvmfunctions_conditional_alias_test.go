// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"fmt"
	"internal/abi"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMFunctionConditionalAlias(t *testing.T) {
	ptr, i8, i64 := GlobalCtxt.PointerType(0), GlobalCtxt.Int8Type(), GlobalCtxt.Int64Type()
	type scenario struct {
		name, mode  string
		size, count int64
		fresh       bool
	}
	var cases []scenario
	for _, name := range []string{"makemap", "makemap64"} {
		cases = append(cases, scenario{name: name, mode: "nil", fresh: true}, scenario{name: name, mode: "supplied"})
	}
	for _, name := range []string{"makeslice", "makeslice64", "makeslicecopy"} {
		for _, s := range []scenario{{mode: "positive", size: 8, count: 4, fresh: true}, {mode: "zero-element", size: 0, count: 4}, {mode: "zero-count", size: 8, count: 0}, {mode: "negative", size: 8, count: -1}, {mode: "overflow", size: 8, count: 1 << 62}, {mode: "target32-overflow", size: 8, count: 1 << 29}, {mode: "dynamic-count", size: 8}, {mode: "dynamic-type", size: -1, count: 4}} {
			s.name = name
			cases = append(cases, s)
		}
	}
	for _, name := range []string{"convT16", "convT32", "convT64"} {
		for _, v := range []int64{0, abi.StaticUint64sCount - 1, abi.StaticUint64sCount, abi.StaticUint64sCount + 1} {
			cases = append(cases, scenario{name: name, mode: "constant", count: v, fresh: v >= abi.StaticUint64sCount})
		}
		cases = append(cases, scenario{name: name, mode: "dynamic"})
	}
	cases = append(cases, scenario{name: "convTstring", mode: "literal", count: 1, fresh: true}, scenario{name: "convTstring", mode: "literal", count: 0}, scenario{name: "convTstring", mode: "made", count: 3, fresh: true}, scenario{name: "convTstring", mode: "dynamic"}, scenario{name: "convTslice", mode: "global", fresh: true}, scenario{name: "convTslice", mode: "local", fresh: true}, scenario{name: "convTslice", mode: "nil-positive-length"}, scenario{name: "convTslice", mode: "dynamic"})
	for _, tc := range cases {
		for _, modeled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/%d/model=%v", tc.name, tc.mode, tc.count, modeled), func(t *testing.T) {
				old := CurrentModule
				oldPtrSize := types.PtrSize
				if tc.mode == "target32-overflow" {
					types.PtrSize = 4
				}
				defer func() { types.PtrSize = oldPtrSize }()
				m := GlobalCtxt.NewModule("conditional_alias")
				CurrentModule = m
				defer func() { CurrentModule = old; m.Dispose() }()
				var params []llvm.Type
				switch tc.name {
				case "makemap", "makemap64":
					params = []llvm.Type{ptr, i64, ptr}
				case "makeslice", "makeslice64":
					params = []llvm.Type{ptr, i64, i64}
				case "makeslicecopy":
					params = []llvm.Type{ptr, i64, i64, ptr}
				case "convT16":
					params = []llvm.Type{GlobalCtxt.Int16Type()}
				case "convT32":
					params = []llvm.Type{GlobalCtxt.Int32Type()}
				case "convT64":
					params = []llvm.Type{i64}
				case "convTstring":
					params = []llvm.Type{llvm.StructType([]llvm.Type{ptr, i64}, false)}
				case "convTslice":
					params = []llvm.Type{llvm.StructType([]llvm.Type{ptr, i64, i64}, false)}
				}
				sig := llvmFuncSignature{Type: llvm.FunctionType(ptr, params, false), ClosureContextIndex: -1}
				callee := getOrInsertLLVMFunction("runtime."+tc.name, sig, goABIInternalCallConv)
				fn := llvm.AddFunction(m, "probe", llvm.FunctionType(i8, append([]llvm.Type{ptr}, params...), false))
				b := GlobalCtxt.NewBuilder()
				defer b.Dispose()
				b.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
				args := fn.Params()[1:]
				source := &Value{Args: []*Value{{Op: OpArg}}}
				switch tc.name {
				case "makemap", "makemap64":
					if tc.mode == "nil" {
						args[2] = llvm.ConstNull(ptr)
					}
				case "makeslice", "makeslice64", "makeslicecopy":
					if tc.size >= 0 {
						typ := types.NewArray(types.Types[types.TUINT8], tc.size)
						types.CalcSize(typ)
						sym := &obj.LSym{Name: "type:conditional-alias"}
						sym.NewTypeInfo().Type = typ
						source.Args[0] = &Value{Op: OpAddr, Aux: sym}
					}
					index := 2
					if tc.name == "makeslicecopy" {
						index = 1
					}
					if tc.mode != "dynamic-count" {
						args[index] = llvm.ConstInt(i64, uint64(tc.count), true)
					}
				case "convT16", "convT32", "convT64":
					if tc.mode == "constant" {
						args[0] = llvm.ConstInt(params[0], uint64(tc.count), false)
					}
				case "convTstring":
					if tc.mode == "literal" {
						s := ""
						if tc.count > 0 {
							s = "x"
						}
						source.Args[0] = &Value{Op: OpConstString, Aux: StringToAux(s)}
					}
					if tc.mode == "made" {
						source.Args[0] = &Value{Op: OpStringMake, Args: []*Value{{Op: OpArg}, {Op: OpConst64, AuxInt: tc.count}}}
					}
				case "convTslice":
					op := OpConstNil
					if tc.mode == "global" {
						op = OpAddr
					}
					if tc.mode == "local" {
						op = OpLocalAddr
					}
					if tc.mode != "dynamic" {
						source.Args[0] = &Value{Op: OpSliceMake, Args: []*Value{{Op: op}, {Op: OpConst64, AuxInt: 1}, {Op: OpConst64, AuxInt: 1}}}
					}
				}
				call := b.CreateCall(sig.Type, callee, args, "result")
				call.SetInstructionCallConv(goABIInternalCallConv)
				if modeled {
					llvmFunctions.bindCall(call, callee, args, goABIInternalCallConv, source)
				}
				if got := call.GetCallSiteEnumAttribute(0, llvm.AttributeKindID("noalias")).C != nil; got != (modeled && tc.fresh) {
					t.Fatalf("noalias=%v want=%v", got, modeled && tc.fresh)
				}
				if callee.GetEnumAttributeAtIndex(0, llvm.AttributeKindID("noalias")).C != nil {
					t.Fatal("call-specific attribute leaked")
				}
				if call.GetCallSiteEnumAttribute(llvmAttributeFunctionIndex, llvm.AttributeKindID("allockind")).C != nil {
					t.Fatal("unexpected allocation elision contract")
				}
				// Test alias consequences on fresh cases, with an identical unannotated
				// control. Do not write into the shared read-only scalar cache in negatives.
				if tc.fresh {
					b.CreateStore(llvm.ConstInt(i8, 17, false), fn.Param(0))
					b.CreateStore(llvm.ConstInt(i8, 29, false), call)
					b.CreateRet(b.CreateLoad(i8, fn.Param(0), "old"))
				} else {
					b.CreateRet(llvm.ConstInt(i8, 0, false))
				}
				opts := llvm.NewPassBuilderOptions()
				defer opts.Dispose()
				opts.SetVerifyEach(true)
				if err := m.RunPasses("function(instcombine,early-cse<memssa>,gvn)", llvm.TargetMachine{}, opts); err != nil {
					t.Fatal(err)
				}
				if tc.fresh {
					if got := strings.Contains(fn.String(), "ret i8 17"); got != modeled {
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
