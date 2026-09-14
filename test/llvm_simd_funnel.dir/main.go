// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"simd/archsimd"
)

type integer interface {
	int16 | uint16 | int32 | uint32 | int64 | uint64
}
type unsigned interface{ uint16 | uint32 | uint64 }

var trace = os.Getenv("GOALLC_SIMD_FUNNEL_TRACE") != ""
var enabled, fallback int

func fmtTrace() { fmt.Printf("enabled=%d fallback=%d\n", enabled, fallback) }

func check[T integer, U unsigned](name string, bits, lanes int, fn func([]T, []T, []U, uint64, []T)) {
	modes := 4
	if bits >= 32 {
		modes = 6
	}
	for mode := 0; mode < modes; mode++ {
		available := archsimd.X86.AVX512VBMI2()
		if mode >= 4 {
			available = archsimd.X86.AVX512()
		}
		if available {
			enabled++
		} else {
			fallback++
		}
	}
	x, y, counts := make([]T, lanes), make([]T, lanes), make([]U, lanes)
	inputs := []uint64{0, 1, ^uint64(0), 0x87654321fedcba98, 0x1020408010204080, 1 << (bits - 1)}
	shifts := []uint64{0, 1, uint64(bits - 1), uint64(bits), uint64(bits + 1), 127, 128, 255, 256, 1 << 32, ^uint64(0)}
	mask := ^uint64(0) >> (64 - bits)
	for k, count := range shifts {
		for i := range x {
			x[i] = T(inputs[(k+i)%len(inputs)])
			y[i] = T(inputs[(k+3*i+1)%len(inputs)])
			counts[i] = U(shifts[(k+i)%len(shifts)])
		}
		out := make([]T, modes*lanes)
		fn(x, y, counts, count, out)
		for mode := 0; mode < modes; mode++ {
			for i := range x {
				a, b := uint64(x[i])&mask, uint64(y[i])&mask
				n := count
				if mode >= 2 {
					n = uint64(counts[i])
				}
				n %= uint64(bits)
				if mode >= 4 {
					b = a
				}
				want := a
				if n != 0 {
					if mode%2 == 0 {
						want = a<<n | b>>(uint64(bits)-n)
					} else {
						want = a>>n | b<<(uint64(bits)-n)
					}
				}
				available := archsimd.X86.AVX512VBMI2()
				if mode >= 4 {
					available = archsimd.X86.AVX512()
				}
				if !available {
					want = 0
				}
				if got := out[mode*lanes+i]; got != T(want) {
					panic(fmt.Sprintf("%s case %d mode %d lane %d: got %x want %x", name, k, mode, i, got, T(want)))
				}
			}
		}
	}
}
