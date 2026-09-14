// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"os"
	"simd/archsimd"
)

func check[T float32 | float64](name string, lanes int, fn func([]T, []T, []T)) {
	values := []float64{0, math.Copysign(0, -1), 1, -1, 1.5, -2.75, math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat64, math.MaxFloat32, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	exponents := []float64{-2048, -150, -149.5, -3.5, -0.5, 0, 0.5, 2, 3.5, 128, 1024, 2048}
	for start := range values {
		for _, exponent := range exponents {
			x, y, out := make([]T, lanes), make([]T, lanes), make([]T, lanes)
			for i := range x {
				x[i] = T(values[(start+i)%len(values)])
				y[i] = T(exponent + float64(i%3)/4)
			}
			fn(x, y, out)
			for i, got := range out {
				var want T
				if archsimd.X86.AVX512() {
					want = T(math.Ldexp(float64(x[i]), int(math.Floor(float64(y[i])))))
				}
				g, w := float64(got), float64(want)
				if math.IsNaN(g) && math.IsNaN(w) {
					continue
				}
				if math.Float64bits(g) != math.Float64bits(w) {
					panic(fmt.Sprintf("%s lane=%d x=%g y=%g got=%g want=%g", name, i, x[i], y[i], got, want))
				}
			}
		}
	}
}

func main() {
	check("f32x4", 4, scale32x4)
	check("f32x8", 8, scale32x8)
	check("f32x16", 16, scale32x16)
	check("f64x2", 2, scale64x2)
	check("f64x4", 4, scale64x4)
	check("f64x8", 8, scale64x8)
	if os.Getenv("GOALLC_SIMD_SCALE_TRACE") != "" {
		fmt.Printf("AVX512=%v\n", archsimd.X86.AVX512())
	}
}
