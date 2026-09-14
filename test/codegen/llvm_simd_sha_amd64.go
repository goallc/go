// asmcheck

//go:build goexperiment.simd && amd64

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "simd/archsimd"

// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha1rnds4({{.*}}i8 3)
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha1nexte(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha1msg1(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha1msg2(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha256rnds2(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha256msg1(
// LLVM-AMD64-DAG: call <4 x i32> @llvm.x86.sha256msg2(
// LLVM-AMD64-DAG: !{!"x86.sha"}
// LLVM-OPT-AMD64-DAG: define internal {{.*}} @"codegen.shaOps<goallc.fmv.sha>"({{.*}}) [[SHAATTR:#[0-9]+]]
// LLVM-OPT-AMD64-DAG: attributes [[SHAATTR]] = { {{.*}}"target-features"="{{[^"]*}}+sha{{[^"]*}}"
// LLVM-ASM-AMD64-DAG: SHA1RNDS4
// LLVM-ASM-AMD64-DAG: SHA1NEXTE
// LLVM-ASM-AMD64-DAG: SHA1MSG1
// LLVM-ASM-AMD64-DAG: SHA1MSG2
// LLVM-ASM-AMD64-DAG: SHA256RNDS2
// LLVM-ASM-AMD64-DAG: SHA256MSG1
// LLVM-ASM-AMD64-DAG: SHA256MSG2
//
//go:noinline
func shaOps(x, y, z archsimd.Uint32x4) archsimd.Uint32x4 {
	if !archsimd.X86.SHA() {
		return x
	}
	x = x.SHA1FourRounds(3, y)
	x = x.SHA1NextE(y)
	x = x.SHA1Message1(y)
	x = x.SHA1Message2(y)
	x = x.SHA256TwoRounds(y, z)
	x = x.SHA256Message1(y)
	return x.SHA256Message2(y)
}
