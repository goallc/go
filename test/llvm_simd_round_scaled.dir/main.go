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

func check[T float32 | float64](name string, lanes int, fn func([]T, [4][]T, uint8)) {
	values := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 1.5, -2.5, 1.25, -1.75, 0.1, -0.1, 0x1.0001p0, -0x1.0001p0, math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, math.MaxFloat32, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	rounders := []func(float64) float64{math.RoundToEven, math.Floor, math.Ceil, math.Trunc}
	for start := range values {
		for _, prec := range []uint8{0, 1, 2, 7, 8, 15, 16, 17, 127, 128, 255} {
			x := make([]T, lanes)
			out := [4][]T{}
			for op := range out {
				out[op] = make([]T, lanes)
			}
			for i := range x {
				x[i] = T(values[(start+i)%len(values)])
			}
			fn(x, out, prec)
			for op, result := range out {
				for i, got := range result {
					want := float64(0)
					if archsimd.X86.AVX512() {
						want = float64(x[i])
						// Large inputs are already integral. Avoid an overflowing
						// reference intermediate: RNDSCALE uses an unlimited exponent.
						if math.Abs(want) < 0x1p52 {
							p := int(prec & 15)
							want = math.Ldexp(rounders[op](math.Ldexp(want, p)), -p)
						}
					}
					g := float64(got)
					if math.IsNaN(g) && math.IsNaN(want) {
						continue
					}
					if math.Float64bits(g) != math.Float64bits(want) {
						panic(fmt.Sprintf("%s op=%d prec=%d x=%g got=%g want=%g", name, op, prec, x[i], g, want))
					}
				}
			}
		}
	}
}

func main() {
	check("f32x4", 4, round32x4)
	check("f32x8", 8, round32x8)
	check("f32x16", 16, round32x16)
	check("f64x2", 2, round64x2)
	check("f64x4", 4, round64x4)
	check("f64x8", 8, round64x8)
	if os.Getenv("GOALLC_SIMD_ROUND_TRACE") != "" {
		fmt.Printf("AVX512=%v\n", archsimd.X86.AVX512())
	}
}
