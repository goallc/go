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

func TestLLVMScaledPointerOffset(t *testing.T) {
	for _, test := range []struct {
		name    string
		count   int64
		narrow  bool
		dynamic bool
		shared  bool
		scale   uint64
	}{
		{name: "two_bytes", count: 1, scale: 2},
		{name: "sixteen_bytes", count: 4, scale: 16},
		{name: "large_element", count: 31, scale: 1 << 31},
		{name: "zero_shift", count: 0},
		{name: "array_length_overflow", count: 32},
		{name: "overshift", count: 64},
		{name: "unsigned_large_count", count: -1},
		{name: "narrow_offset_wraps_first", count: 4, narrow: true},
		{name: "dynamic_count", dynamic: true},
		{name: "shared_offset", count: 4, shared: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := GlobalCtxt.NewModule(test.name)
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)
			pointer := GlobalCtxt.PointerType(0)
			integer := GlobalCtxt.Int64Type()
			goInteger := types.Types[types.TUINT64]
			shiftOp := OpLsh64x64
			if test.narrow {
				integer = GlobalCtxt.Int32Type()
				goInteger = types.Types[types.TUINT32]
				shiftOp = OpLsh32x64
			}
			fn := llvm.AddFunction(module, "address", llvm.FunctionType(pointer, []llvm.Type{pointer, integer, GlobalCtxt.Int64Type(), pointer}, false))
			entryLLVM := llvm.AddBasicBlock(fn, "entry")
			entry := &Block{ID: 1, Kind: BlockRet}
			base := &Value{ID: 1, Type: types.Types[types.TUNSAFEPTR]}
			index := &Value{ID: 2, Type: goInteger}
			count := &Value{ID: 3, Op: OpConst64, Type: types.Types[types.TUINT64], AuxInt: test.count}
			shift := &Value{ID: 4, Op: shiftOp, Type: goInteger, Block: entry, Args: []*Value{index, count}, Uses: 1}
			address := &Value{ID: 5, Op: OpAddPtr, Type: base.Type, Block: entry, Args: []*Value{base, shift}}
			context := &LLVMFuncContext{
				BBs: map[ID]llvm.BasicBlock{entry.ID: entryLLVM},
				Vs:  map[ID]llvm.Value{base.ID: fn.Param(0), index.ID: fn.Param(1)},
				LF:  fn, b: builder, ResultCount: 1, ReturnCount: 1,
			}
			if test.dynamic {
				count.Op = OpArg
				context.Vs[count.ID] = fn.Param(2)
			}
			if test.shared {
				shift.Uses++
			}
			entry.Controls[0] = address
			// Normal ordered emission may have already materialized the shift.
			// Leave it available for debug values and let LLVM remove it if dead.
			context.CompileBlock(entry, []*Value{shift, address})
			if test.shared {
				builder.SetInsertPointBefore(entryLLVM.LastInstruction())
				builder.CreateStore(context.Vs[shift.ID], fn.Param(3))
			}
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("invalid address IR: %v\n%s", err, module.String())
			}
			got := context.Vs[address.ID].String()
			want := "getelementptr i8,"
			if test.scale != 0 {
				want = fmt.Sprintf("getelementptr [%d x i8],", test.scale)
			}
			if !strings.Contains(got, want) || strings.Contains(got, "inbounds") {
				t.Fatalf("address = %s, want %s without inbounds", got, want)
			}
			if test.scale != 0 {
				options := llvm.NewPassBuilderOptions()
				defer options.Dispose()
				options.SetVerifyEach(true)
				if err := module.RunPasses("function(instcombine)", llvm.TargetMachine{}, options); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(module.String(), " shl ") || !strings.Contains(module.String(), want) {
					t.Fatalf("scaled address did not eliminate the dead shift\n%s", module.String())
				}
			}
		})
	}
}
