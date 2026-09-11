// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGoALLCIntegerConversionShape(t *testing.T) {
	operand := func(name string) Operand {
		_, bits, lanes, _ := goALLCLaneFromGoType(&name)
		width := bits * lanes
		return Operand{Class: "vreg", Go: &name, Bits: &width}
	}
	for _, test := range []struct {
		in, out, lowering string
		valid             bool
	}{
		{"Int8x16", "Int16x16", "extend-integer", true},
		{"Uint8x16", "Uint64x2", "extend-integer", true},
		{"Int16x32", "Int8x32", "truncate-integer", true},
		{"Uint64x2", "Uint8x16", "truncate-integer", true},
		{"Float32x4", "Int64x2", "extend-integer", false},
		{"Int8x16", "Uint16x8", "extend-integer", false},
		{"Int16x8", "Int8x16", "extend-integer", false},
		{"Int8x16", "Int16x32", "extend-integer", false},
		{"Int32x16", "Int16x8", "truncate-integer", false},
		{"Int8x16", "Int16x16", "truncate-integer", false},
		{"Int16x8", "Int16x8", "truncate-integer", false},
	} {
		t.Run(test.in+"-"+test.out+"-"+test.lowering, func(t *testing.T) {
			op := Operation{Go: "Conversion", In: []Operand{operand(test.in)}}
			op.Out = []Operand{operand(test.out)}
			defer func() {
				failed := recover() != nil
				if failed == test.valid {
					t.Errorf("shape accepted=%v, want %v", !failed, test.valid)
				}
			}()
			validateGoALLCLowering(op, op, test.lowering, PureVregIn, OneVregOut, NoMask, NoImm)
		})
	}
}
