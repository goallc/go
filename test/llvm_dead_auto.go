// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type ints struct{ a, b, c, d, e, f int }
type floats struct{ a, b, c, d, e, f float64 }
type mixed struct {
	label string
	data  []int
	fn    func() int
}

// Only one register piece remains live after store-to-load forwarding.
// Removing the parameter's unread stores must not lose that input value.
//
//go:noinline
func lastInt(v ints) int { return v.f }

//go:noinline
func lastFloat(v floats) float64 { return v.f }

//go:noinline
func callback(v mixed) func() int { return v.fn }

func main() {
	for i := -16; i < 16; i++ {
		if lastInt(ints{1, 2, 3, 4, 5, i}) != i {
			panic("lost integer parameter piece")
		}
		want := float64(i) + 0.25
		if lastFloat(floats{1, 2, 3, 4, 5, want}) != want {
			panic("lost floating-point parameter piece")
		}
		fn := callback(mixed{"unused", []int{1, 2}, func() int { return i }})
		if fn() != i {
			panic("lost pointer parameter piece")
		}
	}
}
