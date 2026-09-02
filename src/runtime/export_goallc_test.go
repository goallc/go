// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

const (
	GoALLCCPUFeatureSSE3ForTest         = goallcCPUFeatureSSE3
	GoALLCCPUFeatureSSSE3ForTest        = goallcCPUFeatureSSSE3
	GoALLCCPUFeatureSSE41ForTest        = goallcCPUFeatureSSE41
	GoALLCCPUFeatureSSE42ForTest        = goallcCPUFeatureSSE42
	GoALLCCPUFeatureAVXForTest          = goallcCPUFeatureAVX
	GoALLCCPUFeatureFMAForTest          = goallcCPUFeatureFMA
	GoALLCCPUFeaturesInitializedForTest = goallcCPUFeaturesInitialized
	GoALLCCPUFeaturePOPCNTForTest       = goallcCPUFeaturePOPCNT
	GoALLCCPUFeatureARM64LSEForTest     = goallcCPUFeatureARM64LSE
	GoALLCCPUFeatureAVX2ForTest         = goallcCPUFeatureAVX2
	GoALLCCPUFeatureAVX512ForTest       = goallcCPUFeatureAVX512
)

func GoALLCCPUFeaturesForTest() uint64 {
	return goallcCPUFeatures
}
