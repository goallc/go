// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmbackend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPassPluginFromGoToolchain(t *testing.T) {
	goRoot := t.TempDir()
	lib := filepath.Join(goRoot, "pkg", "goallc-llvmplugin", "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
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
	payloadRoot := t.TempDir()
	payloadLib := filepath.Join(payloadRoot, "lib")
	if err := os.Mkdir(payloadLib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadLib, name), []byte("payload decoy"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOROOT", goRoot)
	t.Setenv("GOALLC_LLVM_DIR", payloadRoot)
	got, err := PassPlugin()
	if err != nil {
		t.Fatal(err)
	}
	if got != plugin {
		t.Fatalf("PassPlugin() = %q, want %q", got, plugin)
	}
}

func TestPassPluginDoesNotSearchLLVMPayload(t *testing.T) {
	name, err := passPluginFilename()
	if err != nil {
		t.Skip(err)
	}
	payloadRoot := t.TempDir()
	payloadLib := filepath.Join(payloadRoot, "lib")
	if err := os.Mkdir(payloadLib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadLib, name), []byte("payload decoy"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOROOT", t.TempDir())
	t.Setenv("GOALLC_LLVM_DIR", payloadRoot)
	if plugin, err := PassPlugin(); err == nil {
		t.Fatalf("PassPlugin() = %q, want an error for a payload-only plugin", plugin)
	}
}

func TestIdentityTracksRuntimeFiles(t *testing.T) {
	goRoot := t.TempDir()
	pluginLib := filepath.Join(goRoot, "pkg", "goallc-llvmplugin", "lib")
	if err := os.MkdirAll(pluginLib, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := passPluginFilename()
	if err != nil {
		t.Skip(err)
	}
	plugin := filepath.Join(pluginLib, name)
	if err := os.WriteFile(plugin, []byte("plugin one"), 0o644); err != nil {
		t.Fatal(err)
	}
	payloadRoot := t.TempDir()
	lib := filepath.Join(payloadRoot, "lib")
	if err := os.Mkdir(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	llvm := filepath.Join(lib, "libLLVM.test.dylib")
	if err := os.WriteFile(llvm, []byte("llvm one"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOROOT", goRoot)
	t.Setenv("GOALLC_LLVM_DIR", payloadRoot)

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
