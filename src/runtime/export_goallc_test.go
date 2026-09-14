// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

const (
	GoALLCCPUFeatureSSE3ForTest            = goallcCPUFeatureSSE3
	GoALLCCPUFeatureSSSE3ForTest           = goallcCPUFeatureSSSE3
	GoALLCCPUFeatureSSE41ForTest           = goallcCPUFeatureSSE41
	GoALLCCPUFeatureSSE42ForTest           = goallcCPUFeatureSSE42
	GoALLCCPUFeatureAVXForTest             = goallcCPUFeatureAVX
	GoALLCCPUFeatureFMAForTest             = goallcCPUFeatureFMA
	GoALLCCPUFeaturesInitializedForTest    = goallcCPUFeaturesInitialized
	GoALLCCPUFeaturePOPCNTForTest          = goallcCPUFeaturePOPCNT
	GoALLCCPUFeatureARM64LSEForTest        = goallcCPUFeatureARM64LSE
	GoALLCCPUFeatureARM64PMULLForTest      = goallcCPUFeatureARM64PMULL
	GoALLCCPUFeatureAVX2ForTest            = goallcCPUFeatureAVX2
	GoALLCCPUFeatureAVX512ForTest          = goallcCPUFeatureAVX512
	GoALLCCPUFeatureAVX512BITALGForTest    = goallcCPUFeatureAVX512BITALG
	GoALLCCPUFeatureAVX512VPOPCNTDQForTest = goallcCPUFeatureAVX512VPOPCNTDQ
	GoALLCCPUFeatureAVX512VBMIForTest      = goallcCPUFeatureAVX512VBMI
	GoALLCCPUFeatureAVX512VBMI2ForTest     = goallcCPUFeatureAVX512VBMI2
	GoALLCCPUFeatureAESForTest             = goallcCPUFeatureAES
	GoALLCCPUFeaturePCLMULQDQForTest       = goallcCPUFeaturePCLMULQDQ
	GoALLCCPUFeatureVAESForTest            = goallcCPUFeatureVAES

	GoALLCCPUFeatureVPCLMULQDQForTest       = goallcCPUFeatureVPCLMULQDQ
	GoALLCCPUFeatureAVX512VPCLMULQDQForTest = goallcCPUFeatureAVX512VPCLMULQDQ
)

func GoALLCCPUFeaturesForTest() uint64 {
	return goallcCPUFeatures
}
