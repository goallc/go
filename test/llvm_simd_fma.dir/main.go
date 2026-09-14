// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/big"
	"os"
)

type float interface{ float32 | float64 }

var enabledCases, fallbackCases int

// Use exact rational arithmetic for finite operands to avoid double rounding
// when using float64 FMA as an oracle for float32.
func reference[T float](x, y, z T) T {
	a, b, c := float64(x), float64(y), float64(z)
	if math.IsNaN(a) || math.IsNaN(b) || math.IsNaN(c) || math.IsInf(a, 0) || math.IsInf(b, 0) || math.IsInf(c, 0) {
		return T(math.FMA(a, b, c))
	}
	r := new(big.Rat).Mul(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b))
	r.Add(r, new(big.Rat).SetFloat64(c))
	if r.Sign() == 0 {
		return T(math.FMA(a, b, c)) // Preserve the sign of an exact zero.
	}
	if _, ok := any(x).(float32); ok {
		v, _ := r.Float32()
		return T(v)
	}
	v, _ := r.Float64()
	return T(v)
}

func check[T float](name string, n, modes int, enabled bool, run func(x, y, z, out []T)) {
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
	data := [][3]float64{
		{2, 3, 1}, {-2, 3, -1},
		{1 + 0x1p-13, 1 - 0x1p-13, -1},
		{1 + 0x1p-27, 1 - 0x1p-27, -1},
		{math.MaxFloat32, 2, -math.MaxFloat32},
		{math.MaxFloat64, 2, -math.MaxFloat64},
		{math.SmallestNonzeroFloat32, 0.5, 0},
		{math.SmallestNonzeroFloat64, 0.5, 0},
		{math.SmallestNonzeroFloat32, 0.5, math.SmallestNonzeroFloat32},
		{math.SmallestNonzeroFloat64, 0.5, math.SmallestNonzeroFloat64},
		{0, -1, math.Copysign(0, -1)}, {0, 1, 0},
		{math.Inf(1), 1, 2}, {math.Inf(1), 0, 1},
		{math.Inf(1), 1, math.Inf(-1)}, {math.NaN(), 1, 2},
	}
	x, y, z, out := make([]T, n), make([]T, n), make([]T, n), make([]T, n*modes)
	for k := range data {
		for i := range x {
			d := data[(k+i)%len(data)]
			x[i], y[i], z[i] = T(d[0]), T(d[1]), T(d[2])
		}
		run(x, y, z, out)
		for mode := 0; mode < modes; mode++ {
			for i := range x {
				addend := z[i]
				if mode == 1 && i%2 != 0 || mode == 2 && i%2 == 0 {
					addend = -addend
				}
				var want T
				if enabled {
					want = reference(x[i], y[i], addend)
				}
				got := out[mode*n+i]
				if math.IsNaN(float64(got)) && math.IsNaN(float64(want)) {
					continue
				}
				if math.Float64bits(float64(got)) != math.Float64bits(float64(want)) {
					panic(fmt.Sprintf("%s mode=%d lane=%d x=%v y=%v z=%v got=%v want=%v", name, mode, i, x[i], y[i], z[i], got, want))
				}
			}
		}
	}
}

func main() {
	checkArch()
	if os.Getenv("GOALLC_SIMD_FMA_TRACE") == "1" {
		fmt.Printf("fma: enabled=%d fallback=%d\n", enabledCases, fallbackCases)
	}
}
