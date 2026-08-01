// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func runLLVMWriteBarrierHelperTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("write-barrier helper IR and RS4GC expectations are qualified on darwin/arm64")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "writebarrier.go")
	const program = `package p

type Big [32]*int
type Move8 [8]*int

func ordinary(*int)

//go:noinline
func heapZero(dst *Big, live *int) {
	*dst = Big{}
	ordinary(live)
}

//go:noinline
func heapMove(dst, src *Move8) {
	*dst = *src
}
`
	if err := os.WriteFile(source, []byte(program), 0o666); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, "writebarrier.a")
	irPath := archive + ".ll"
	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=p", "-enablellvm", "-llvmironly", "-o", archive, source)
	ir, err := os.ReadFile(irPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, check := range []struct {
		name string
		body []byte
		want [][]byte
	}{
		{
			name: "wbZero",
			body: llvmAllocaIRFunction(t, ir, "p.heapZero"),
			want: [][]byte{
				[]byte("ptrtoint ptr %dst to i64"),
				[]byte("call goabiinternal void @runtime.wbZero(i64 ptrtoint (ptr @\"type:p.Big\" to i64), i64"),
			},
		},
		{
			name: "wbMove",
			body: llvmAllocaIRFunction(t, ir, "p.heapMove"),
			want: [][]byte{
				[]byte("ptrtoint ptr %dst to i64"),
				[]byte("ptrtoint ptr %src to i64"),
				[]byte("call goabiinternal void @runtime.wbMove(i64 ptrtoint (ptr @\"type:p.Move8\" to i64), i64"),
			},
		},
	} {
		t.Run(check.name, func(t *testing.T) {
			for _, want := range check.want {
				if !bytes.Contains(check.body, want) {
					t.Fatalf("input IR does not contain %q\n%s", want, check.body)
				}
			}
		})
	}
	for _, want := range [][]byte{
		[]byte("declare goabiinternal void @runtime.wbZero(i64, i64)"),
		[]byte("declare goabiinternal void @runtime.wbMove(i64, i64, i64)"),
		[]byte(`"gc-leaf-function"`),
	} {
		if !bytes.Contains(ir, want) {
			t.Fatalf("input IR does not contain %q", want)
		}
	}

	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", irPath)
	optimized := runLLVMABICommand(t, nil, opt, "-passes=default<O2>", "-S", "-o", "-", irPath)
	for _, name := range []string{"runtime.wbZero", "runtime.wbMove", "gc-leaf-function"} {
		if !bytes.Contains(optimized, []byte(name)) {
			t.Fatalf("optimized IR lost %q", name)
		}
	}

	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	rewritten := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-goallc-pass-plugin-emit-ir",
		"-filetype=null", "-o", os.DevNull, irPath)
	for _, tc := range []struct {
		function string
		helper   string
	}{
		{"p.heapZero", "runtime.wbZero"},
		{"p.heapMove", "runtime.wbMove"},
	} {
		body := llvmAllocaIRFunction(t, rewritten, tc.function)
		if !bytes.Contains(body, []byte("call goabiinternal void @"+tc.helper)) {
			t.Fatalf("RS4GC output lost direct leaf call to %s\n%s", tc.helper, body)
		}
		if bytes.Contains(body, []byte("elementtype(void (i64")) {
			t.Fatalf("RS4GC statepointized raw helper %s\n%s", tc.helper, body)
		}
	}
	zero := llvmAllocaIRFunction(t, rewritten, "p.heapZero")
	if got := bytes.Count(zero, []byte("@llvm.experimental.gc.statepoint")); got != 1 {
		t.Fatalf("ordinary positive-control statepoints=%d, want 1\n%s", got, zero)
	}
	runLLVMABICommand(t, rewritten, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")

	object := filepath.Join(dir, "writebarrier.o")
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-verify-machineinstrs", "-filetype=obj", "-o", object, irPath)
	document := readLLVMABIObject(t, object)
	for _, name := range []string{"p.heapZero", "p.heapMove"} {
		symbol := findLLVMABISymbol(t, document, name)
		var values []int32
		for _, pcdata := range symbol.Function.PCData {
			if pcdata.Kind != "unsafe_point" {
				continue
			}
			for _, pcRange := range pcdata.Ranges {
				values = append(values, pcRange.Value)
			}
		}
		if want := []int32{-1, -2, -1}; !slices.Equal(values, want) {
			t.Fatalf("%s PCDATA_UnsafePoint values=%v, want one continuous unsafe interval %v", name, values, want)
		}
	}
}
