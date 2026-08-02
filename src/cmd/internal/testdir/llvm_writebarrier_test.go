// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type llvmWriteBarrierArchitectureChecks struct {
	moveType     string
	moveElements int
}

var llvmWriteBarrierChecks = map[string]llvmWriteBarrierArchitectureChecks{
	"darwin/arm64": {moveType: "Move8", moveElements: 8},
	"linux/amd64":  {moveType: "Move2", moveElements: 2},
}

func runLLVMWriteBarrierHelperTest(t *testing.T) {
	t.Helper()
	checks, ok := llvmWriteBarrierChecks[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		t.Skip("write-barrier helper IR and RS4GC expectations are qualified on darwin/arm64 and linux/amd64")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "writebarrier.go")
	program := fmt.Sprintf(`package p

type Big [32]*int
type %[1]s [%[2]d]*int

func ordinary(*int)

//go:noinline
func safe(x int) int { return x + 1 }

//go:noinline
func heapZero(dst *Big, live *int) {
	*dst = Big{}
	ordinary(live)
}

//go:noinline
func heapMove(dst, src *%[1]s) {
	*dst = *src
}
`, checks.moveType, checks.moveElements)
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
				[]byte("call goabiinternal void @runtime.wbZero(ptr @\"type:p.Big\", ptr %dst)"),
			},
		},
		{
			name: "wbMove",
			body: llvmAllocaIRFunction(t, ir, "p.heapMove"),
			want: [][]byte{
				[]byte(fmt.Sprintf("call goabiinternal void @runtime.wbMove(ptr @\"type:p.%s\", ptr %%dst, ptr %%src)", checks.moveType)),
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
		[]byte("declare goabiinternal void @runtime.wbZero(ptr, ptr)"),
		[]byte("declare goabiinternal void @runtime.wbMove(ptr, ptr, ptr)"),
		[]byte(`"gc-leaf-function"`),
		[]byte(`"go-async-unsafe"`),
	} {
		if !bytes.Contains(ir, want) {
			t.Fatalf("input IR does not contain %q", want)
		}
	}
	if bytes.Contains(ir, []byte("llvm.go.gc.unsafe.point")) {
		t.Fatalf("input IR still contains unsafe-point marker calls")
	}

	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", irPath)
	optimized := runLLVMABICommand(t, nil, opt, "-passes=default<O2>", "-S", "-o", "-", irPath)
	for _, name := range []string{"runtime.wbZero", "runtime.wbMove", "gc-leaf-function", "go-async-unsafe"} {
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
		for _, line := range bytes.Split(body, []byte{'\n'}) {
			if bytes.Contains(line, []byte("@llvm.experimental.gc.statepoint")) && bytes.Contains(line, []byte("@"+tc.helper)) {
				t.Fatalf("RS4GC statepointized raw helper %s\n%s", tc.helper, body)
			}
		}
	}
	zero := llvmAllocaIRFunction(t, rewritten, "p.heapZero")
	var ordinaryStatepoints, nilCheckStatepoints int
	for _, line := range bytes.Split(zero, []byte{'\n'}) {
		if !bytes.Contains(line, []byte("@llvm.experimental.gc.statepoint")) {
			continue
		}
		switch {
		case bytes.Contains(line, []byte("@p.ordinary")):
			ordinaryStatepoints++
		case bytes.Contains(line, []byte("@runtime.panicmem")):
			nilCheckStatepoints++
		default:
			t.Fatalf("unexpected heapZero statepoint\n%s", line)
		}
	}
	if ordinaryStatepoints != 1 || nilCheckStatepoints != 2 {
		t.Fatalf("heapZero statepoints: ordinary=%d nilcheck=%d, want 1 and 2\n%s",
			ordinaryStatepoints, nilCheckStatepoints, zero)
	}
	runLLVMABICommand(t, rewritten, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")

	object := filepath.Join(dir, "writebarrier.o")
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-verify-machineinstrs", "-filetype=obj", "-o", object, irPath)
	document := readLLVMABIObject(t, object)
	for _, tc := range []struct {
		name string
		want int32
	}{
		{"p.heapZero", -2},
		{"p.heapMove", -2},
		{"p.safe", -2},
	} {
		symbol := findLLVMABISymbol(t, document, tc.name)
		var values []int32
		for _, pcdata := range symbol.Function.PCData {
			if pcdata.Kind != "unsafe_point" {
				continue
			}
			for _, pcRange := range pcdata.Ranges {
				values = append(values, pcRange.Value)
			}
		}
		if len(values) != 1 || values[0] != tc.want {
			t.Fatalf("%s PCDATA_UnsafePoint values=%v, want whole-function value %d", tc.name, values, tc.want)
		}
	}
}
