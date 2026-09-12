// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGoALLCDynamicShuffleShape(t *testing.T) {
	operand := func(name string) Operand {
		_, bits, lanes, _ := goALLCLaneFromGoType(&name)
		width := bits * lanes
		return Operand{Class: "vreg", Go: &name, Bits: &width}
	}
	for _, test := range []struct {
		x, y, indices, out, lowering, order string
		valid                               bool
	}{
		{"Float32x8", "", "Uint32x8", "Float32x8", "permute", "21Type1", true},
		{"Int8x64", "", "Uint8x64", "Int8x64", "permute", "21Type1", true},
		{"Float64x8", "Float64x8", "Uint64x8", "Float64x8", "concat-permute", "231Type1", true},
		{"Int8x16", "", "Int8x16", "Int8x16", "lookup-or-zero", "", true},
		{"Uint8x16", "", "Uint8x16", "Uint8x16", "lookup-or-zero", "", true},
		{"Uint8x16", "", "Int8x16", "Uint8x16", "permute-or-zero", "", true},
		{"Int8x64", "", "Int8x64", "Int8x64", "permute-or-zero-128", "", true},
		{"Float32x8", "", "Float32x8", "Float32x8", "permute", "21Type1", false},
		{"Float8x16", "", "Uint8x16", "Float8x16", "permute", "21Type1", false},
		{"Int32x4", "", "Uint32x4", "Int32x4", "permute", "21Type1", false},
		{"Float32x8", "", "Int32x8", "Float32x8", "permute", "21Type1", false},
		{"Float32x8", "", "Uint16x16", "Float32x8", "permute", "21Type1", false},
		{"Float32x8", "", "Uint32x4", "Float32x8", "permute", "21Type1", false},
		{"Float32x8", "", "Uint32x8", "Float32x4", "permute", "21Type1", false},
		{"Float32x8", "", "Uint32x8", "Float32x8", "permute", "", false},
		{"Float64x8", "Int64x8", "Uint64x8", "Float64x8", "concat-permute", "231Type1", false},
		{"Float64x8", "", "Uint64x8", "Float64x8", "concat-permute", "231Type1", false},
		{"Int8x32", "", "Int8x32", "Int8x32", "lookup-or-zero", "", false},
		{"Uint8x16", "", "Int8x16", "Uint8x16", "lookup-or-zero", "", false},
		{"Uint8x16", "", "Uint8x16", "Uint8x16", "permute-or-zero", "", false},
		{"Int8x32", "", "Int8x32", "Int8x32", "permute-or-zero", "", false},
		{"Int8x16", "", "Int8x16", "Int8x16", "permute-or-zero-128", "", false},
	} {
		t.Run(test.lowering+"/"+test.x+"/"+test.indices, func(t *testing.T) {
			op := Operation{Go: "Shuffle", In: []Operand{operand(test.x)}, Out: []Operand{operand(test.out)}, OperandOrder: &test.order}
			if test.y != "" {
				op.In = append(op.In, operand(test.y))
			}
			op.In = append(op.In, operand(test.indices))
			if test.lowering == "permute" || test.lowering == "concat-permute" {
				op.In = append([]Operand{op.In[len(op.In)-1]}, op.In[:len(op.In)-1]...)
			}
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

func TestGoALLCDynamicShuffleListAndControlShape(t *testing.T) {
	name, width, zero, one := "Int8x16", 128, 0, 1
	operand := Operand{Class: "vreg", Go: &name, Bits: &width}
	for _, test := range []struct {
		list  *int
		input inShape
		mask  maskShape
		imm   immShape
		valid bool
	}{
		{&zero, VlistIn, NoMask, NoImm, true},
		{&one, VlistIn, NoMask, NoImm, false},
		{nil, PureVregIn, NoMask, VarImm, false},
		{nil, PureVregIn, OneMask, NoImm, false},
	} {
		op := Operation{Go: "LookupOrZero", In: []Operand{operand, operand}, Out: []Operand{operand}}
		op.In[0].ListNumber = test.list
		func() {
			defer func() {
				if failed := recover() != nil; failed == test.valid {
					t.Errorf("list/control shape accepted=%v, want %v", !failed, test.valid)
				}
			}()
			validateGoALLCLowering(op, op, "lookup-or-zero", test.input, OneVregOut, test.mask, test.imm)
		}()
	}
	if got := goALLCCPUProfile("amd64", "AVX512VBMI"); got != "x86.avx512vbmi" {
		t.Fatalf("VBMI profile = %q", got)
	}
}
