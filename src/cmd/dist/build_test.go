// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"internal/platform"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMustLinkExternal verifies that the mustLinkExternal helper
// function matches internal/platform.MustLinkExternal.
func TestMustLinkExternal(t *testing.T) {
	oldBuildGoallc := buildGoallc
	buildGoallc = false
	defer func() {
		buildGoallc = oldBuildGoallc
	}()

	for _, goos := range okgoos {
		for _, goarch := range okgoarch {
			for _, cgoEnabled := range []bool{true, false} {
				got := mustLinkExternal(goos, goarch, cgoEnabled)
				want := platform.MustLinkExternal(goos, goarch, cgoEnabled)
				if got != want {
					t.Errorf("mustLinkExternal(%q, %q, %v) = %v; want %v", goos, goarch, cgoEnabled, got, want)
				}
			}
		}
	}
}

func TestRequiredBootstrapVersion(t *testing.T) {
	testCases := map[string]string{
		"1.22": "1.20",
		"1.23": "1.20",
		"1.24": "1.22",
		"1.25": "1.22",
		"1.26": "1.24",
		"1.27": "1.24",
	}

	for v, want := range testCases {
		if got := requiredBootstrapVersion(v); got != want {
			t.Errorf("requiredBootstrapVersion(%v): got %v, want %v", v, got, want)
		}
	}
}

func TestGoallcPluginInputID(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	payload := filepath.Join(root, "payload")
	for _, dir := range []string{source, filepath.Join(payload, "bin"), filepath.Join(payload, "lib", "cmake", "llvm")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(source, "CMakeLists.txt"):                            "project(test)",
		filepath.Join(source, "plugin.cpp"):                                "int plugin;",
		filepath.Join(payload, "bin", "llc"):                               "llc-v1",
		filepath.Join(payload, "bin", "llvm-config"):                       "llvm-config-v1",
		filepath.Join(payload, "lib", "cmake", "llvm", "LLVMConfig.cmake"): "llvm-config-cmake-v1",
		filepath.Join(payload, "lib", "libLLVM.dylib"):                     "libllvm-v1",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	llc := filepath.Join(payload, "bin", "llc")
	llvmConfig := filepath.Join(payload, "bin", "llvm-config")
	library := filepath.Join(payload, "lib", "libLLVM.dylib")
	want, err := goallcPluginInputID(source, llc, llvmConfig, library)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(llc, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := goallcPluginInputID(source, llc, llvmConfig, library)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("timestamp-only change altered input ID: %q != %q", got, want)
	}
	if err := os.WriteFile(llc, []byte("llc-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := goallcPluginInputID(source, llc, llvmConfig, library)
	if err != nil {
		t.Fatal(err)
	}
	if changed == want {
		t.Fatal("llc content change did not alter plugin input ID")
	}
	if err := os.WriteFile(llc, []byte("llc-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(library, []byte("libllvm-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err = goallcPluginInputID(source, llc, llvmConfig, library)
	if err != nil {
		t.Fatal(err)
	}
	if changed == want {
		t.Fatal("libLLVM content change did not alter plugin input ID")
	}
}
