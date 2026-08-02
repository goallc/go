// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmSwitchInterface interface {
	Value(int) int
}

type llvmDoubleSwitchInterface interface {
	Double() int
}

type llvmSwitchValue int
type llvmDoubleSwitchValue int

//go:noinline
func (v llvmSwitchValue) Value(delta int) int {
	return int(v) + delta
}

func (v llvmDoubleSwitchValue) Double() int {
	return int(v) * 2
}

// LLVM-DAG: @codegen..interfaceSwitch.0 = internal global <{ ptr, [8 x i8], ptr, ptr }>
// LLVM-DAG: load atomic ptr, ptr @codegen..interfaceSwitch.0 seq_cst
// LLVM-DAG: call goabiinternal { i64, ptr } @runtime.interfaceSwitch(ptr @codegen..interfaceSwitch.0, ptr
// LLVM-DAG: declare !goobj.builtin !{{[0-9]+}} goabiinternal { i64, ptr } @runtime.interfaceSwitch(ptr, ptr)
// LLVM-DAG: icmp eq ptr
// LLVM-DAG: @"type:codegen.llvmSwitchValue"
// LLVM-DAG: @"type:string"
// LLVM-DAG: ret i64 -1
// LLVM: !goobj.gotype = !{
// LLVM-DAG: !{ptr @codegen..interfaceSwitch.0, ptr @
func classifyLLVMInterface(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case llvmSwitchValue:
		return int(x)
	case string:
		return len(x)
	case llvmSwitchInterface:
		return x.Value(1)
	case llvmDoubleSwitchInterface:
		return x.Double()
	default:
		return -1
	}
}
