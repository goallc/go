// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"bytes"
	"internal/testenv"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestLLVMImportFingerprint(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustInternalLink(t, testenv.NoSpecialBuildTypes)
	dir := t.TempDir()
	fixtures := filepath.Join(testenv.GOROOT(t), "src/cmd/link/testdata/testIndexMismatch")
	goTool := testenv.GoToolPath(t)
	config := filepath.Join(dir, "importcfg")
	a := filepath.Join(dir, "a.o")
	main := filepath.Join(dir, "main.o")
	exe := filepath.Join(dir, "main.exe")
	testenv.WriteImportcfg(t, config, map[string]string{"a": a}, "runtime")
	// Also exercise the split compiler/linker object path.
	for _, mode := range []string{"combined", "split"} {
		t.Run(mode, func(t *testing.T) {
			compile := func(pkg, src, out string, extra ...string) {
				t.Helper()
				args := []string{"tool", "compile", "-enablellvm", "-importcfg=" + config, "-p=" + pkg, "-o", out}
				args = append(args, extra...)
				args = append(args, filepath.Join(fixtures, src))
				if output, err := testenv.Command(t, goTool, args...).CombinedOutput(); err != nil {
					t.Fatalf("compile %s: %v\n%s", src, err, output)
				}
			}

			compile("a", "a.go", a)
			linkObject := main
			var extra []string
			if mode == "split" {
				linkObject = filepath.Join(dir, "main.link.o")
				extra = []string{"-linkobj=" + linkObject}
			}
			compile("main", "main.go", main, extra...)
			link := func() ([]byte, error) {
				return testenv.Command(t, goTool, "tool", "link", "-importcfg="+config, "-o", exe, linkObject).CombinedOutput()
			}
			if out, err := link(); err != nil {
				t.Fatalf("consistent objects did not link: %v\n%s", err, out)
			}
			compile("a", "b.go", a)
			if out, err := link(); err == nil || !bytes.Contains(out, []byte("fingerprint mismatch")) {
				t.Fatalf("mismatched import: want fingerprint mismatch, got %v\n%s", err, out)
			}
		})
	}
}

func TestLLVMLinknameChecks(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	fixtures := filepath.Join(testenv.GOROOT(t), "src/cmd/link/testdata/linkname")
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"ok.go", true}, {"push.go", true}, {"textvar", true},
		{"coro.go", false}, {"coro_asm", false},
		{"coro2.go", false}, {"builtin.go", false},
		{"fastrand.go", true}, {"badlinkname.go", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := testenv.Command(t, testenv.GoToolPath(t), "build", "-gcflags=-enablellvm", "-o", filepath.Join(t.TempDir(), "main.exe"), "./"+tc.name)
			cmd.Dir = fixtures
			out, err := cmd.CombinedOutput()
			if tc.ok && err != nil {
				t.Fatalf("valid linkname failed: %v\n%s", err, out)
			}
			if !tc.ok && (err == nil || !bytes.Contains(out, []byte("invalid reference"))) {
				t.Fatalf("invalid linkname: want invalid reference, got %v\n%s", err, out)
			}
		})
	}
}

func TestLLVMRejectLargeSymbol(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	if unsafe.Sizeof(uintptr(0)) < 8 {
		t.Skip("requires a 64-bit host")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "p.go")
	if err := os.WriteFile(src, []byte("package p; var x [1<<32]byte"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-p=p", "-o", filepath.Join(dir, "p.o"), src).CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("symbol too large")) {
		t.Fatalf("want symbol too large, got %v\n%s", err, out)
	}
}
