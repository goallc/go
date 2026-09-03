// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"simd"
)

func checkDispatchFMVCase(test dispatchFMVCase) {
	var x, y, portableDst, wide512Dst [64]int8
	var x256, y256, wide256Dst [32]int8
	for i := range x {
		x[i] = int8(i*3 + 1)
		y[i] = int8(i*5 - 63)
		wide512Dst[i] = -1
		portableDst[i] = -1
		if i < len(x256) {
			x256[i] = x[i]
			y256[i] = y[i]
			wide256Dst[i] = -1
		}
	}

	lanes, usedAVX2, usedAVX512 := dispatchFMVOperation(
		&portableDst, &wide256Dst, &wide512Dst, &x, &y, &x256, &y256,
	)
	if simd.VectorBitSize() != test.width || simd.Emulated() != test.emulated {
		panic(fmt.Sprintf("%s: SIMD mode = (%d, %v), want (%d, %v)",
			test.name, simd.VectorBitSize(), simd.Emulated(), test.width, test.emulated))
	}
	wantLanes := test.width / 8
	if lanes != wantLanes {
		panic(fmt.Sprintf("%s: lanes = %d, want %d", test.name, lanes, wantLanes))
	}
	if usedAVX2 != test.usedAVX2 || usedAVX512 != test.usedAVX512 {
		panic(fmt.Sprintf("%s: CPU paths = (%v, %v), want (%v, %v)",
			test.name, usedAVX2, usedAVX512, test.usedAVX2, test.usedAVX512))
	}

	for i := 0; i < lanes; i++ {
		if portableDst[i] != x[i]+y[i] {
			panic(fmt.Sprintf("%s: portable lane %d = %d, want %d",
				test.name, i, portableDst[i], x[i]+y[i]))
		}
	}
	for i := range wide256Dst {
		if wide256Dst[i] != x256[i]+y256[i] {
			panic(fmt.Sprintf("%s: 256-bit lane %d = %d, want %d",
				test.name, i, wide256Dst[i], x256[i]+y256[i]))
		}
	}
	for i := range wide512Dst {
		if wide512Dst[i] != x[i]+y[i] {
			panic(fmt.Sprintf("%s: 512-bit lane %d = %d, want %d",
				test.name, i, wide512Dst[i], x[i]+y[i]))
		}
	}

	fmt.Printf("%-24s width=%3d emulated=%-5v avx2=%-5v avx512=%-5v\n",
		test.name, test.width, test.emulated, usedAVX2, usedAVX512)
}
