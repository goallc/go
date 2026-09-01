// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"math/bits"
	"sync/atomic"
)

var counter uint64

//go:noinline
func cpuMath(x, y, z float64) (float64, float64) {
	return math.Floor(x), math.FMA(x, y, z)
}

//go:noinline
func cpuBits(x uint64) int {
	return bits.OnesCount64(x)
}

//go:noinline
func cpuAtomic(delta uint64) uint64 {
	return atomic.AddUint64(&counter, delta)
}

func main() {
	floor, fma := cpuMath(3.75, 2, 3)
	if floor != 3 || fma != 10.5 || cpuBits(0xf0f0) != 8 || cpuAtomic(7) != 7 {
		panic("bad CPU-feature multiversion result")
	}
}
