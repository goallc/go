// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"internal/platform"
	"internal/testenv"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Test that compiler-generated wrappers that end in a tail call still tell the
// race detector that the wrapper has returned. This is a regression test for
// #81478, where the missing racefuncexit call made every wrapper call leak an
// entry in the race detector's shadow stack, which in turn made later
// synchronization operations dramatically slower.
func TestIssue81478(t *testing.T) {
	if !platform.RaceDetectorSupported(runtime.GOOS, runtime.GOARCH) {
		t.Skipf("race detector not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	testenv.MustHaveGoBuild(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "x.go")
	if err := os.WriteFile(src, []byte(issue81478src), 0644); err != nil {
		t.Fatalf("could not write file: %v", err)
	}

	obj := filepath.Join(dir, "x.o")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-race", "-p=main", "-o", obj, src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile failed: %v\n%s", err, out)
	}

	// Inspect the emitted object so the same check covers both the native and
	// LLVM backends, independently of the compiler's -S output format.
	cmd = testenv.Command(t, testenv.GoToolPath(t), "tool", "objdump", obj)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("objdump failed: %v\n%s", err, out)
	}

	// W.F tail calls through an interface, S.G tail calls a static function.
	// Whether the tail call is actually emitted is architecture-dependent, but
	// either way the enter/exit instrumentation must be balanced.
	for _, wrapper := range []string{"main.(*W).F", "main.(*S).G"} {
		body, ok := funcBody(string(out), wrapper)
		if !ok {
			t.Errorf("no assembly found for %s", wrapper)
			continue
		}
		enter := strings.Count(body, "runtime.racefuncenter")
		exit := strings.Count(body, "runtime.racefuncexit")
		if enter == 0 {
			t.Errorf("%s: not instrumented for the race detector\n%s", wrapper, body)
		} else if enter != exit {
			t.Errorf("%s: unbalanced race instrumentation: %d racefuncenter, %d racefuncexit\n%s", wrapper, enter, exit, body)
		}
	}
}

// funcBody returns the part of the objdump output that belongs to the symbol fn:
// its header line plus the indented instructions and their relocation annotations.
func funcBody(out, fn string) (string, bool) {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "TEXT "+fn+"(SB)") {
			continue
		}
		end := i + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
			end++
		}
		return strings.Join(lines[i:end], "\n"), true
	}
	return "", false
}

var issue81478src = `
package main

type I interface{ F() }

type T struct{ n int }

func (t *T) F() { t.n++ }

//go:noinline
func (t *T) G() { t.n++ }

// W.F is a wrapper around an embedded interface method.
type W struct{ I }

// S.G is a wrapper around an embedded pointer's method.
type S struct{ *T }

var _ I = &W{}
var _ = (&S{}).G

func main() {}
`
