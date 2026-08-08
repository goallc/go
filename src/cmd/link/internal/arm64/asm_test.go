// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64

import (
	"cmd/link/internal/ld"
	"testing"
)

func TestMachoPCRelRelocType(t *testing.T) {
	tests := []struct {
		name      string
		inst      uint32
		machoType uint32
		pcrel     bool
		ok        bool
	}{
		{"adrp", 0x90000000, ld.MACHO_ARM64_RELOC_PAGE21, true, true},
		{"add", 0x91000000, ld.MACHO_ARM64_RELOC_PAGEOFF12, false, true},
		{"ldr64", 0xf9400000, ld.MACHO_ARM64_RELOC_PAGEOFF12, false, true},
		{"str128", 0x3d800000, ld.MACHO_ARM64_RELOC_PAGEOFF12, false, true},
		{"unsupported", 0xd503201f, 0, false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machoType, pcrel, ok := machoPCRelRelocType(test.inst)
			if machoType != test.machoType || pcrel != test.pcrel || ok != test.ok {
				t.Fatalf("machoPCRelRelocType(%#08x) = (%d, %t, %t), want (%d, %t, %t)",
					test.inst, machoType, pcrel, ok, test.machoType, test.pcrel, test.ok)
			}
		})
	}
}
