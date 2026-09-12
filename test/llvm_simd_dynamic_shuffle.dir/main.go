// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

var enabledCases, fallbackCases, skippedCases, checkedControls int

func main() {
	checkArch()
	if os.Getenv("GOALLC_SIMD_SHUFFLE_TRACE") == "1" {
		fmt.Printf("dynamic shuffles: enabled=%d fallback=%d skipped=%d controls=%d\n", enabledCases, fallbackCases, skippedCases, checkedControls)
	}
}
