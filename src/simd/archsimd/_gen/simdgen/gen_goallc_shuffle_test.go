// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestGoALLCStaticShuffleShape(t *testing.T) {
	operand := func(name string) Operand {
		_, bits, lanes, _ := goALLCLaneFromGoType(&name)
		width := bits * lanes
		return Operand{Class: "vreg", Go: &name, Bits: &width}
	}
	for _, test := range []struct {
		x, y, out, lowering string
		valid               bool
	}{
		{"Int8x32", "", "Int8x16", "get-low", true},
		{"Float64x8", "", "Float64x4", "get-high", true},
		{"Int32x8", "Int32x4", "Int32x8", "set-low", true},
		{"Uint64x8", "Uint64x4", "Uint64x8", "set-high", true},
		{"Int8x16", "", "Int8x64", "broadcast-low", true},
		{"Float64x2", "", "Float64x2", "broadcast-low", true},
		{"Int32x4", "Int32x4", "Int32x4", "interleave-low", true},
		{"Uint16x32", "Uint16x32", "Uint16x32", "interleave-high-128", true},
		{"Int8x16", "Int8x16", "Int8x16", "concat-even", true},
		{"Int64x2", "Int64x2", "Int64x2", "interleave-odd", true},
		{"Int8x16", "", "Int8x16", "get-low", false},
		{"Int32x8", "", "Float32x4", "get-high", false},
		{"Int32x8", "Int32x8", "Int32x8", "set-low", false},
		{"Int32x8", "Uint32x4", "Int32x8", "set-high", false},
		{"Int8x32", "", "Int8x64", "broadcast-low", false},
		{"Float32x4", "", "Float64x4", "broadcast-low", false},
		{"Int8x16", "", "Int8x8", "broadcast-low", false},
		{"Int16x8", "Int16x16", "Int16x8", "interleave-low", false},
		{"Int16x8", "Int16x8", "Int16x16", "concat-even", false},
	} {
		t.Run(test.lowering+"/"+test.x+"/"+test.y+"/"+test.out, func(t *testing.T) {
			op := Operation{Go: "Shuffle", In: []Operand{operand(test.x)}, Out: []Operand{operand(test.out)}}
			if test.y != "" {
				op.In = append(op.In, operand(test.y))
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

func TestGoALLCMaskedBroadcastRemainsPlanned(t *testing.T) {
	lowering := "broadcast-low"
	op := Operation{LLVMLowering: &lowering}
	if got := goALLCSIMDDescriptor(op, op, OneKmaskIn, OneVregOut, OneMask, NoImm); !got.IsZero() {
		t.Fatal("unmasked broadcast recipe must not claim masked semantics")
	}
	if plan, ok := goALLCSIMDPlanForGenericOp("broadcast1To4MaskedInt32x4"); !ok || plan != goALLCSIMDPlanMask {
		t.Fatal("masked broadcast must remain in the reviewed mask plan")
	}
}
