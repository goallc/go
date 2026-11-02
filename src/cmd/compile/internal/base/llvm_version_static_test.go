// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap && staticllvm

package base

import (
	"os"
	"testing"
)

func TestStaticLLVMVersionWithoutPayload(t *testing.T) {
	t.Setenv("GOROOT", t.TempDir())
	t.Setenv("GOALLC_LLVM_DIR", t.TempDir())
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"compile", "-V=full"}
	first := compileVersionFlagFullSuffix("action-one/content-one")
	if first != compileVersionFlagFullSuffix("action-two/content-one") {
		t.Fatal("bootstrap action changed static compiler identity")
	}
	if first == compileVersionFlagFullSuffix("action-one/content-two") {
		t.Fatal("linked compiler content did not change static compiler identity")
	}
}
