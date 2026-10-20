// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"bytes"
	"cmd/internal/archive"
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMSlimArchive(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "p.go")
			if err := os.WriteFile(source, []byte("package p\nfunc F(x int) int { return x + 1 }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "p.a")
			cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-llvm-lto", "-llvm-keep-ir", "-p=p", "-o", output, source)
			cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compile: %v\n%s", err, out)
			}
			f, err := os.Open(output)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			a, err := archive.Parse(f, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Entries) != 2 || a.Entries[0].Name != "__.PKGDEF" || a.Entries[1].Name != "__.LLVMBC" {
				t.Fatalf("unexpected slim archive entries: %v", a.Entries)
			}
			entry := a.Entries[1]
			bitcode := make([]byte, entry.Size)
			if _, err := f.ReadAt(bitcode, entry.Offset); err != nil {
				t.Fatal(err)
			}
			bc := filepath.Join(dir, "p.bc")
			if err := os.WriteFile(bc, bitcode, 0600); err != nil {
				t.Fatal(err)
			}
			context := llvm.NewContext()
			defer context.Dispose()
			module, err := context.ParseBitcodeFile(bc)
			if err != nil {
				t.Fatalf("parse bitcode: %v", err)
			}
			defer module.Dispose()
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("verify bitcode: %v", err)
			}
			if module.NamedFunction("p.F").IsNil() {
				t.Fatal("bitcode lost package function")
			}
			ir, err := os.ReadFile(output + ".ll")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(ir, []byte("!goobj.config")) {
				t.Fatal("pre-optimization IR lost GoObj configuration")
			}
			if _, err := os.Stat(output + ".opt.ll"); !os.IsNotExist(err) {
				t.Fatalf("LTO unexpectedly produced optimized IR: %v", err)
			}

			// Export data must remain usable without asking the importer to read
			// bitcode or supplying a fallback machine-code object.
			consumer := filepath.Join(dir, "q.go")
			if err := os.WriteFile(consumer, []byte("package q\nimport \"p\"\nfunc G() int { return p.F(41) }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			importcfg := filepath.Join(dir, "importcfg")
			if err := os.WriteFile(importcfg, []byte("packagefile p="+output+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cmd = testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm=false", "-importcfg", importcfg, "-p=q", "-o", filepath.Join(dir, "q.a"), consumer)
			cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("import slim archive: %v\n%s", err, out)
			}
		})
	}
}

func TestLLVMLTORejectsIncompatibleFlags(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	if err := os.WriteFile(source, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-enablellvm=false", "-N", "-llvm-opt-passes=none", "-linkobj=" + filepath.Join(dir, "link.o"), "-dynlink", "-linkshared", "-race", "-msan", "-asan"} {
		t.Run(strings.SplitN(flag, "=", 2)[0], func(t *testing.T) {
			args := []string{"tool", "compile", "-enablellvm", "-llvm-lto", flag, "-o", filepath.Join(t.TempDir(), "p.a"), source}
			cmd := testenv.Command(t, testenv.GoToolPath(t), args...)
			cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH=amd64")
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte("-llvm-lto requires LLVM")) {
				t.Fatalf("flag %s: error = %v, output:\n%s", flag, err, out)
			}
			if bytes.Contains(out, []byte("internal compiler error")) {
				t.Fatalf("invalid flag reported an internal compiler error:\n%s", out)
			}
		})
	}
}
