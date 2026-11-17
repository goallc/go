// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap && cgo && staticllvm && goallcplugin

package ld

import (
	"os"
	"testing"
)

func TestStaticLLVMVersionWithoutPayload(t *testing.T) {
	t.Setenv("GOROOT", t.TempDir())
	t.Setenv("GOALLC_LLVM_DIR", t.TempDir())
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })
	os.Args = []string{"link", "-lto=thin", "-V=full"}
	first := linkVersionFlagFullSuffix("action-one/content-one")
	if first != linkVersionFlagFullSuffix("action-two/content-one") {
		t.Fatal("bootstrap action changed static linker identity")
	}
	if first == linkVersionFlagFullSuffix("action-one/content-two") {
		t.Fatal("linked linker content did not change static linker identity")
	}
}
