// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"internal/testenv"
	"path/filepath"
	"testing"
)

func TestLinknameVarPreservesWrapperIdentity(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	// A zero-initialized linkname variable must not replace the runtime's
	// Dupok ABI wrapper and thereby hide its linknamestd restriction. Check
	// the actual diagnostic: a later linker panic is not a valid rejection.
	cmd := goCmd(t, "build", "-o", filepath.Join(t.TempDir(), "main.exe"), "./testdata/linkname/coro_var.go")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("invalid reference to runtime.newcoro")) {
		t.Fatalf("want invalid linkname reference, got %v\n%s", err, out)
	}
}
