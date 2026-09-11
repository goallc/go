// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGoALLCConversionShape(t *testing.T) {
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
		{"Int16x8", "Int8x16", "saturate-integer", true},
		{"Int64x2", "Uint32x4", "saturate-integer", true},
		{"Uint64x8", "Uint8x16", "saturate-integer", true},
		{"Int32x4", "Float32x4", "saturate-integer", false},
		{"Int16x8", "Int32x4", "saturate-integer", false},
		{"Int32x16", "Int16x8", "saturate-integer", false},
		{"Float32x4", "Int32x4", "convert-float", true},
		{"Float32x4", "Uint64x4", "convert-float", true},
		{"Float64x2", "Int32x4", "convert-float", true},
		{"Int64x2", "Float32x4", "convert-float", true},
		{"Uint64x8", "Float32x8", "convert-float", true},
		{"Uint32x8", "Float64x8", "convert-float", true},
		{"Float32x4", "Float64x2", "convert-float", true},
		{"Float32x8", "Float64x8", "convert-float", true},
		{"Float64x2", "Float32x4", "convert-float", true},
		{"Float64x8", "Float32x8", "convert-float", true},
		{"Int32x4", "Int64x4", "convert-float", false},
		{"Int16x8", "Float32x8", "convert-float", false},
		{"Float32x4", "Int16x8", "convert-float", false},
		{"Float32x4", "Float32x4", "convert-float", false},
		{"Int32x4", "Float64x2", "convert-float", false},
		{"Float32x4", "Int64x2", "convert-float", false},
		{"Float64x2", "Int32x8", "convert-float", false},
		{"Float32x4", "Int32x8", "convert-float", false},
		{"Float64x4", "Float32x8", "convert-float", false},
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

func TestGoALLCSaturatingPackShape(t *testing.T) {
	operand := func(name string) Operand {
		_, bits, lanes, _ := goALLCLaneFromGoType(&name)
		width := bits * lanes
		return Operand{Class: "vreg", Go: &name, Bits: &width}
	}
	for _, test := range []struct {
		x, y, out string
		valid     bool
	}{
		{"Int32x4", "Int32x4", "Int16x8", true},
		{"Int32x8", "Int32x8", "Uint16x16", true},
		{"Int32x16", "Int32x16", "Int16x32", true},
		{"Int32x4", "Uint32x4", "Int16x8", false},
		{"Int32x8", "Int32x4", "Int16x16", false},
		{"Int32x4", "Int32x4", "Int8x16", false},
		{"Int32x4", "Int32x4", "Int16x16", false},
		{"Int32x4", "Int32x4", "Int16x4", false},
		{"Float32x4", "Float32x4", "Int16x8", false},
		{"Int32x2", "Int32x2", "Int16x4", false},
		{"Int32x8", "", "Int16x16", false},
	} {
		t.Run(test.x+"-"+test.y+"-"+test.out, func(t *testing.T) {
			op := Operation{Go: "SaturatingPack", In: []Operand{operand(test.x)}, Out: []Operand{operand(test.out)}}
			if test.y != "" {
				op.In = append(op.In, operand(test.y))
			}
			defer func() {
				failed := recover() != nil
				if failed == test.valid {
					t.Errorf("shape accepted=%v, want %v", !failed, test.valid)
				}
			}()
			validateGoALLCLowering(op, op, "saturate-integer-pack128", PureVregIn, OneVregOut, NoMask, NoImm)
		})
	}
}
