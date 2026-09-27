// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestLLVMFunctionDeclFile(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	// Inlining helper puts its conditional in Entry's entry block. That
	// block's position must not become Entry's declaration file.
	files := map[string]string{
		"entry.go": `package p

//go:noinline
func Entry(x int) int {
	c := helper(x)
	if c > 0 { return c + 2 }
	return 0
}
`,
		"helper.go": `package p

func helper(x int) int {
	if x > 3 { return x }
	return 1
}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(dir, "p.a")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-p=p", "-llvm-keep-ir", "-o", archive,
		filepath.Join(dir, "entry.go"), filepath.Join(dir, "helper.go"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	for _, suffix := range []string{".ll", ".opt.ll"} {
		data, err := os.ReadFile(archive + suffix)
		if err != nil {
			t.Fatal(err)
		}
		decl := regexp.MustCompile(`DISubprogram\(name: "p.Entry",[^\n]*file: (![0-9]+), line: 4,`).FindSubmatch(data)
		if decl == nil {
			t.Fatalf("%s: missing Entry declaration at line 4", suffix)
		}
		file := regexp.MustCompile(`(?m)^` + string(decl[1]) + ` = !DIFile\(filename: "entry.go",`)
		if !file.Match(data) {
			t.Fatalf("%s: Entry uses its inlined callee's file:\n%s", suffix, data)
		}
	}
}
