// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.llvmDeferNoReturn(i64 %count)
// LLVM-NOT: !goallc.open_defer_slots
// LLVM: call goabiinternal void @runtime.deferproc(
// LLVM: callbr void @llvm.go.defer.edge()
// LLVM-NEXT: to label %{{.*}} [label %[[RECOVERY:[A-Za-z0-9_.]+]]]
// LLVM: [[RECOVERY]]:
// LLVM-NEXT: call goabiinternal void @runtime.deferreturn()
// LLVM-OPT-LABEL: define goabiinternal void @codegen.llvmDeferNoReturn(i64 %count)
// LLVM-OPT-NOT: !goallc.open_defer_slots
// LLVM-OPT: call goabiinternal void @runtime.deferproc(
// LLVM-OPT: callbr void @llvm.go.defer.edge()
// LLVM-OPT-NEXT: to label %{{.*}} [label %[[RECOVERY_OPT:[A-Za-z0-9_.]+]]]
// LLVM-OPT: [[RECOVERY_OPT]]:
// LLVM-OPT-NEXT: call goabiinternal void @runtime.deferreturn()

func llvmDeferNoReturn(count int) {
	for i := 0; i < count; i++ {
		defer func() {}()
	}
	panic(count)
}
