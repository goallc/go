// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

//go:build !compiler_bootstrap && cgo

package ld

import "testing"

func TestLTOVersionMode(t *testing.T) {
	for _, test := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-lto=full", "-V=full"}, true},
		{[]string{"--lto", "full", "-V=full"}, true},
		{[]string{"-lto=full", "-lto="}, false},
		{[]string{"-lto=thin"}, false},
	} {
		if got := ltoVersionEnabled(test.args); got != test.want {
			t.Errorf("%v: got %v, want %v", test.args, got, test.want)
		}
	}
	if llvmVersionBuildID("action-a/content", "llvm-a") != llvmVersionBuildID("action-b/content", "llvm-a") {
		t.Fatal("action ID changed tool content identity")
	}
	if llvmVersionBuildID("action/content", "llvm-a") == llvmVersionBuildID("action/content", "llvm-b") {
		t.Fatal("LLVM update did not change identity")
	}
}
