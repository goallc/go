// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"os"
)

type number interface {
	int16 | int32 | int64 | uint16 | uint32 | uint64 | float32 | float64
}

var enabledCases, fallbackCases int

// Modes: pair add, pair sub, saturated pair add, saturated pair sub,
// and elementwise odd add/even sub.
func check[T number](name string, n, group int, modes []int, enabled bool, run func(x, y, out []T)) {
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
	x, y, got := make([]T, n), make([]T, n), make([]T, n*len(modes))
	data := []uint64{0, 1, 2, ^uint64(0), 0x7fff, 0x8000, 0xffff, 0x7fffffff, 0x80000000, 0xffffffff, 0x7fffffffffffffff, 0x8000000000000000, 0xaaaaaaaaaaaaaaaa}
	floating := false
	switch any(x[0]).(type) {
	case float32, float64:
		floating = true
	}
	floats := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, math.MaxFloat32, math.MaxFloat64, math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat64, math.Inf(1), math.Inf(-1), math.NaN()}
	for a := range data {
		for b := range data {
			for i := range x {
				x[i], y[i] = T(data[(a+i)%len(data)]), T(data[(b+3*i)%len(data)])
				if floating {
					x[i], y[i] = T(floats[(a+i)%len(floats)]), T(floats[(b+3*i)%len(floats)])
				}
			}
			run(x, y, got)
			for m, mode := range modes {
				want := make([]T, 0, n)
				if mode == 4 {
					for i := range x {
						v := x[i] + y[i]
						if i%2 == 0 {
							v = x[i] - y[i]
						}
						want = append(want, v)
					}
				} else {
					// Enumerate each source's pairs in each group, independently
					// of the compiler's shuffle-mask construction.
					for g := 0; g < n; g += group {
						for _, src := range [][]T{x, y} {
							for i := g; i < g+group; i += 2 {
								v := src[i] + src[i+1]
								if mode%2 != 0 {
									v = src[i] - src[i+1]
								}
								if mode >= 2 {
									sum := int64(src[i]) + int64(src[i+1])
									if mode == 3 {
										sum = int64(src[i]) - int64(src[i+1])
									}
									if sum > 32767 {
										sum = 32767
									}
									if sum < -32768 {
										sum = -32768
									}
									v = T(sum)
								}
								want = append(want, v)
							}
						}
					}
				}
				for i, w := range want {
					if !enabled {
						w = 0
					}
					v := got[m*n+i]
					if floating && math.IsNaN(float64(v)) && math.IsNaN(float64(w)) {
						continue
					}
					equal := v == w
					if floating {
						equal = math.Float64bits(float64(v)) == math.Float64bits(float64(w))
					}
					if !equal {
						panic(fmt.Sprintf("%s mode=%d lane=%d x=%v y=%v got=%v want=%v", name, mode, i, x, y, v, w))
					}
				}
			}
		}
	}
}

func main() {
	checkArch()
	if os.Getenv("GOALLC_SIMD_HORIZONTAL_TRACE") == "1" {
		fmt.Printf("horizontal: enabled=%d fallback=%d\n", enabledCases, fallbackCases)
	}
}
