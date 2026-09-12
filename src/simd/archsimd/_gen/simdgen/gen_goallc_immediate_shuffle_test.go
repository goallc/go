// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGoALLCImmediateShuffleShape(t *testing.T) {
	operand := func(name string) Operand {
		_, bits, lanes, _ := goALLCLaneFromGoType(&name)
		width := bits * lanes
		return Operand{Class: "vreg", Go: &name, Bits: &width}
	}
	for _, test := range []struct {
		x, y, out, lowering, order string
		imm                        immShape
		valid                      bool
	}{
		{"Int32x4", "", "Int32x4", "permute-32-128", "", VarImm, true},
		{"Uint32x16", "", "Uint32x16", "permute-32-128", "", VarImm, true},
		{"Int16x8", "", "Int16x8", "permute-low-16-128", "", VarImm, true},
		{"Uint16x32", "", "Uint16x32", "permute-high-16-128", "", VarImm, true},
		{"Float32x8", "Float32x8", "Float32x8", "concat-select-128", "", VarImm, true},
		{"Float64x8", "Float64x8", "Float64x8", "concat-select-128", "", VarImm, true},
		{"Int8x32", "Int8x32", "Int8x32", "concat-permute-128", "II", VarImm, true},
		{"Uint8x64", "Uint8x64", "Uint8x64", "concat-shift-bytes-128", "2I", VarImm, true},
		{"Float32x4", "", "Float32x4", "permute-32-128", "", VarImm, false},
		{"Int16x8", "", "Int16x8", "permute-32-128", "", VarImm, false},
		{"Int32x4", "", "Int32x4", "permute-low-16-128", "", VarImm, false},
		{"Int32x4", "", "Int32x4", "permute-32-128", "", NoImm, false},
		{"Int32x4", "", "Int32x4", "permute-32-128", "", ConstImm, false},
		{"Int32x4", "", "Int32x4", "permute-32-128", "21", VarImm, false},
		{"Float32x8", "Int32x8", "Float32x8", "concat-select-128", "", VarImm, false},
		{"Float64x4", "Float64x4", "Float64x2", "concat-select-128", "", VarImm, false},
		{"Int16x8", "Int16x8", "Int16x8", "concat-select-128", "", VarImm, false},
		{"Int8x64", "Int8x64", "Int8x64", "concat-permute-128", "II", VarImm, false},
		{"Int8x32", "Int8x32", "Int8x32", "concat-permute-128", "", VarImm, false},
		{"Uint16x8", "Uint16x8", "Uint16x8", "concat-shift-bytes-128", "2I", VarImm, false},
		{"Int8x16", "Int8x16", "Int8x16", "concat-shift-bytes-128", "2I", VarImm, false},
	} {
		t.Run(test.lowering+"/"+test.x+"/"+test.y+"/"+test.out, func(t *testing.T) {
			op := Operation{Go: "Shuffle", In: []Operand{operand(test.x)}, Out: []Operand{operand(test.out)}, OperandOrder: &test.order}
			if test.y != "" {
				op.In = append(op.In, operand(test.y))
			}
			defer func() {
				failed := recover() != nil
				if failed == test.valid {
					t.Errorf("shape accepted=%v, want %v", !failed, test.valid)
				}
			}()
			validateGoALLCLowering(op, op, test.lowering, PureVregIn, OneVregOut, NoMask, test.imm)
		})
	}
}
