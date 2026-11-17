// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

//go:build !compiler_bootstrap && cgo

package ld

import (
	"internal/buildcfg"
	"os"
	"testing"
)

func TestLTOVersionMode(t *testing.T) {
	for _, test := range []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-lto=thin", "-V=full"}, true},
		{[]string{"--lto", "thin", "-V=full"}, true},
		{[]string{"-lto=thin", "-lto="}, false},
		{[]string{"-lto=full"}, false},
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

func TestNonLTOVersionIdentity(t *testing.T) {
	oldArgs, oldVersion := os.Args, buildcfg.Version
	t.Cleanup(func() { os.Args, buildcfg.Version = oldArgs, oldVersion })
	os.Args = []string{"link", "-V=full"}
	buildcfg.Version = "go1.27.3-goallc.test"
	if got := linkVersionFlagFullSuffix("action/content"); got != "" {
		t.Fatalf("release suffix = %q, want empty", got)
	}
	buildcfg.Version = "go1.27-devel"
	if got := linkVersionFlagFullSuffix("action/content"); got != " buildID=action/content" {
		t.Fatalf("development suffix = %q", got)
	}
}
