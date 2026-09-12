// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
)

type number interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

func raw[T number](x T, width int) uint64 {
	switch v := any(x).(type) {
	case float32:
		return uint64(math.Float32bits(v))
	case float64:
		return math.Float64bits(v)
	}
	return uint64(x) & (^uint64(0) >> (64 - width))
}

func input[T number](lane, side, round int) T {
	label := uint64(1 + lane + 64*side)
	// Distinct lane labels in the first pass detect group and input-order errors.
	if round == 0 {
		return T(label)
	}
	var zero T
	switch any(zero).(type) {
	case float32:
		values := []uint32{0, 0x80000000, 0x7f800000, 0xff800000, 0x7fc00123, 0xffc00456, 0x7f800001, 0xff800002, 1, 0x80000001, 0x007fffff, 0x00800000, 0x3f800000, 0x7f7fffff}
		return any(math.Float32frombits(values[(round+lane+7*side)%len(values)])).(T)
	case float64:
		values := []uint64{0, 0x8000000000000000, 0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000123, 0xfff8000000000456, 0x7ff0000000000001, 0xfff0000000000002, 1, 0x8000000000000001, 0x000fffffffffffff, 0x0010000000000000, 0x3ff0000000000000, 0x7fefffffffffffff}
		return any(math.Float64frombits(values[(round+lane+7*side)%len(values)])).(T)
	}
	values := []uint64{0, ^uint64(0), 0x8080808080808080, 0x7f7f7f7f7f7f7f7f, 0x5555555555555555, 0xaaaaaaaaaaaaaaaa, 0x0123456789abcdef, 0xfedcba9876543210}
	return T(values[(round+lane+side)%len(values)] ^ (label << uint(round%8)))
}

func checkDynamic[T number, I number](name, family string, bits, n int, abi, enabled bool, shuffle func([]T, []T, []I, []T)) {
	if !abi {
		skippedCases++
		return
	}
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
	x, y, got := make([]T, n), make([]T, n), make([]T, n)
	indices := make([]I, n)
	mask := ^uint64(0) >> (64 - bits)
	controls := make([]uint64, 256)
	for i := range controls {
		controls[i] = uint64(i)
	}
	for i := 8; i < bits; i++ {
		controls = append(controls, uint64(1)<<i, mask^(uint64(1)<<i))
	}
	for round := 0; round < 15; round++ {
		for i := range x {
			x[i] = input[T](i, 0, round)
			y[i] = input[T](i, 1, round)
		}
		for _, control := range controls {
			for i := range indices {
				indices[i] = I((control + uint64(i)) & mask)
			}
			shuffle(x, y, indices, got)
			checkedControls++
			for i, index := range indices {
				var want T
				if enabled {
					j := raw(index, bits)
					source := x
					valid := true
					switch family {
					case "ConcatPermute":
						source = append(append([]T(nil), x...), y...)
					case "LookupOrZero":
						valid = j < uint64(n)
					case "PermuteOrZero", "PermuteOrZeroGrouped":
						valid = j < 128
						if family == "PermuteOrZeroGrouped" {
							start := i / 16 * 16
							source = x[start : start+16]
						}
					}
					if valid {
						want = source[j%uint64(len(source))]
					}
				}
				if raw(got[i], bits) != raw(want, bits) {
					panic(fmt.Sprintf("%s enabled=%v round=%d index=%#x lane=%d got=%#x want=%#x", name, enabled, round, raw(index, bits), i, raw(got[i], bits), raw(want, bits)))
				}
			}
		}
	}
}
