// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math"
	"sync/atomic"
)

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
	checkBoolBoundaries()
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

type boolRecord struct {
	before byte
	flag   bool
	flags  [3]bool
	after  byte
}

//go:noinline
func boolRecordRoundtrip(r boolRecord, b bool) (boolRecord, bool) {
	r.flag = b
	r.flags[1] = !b
	return r, r.flags[0] != r.flags[1]
}

//go:noinline
func boolLoop(p *bool, n int) bool {
	b := *p
	for i := 0; i < n; i++ {
		b = b != (i&1 == 0)
	}
	*p = b
	return b
}

//go:noinline
func boolDeferred(b bool) (r bool) {
	defer func() { r = !r }()
	return b
}

func checkBoolBoundaries() {
	for _, b := range []bool{false, true} {
		r := boolRecord{before: 0x35, flag: !b, flags: [3]bool{true, b, false}, after: 0xa7}
		got, flag := boolRecordRoundtrip(r, b)
		if got.before != 0x35 || got.after != 0xa7 || got.flag != b || got.flags != [3]bool{true, !b, false} || flag != b {
			panic("bool aggregate boundary")
		}
		if boolDeferred(b) != !b {
			panic("bool deferred return")
		}
		for n := 0; n < 9; n++ {
			x := b
			want := b
			if ((n+1)/2)&1 != 0 {
				want = !want
			}
			if boolLoop(&x, n) != want || x != want {
				panic("bool loop phi")
			}
		}
	}
	var x uint32 = 1
	if !atomic.CompareAndSwapUint32(&x, 1, 2) || atomic.CompareAndSwapUint32(&x, 1, 3) || x != 2 {
		panic("bool CAS result")
	}
}
