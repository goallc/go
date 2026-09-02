// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgutil

import (
	"reflect"
	"strings"
	"testing"
)

func testSIMDOpData() SIMDOpData {
	return SIMDOpData{
		Lowering: "add",
		Lane:     "int",
		LaneBits: 8,
		Arch: map[string]SIMDArchData{
			"amd64": {
				CPUProfile: "x86.avx2",
			},
		},
	}
}

func TestSIMDOpDataRoundTrip(t *testing.T) {
	want := testSIMDOpData()
	encoded := EncodeSIMDOpData(want)
	got, err := DecodeSIMDOpData(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descriptor round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMergeSIMDOpData(t *testing.T) {
	amd64 := testSIMDOpData()
	arm64 := testSIMDOpData()
	arm64.Arch = map[string]SIMDArchData{
		"arm64": {},
	}
	merged, err := MergeSIMDOpData("AddInt8x32", amd64, arm64)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Arch) != 2 || merged.Arch["amd64"].CPUProfile != "x86.avx2" {
		t.Fatalf("merged descriptor lost architecture data: %#v", merged.Arch)
	}

	mismatch := arm64
	mismatch.Lowering = "sub"
	if _, err := MergeSIMDOpData("AddInt8x32", amd64, mismatch); err == nil || !strings.Contains(err.Error(), "inconsistent GoALLC lowering kinds") {
		t.Fatalf("inconsistent LLVM descriptor was accepted: %v", err)
	}
}

func TestMergeSIMDOpDataWithUnsupportedArchitecture(t *testing.T) {
	want := testSIMDOpData()
	merged, err := MergeSIMDOpData("AddInt8x32", SIMDOpData{}, want)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merging an unsupported architecture changed the descriptor:\n got: %#v\nwant: %#v", merged, want)
	}
}

func TestSIMDOpDataWithoutLastArchPreservesGenericSemantics(t *testing.T) {
	d := testSIMDOpData().WithoutArch("amd64")
	if d.Lowering != "add" || d.Lane != "int" || d.LaneBits != 8 || len(d.Arch) != 0 {
		t.Fatalf("removing architecture-specific data changed generic semantics: %#v", d)
	}
}
