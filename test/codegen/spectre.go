// asmcheck -gcflags=-spectre=index

//go:build amd64

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

func IndexArray(x *[10]int, i int) int {
	// amd64:`CMOVQCC`
	// LLVM-OPT-DAG: define {{.*}} @codegen.IndexArray(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaeq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.IndexArray(SB)
	// LLVM-ASM-DAG: CMOVAE
	return x[i]
}

func IndexString(x string, i int) byte {
	// amd64:`CMOVQ(LS|CC)`
	// LLVM-OPT-DAG: define {{.*}} @codegen.IndexString(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaeq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.IndexString(SB)
	// LLVM-ASM-DAG: CMOVAE
	return x[i]
}

func IndexSlice(x []float64, i int) float64 {
	// amd64:`CMOVQ(LS|CC)`
	// LLVM-OPT-DAG: define {{.*}} @codegen.IndexSlice(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaeq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.IndexSlice(SB)
	// LLVM-ASM-DAG: CMOVAE
	return x[i]
}

func SliceArray(x *[10]int, i, j int) []int {
	// amd64:`CMOVQHI`
	// LLVM-OPT-DAG: define {{.*}} @codegen.SliceArray(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.SliceArray(SB)
	// LLVM-ASM-DAG: CMOVA {{[A-Z0-9]+}},
	return x[i:j]
}

func SliceString(x string, i, j int) string {
	// amd64:`CMOVQHI`
	// LLVM-OPT-DAG: define {{.*}} @codegen.SliceString(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.SliceString(SB)
	// LLVM-ASM-DAG: CMOVA {{[A-Z0-9]+}},
	return x[i:j]
}

func SliceSlice(x []float64, i, j int) []float64 {
	// amd64:`CMOVQHI`
	// LLVM-OPT-DAG: define {{.*}} @codegen.SliceSlice(
	// LLVM-OPT-DAG: asm "cmpq $2, $0; cmovaq $3, $0"
	// LLVM-ASM-DAG: TEXT codegen.SliceSlice(SB)
	// LLVM-ASM-DAG: CMOVA {{[A-Z0-9]+}},
	return x[i:j]
}
