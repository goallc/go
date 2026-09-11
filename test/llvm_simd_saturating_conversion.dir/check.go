// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

type integer interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Use scalar bounds rather than another SIMD operation as the oracle. In
// particular, unsigned source values must not be reinterpreted as signed.
func check[I, O integer](name string, inBits, inLanes, outBits, outLanes int, signedIn, signedOut, packed, abi, enabled bool, convert func([]I, []I, []O)) {
	if !abi {
		skippedCases++
		return
	}
	var low int64
	high := uint64(1)<<outBits - 1
	if signedOut {
		high >>= 1
		low = -int64(high) - 1
	}
	patterns := []uint64{
		0, 1, 2, 7, 17, 31, 63, 127, 128, 255, 256,
		32767, 32768, 65535, 65536, 1<<31 - 1, 1 << 31, 1<<32 - 1, 1 << 32,
		1<<63 - 1, 1 << 63, ^uint64(0), ^uint64(1),
		0x5555555555555555, 0xaaaaaaaaaaaaaaaa,
		uint64(low - 1), uint64(low), uint64(low + 1), high - 1, high, high + 1,
	}
	clamp := func(v I) O {
		if signedIn {
			n := int64(v)
			if n < low {
				n = low
			}
			if n > int64(high) {
				n = int64(high)
			}
			return O(n)
		}
		n := uint64(v)
		if n > high {
			n = high
		}
		return O(n)
	}
	x, y, got := make([]I, inLanes), make([]I, inLanes), make([]O, outLanes)
	for round := 0; round < 64; round++ {
		for i := range x {
			x[i] = I(patterns[(round+5*i)%len(patterns)])
			y[i] = I(patterns[(round+7*i+11)%len(patterns)])
			// Lane-distinct, in-range inputs expose the group and operand order.
			if round%4 == 3 {
				x[i] = I(1 + i + round)
				y[i] = I(91 - i - round)
			}
		}
		for i := range got {
			got[i] = O(99)
		}
		convert(x, y, got)
		for i, value := range got {
			var want O
			if enabled {
				if !packed {
					if i < inLanes {
						want = clamp(x[i])
					}
				} else {
					group := 128 / inBits
					lane := i/(2*group)*group + i%group
					if i%(2*group) < group {
						want = clamp(x[lane])
					} else {
						want = clamp(y[lane])
					}
				}
			}
			if value != want {
				panic(fmt.Sprintf("%s round=%d lane=%d got=%v want=%v", name, round, i, value, want))
			}
		}
	}
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
}
