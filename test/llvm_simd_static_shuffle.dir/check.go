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

// Route scalar lane slices independently of the LLVM mask construction.
func expected[T number](recipe string, width int, x, y []T, resultLanes int) []T {
	out := make([]T, resultLanes)
	switch recipe {
	case "get-low":
		copy(out, x[:len(x)/2])
	case "get-high":
		copy(out, x[len(x)/2:])
	case "set-low":
		copy(out, x)
		copy(out, y)
	case "set-high":
		copy(out, x)
		copy(out[len(x)/2:], y)
	case "broadcast-low":
		for i := range out {
			out[i] = x[0]
		}
	case "concat-even", "concat-odd":
		parity := 0
		if recipe == "concat-odd" {
			parity = 1
		}
		position := 0
		for _, source := range [][]T{x, y} {
			for i := parity; i < len(source); i += 2 {
				out[position] = source[i]
				position++
			}
		}
	default:
		group := len(x)
		if recipe == "interleave-low-128" || recipe == "interleave-high-128" {
			group = 128 / width
		}
		for base := 0; base < len(x); base += group {
			left, right := x[base:base+group], y[base:base+group]
			var selected []int
			switch recipe {
			case "interleave-low", "interleave-low-128":
				for i := 0; i < group/2; i++ {
					selected = append(selected, i)
				}
			case "interleave-high", "interleave-high-128":
				for i := group / 2; i < group; i++ {
					selected = append(selected, i)
				}
			case "interleave-even":
				for i := 0; i < group; i += 2 {
					selected = append(selected, i)
				}
			case "interleave-odd":
				for i := 1; i < group; i += 2 {
					selected = append(selected, i)
				}
			default:
				panic(recipe)
			}
			for i, index := range selected {
				out[base+2*i], out[base+2*i+1] = left[index], right[index]
			}
		}
	}
	return out
}

func check[T number](name, recipe string, width, sourceLanes, otherLanes, resultLanes int, abi, enabled bool, shuffle func([]T, []T, []T)) {
	if !abi {
		skippedCases++
		return
	}
	x, y, got := make([]T, sourceLanes), make([]T, otherLanes), make([]T, resultLanes)
	for round := 0; round < 20; round++ {
		for i := range x {
			x[i] = input[T](i, 0, round)
		}
		for i := range y {
			y[i] = input[T](i, 1, round)
		}
		for i := range got {
			got[i] = T(99)
		}
		shuffle(x, y, got)
		want := make([]T, resultLanes)
		if enabled {
			want = expected(recipe, width, x, y, resultLanes)
		}
		for i := range got {
			if raw(got[i], width) != raw(want[i], width) {
				panic(fmt.Sprintf("%s round=%d lane=%d got=%x want=%x", name, round, i, raw(got[i], width), raw(want[i], width)))
			}
		}
	}
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
}
