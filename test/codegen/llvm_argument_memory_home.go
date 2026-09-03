// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmArgumentStrings3 struct {
	a, b, c string
}

type llvmArgumentStringArray [2]string

// The slice move-to-heap check compares a pointer against the active frame and
// is a real SP value use. Keep materializing stackaddress for that path even
// though LocalAddr-only functions below do not need it.
//
// LLVM-LABEL: define goabiinternal { ptr, i64, i64 } @codegen.llvmMoveToHeapUsesStackAddress(i64 %n)
// LLVM-NOT: @llvm.stackaddress.p0()
// LLVM: call goabiinternal void @codegen.llvmStackAddressMayGrow(
// LLVM: [[FRAME_SP:%.*]] = call ptr @llvm.stackaddress.p0()
// LLVM-NEXT: {{%.*}} = call i64 @llvm.go.pointer.address.i64.p0(ptr [[FRAME_SP]])
// LLVM: [[POINTER_SP:%.*]] = call ptr @llvm.stackaddress.p0()
// LLVM-NEXT: {{%.*}} = call i64 @llvm.go.pointer.address.i64.p0(ptr [[POINTER_SP]])
// LLVM-OPT-LABEL: define goabiinternal { ptr, i64, i64 } @codegen.llvmMoveToHeapUsesStackAddress(i64 %n)
// LLVM-OPT-NOT: @llvm.stackaddress.p0()
// LLVM-OPT: call goabiinternal void @codegen.llvmStackAddressMayGrow(
// LLVM-OPT: [[FRAME_SP_OPT:%.*]] = {{.*}}call ptr @llvm.stackaddress.p0()
// LLVM-OPT-NEXT: {{%.*}} = {{.*}}call i64 @llvm.go.pointer.address.i64.p0(ptr [[FRAME_SP_OPT]])
// LLVM-OPT: [[POINTER_SP_OPT:%.*]] = {{.*}}call ptr @llvm.stackaddress.p0()
// LLVM-OPT-NEXT: {{%.*}} = {{.*}}call i64 @llvm.go.pointer.address.i64.p0(ptr [[POINTER_SP_OPT]])

// llvmArgumentStrings3 fits wholly in the ABIInternal integer-register budget
// but is too large for Go SSA's aggregate-value limit. LLVM gives only this
// memory-backed parameter a complete local home instead of reconstructing its
// individual ABI register pieces from the aggregate formal parameter.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %x)
// LLVM-NOT: llvm.stackaddress
// LLVM: [[HOME:%.*]] = alloca %codegen.llvmArgumentStrings3, align 8
// LLVM: store %codegen.llvmArgumentStrings3 %x, ptr [[HOME]], align 8
// LLVM: load ptr, ptr [[HOME]], align 8
// LLVM-NOT: .arg
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmRegisterArgumentMemoryHome(%codegen.llvmArgumentStrings3 %x)
// LLVM-OPT-NOT: llvm.stackaddress
// LLVM-OPT-NOT: alloca
// LLVM-OPT-NOT: load
// LLVM-OPT-NOT: store
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 0, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 1, 1
// LLVM-OPT: extractvalue %codegen.llvmArgumentStrings3 %x, 2, 1
// LLVM-OPT: ret i64
func llvmRegisterArgumentMemoryHome(x llvmArgumentStrings3) int {
	return len(x.a) + len(x.b) + len(x.c)
}

// Non-trivial arrays are assigned wholly to the ABI stack. Typed byval exposes
// that incoming Go parameter copy directly, so LocalAddr needs no second local
// home or aggregate reconstruction.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr byval([2 x { ptr, i64 }]) align 8 %x)
// LLVM-NOT: alloca
// LLVM: getelementptr i8, ptr %x, i64 0
// LLVM: getelementptr i8, ptr %x, i64 16
// LLVM: load { ptr, i64 }
// LLVM: load { ptr, i64 }
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr{{.*}}byval([2 x { ptr, i64 }]) align 8{{.*}} %x)
// LLVM-OPT-NOT: alloca
// LLVM-OPT: getelementptr {{.*}}ptr %x, i64 8
// LLVM-OPT: load i64
// LLVM-OPT: getelementptr {{.*}}ptr %x, i64 24
// LLVM-OPT: load i64
// LLVM-OPT: ret i64
//
//go:noinline
func llvmStackArgumentMemoryHome(x llvmArgumentStringArray) int {
	return len(x[0]) + len(x[1])
}

// A stack-assigned value that already resides in memory is the byval source
// directly. The frontend must not load the complete aggregate and materialize
// a second temporary object before the call.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmForwardStackArgumentMemory(ptr byval([2 x { ptr, i64 }]) align 8 %x)
// LLVM-NOT: alloca
// LLVM: call goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr byval([2 x { ptr, i64 }]) align 8 %x)
// LLVM: ret i64
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmForwardStackArgumentMemory(ptr{{.*}}byval([2 x { ptr, i64 }]) align 8{{.*}} %x)
// LLVM-OPT-NOT: alloca
// LLVM-OPT: call goabiinternal i64 @codegen.llvmStackArgumentMemoryHome(ptr{{.*}}byval([2 x { ptr, i64 }]) align 8{{.*}} %x)
// LLVM-OPT: ret i64
//
//go:noinline
func llvmForwardStackArgumentMemory(x llvmArgumentStringArray) int {
	return llvmStackArgumentMemoryHome(x)
}

// A register-assigned parameter that Go SSA can use directly remains an LLVM
// SSA value and does not acquire a memory home.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmDirectRegisterArgument(i64 %x)
// LLVM-NOT: alloca
// LLVM: add i64 %x, 1
// LLVM: ret i64
func llvmDirectRegisterArgument(x int) int {
	return x + 1
}

func llvmMoveToHeapUsesStackAddress(n int) []int {
	var values []int
	for i := 0; i < n; i++ {
		values = append(values, i)
	}
	llvmStackAddressMayGrow(&n)
	return values
}

//go:noescape
func llvmStackAddressMayGrow(*int)
