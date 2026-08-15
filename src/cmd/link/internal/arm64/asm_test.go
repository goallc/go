// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64

import "testing"

func TestClassifyPCRelInstruction(t *testing.T) {
	tests := []struct {
		name string
		inst uint32
		want arm64PCRelInstruction
		ok   bool
	}{
		{"adrp", 0x90000008, arm64PCRelADRP, true},
		{"add", 0x91000008, arm64PCRelADD, true},
		{"ldrb", 0x39400100, arm64PCRelLDST8, true},
		{"ldrh", 0x79400100, arm64PCRelLDST16, true},
		{"ldrw", 0xb9400100, arm64PCRelLDST32, true},
		{"ldrx", 0xf9400100, arm64PCRelLDST64, true},
		{"strq", 0x3d800000, arm64PCRelLDST128, true},
		{"nop", 0xd503201f, 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classifyPCRelInstruction(test.inst)
			if got != test.want || ok != test.ok {
				t.Fatalf("classifyPCRelInstruction(%#08x) = (%d, %t), want (%d, %t)", test.inst, got, ok, test.want, test.ok)
			}
		})
	}
}
