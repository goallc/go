// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"simd/archsimd/_gen/internal/goallccpu"
	"testing"
)

func TestGoALLCCPUProfileAliases(t *testing.T) {
	for _, test := range []struct{ feature, want string }{
		{"AVX", "x86.avx"}, {"AVX2", "x86.avx2"}, {"AVX512", "x86.avx512"},
		{"AVXAES", "x86.avxaes"}, {"AVXPCLMULQDQ", "x86.avxpclmulqdq"}, {"VAES", "x86.vaes"},
		{"AVX512VBMI", "x86.avx512vbmi"}, {"AVX512BITALG", "x86.avx512bitalg"},
		{"AVX512VPOPCNTDQ", "x86.avx512vpopcntdq"}, {"FMA", "x86.fma"}, {"SHA", ""}, {"", ""},
	} {
		if got := goallccpu.ProfileForSIMD("amd64", test.feature); got != test.want {
			t.Errorf("%s = %q, want %q", test.feature, got, test.want)
		}
	}
	if got := goallccpu.ProfileForSIMD("arm64", "AVX512VBMI"); got != "" {
		t.Errorf("cross-architecture profile %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("unknown AVX512 extension was accepted")
		}
	}()
	goallccpu.ProfileForSIMD("amd64", "AVX512FUTURE")
}
