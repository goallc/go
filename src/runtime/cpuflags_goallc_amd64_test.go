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
		{cpu.X86.HasFMA, runtime.GoALLCCPUFeatureFMAForTest},
	} {
		if feature.enabled {
			want |= feature.bit
		}
	}
	if got := runtime.GoALLCCPUFeaturesForTest(); got != want {
		t.Fatalf("goallcCPUFeatures = %#x, want effective internal/cpu snapshot %#x", got, want)
	}
}
