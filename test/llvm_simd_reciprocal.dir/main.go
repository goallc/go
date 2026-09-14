// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/bits"
	"os"
	"simd/archsimd"
)

var digest uint64

func check[T float32 | float64](name string, lanes int, legacy bool, fn func([]T, []T, []T)) {
	enabled := archsimd.X86.AVX512()
	bound := 0x1p-14
	if legacy {
		enabled = archsimd.X86.AVX()
		bound = 1.5 * 0x1p-12
	}
	// Intel's RCPPS/RSQRTPS and RCP14/RSQRT14 have different error and
	// denormal contracts. Check the instruction selected by the Go API.
	values := []float64{0, math.Copysign(0, -1), 1, -1, 2, 3, 0.1, -2.75, math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat64, math.MaxFloat32, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	for start := range values {
		x, reciprocal, rsqrt := make([]T, lanes), make([]T, lanes), make([]T, lanes)
		for i := range x {
			x[i] = T(values[(start+i)%len(values)])
		}
		fn(x, reciprocal, rsqrt)
		for op, out := range [][]T{reciprocal, rsqrt} {
			for i, got := range out {
				g, w := float64(got), float64(0)
				if enabled {
					a := float64(x[i])
					if legacy && math.Abs(a) < 0x1p-126 {
						a = math.Copysign(0, a)
					}
					w = 1 / a
					if op == 1 {
						w = 1 / math.Sqrt(a)
					}
					if legacy && math.Abs(w) < 0x1p-126 {
						w = math.Copysign(0, w)
					}
					if math.IsInf(float64(T(w)), 0) {
						w = float64(T(w))
					}
				}
				if math.IsNaN(g) && math.IsNaN(w) {
					digest = bits.RotateLeft64(digest, 7)*1099511628211 ^ 0x7ff8000000000000
					continue
				}
				ok := math.Float64bits(g) == math.Float64bits(w)
				if w != 0 && !math.IsInf(w, 0) && !math.IsNaN(w) {
					ok = math.Abs((g-w)/w) <= bound
				}
				if !ok {
					panic(fmt.Sprintf("%s op=%d x=%g got=%g want=%g bound=%g", name, op, x[i], g, w, bound))
				}
				digest = bits.RotateLeft64(digest, 7)*1099511628211 ^ math.Float64bits(g)
			}
		}
	}
}

func main() {
	check("f32x4", 4, true, recip32x4)
	check("f32x8", 8, true, recip32x8)
	check("f32x16", 16, false, recip32x16)
	check("f64x2", 2, false, recip64x2)
	check("f64x4", 4, false, recip64x4)
	check("f64x8", 8, false, recip64x8)
	if os.Getenv("GOALLC_SIMD_RECIP_TRACE") != "" {
		fmt.Printf("AVX=%v AVX512=%v digest=%016x\n", archsimd.X86.AVX(), archsimd.X86.AVX512(), digest)
	}
}
