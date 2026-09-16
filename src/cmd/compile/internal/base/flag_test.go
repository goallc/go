// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"bytes"
	"internal/testenv"
	"os"
	"path/filepath"
	"testing"
)

func TestLLVMDisablesConcurrentBackend(t *testing.T) {
	old := Flag
	defer func() {
		Flag = old
	}()

	Flag = CmdFlags{}
	if !concurrentFlagOk() {
		t.Fatal("default flags unexpectedly disable concurrent compilation")
	}
	Flag.EnableLLVM = true
	if concurrentFlagOk() {
		t.Fatal("-enablellvm must disable concurrent compilation")
	}
}

func TestLLVMDeterministicOutput(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	// All four source names refer to the same SSA argument. Iterating
	// NamedValues as a map used to give that argument a random LLVM name.
	code := "package p; func F(p *int, n int) int { q := p; r := q; s := r; if n > 0 { return *s }; return *q }\n"
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "p.a")
	want := make(map[string][]byte)
	for i := 0; i < 16; i++ {
		cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-p=p", "-llvm-keep-ir", "-o", archive, source)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("compile %d: %v\n%s", i, err, out)
		}
		for _, suffix := range []string{"", ".ll", ".opt.ll", ".precodegen.ll"} {
			got, err := os.ReadFile(archive + suffix)
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				want[suffix] = got
			} else if !bytes.Equal(got, want[suffix]) {
				t.Fatalf("compile %d produced different output for %s", i, filepath.Base(archive+suffix))
			}
		}
	}
}

func TestLLVMInlineClosureDebugName(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	code := "package p; func closure(x int) func() int { return func() int { return x } }; var Sink func() int; func F(x int) { Sink = closure(x) }\n"
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "p.a")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-p=p", "-llvm-keep-ir", "-o", archive, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(ir, []byte(`linkageName: "p.closure.func1#`)) {
		t.Fatal("test did not create an inline closure identity")
	}
	if bytes.Contains(ir, []byte(`!DISubprogram(name: "p.closure.func1#`)) {
		t.Fatal("temporary inline hash leaked into the DWARF function name")
	}
}
