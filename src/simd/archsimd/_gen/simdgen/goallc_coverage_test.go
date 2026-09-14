// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Check the merged, checked-in opcode table as well as the generator's current
// architecture: neither amd64 nor arm64 should retain unimplemented operations.
func TestGoALLCSIMDCoversGeneratedOps(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "cmd", "compile", "internal", "ssa", "_gen", "simdgenericOps.go")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		_, archs, ok := strings.Cut(line, "// ARCH:")
		if !ok {
			continue
		}
		arches := strings.Split(strings.TrimSpace(archs), ",")
		if !slices.Contains(arches, "amd64") && !slices.Contains(arches, "arm64") {
			continue
		}
		if !strings.Contains(line, `simd: "`) {
			t.Errorf("missing LLVM lowering: %s", line)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no generated SIMD operations found")
	}
	t.Logf("implemented amd64/arm64 SIMD operations: %d", count)
}
