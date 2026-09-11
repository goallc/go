//go:build amd64 || arm64

// run -goexperiment simd -godebug simd=+128 -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"os"
	"simd"
)

//go:noinline
func portableIntToFloat(x simd.Int32s) simd.Float32s {
	return x.ConvertToFloat32()
}

//go:noinline
func portableFloatToInt(x simd.Float32s) simd.Int32s {
	return x.ConvertToInt32()
}

func main() {
	ints := []int32{0, 1, -1, 1<<24 - 1, 1<<24 + 1, -(1<<24 + 1), 1<<24 + 3, 1<<31 - 1, -1 << 31}
	// Keep float-to-int inputs in range: scalar emulation need not have the
	// architecture-specific overflow semantics tested by the archsimd fixture.
	floats := []float32{0, math.Float32frombits(1 << 31), 1.75, -1.75, 123456.5, -123456.5, math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, 2147483520, -2147483648}
	lanes := simd.Int32s{}.Len()
	x, y := make([]int32, lanes), make([]float32, lanes)
	gotInt, gotFloat := make([]int32, lanes), make([]float32, lanes)
	for offset := range 16 {
		for i := range lanes {
			x[i], y[i] = ints[(i+offset)%len(ints)], floats[(i+offset)%len(floats)]
		}
		portableIntToFloat(simd.LoadInt32s(x)).Store(gotFloat)
		portableFloatToInt(simd.LoadFloat32s(y)).Store(gotInt)
		for i := range lanes {
			if math.Float32bits(gotFloat[i]) != math.Float32bits(float32(x[i])) || gotInt[i] != int32(y[i]) {
				panic(fmt.Sprintf("portable conversion offset=%d lane=%d: float=%g int=%d", offset, i, gotFloat[i], gotInt[i]))
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_FLOAT_TRACE") == "1" {
		fmt.Printf("portable floating conversions: bits=%d emulated=%t\n", simd.VectorBitSize(), simd.Emulated())
	}
}
