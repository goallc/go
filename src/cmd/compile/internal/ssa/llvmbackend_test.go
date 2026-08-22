// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"slices"
	"testing"
)

func TestLLVMCodeGenOptions(t *testing.T) {
	want := []string{"-trap-unreachable", "-disable-machine-cse", "-disable-lsr"}
	if got := llvmCodeGenOptions(); !slices.Equal(got, want) {
		t.Errorf("llvmCodeGenOptions() = %q, want %q", got, want)
	}
}
