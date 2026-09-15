// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "math"

//go:noinline
func branch(x, y float64) int {
	if x < y {
		return 1
	}
	return 0
}

//go:noinline
func consume(b bool) bool { return b }

//go:noinline
func mixed(x, y float64, out *bool) bool {
	b := x < y
	*out = b
	if b {
		return consume(b)
	}
	return b
}

//go:noinline
func merged(x, y float64, b bool) bool {
	if x < 0 {
		b = y < 0
	}
	return b
}

func main() {
	nan := math.Float64frombits(0x7ff8000000000001)
	for _, test := range []struct {
		x, y float64
		want bool
	}{
		{1, 2, true}, {2, 1, false}, {1, 1, false},
		{nan, 1, false}, {1, nan, false}, {nan, nan, false},
		{math.Inf(-1), math.Inf(1), true},
	} {
		wantInt := 0
		if test.want {
			wantInt = 1
		}
		if branch(test.x, test.y) != wantInt {
			panic("branch bool")
		}
		var stored bool
		if mixed(test.x, test.y, &stored) != test.want || stored != test.want {
			panic("mixed bool")
		}
	}
	if !merged(-1, -2, false) || merged(-1, 2, true) || !merged(1, -2, true) || merged(1, -2, false) {
		panic("phi bool")
	}
}
