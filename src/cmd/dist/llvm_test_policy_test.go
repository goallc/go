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
	oldArch := goarch
	t.Cleanup(func() { llvmFailures, goarch = old, oldArch })
	goarch = "amd64"
	t.Setenv("GOEXPERIMENT", "simd")
	llvmFailures.Tests = map[string][]llvmTestFailure{
		"p": {
			{Test: "Test/file.go", Reason: "fixture"},
			{Test: "TestSIMD", Reason: "requires simd", WithoutExperiment: "simd"},
			{Test: "TestARM64", Reason: "arm64 fixture", GOARCH: "arm64"},
		},
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
	goarch = "arm64"
	if got := llvmSkipPattern("p"); got != "^Test$/^file\\.go$|^TestSIMD$|^TestARM64$" {
		t.Fatal(got)
	}
}

func TestLLVMFailedTestMain(t *testing.T) {
	old, oldMatches := llvmFailures, stdMatches
	t.Cleanup(func() { llvmFailures, stdMatches = old, oldMatches })
	stdMatches = nil
	llvmFailures.Tests = nil
	llvmFailures.Packages = map[string]string{"broken": "TestMain build failure"}
	runner := &tester{}
	runner.registerStdTest("broken")
	runner.registerStdTest("working")
	if len(runner.tests) != 2 || len(stdMatches) != 1 || stdMatches[0] != "working" {
		t.Fatalf("tests=%v, batch=%v", runner.tests, stdMatches)
	}
	// Retain the entry in dist -list, but skip before launching TestMain.
	if err := runner.tests[0].fn(&runner.tests[0]); err != nil || len(runner.worklist) != 0 {
		t.Fatalf("skip queued work: %v, %v", err, runner.worklist)
	}
}
