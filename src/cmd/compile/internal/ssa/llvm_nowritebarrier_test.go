// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"internal/testenv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLLVMYesWriteBarrierRecBoundary(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	const code = `package runtime
var sink *int
//go:nowritebarrierrec
func prohibited(p *int) { allowed(p) }
//go:yeswritebarrierrec
func allowed(p *int) { sink = p }
func ordinary(p *int) { sink = p }
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"O0", "O2"} {
		t.Run(level, func(t *testing.T) {
			archive := filepath.Join(dir, level+".a")
			cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile",
				"-enablellvm", "-+", "-p=runtime", "-llvm-keep-ir",
				"-llvm-opt-passes=default<"+level+">", "-o", archive, source)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compile: %v\n%s", err, out)
			}
			for _, suffix := range []string{".ll", ".opt.ll"} {
				data, err := os.ReadFile(archive + suffix)
				if err != nil {
					t.Fatal(err)
				}
				ir := string(data)
				// Check the declaration's policy independently of the inliner's
				// cost model, with an ordinary runtime function as a control.
				for name, want := range map[string]bool{"allowed": true, "ordinary": false} {
					def := regexp.MustCompile(`(?m)^define [^\n]*@runtime\.` + name + `\([^\n]* #([0-9]+)[^\n]*`).FindStringSubmatch(ir)
					if def == nil {
						t.Fatalf("%s: missing definition of %s", suffix, name)
					}
					attrs := regexp.MustCompile(`(?m)^attributes #` + def[1] + ` = \{([^\n]*)\}`).FindStringSubmatch(ir)
					if attrs == nil {
						t.Fatalf("%s: missing attributes of %s", suffix, name)
					}
					if got := strings.Contains(attrs[1], "noinline"); got != want {
						t.Errorf("%s: %s noinline = %v, want %v", suffix, name, got, want)
					}
				}
				if !strings.Contains(ir, "call goabiinternal void @runtime.allowed(") {
					t.Errorf("%s: missing call across yeswritebarrierrec boundary", suffix)
				}
			}
		})
	}
}
