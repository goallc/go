// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import "testing"

func TestLLVMValueIsWriteBarrierTombstone(t *testing.T) {
	tests := []struct {
		name string
		v    *Value
		want bool
	}{
		{name: "dead invalid", v: &Value{Op: OpInvalid}, want: true},
		{name: "live invalid", v: &Value{Op: OpInvalid, Uses: 1}},
		{name: "invalid with argument", v: &Value{Op: OpInvalid, Args: []*Value{{}}}},
		{name: "invalid with auxiliary value", v: &Value{Op: OpInvalid, Aux: AuxMark}},
		{name: "ordinary dead value", v: &Value{Op: OpConstNil}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmValueIsWriteBarrierTombstone(test.v); got != test.want {
				t.Fatalf("llvmValueIsWriteBarrierTombstone(%s, uses=%d) = %t, want %t", test.v.Op, test.v.Uses, got, test.want)
			}
		})
	}
}
