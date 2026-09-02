// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func resetWasmOpsForTest() {
	wasmOps = nil
	for _, typ := range append(append([]*simdType{}, allTypes...), masks...) {
		typ.Methods = make(map[string]*wasmOp)
	}
}

func TestGoALLCDescriptorAnnotations(t *testing.T) {
	resetWasmOpsForTest()
	t.Cleanup(resetWasmOpsForTest)
	initWasmOps()
	bySSAName := make(map[string]*wasmOp)
	for _, op := range wasmOps {
		if op.DefinesGeneric() {
			bySSAName[op.SsaGenOp()] = op
		}
	}

	for _, test := range []struct {
		name     string
		lowering string
		lane     string
		bits     int
		lanes    int
	}{
		{name: "AddInt8x16", lowering: "add", lane: "int", bits: 8, lanes: 16},
		{name: "NotEqualUint64x2", lowering: "not-equal", lane: "uint", bits: 64, lanes: 2},
		{name: "DivFloat64x2", lowering: "div", lane: "float", bits: 64, lanes: 2},
	} {
		op := bySSAName[test.name]
		if op == nil {
			t.Fatalf("missing wasm generic op %s", test.name)
		}
		d := op.goALLCDescriptor()
		if d.Lowering != test.lowering || d.Width != 128 || d.Lane != test.lane || d.LaneBits != test.bits || d.Lanes != test.lanes {
			t.Errorf("%s descriptor = %#v", test.name, d)
		}
		if d.Arch["wasm"].CPUFeature != "simd128" {
			t.Errorf("%s did not preserve the wasm feature: %#v", test.name, d.Arch)
		}
	}

	unmarked := bySSAName["AddSaturatedInt8x16"]
	if unmarked == nil {
		t.Fatal("missing unmarked saturated wasm op")
	}
	if d := unmarked.goALLCDescriptor(); !d.IsZero() {
		t.Fatalf("unmarked wasm op received an inferred lowering: %#v", d)
	}
}
