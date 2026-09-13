// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

type integer interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64
}

var enabledCases, fallbackCases int

func check[T, U integer](name string, n int, even, enabled bool, multiply func([]T, []T, []U)) {
	if enabled {
		enabledCases++
	} else {
		fallbackCases++
	}
	x, y, got := make([]T, n), make([]T, n), make([]U, n/2)
	// Converting these patterns to T gives signed extrema and large unsigned
	// operands. Calculate products in U, never in the narrow input type.
	data := []uint64{0, 1, 2, ^uint64(0), 0x80, 0x7f, 0x8000, 0x7fff,
		0x80000000, 0x7fffffff, 0xaaaaaaaa, 0x55555555, 0xfffffffe}
	for a := range data {
		for b := range data {
			for i := range x {
				x[i], y[i] = T(data[(a+i)%len(data)]), T(data[(b+3*i)%len(data)])
			}
			multiply(x, y, got)
			for i, result := range got {
				j := i
				if even {
					j *= 2
				}
				var want U
				if enabled {
					want = U(x[j]) * U(y[j])
				}
				if result != want {
					panic(fmt.Sprintf("%s lane=%d x=%v y=%v got=%v want=%v", name, i, x[j], y[j], result, want))
				}
			}
		}
	}
}

func main() {
	checkArch()
	if os.Getenv("GOALLC_SIMD_WIDEN_TRACE") == "1" {
		fmt.Printf("widen multiply: enabled=%d fallback=%d\n", enabledCases, fallbackCases)
	}
}
