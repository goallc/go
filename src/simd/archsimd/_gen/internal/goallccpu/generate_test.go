// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goallccpu

import (
	"strings"
	"testing"
)

func TestRuntimeBitABI(t *testing.T) {
	// Existing object files contain these literal numbers. Do not regenerate
	// this compatibility fixture when adding new features to the registry.
	want := map[string]uint{
		"SSE3": 0, "SSSE3": 1, "SSE41": 2, "SSE42": 3,
		"AVX": 4, "FMA": 5, "INITIALIZED": 6, "POPCNT": 7,
		"ARM64LSE": 8, "AVX2": 9, "AVX512": 10,
		"AVX512BITALG": 11, "AVX512VPOPCNTDQ": 12, "AVX512VBMI": 13,
	}
	for _, f := range features {
		if bit, ok := want[f.Name]; ok {
			if f.Bit != bit {
				t.Errorf("%s changed ABI bit %d to %d", f.Name, bit, f.Bit)
			}
			delete(want, f.Name)
		} else if f.Bit <= 13 {
			t.Errorf("new feature %s reuses reserved bit %d", f.Name, f.Bit)
		}
	}
	if len(want) != 0 {
		t.Errorf("removed ABI features: %v", want)
	}
}

func TestPredicateAndCapabilities(t *testing.T) {
	r, err := resolve(features, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		bit     uint
		caps    uint64
		targets string
	}{
		{"SSE41", 2, 4, "sse4.1"},
		{"AVX", 4, 16, "avx"},
		{"AVX2", 9, 528, "avx,avx2"},
		{"AVX512", 10, 1552, "avx,avx2,avx512f,avx512cd,avx512bw,avx512dq,avx512vl"},
		{"AVX512BITALG", 11, 3600, "avx,avx2,avx512f,avx512cd,avx512bw,avx512dq,avx512vl,avx512bitalg"},
		{"AVX512VPOPCNTDQ", 12, 5648, "avx,avx2,avx512f,avx512cd,avx512bw,avx512dq,avx512vl,avx512vpopcntdq"},
		{"AVX512VBMI", 13, 9744, "avx,avx2,avx512f,avx512cd,avx512bw,avx512dq,avx512vl,avx512vbmi"},
		{"FMA", 5, 32, "fma"},
		{"POPCNT", 7, 128, "popcnt"},
		{"ARM64LSE", 8, 256, "lse"},
	} {
		f := r[test.name]
		if f.Bit != test.bit || f.Mask != test.caps || strings.Join(f.Targets, ",") != test.targets {
			t.Errorf("%s: got bit=%d caps=%d targets=%v, want %+v", test.name, f.Bit, f.Mask, f.Targets, test)
		}
	}
}
