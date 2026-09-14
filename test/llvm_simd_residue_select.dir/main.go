// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/big"
	"os"
	"simd/archsimd"
)

func check[T float32 | float64](name string, lanes int, precision uint, fn func([]T, [4][]T, uint8)) {
	values := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 1.5, -2.5, 1.25, -1.75, 0.1, -0.1, 0x1.0001p0, -0x1.0001p0, math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64, math.MaxFloat32, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	rounders := []func(float64) float64{math.RoundToEven, math.Floor, math.Ceil, math.Trunc}
	modes := []big.RoundingMode{big.ToNearestEven, big.ToNegativeInf, big.ToPositiveInf, big.ToZero}
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
						a := float64(x[i])
						if math.IsNaN(a) {
							want = a
						} else if !math.IsInf(a, 0) {
							if math.Abs(a) < 0x1p52 {
								p := int(prec & 15)
								rounded := math.Ldexp(rounders[op](math.Ldexp(a, p)), -p)
								// REDUCE applies the same directed rounding to the
								// subtraction, including inexact tiny-input cases.
								diff := new(big.Float).SetPrec(precision).SetMode(modes[op])
								want, _ = diff.Sub(big.NewFloat(a), big.NewFloat(rounded)).Float64()
							}
							if want == 0 {
								want = 0
								if op == 1 {
									want = math.Copysign(0, -1)
								}
							}
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
	check("f32x4", 4, 24, residue32x4)
	check("f32x8", 8, 24, residue32x8)
	check("f32x16", 16, 24, residue32x16)
	check("f64x2", 2, 53, residue64x2)
	check("f64x4", 4, 53, residue64x4)
	check("f64x8", 8, 53, residue64x8)
	for start := range 256 {
		x, y, out := make([]int8, 32), make([]int8, 32), make([]int8, 32)
		for i := range x {
			x[i], y[i] = int8(start+i), int8(start*37-i)
		}
		selectBytes(x, y, out)
		for i, got := range out {
			want := int8(0)
			if archsimd.X86.AVX2() {
				want = max(x[i], y[i])
			}
			if got != want {
				panic(fmt.Sprintf("select lane=%d got=%d want=%d", i, got, want))
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_RESIDUE_TRACE") != "" {
		fmt.Printf("AVX2=%v AVX512=%v\n", archsimd.X86.AVX2(), archsimd.X86.AVX512())
	}
}
