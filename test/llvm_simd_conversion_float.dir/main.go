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
	if os.Getenv("GOALLC_SIMD_FLOAT_TRACE") == "1" {
		fmt.Printf("floating conversions: enabled=%d fallback=%d skipped=%d\n", enabledCases, fallbackCases, skippedCases)
	}
}
