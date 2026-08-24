// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64

import (
	"debug/elf"
	"testing"
)

func TestELFRelocTypeForPCRel(t *testing.T) {
	tests := []struct {
		name string
		insn uint32
		want elf.R_AARCH64
	}{
		{"ADRP", 0x90000000, elf.R_AARCH64_ADR_PREL_PG_HI21},
		{"ADD", 0x91000000, elf.R_AARCH64_ADD_ABS_LO12_NC},
		{"STRB", 0x39000000, elf.R_AARCH64_LDST8_ABS_LO12_NC},
		{"STRH", 0x79000000, elf.R_AARCH64_LDST16_ABS_LO12_NC},
		{"STRW", 0xb9000000, elf.R_AARCH64_LDST32_ABS_LO12_NC},
		{"STRX", 0xf9000000, elf.R_AARCH64_LDST64_ABS_LO12_NC},
		{"STRQ", 0x3d800000, elf.R_AARCH64_LDST128_ABS_LO12_NC},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := elfRelocTypeForPCRel(test.insn)
			if !ok || got != test.want {
				t.Fatalf("elfRelocTypeForPCRel(%#x) = (%v, %v), want (%v, true)", test.insn, got, ok, test.want)
			}
		})
	}

	if got, ok := elfRelocTypeForPCRel(0xd503201f); ok {
		t.Fatalf("elfRelocTypeForPCRel(NOP) = (%v, true), want (_, false)", got)
	}
}
