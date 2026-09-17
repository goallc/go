// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"testing"
)

func TestLLVMFailureSelection(t *testing.T) {
	old := llvmFailures
	t.Cleanup(func() { llvmFailures = old })
	t.Setenv("GOEXPERIMENT", "simd")
	llvmFailures.Tests = map[string][]llvmTestFailure{
		"p": {{Test: "Test/file.go", Reason: "fixture"}, {Test: "TestSIMD", Reason: "requires simd", WithoutExperiment: "simd"}},
	}
	if got := llvmSkipPattern("p"); got != "^Test$/^file\\.go$" {
		t.Fatal(got)
	}
	if got := llvmSkipPattern("other"); got != "" {
		t.Fatal(got)
	}
	os.Setenv("GOEXPERIMENT", "simd,nosimd")
	if got := llvmSkipPattern("p"); got != "^Test$/^file\\.go$|^TestSIMD$" {
		t.Fatal(got)
	}
}
