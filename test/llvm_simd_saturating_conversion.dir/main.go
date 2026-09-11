// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

var enabledCases, fallbackCases, skippedCases int

func main() {
	checkCommon()
	checkArch()
	// Optional execution evidence for real-machine and SDE runs. The normal
	// testdir invocation remains silent.
	if os.Getenv("GOALLC_SIMD_SAT_TRACE") == "1" {
		fmt.Printf("saturating conversions: enabled=%d fallback=%d skipped=%d\n", enabledCases, fallbackCases, skippedCases)
	}
}
