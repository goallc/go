// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Keep failures explicit rather than filtering entire test recipe categories.
// GOALLC_TEST_KNOWN_FAILURES=0 runs the unfiltered suite for requalification.
var llvmFailures struct {
	Tests map[string][]llvmTestFailure `json:"tests"`
	Modes map[string]string            `json:"modes"`
	// Packages is reserved for failures in TestMain, before -skip takes effect.
	Packages map[string]string `json:"packages"`
}

type llvmTestFailure struct {
	Test              string `json:"test"`
	Reason            string `json:"reason"`
	WithoutExperiment string `json:"without_experiment,omitempty"`
	GOARCH            string `json:"goarch,omitempty"`
}

func loadLLVMTestFailures() {
	if os.Getenv("GOALLC_TEST_KNOWN_FAILURES") == "0" || strings.Contains(gogcflags, "-enablellvm=false") {
		return
	}
	filename := filepath.Join(goroot, "test", "llvm_known_failures.json")
	data, err := os.ReadFile(filename)
	if err != nil {
		fatalf("reading known LLVM failures: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&llvmFailures); err != nil {
		fatalf("%s: %v", filename, err)
	}
	for pkg, entries := range llvmFailures.Tests {
		seen := make(map[string]bool)
		for _, entry := range entries {
			if pkg == "" || entry.Test == "" || entry.Reason == "" || seen[entry.Test] {
				fatalf("invalid or duplicate known LLVM failure: %s %q", pkg, entry.Test)
			}
			seen[entry.Test] = true
		}
	}
	for name, reason := range llvmFailures.Modes {
		if !strings.Contains(name, ":") || reason == "" {
			fatalf("invalid known LLVM test mode: %q", name)
		}
	}
	for pkg, reason := range llvmFailures.Packages {
		if pkg == "" || strings.Contains(pkg, ":") || reason == "" {
			fatalf("invalid known LLVM package failure: %q", pkg)
		}
	}
}

func llvmSkipPattern(pkg string) string {
	var patterns []string
	for _, entry := range llvmFailures.Tests[pkg] {
		if entry.GOARCH != "" && entry.GOARCH != goarch {
			continue
		}
		if entry.WithoutExperiment != "" && experimentEnabled(entry.WithoutExperiment) {
			continue
		}
		parts := strings.Split(entry.Test, "/")
		for i, part := range parts {
			parts[i] = "^" + regexp.QuoteMeta(part) + "$"
		}
		patterns = append(patterns, strings.Join(parts, "/"))
		fmt.Printf("LLVM known failure: SKIP %s %s: %s\n", pkg, entry.Test, entry.Reason)
	}
	return strings.Join(patterns, "|")
}

func experimentEnabled(name string) bool {
	enabled := false
	for _, value := range strings.Split(os.Getenv("GOEXPERIMENT"), ",") {
		if value == name {
			enabled = true
		}
		if value == "no"+name || value == "none" {
			enabled = false
		}
	}
	return enabled
}
