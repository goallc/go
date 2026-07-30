// asmcheck

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-DAG: @codegen.wsp = global <{ [256 x i8] }> {{.*}}, section ".noptrdata"
// LLVM-DAG: define goabiinternal i8 @codegen.zeroExtArgUint16([2 x i16]
// LLVM-DAG: zext i16 {{%.*}} to i64
// LLVM-DAG: icmp ult i64 {{%.*}}, 256
// LLVM-DAG: getelementptr i8, ptr @codegen.wsp, i64
// LLVM-DAG: define goabiinternal i8 @codegen.zeroExtArgByte([2 x i8]
// LLVM-DAG: zext i8 {{%.*}} to i64

var wsp = [256]bool{
	' ':  true,
	'\t': true,
	'\n': true,
	'\r': true,
}

func zeroExtArgByte(ch [2]byte) bool {
	return wsp[ch[0]] // amd64:-"MOVBLZX ..,.."
}

func zeroExtArgUint16(ch [2]uint16) bool {
	return wsp[ch[0]] // amd64:-"MOVWLZX ..,.."
}
