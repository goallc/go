// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmbackend

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPassPluginFromGoToolchain(t *testing.T) {
	t.Setenv("GOALLC_TOOLCHAIN_ROOT", "")
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

	// An explicit toolchain root must not fall back to a different GOROOT's
	// plugin when the selected toolchain is incomplete.
	t.Setenv("GOALLC_TOOLCHAIN_ROOT", t.TempDir())
	if plugin, err := PassPlugin(); err == nil {
		t.Fatalf("PassPlugin() = %q, want an error for an incomplete toolchain", plugin)
	}
}

func TestPassPluginDoesNotSearchLLVMPayload(t *testing.T) {
	t.Setenv("GOALLC_TOOLCHAIN_ROOT", "")
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
	for _, separateRoot := range []bool{false, true} {
		for _, explicitPayload := range []bool{false, true} {
			t.Run(fmt.Sprintf("toolchainRoot=%t/explicitPayload=%t", separateRoot, explicitPayload), func(t *testing.T) {
				testIdentityTracksRuntimeFiles(t, separateRoot, explicitPayload)
			})
		}
	}
}

func testIdentityTracksRuntimeFiles(t *testing.T, separateRoot, explicit bool) {
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
	t.Setenv("GOALLC_TOOLCHAIN_ROOT", "")
	t.Setenv("GOROOT", goRoot)
	if separateRoot {
		t.Setenv("GOROOT", t.TempDir())
		t.Setenv("GOALLC_TOOLCHAIN_ROOT", goRoot)
	}
	if explicit {
		t.Setenv("GOALLC_LLVM_DIR", payloadRoot)
	} else {
		t.Setenv("GOALLC_LLVM_DIR", "")
		if err := os.WriteFile(filepath.Join(goRoot, "pkg", "goallc-llvm-payload"), []byte(payloadRoot), 0o644); err != nil {
			t.Fatal(err)
		}
	}

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
