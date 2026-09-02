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
		Lowering:  "add",
		Width:     256,
		Lane:      "int",
		LaneBits:  8,
		Lanes:     32,
		Input:     "pure-vreg",
		Output:    "vreg",
		Immediate: "none",
		Mask:      "none",
		Memory:    "none",
		Inputs:    "vreg:Int8x32|vreg:Int8x32",
		Outputs:   "vreg:Int8x32",
		Arch: map[string]SIMDArchData{
			"amd64": {
				CPUFeature:    "AVX2",
				CPUProfile:    "x86.avx2",
				Input:         "pure-vreg",
				Output:        "vreg",
				Immediate:     "none",
				Mask:          "none",
				Inputs:        "vreg:Int8x32|vreg:Int8x32",
				Outputs:       "vreg:Int8x32",
				MemoryFeature: "vbcst",
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
		"arm64": {CPUFeature: "SVE", Input: "pure-vreg", Output: "vreg", Immediate: "none", Mask: "none"},
	}
	merged, err := MergeSIMDOpData("AddInt8x32", amd64, arm64)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Arch) != 2 || merged.Arch["amd64"].CPUProfile != "x86.avx2" || merged.Arch["arm64"].CPUFeature != "SVE" {
		t.Fatalf("merged descriptor lost architecture data: %#v", merged.Arch)
	}

	mismatch := arm64
	mismatch.Lanes = 16
	if _, err := MergeSIMDOpData("AddInt8x32", amd64, mismatch); err == nil || !strings.Contains(err.Error(), "inconsistent generic descriptors") {
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
