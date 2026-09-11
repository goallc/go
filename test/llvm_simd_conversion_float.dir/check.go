// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"math/big"
	"runtime"
)

type number interface {
	int32 | int64 | uint32 | uint64 | float32 | float64
}

// Integer and floating inputs are constructed independently of SIMD casts.
// Include representability limits, both neighbors of every boundary, ties,
// infinities, signed zero, subnormals, and quiet/signaling NaNs.
func inputs[I number](kind string, bits int) []I {
	var result []I
	if kind != "float" {
		values := []uint64{0, 1, 2, 3, 7, 31, 63, 127, 255, 0x5555555555555555, 0xaaaaaaaaaaaaaaaa, ^uint64(0)}
		for _, bit := range []uint{24, 31, 32, 53, 62, 63} {
			n := uint64(1) << bit
			for _, delta := range []uint64{0, 1, 2, 3, 127, 128, 129, 255, 256, 257} {
				values = append(values, n-delta, n+delta, -(n - delta), -(n + delta))
			}
			// Halfway +/- 1 at large magnitudes exposes double rounding
			// through float64 before conversion to float32.
			for _, precision := range []uint{24, 53} {
				if bit < precision {
					continue
				}
				half := uint64(1) << (bit - precision)
				for _, midpoint := range []uint64{n + half, n + 3*half} {
					values = append(values, midpoint-1, midpoint, midpoint+1, -(midpoint - 1), -midpoint, -(midpoint + 1))
				}
			}
		}
		for _, n := range values {
			result = append(result, I(n))
		}
		return result
	}
	values := []float64{0, math.Copysign(0, -1), 1, -1, 1.25, -1.25, 1.75, -1.75,
		0.5, -0.5, math.Nextafter(-1, 0), math.Nextafter(-1, -2),
		math.Inf(1), math.Inf(-1), math.MaxFloat64, -math.MaxFloat64,
		math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32,
		math.MaxFloat32, -math.MaxFloat32,
		1 + math.Ldexp(1, -24), 1 + math.Ldexp(3, -24),
		math.Ldexp(1, -150), math.Ldexp(3, -150),
	}
	for _, midpoint := range []float64{1 + math.Ldexp(1, -24), 1 + math.Ldexp(3, -24), math.Ldexp(1, -150), math.Ldexp(3, -150), math.MaxFloat32 + math.Ldexp(1, 103)} {
		for _, sign := range []float64{1, -1} {
			x := sign * midpoint
			values = append(values, math.Nextafter(x, math.Inf(-1)), x, math.Nextafter(x, math.Inf(1)))
		}
	}
	for _, exponent := range []int{0, 24, 31, 32, 53, 63, 64, 127, -126} {
		n := math.Ldexp(1, exponent)
		values = append(values, n, -n, n-1, n+1, -n-1, -n+1, n-0.5, -n-0.5)
		for _, sign := range []float64{1, -1} {
			x := sign * n
			if bits == 32 {
				values = append(values, float64(math.Nextafter32(float32(x), float32(math.Inf(-1)))), float64(math.Nextafter32(float32(x), float32(math.Inf(1)))))
			} else {
				values = append(values, math.Nextafter(x, math.Inf(-1)), math.Nextafter(x, math.Inf(1)))
			}
		}
	}
	for _, n := range values {
		result = append(result, I(n))
	}
	// Avoid conversion through float64 when constructing float32 NaNs.
	if bits == 32 {
		for _, raw := range []uint32{0x7fc00001, 0xffc12345, 0x7f800001, 0xff800001} {
			result = append(result, any(math.Float32frombits(raw)).(I))
		}
	} else {
		for _, raw := range []uint64{0x7ff8000000000001, 0xfff8123456789abc, 0x7ff0000000000001, 0xfff0000000000001} {
			result = append(result, any(math.Float64frombits(raw)).(I))
		}
	}
	return result
}

// Model the architecture-defined CVTT / FCVTZ results explicitly instead of
// using a scalar cast outside its representable range.
func floatToInteger[O number](x float64, kind string, bits int) O {
	if kind == "int" {
		low := int64(-1) << (bits - 1)
		high := uint64(1)<<(bits-1) - 1
		limit := math.Ldexp(1, bits-1)
		if math.IsNaN(x) {
			if runtime.GOARCH == "amd64" {
				return O(low)
			}
			return 0
		}
		if x >= limit {
			if runtime.GOARCH == "amd64" {
				return O(low)
			}
			return O(high)
		}
		if x <= -limit {
			return O(low)
		}
		return O(int64(x))
	}
	high := ^uint64(0) >> (64 - bits)
	if math.IsNaN(x) {
		if runtime.GOARCH == "amd64" {
			return O(high)
		}
		return 0
	}
	if x < 0 {
		if runtime.GOARCH == "amd64" && x <= -1 {
			return O(high)
		}
		return 0
	}
	if x >= math.Ldexp(1, bits) {
		return O(high)
	}
	return O(uint64(x))
}

// big.Float supplies an exact-value, round-to-nearest-even oracle, including
// direct int64/uint64-to-float32 rounding without a float64 intermediate.
func expected[I, O number](x I, src, dst string, dstBits int) O {
	if dst != "float" {
		return floatToInteger[O](float64(x), dst, dstBits)
	}
	var exact big.Float
	switch src {
	case "int":
		exact.SetInt64(int64(x))
	case "uint":
		exact.SetUint64(uint64(x))
	case "float":
		if math.IsNaN(float64(x)) {
			return O(math.NaN())
		}
		exact.SetFloat64(float64(x))
	}
	if dstBits == 32 {
		n, _ := exact.Float32()
		return O(n)
	}
	n, _ := exact.Float64()
	return O(n)
}

func equal[O number](x, y O, kind string, bits int) bool {
	if kind != "float" {
		return x == y
	}
	if math.IsNaN(float64(x)) {
		return math.IsNaN(float64(y))
	}
	if bits == 32 {
		return math.Float32bits(float32(x)) == math.Float32bits(float32(y))
	}
	return math.Float64bits(float64(x)) == math.Float64bits(float64(y))
}

func check[I, O number](name, src string, srcBits, srcLanes int, dst string, dstBits, dstLanes int, abi, enabled bool, convert func([]I, []O)) {
	if !abi {
		skippedCases++
		return
	}
	patterns := inputs[I](src, srcBits)
	x, got := make([]I, srcLanes), make([]O, dstLanes)
	for round := 0; round < len(patterns); round++ {
		for i := range x {
			x[i] = patterns[(round+7*i)%len(patterns)]
		}
		for i := range got {
			got[i] = O(99)
		}
		convert(x, got)
		for i, value := range got {
			var want O
			if enabled && i < srcLanes {
				want = expected[I, O](x[i], src, dst, dstBits)
			}
			if !equal(value, want, dst, dstBits) {
				panic(fmt.Sprintf("%s round=%d lane=%d got=%v want=%v input=%v", name, round, i, value, want, x))
			}
		}
	}
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
}
