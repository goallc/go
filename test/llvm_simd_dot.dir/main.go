// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

var trace = os.Getenv("GOALLC_SIMD_DOT_TRACE") != ""
var enabled, fallback int

func fmtTrace() { fmt.Printf("enabled=%d fallback=%d\n", enabled, fallback) }

func check(width int, available bool, fn func([]int16, []int16, []uint8, []int8, []int32, []int16, []uint64)) {
	if available {
		enabled++
	} else {
		fallback++
	}
	n := width / 8
	x, y := make([]int16, n/2), make([]int16, n/2)
	u, s := make([]uint8, n), make([]int8, n)
	d, sat, sad := make([]int32, n/4), make([]int16, n/2), make([]uint64, n/8)
	words := []int16{-32768, 32767, -1, 0, 1, -12345, 23456}
	bytes := []uint8{0, 1, 127, 128, 254, 255, 57}
	for a := 0; a < 2*len(words); a++ {
		for b := 0; b < len(words); b++ {
			// Exercise distinct lanes and uniform boundary inputs, including
			// the -32768 * -32768 pair whose int32 sum wraps.
			for i := range x {
				k := i
				if a >= len(words) {
					k = 0
				}
				x[i] = words[(a+k)%len(words)]
				y[i] = words[(b+3*k)%len(words)]
			}
			for i := range u {
				k := i
				if a >= len(words) {
					k = 0
				}
				u[i] = bytes[(a+k)%len(bytes)]
				s[i] = int8(bytes[(b+3*k)%len(bytes)])
			}
			fn(x, y, u, s, d, sat, sad)
			for i, got := range d {
				want := int32(int64(x[2*i])*int64(y[2*i]) + int64(x[2*i+1])*int64(y[2*i+1]))
				if !available {
					want = 0
				}
				if got != want {
					panic(fmt.Sprintf("dot%d case %d/%d lane %d: got %d want %d", width, a, b, i, got, want))
				}
			}
			for i, got := range sat {
				want := int32(u[2*i])*int32(s[2*i]) + int32(u[2*i+1])*int32(s[2*i+1])
				if want < -32768 {
					want = -32768
				}
				if want > 32767 {
					want = 32767
				}
				if !available {
					want = 0
				}
				if got != int16(want) {
					panic(fmt.Sprintf("sat%d case %d/%d lane %d: got %d want %d", width, a, b, i, got, want))
				}
			}
			for i, got := range sad {
				var want uint64
				for j := 8 * i; j < 8*(i+1); j++ {
					diff := int(u[j]) - int(uint8(s[j]))
					if diff < 0 {
						diff = -diff
					}
					want += uint64(diff)
				}
				if !available {
					want = 0
				}
				if got != want {
					panic(fmt.Sprintf("sad%d case %d/%d lane %d: got %d want %d", width, a, b, i, got, want))
				}
			}
		}
	}
}
