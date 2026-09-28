// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLVMRuntimeLinknameAsyncPolicy(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	const code = `package p
import _ "unsafe"
//go:linkname renamed other.entry
func renamed(v int) int { return v + 1 }
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, compilingRuntime := range []bool{false, true} {
		name := "ordinary"
		if compilingRuntime {
			name = "runtime"
		}
		t.Run(name, func(t *testing.T) {
			archive := filepath.Join(dir, name+".a")
			args := []string{"tool", "compile", "-enablellvm", "-p=" + name, "-llvm-keep-ir", "-o", archive}
			if compilingRuntime {
				args = append(args, "-+")
			}
			args = append(args, source)
			if out, err := testenv.Command(t, testenv.GoToolPath(t), args...).CombinedOutput(); err != nil {
				t.Fatalf("compile: %v\n%s", err, out)
			}
			for _, suffix := range []string{".ll", ".opt.ll"} {
				data, err := os.ReadFile(archive + suffix)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "@other.entry(") {
					t.Fatalf("%s: missing renamed function", suffix)
				}
				if all := strings.Contains(string(data), `"go-async-unsafe"="all"`); all != compilingRuntime {
					t.Fatalf("%s: whole-function async policy = %v, want %v", suffix, all, compilingRuntime)
				}
			}
		})
	}
}
