// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmbackend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPassPluginFromPayload(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	if err := os.Mkdir(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := passPluginFilename()
	if err != nil {
		t.Skip(err)
	}
	plugin := filepath.Join(lib, name)
	if err := os.WriteFile(plugin, []byte("plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOALLC_LLVM_DIR", root)
	got, err := PassPlugin()
	if err != nil {
		t.Fatal(err)
	}
	if got != plugin {
		t.Fatalf("PassPlugin() = %q, want %q", got, plugin)
	}
}

func TestIdentityTracksRuntimeFiles(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	if err := os.Mkdir(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := passPluginFilename()
	if err != nil {
		t.Skip(err)
	}
	plugin := filepath.Join(lib, name)
	if err := os.WriteFile(plugin, []byte("plugin one"), 0o644); err != nil {
		t.Fatal(err)
	}
	llvm := filepath.Join(lib, "libLLVM.test.dylib")
	if err := os.WriteFile(llvm, []byte("llvm one"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOALLC_LLVM_DIR", root)

	first, err := Identity()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, []byte("plugin two"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Identity()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("plugin content change did not alter LLVM backend identity")
	}
	if err := os.WriteFile(llvm, []byte("llvm two"), 0o644); err != nil {
		t.Fatal(err)
	}
	third, err := Identity()
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("libLLVM content change did not alter LLVM backend identity")
	}
}
