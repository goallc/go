// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallGoallcRuntime(t *testing.T) {
	oldRoot, oldTool, oldLLVM, oldOS, oldLink := goroot, tooldir, goallcLLVMDir, gohostos, goallcLLVMLink
	defer func() {
		goroot, tooldir, goallcLLVMDir, gohostos, goallcLLVMLink = oldRoot, oldTool, oldLLVM, oldOS, oldLink
	}()
	goroot = t.TempDir()
	tooldir = filepath.Join(goroot, "pkg", "tool", "linux_amd64")
	goallcLLVMDir = t.TempDir()
	gohostos = "linux"
	goallcLLVMLink = "dynamic"
	files := map[string]string{
		filepath.Join(goroot, "pkg", "goallc-llvmplugin", "lib", "GoALLCStatepoints.so"): "plugin",
		filepath.Join(goallcLLVMDir, "lib", "libLLVM.so.23.1"):                           "llvm",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("libLLVM.so.23.1", filepath.Join(goallcLLVMDir, "lib", "libLLVM.so")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	installGoallcRuntime()
	// Removing the build payload must not break any installed runtime alias.
	if err := os.RemoveAll(goallcLLVMDir); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(t.TempDir(), "lib")
	if err := os.CopyFS(copied, os.DirFS(filepath.Join(tooldir, "lib"))); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"GoALLCStatepoints.so": "plugin", "libLLVM.so": "llvm", "libLLVM.so.23.1": "llvm"} {
		got, err := os.ReadFile(filepath.Join(copied, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestStaticGoallcRuntimeRemovesDynamicInstallation(t *testing.T) {
	oldTool, oldLink := tooldir, goallcLLVMLink
	t.Cleanup(func() { tooldir, goallcLLVMLink = oldTool, oldLink })
	tooldir, goallcLLVMLink = t.TempDir(), "static"
	lib := filepath.Join(tooldir, "lib")
	if err := os.Mkdir(lib, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "GoALLCStatepoints.so"), []byte("old plugin"), 0644); err != nil {
		t.Fatal(err)
	}
	installGoallcRuntime()
	if _, err := os.Stat(lib); !os.IsNotExist(err) {
		t.Fatalf("static toolchain retained runtime directory: %v", err)
	}
	// Installation must also work without any plugin build or LLVM payload.
	installGoallcRuntime()
}
