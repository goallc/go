// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"testing"

	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
)

func TestLLVMGoObjSections(t *testing.T) {
	dataTests := []struct {
		kind objabi.SymKind
		want string
	}{
		{objabi.SRODATA, ".rodata"},
		{objabi.SRODATAFIPS, ".rodata.fips"},
		{objabi.SNOPTRDATA, ".noptrdata"},
		{objabi.SNOPTRDATAFIPS, ".noptrdata.fips"},
		{objabi.SDATA, ".data"},
		{objabi.SDATAFIPS, ".data.fips"},
	}
	for _, test := range dataTests {
		s := &obj.LSym{Name: "test", Type: test.kind}
		if got := llvmDataSection(s); got != test.want {
			t.Errorf("llvmDataSection(%s) = %q, want %q", test.kind, got, test.want)
		}
	}

	if got := llvmFunctionSection(&obj.LSym{Type: objabi.STEXT}); got != "" {
		t.Errorf("ordinary text section = %q, want default section", got)
	}
	if got := llvmFunctionSection(&obj.LSym{Type: objabi.STEXTFIPS}); got != ".text.fips" {
		t.Errorf("FIPS text section = %q, want .text.fips", got)
	}
}

func TestLLVMDescriptorGoTypeCanonicalizesPredeclaredAliases(t *testing.T) {
	tests := []struct {
		name       string
		alias      *types.Type
		underlying *types.Type
	}{
		{"any", types.AnyType, types.Types[types.TINTER]},
		{"byte", types.ByteType, types.Types[types.TUINT8]},
		{"rune", types.RuneType, types.Types[types.TINT32]},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &obj.LSym{Name: "type:" + test.name}
			s.NewTypeInfo().Type = test.alias
			if got := llvmDescriptorGoType(s); got != test.underlying {
				t.Fatalf("llvmDescriptorGoType() = %v, want %v", got, test.underlying)
			}
		})
	}
}
