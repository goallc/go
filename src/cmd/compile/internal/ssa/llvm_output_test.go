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
)

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
		for _, suffix := range []string{"", ".ll", ".opt.ll"} {
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
