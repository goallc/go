// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "internal/runtime/atomic"

const (
	GoALLCCPUFeatureSSE3ForTest         = goallcCPUFeatureSSE3
	GoALLCCPUFeatureSSSE3ForTest        = goallcCPUFeatureSSSE3
	GoALLCCPUFeatureSSE41ForTest        = goallcCPUFeatureSSE41
	GoALLCCPUFeatureSSE42ForTest        = goallcCPUFeatureSSE42
	GoALLCCPUFeatureAVXForTest          = goallcCPUFeatureAVX
	GoALLCCPUFeatureFMAForTest          = goallcCPUFeatureFMA
	GoALLCCPUFeaturesInitializedForTest = goallcCPUFeaturesInitialized
)

func GoALLCCPUFeaturesForTest() uint64 {
	return atomic.Load64(&goallcCPUFeatures)
}
