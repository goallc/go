// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// llvmExplicitNilcheckPhi makes a nilcheck continuation the predecessor of a
// Go SSA join block. LLVM verification therefore also checks the rewritten
// predecessor used by the join phi.
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmExplicitNilcheckPhi(ptr %p, i8 %take)
// LLVM-NOT: llvm.goallc.nilcheck
// LLVM: phi i64
// LLVM: icmp eq ptr %p, null
// LLVM: br i1 {{%.*}}, label {{%.*}}, label {{%.*}}, {{.*}}!make.implicit [[GO_NILCHECK:![0-9]+]]
// LLVM: call goabiinternal void @runtime.panicmem()
// LLVM-NEXT: unreachable
// LLVM: declare goabiinternal void @runtime.panicmem()
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmExplicitNilcheckTwice(ptr %p, ptr %q)
// LLVM-NOT: llvm.goallc.nilcheck
// LLVM: icmp eq ptr %p, null
// LLVM: call goabiinternal void @runtime.panicmem()
// LLVM-NEXT: unreachable
// LLVM: [[FIRST_CONT:nilcheck\.notnil[0-9]*]]:
// LLVM-NEXT: icmp eq ptr %q, null
// LLVM: call goabiinternal void @runtime.panicmem()
// LLVM-NEXT: unreachable
// LLVM: [[SECOND_CONT:nilcheck\.notnil[0-9]*]]:
// LLVM: load i64, ptr %p
// LLVM: load i64, ptr %q
//
// LLVM-LABEL: define goabiinternal i64 @codegen.llvmExplicitNilcheck(ptr %p)
// LLVM-NOT: llvm.goallc.nilcheck
// LLVM: [[ISNIL:%.*]] = icmp eq ptr %p, null
// LLVM-NEXT: br i1 [[ISNIL]], label %[[NIL:.*\.nil]], label %[[CONT:.*\.notnil]], {{.*}}!make.implicit [[GO_NILCHECK]]
// LLVM: [[NIL]]:
// LLVM-NEXT: call goabiinternal void @runtime.panicmem()
// LLVM-NEXT: unreachable
// LLVM: [[CONT]]:
// LLVM-NOT: load volatile i8
// LLVM-NOT: !goallc.nilcheck
// LLVM-NOT: !annotation
// LLVM: load i64, ptr %p
// LLVM-NOT: "gc-leaf-function"
// LLVM-NOT: llvm.goallc.nilcheck
//
// LLVM-OPT-LABEL: define goabiinternal i64 @codegen.llvmExplicitNilcheck(
// LLVM-OPT-NOT: llvm.goallc.nilcheck
// LLVM-OPT: icmp eq ptr %p, null
// LLVM-OPT: br i1 {{%.*}}, label %[[OPTNIL:[^,]+]], label %[[OPTCONT:[^,]+]], {{.*}}!make.implicit [[GO_NILCHECK_OPT:![0-9]+]]
// LLVM-OPT: [[OPTNIL]]:
// LLVM-OPT-NEXT: call goabiinternal void @runtime.panicmem()
// LLVM-OPT-NEXT: unreachable
// LLVM-OPT: [[OPTCONT]]:
// LLVM-OPT-NOT: load volatile i8
// LLVM-OPT-NOT: !goallc.nilcheck
// LLVM-OPT-NOT: !annotation
// LLVM-OPT: load i64, ptr %p
// LLVM-OPT-NOT: llvm.goallc.nilcheck
// LLVM: [[GO_NILCHECK]] = !{!"goallc"}
// LLVM-OPT: [[GO_NILCHECK_OPT]] = !{!"goallc"}
func llvmExplicitNilcheck(p *int) int {
	return *p
}

func llvmExplicitNilcheckTwice(p, q *int) int {
	return *p + *q
}

func llvmExplicitNilcheckPhi(p *int, take bool) int {
	value := 7
	if take {
		value = *p
	}
	return value
}
