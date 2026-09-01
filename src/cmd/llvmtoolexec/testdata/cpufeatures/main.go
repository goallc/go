// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "math"

//go:noinline
func cpuMath(x, y, z float64) (float64, float64) {
	return math.Floor(x), math.FMA(x, y, z)
}

func main() {
	floor, fma := cpuMath(3.75, 2, 3)
	if floor != 3 || fma != 10.5 {
		panic("bad CPU-feature multiversion result")
	}
}
