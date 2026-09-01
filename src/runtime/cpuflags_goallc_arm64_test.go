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
	if cpu.ARM64.HasATOMICS {
		want |= runtime.GoALLCCPUFeatureARM64LSEForTest
	}
	if got := runtime.GoALLCCPUFeaturesForTest(); got != want {
		t.Fatalf("goallcCPUFeatures = %#x, want effective internal/cpu snapshot %#x", got, want)
	}
}
