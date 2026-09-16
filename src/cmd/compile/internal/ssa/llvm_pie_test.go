// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLLVMPIE(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ELF PIE regression")
	}
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveBuildMode(t, "pie")
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte("package main; import \"fmt\"; func main() { fmt.Println(\"LLVM PIE\") }"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"internal", "external"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "internal" {
				testenv.MustInternalLinkPIE(t)
			} else {
				testenv.MustHaveCGO(t)
			}
			exe := filepath.Join(t.TempDir(), "main.exe")
			cmd := testenv.Command(t, testenv.GoToolPath(t), "build", "-gcflags=all=-enablellvm", "-buildmode=pie", "-ldflags=-linkmode="+mode, "-o", exe, src)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("PIE build: %v\n%s", err, out)
			}
			check := func(t *testing.T, path string) {
				t.Helper()
				if out, err := testenv.Command(t, path).CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "LLVM PIE" {
					t.Fatalf("PIE execution: %v\n%s", err, out)
				}
			}
			check(t, exe)
			t.Run("stripped", func(t *testing.T) {
				strip, err := exec.LookPath("strip")
				if err != nil {
					t.Skip("strip unavailable")
				}
				stripped := exe + ".stripped"
				if out, err := testenv.Command(t, strip, "-o", stripped, exe).CombinedOutput(); err != nil {
					t.Fatalf("strip: %v\n%s", err, out)
				}
				check(t, stripped)
			})
		})
	}
}
