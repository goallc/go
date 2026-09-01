// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/cpu"
	"internal/runtime/atomic"
	"testing"
)

func TestGoALLCCPUFeaturesSnapshot(t *testing.T) {
	want := uint64(goallcCPUFeaturesInitialized)
	for _, feature := range []struct {
		enabled bool
		bit     uint64
	}{
		{cpu.X86.HasSSE3, goallcCPUFeatureSSE3},
		{cpu.X86.HasSSSE3, goallcCPUFeatureSSSE3},
		{cpu.X86.HasSSE41, goallcCPUFeatureSSE41},
		{cpu.X86.HasSSE42, goallcCPUFeatureSSE42},
		{cpu.X86.HasAVX, goallcCPUFeatureAVX},
		{cpu.X86.HasFMA, goallcCPUFeatureFMA},
	} {
		if feature.enabled {
			want |= feature.bit
		}
	}
	if got := atomic.Load64(&goallcCPUFeatures); got != want {
		t.Fatalf("goallcCPUFeatures = %#x, want effective internal/cpu snapshot %#x", got, want)
	}
}
