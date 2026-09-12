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

func expectedImmediate[T number](family string, width, control int, x, y []T) []T {
	out := append([]T(nil), x...)
	group := 128 / width
	if family == "ConcatPermute128Scalars" {
		src := append(append([]T(nil), x...), y...)
		copy(out[:group], src[(control&3)*group:((control&3)+1)*group])
		copy(out[group:], src[(control>>2)*group:((control>>2)+1)*group])
		return out
	}
	for base := 0; base < len(x); base += group {
		left, right := x[base:base+group], y[base:base+group]
		switch {
		case family == "ConcatShiftBytesRight" || family == "ConcatShiftBytesRightGrouped":
			src := append(append(append([]T(nil), right...), left...), make([]T, 256+group)...)
			copy(out[base:base+group], src[control:control+group])
		case family == "concatSelectedConstant" || family == "concatSelectedConstantGrouped":
			src := append(append([]T(nil), left...), right...)
			c, field, mask := control, 3, 7
			if width == 64 {
				field, mask = 2, 3
			}
			for i := 0; i < group; i++ {
				out[base+i] = src[c&mask]
				c >>= field
			}
		default:
			offset := 0
			if family == "permuteScalarsHi" || family == "permuteScalarsHiGrouped" {
				offset = 4
			}
			c := control
			for i := 0; i < 4; i++ {
				out[base+offset+i] = left[offset+(c&3)]
				c >>= 2
			}
		}
	}
	return out
}

func checkImmediate[T number](name, family string, width, lanes, controls int, abi, enabled bool, shuffle func([]T, []T, []T, uint16)) {
	if !abi {
		skippedCases++
		return
	}
	x, y, got := make([]T, lanes), make([]T, lanes), make([]T, lanes)
	for round := 0; round < 15; round++ {
		for i := range x {
			x[i] = input[T](i, 0, round)
			y[i] = input[T](i, 1, round)
		}
		for control := 0; control < controls; control++ {
			shuffle(x, y, got, uint16(control))
			want := make([]T, lanes)
			if enabled {
				want = expectedImmediate(family, width, control, x, y)
			}
			for i := range got {
				if raw(got[i], width) != raw(want[i], width) {
					panic(fmt.Sprintf("%s control=%d round=%d lane=%d got=%x want=%x", name, control, round, i, raw(got[i], width), raw(want[i], width)))
				}
			}
			checkedControls++
		}
	}
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
}
