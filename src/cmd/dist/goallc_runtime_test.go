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
	oldRoot, oldTool, oldLLVM, oldOS := goroot, tooldir, goallcLLVMDir, gohostos
	defer func() { goroot, tooldir, goallcLLVMDir, gohostos = oldRoot, oldTool, oldLLVM, oldOS }()
	goroot = t.TempDir()
	tooldir = filepath.Join(goroot, "pkg", "tool", "linux_amd64")
	goallcLLVMDir = t.TempDir()
	gohostos = "linux"
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

func TestGoallcWindowsHeaders(t *testing.T) {
	payload, binding := t.TempDir(), t.TempDir()
	include := filepath.Join(payload, "include")
	if err := os.Mkdir(include, 0755); err != nil {
		t.Fatal(err)
	}
	header := filepath.Join(include, "test.h")
	destination := filepath.Join(binding, "llvm")
	for _, content := range []string{"old header", "updated header"} {
		if err := os.WriteFile(header, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := copyGoallcLLVMHeaders(destination, payload); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(destination, "include", "test.h"))
		if err != nil || string(got) != content {
			t.Fatalf("copied header = %q, %v", got, err)
		}
	}
	unowned := t.TempDir()
	if err := copyGoallcLLVMHeaders(unowned, payload); err == nil {
		t.Fatal("replaced an unowned directory")
	}
}

func TestGoallcWindowsLLVMTool(t *testing.T) {
	oldOS, oldDir := gohostos, goallcLLVMDir
	defer func() { gohostos, goallcLLVMDir = oldOS, oldDir }()
	gohostos, goallcLLVMDir = "windows", t.TempDir()
	tool := goallcLLVMTool("llc")
	if filepath.Base(tool) != "llc.exe" {
		t.Fatalf("Windows LLVM tool = %s", tool)
	}
	if err := os.Mkdir(filepath.Dir(tool), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := requireExecutableFile(tool); err != nil {
		t.Fatalf("Windows executables do not need Unix mode bits: %v", err)
	}
}
