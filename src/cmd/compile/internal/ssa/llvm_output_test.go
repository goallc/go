// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"bytes"
	"internal/platform"
	"internal/testenv"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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

// TestLLVMOptimizationFlags checks the emitted IR, so both frontend and LLVM
// optimizations must honor the requested mode.
func TestLLVMOptimizationFlags(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	code := `package p
func Callee(x int) int { return x + 1 }
func Caller(x int) int { return Callee(x) }
//go:noinline
func Identity(x int) int { y := x + 1; return y - 1 }
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                     string
		flags                    []string
		wantCall, wantArithmetic bool
	}{
		{"default", nil, false, false},
		{"noinline", []string{"-l"}, true, false},
		{"noopt", []string{"-N"}, false, true},
		{"debug", []string{"-N", "-l"}, true, true},
		{"inline-again", []string{"-l=2"}, false, false},
		{"explicit-o2", []string{"-N", "-l", "-llvm-opt-passes=default<O2>"}, true, false},
		{"explicit-none", []string{"-N", "-l", "-llvm-opt-passes=none"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive := filepath.Join(dir, tc.name+".a")
			args := []string{"tool", "compile", "-enablellvm", "-p=p", "-llvm-keep-ir", "-o", archive}
			args = append(args, tc.flags...)
			args = append(args, source)
			if out, err := testenv.Command(t, testenv.GoToolPath(t), args...).CombinedOutput(); err != nil {
				t.Fatalf("compile: %v\n%s", err, out)
			}
			data, err := os.ReadFile(archive + ".opt.ll")
			if err != nil {
				t.Fatal(err)
			}
			body := func(name string) string {
				re := regexp.MustCompile(`(?ms)^define [^\n]*@p\.` + name + `\([^\n]*\n(.*?)^}`)
				m := re.FindSubmatch(data)
				if m == nil {
					t.Fatalf("missing IR for %s", name)
				}
				return string(m[1])
			}
			caller := body("Caller")
			if got := strings.Contains(caller, "@p.Callee("); got != tc.wantCall {
				t.Errorf("Callee call = %v, want %v:\n%s", got, tc.wantCall, caller)
			}
			identity := body("Identity")
			if got := strings.Contains(identity, "add i64") || strings.Contains(identity, "sub i64"); got != tc.wantArithmetic {
				t.Errorf("Identity arithmetic = %v, want %v:\n%s", got, tc.wantArithmetic, identity)
			}
		})
	}
}

// Heap results must describe the source variable through its canonical heap
// pointer home, rather than only describing the compiler's &result temporary.
func TestLLVMHeapDebugLocation(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	code := `package p
var Escaped *int
//go:noinline
func HeapResult() (result int) {
	Escaped = &result
	result = 42
	return
}
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "p.a")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-N", "-l", "-llvm-keep-ir", "-p=p", "-o", archive, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	id := regexp.MustCompile(`(?m)^(![0-9]+) = !DILocalVariable\(name: "result",`).FindSubmatch(ir)
	if id == nil {
		t.Fatal("missing result variable metadata")
	}
	declare := regexp.MustCompile(`#dbg_declare\(ptr %[^,]+, ` + string(id[1]) + `, !DIExpression\(DW_OP_deref\),`)
	if !declare.Match(ir) {
		t.Fatalf("heap result has no indirect location:\n%s", ir)
	}
}

// Source SSA descriptions must survive ABI copy elision without asking the
// backend to reconstruct variables from temporary stack addresses.
func TestLLVMSSADebugValues(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "p.go")
	code := `package p
//go:noinline
func Callee(x int) int { return x + 1 }
//go:noinline
func Scalar(x int) int {
	value := x + 7
	callResult := Callee(value)
	return callResult + value
}
type Pair struct { A, B int }
//go:noinline
func Pieces(x int) int {
	pair := Pair{x+3, x*5}
	return Callee(pair.A) + pair.B
}
//go:noinline
func Source(x int) (int, int, int, int, int, int, int, int, int, int) {
	return x, x, x, x, x, x, x, x, x, x+1
}
//go:noinline
func MemoryResult(x int) int {
	_, _, _, _, _, _, _, _, _, memoryResult := Source(x)
	return Callee(memoryResult)
}
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "p.a")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-l", "-llvm-keep-ir", "-p=p", "-o", archive, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	for _, suffix := range []string{".ll", ".opt.ll"} {
		data, err := os.ReadFile(archive + suffix)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ name, expression string }{
			{"value", ""}, {"callResult", ""}, {"memoryResult", ""},
			{"pair", "DW_OP_LLVM_fragment, 0, 64"},
			{"pair", "DW_OP_LLVM_fragment, 64, 64"},
		} {
			id := regexp.MustCompile(`(?m)^(![0-9]+) = !DILocalVariable\(name: "` + tc.name + `",`).FindSubmatch(data)
			if id == nil {
				t.Fatalf("%s: missing %s metadata", suffix, tc.name)
			}
			value := regexp.MustCompile(`#dbg_value\(i64 %[^,]+, ` + string(id[1]) + `, !DIExpression\(` + tc.expression + `\),`)
			if !value.Match(data) {
				t.Errorf("%s: missing SSA description for %s (%s)\n%s", suffix, tc.name, tc.expression, data)
			}
		}
	}
}

// Race exit instrumentation separates aggregate result reads from their ABI
// stores. Keep those reads before racefuncexit without constructing enormous
// first-class LLVM values.
func TestLLVMMemorySnapshotRace(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	if !platform.RaceDetectorSupported(runtime.GOOS, runtime.GOARCH) {
		t.Skip("race detector not supported")
	}
	source := filepath.Join(runtime.GOROOT(), "test", "llvm_memory_snapshot.go")
	for _, flags := range []string{"-enablellvm", "-enablellvm -N -l"} {
		t.Run(flags, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "snapshot.exe")
			cmd := testenv.Command(t, testenv.GoToolPath(t), "build", "-race", "-gcflags="+flags, "-o", exe, source)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			if out, err := testenv.Command(t, exe).CombinedOutput(); err != nil {
				t.Fatalf("run: %v\n%s", err, out)
			}
		})
	}
}
