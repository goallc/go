// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime_test

import (
	"internal/cpu"
	"runtime"
	"testing"
)

func TestGoALLCCPUFeaturesSnapshot(t *testing.T) {
	want := uint64(runtime.GoALLCCPUFeaturesInitializedForTest)
	for _, feature := range []struct {
		enabled bool
		bit     uint64
	}{
		{cpu.X86.HasSSE3, runtime.GoALLCCPUFeatureSSE3ForTest},
		{cpu.X86.HasSSSE3, runtime.GoALLCCPUFeatureSSSE3ForTest},
		{cpu.X86.HasSSE41, runtime.GoALLCCPUFeatureSSE41ForTest},
		{cpu.X86.HasSSE42, runtime.GoALLCCPUFeatureSSE42ForTest},
		{cpu.X86.HasAVX, runtime.GoALLCCPUFeatureAVXForTest},
		{cpu.X86.HasAVX2, runtime.GoALLCCPUFeatureAVX2ForTest},
		{cpu.X86.HasAVX512, runtime.GoALLCCPUFeatureAVX512ForTest},
		{cpu.X86.HasAVX512BITALG, runtime.GoALLCCPUFeatureAVX512BITALGForTest},
		{cpu.X86.HasAVX512VPOPCNTDQ, runtime.GoALLCCPUFeatureAVX512VPOPCNTDQForTest},
		{cpu.X86.HasFMA, runtime.GoALLCCPUFeatureFMAForTest},
		{cpu.X86.HasPOPCNT, runtime.GoALLCCPUFeaturePOPCNTForTest},
	} {
		if feature.enabled {
			want |= feature.bit
		}
	}
	if got := runtime.GoALLCCPUFeaturesForTest(); got != want {
		t.Fatalf("goallcCPUFeatures = %#x, want effective internal/cpu snapshot %#x", got, want)
	}
}

func TestGoALLCCPUWideFeatureABI(t *testing.T) {
	if got, want := runtime.GoALLCCPUFeatureAVX2ForTest, uint64(1<<9); got != want {
		t.Fatalf("AVX2 feature bit = %#x, want append-only ABI bit %#x", got, want)
	}
	if got, want := runtime.GoALLCCPUFeatureAVX512ForTest, uint64(1<<10); got != want {
		t.Fatalf("AVX-512 feature bit = %#x, want append-only ABI bit %#x", got, want)
	}
	if got, want := runtime.GoALLCCPUFeatureAVX512BITALGForTest, uint64(1<<11); got != want {
		t.Fatalf("AVX-512 BITALG feature bit = %#x, want append-only ABI bit %#x", got, want)
	}
	if got, want := runtime.GoALLCCPUFeatureAVX512VPOPCNTDQForTest, uint64(1<<12); got != want {
		t.Fatalf("AVX-512 VPOPCNTDQ feature bit = %#x, want append-only ABI bit %#x", got, want)
	}
}
