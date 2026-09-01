// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"math/bits"
	"runtime"
	"sync/atomic"
)

var counter uint64

//go:noinline
func cpuMath(x, y, z float64) (float64, float64, string) {
	return math.Floor(x), math.FMA(x, y, z), callerName()
}

//go:noinline
func cpuBits(x uint64) int {
	return bits.OnesCount64(x)
}

//go:noinline
func cpuAtomic(delta uint64) (uint64, string) {
	return atomic.AddUint64(&counter, delta), callerName()
}

//go:noinline
func callerName() string {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return ""
	}
	f := runtime.FuncForPC(pc)
	if f == nil {
		return ""
	}
	return f.Name()
}

func main() {
	floor, fma, mathName := cpuMath(3.75, 2, 3)
	atomicValue, atomicName := cpuAtomic(7)
	if floor != 3 || fma != 10.5 || cpuBits(0xf0f0) != 8 || atomicValue != 7 ||
		mathName != "main.cpuMath" || atomicName != "main.cpuAtomic" {
		panic("bad CPU-feature multiversion result")
	}
}
