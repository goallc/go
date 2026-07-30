// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

func runLLVMAllocaStatepointTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("exact alloca statepoint frame and stack-map expectations are qualified on darwin/arm64")
	}

	dir := t.TempDir()
	source := filepath.Join(gorootTestDir, "abi", "llvm_alloca_statepoint.go")
	nativeObject := filepath.Join(dir, "native.o")
	goallcArchive := filepath.Join(dir, "goallc.a")
	goallcIR := goallcArchive + ".ll"
	goallcObject := filepath.Join(dir, "goallc.o")

	nativeAssembly := runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-S", "-l", "-p=p", "-o", nativeObject, source)
	for _, pattern := range []string{
		`TEXT\s+p\.localAcrossSafepoints\(SB\), ABIInternal, \$[0-9]+-16`,
		`FUNCDATA\s+\$2,\s+p\.localAcrossSafepoints\.stkobj\(SB\)`,
		`(?s)p\.localAcrossSafepoints.*?CALL\s+p\.mutateLocal\(SB\).*?CALL\s+p\.safepoint\(SB\)`,
	} {
		if !regexp.MustCompile(pattern).Match(nativeAssembly) {
			t.Fatalf("native alloca assembly does not match %q", pattern)
		}
	}

	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=p", "-enablellvm", "-llvmironly", "-o", goallcArchive, source)
	inputIR, err := os.ReadFile(goallcIR)
	if err != nil {
		t.Fatal(err)
	}
	inputFunction := llvmAllocaIRFunction(t, inputIR, "p.localAcrossSafepoints")
	for _, pattern := range []string{
		`%p\.pointerLocal = type \{ ptr, i64, ptr, \[2 x ptr\] \}`,
		`alloca %p\.pointerLocal, align 8`,
		`call goabiinternal void @p\.mutateLocal`,
		`call goabiinternal void @p\.safepoint`,
	} {
		if !regexp.MustCompile(pattern).Match(inputIR) &&
			!regexp.MustCompile(pattern).Match(inputFunction) {
			t.Fatalf("input alloca IR does not match %q\n%s", pattern, inputFunction)
		}
	}

	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", goallcIR)
	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	rewrittenIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-goallc-pass-plugin-emit-ir",
		"-filetype=null", "-o", "-", goallcIR)
	rewrittenFunction := llvmAllocaIRFunction(t, rewrittenIR, "p.localAcrossSafepoints")

	// Four static ordinary call sites each keep all four pointer leaves live.
	// The canonical loads are marked so SelectionDAG must use the corresponding
	// alloca subslot rather than silently allocating a separate spill.
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("!llvm.statepoint.fixed_stack_home")), 16; got != want {
		t.Fatalf("fixed-stack-home loads=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("@llvm.experimental.gc.statepoint")), 4; got != want {
		t.Fatalf("ordinary statepoints=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("@llvm.experimental.gc.relocate")), 16; got != want {
		t.Fatalf("alloca relocates=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	runLLVMABICommand(t, rewrittenIR, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")

	machineIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-stop-after=finalize-isel",
		"-o", "-", goallcIR)
	machineFunction := llvmABIMachineFunction(t, machineIR, "p.localAcrossSafepoints")
	if !regexp.MustCompile(`(?s)stack:.*?size:\s+40,\s+alignment:\s+16`).Match(machineFunction) {
		t.Fatalf("MIR has no 40-byte pointer alloca\n%s", machineFunction)
	}
	ordinaryStatepoints := regexp.MustCompile(`(?m)^.*STATEPOINT.*%stack\.[0-9]+.*$`).
		FindAll(machineFunction, -1)
	if got, want := len(ordinaryStatepoints), 4; got != want {
		t.Fatalf("MIR alloca-backed STATEPOINT count=%d, want %d\n%s",
			got, want, machineFunction)
	}
	for _, statepoint := range ordinaryStatepoints {
		stackObject := regexp.MustCompile(`(%stack\.[^,\s]+)`).FindSubmatch(statepoint)
		if len(stackObject) != 2 {
			t.Fatalf("STATEPOINT has no alloca frame index: %s", statepoint)
		}
		for _, object := range regexp.MustCompile(`%stack\.[^,\s]+`).FindAll(statepoint, -1) {
			if !bytes.Equal(object, stackObject[1]) {
				t.Fatalf("STATEPOINT allocated a separate root spill %s beside %s: %s",
					object, stackObject[1], statepoint)
			}
		}
		for _, offset := range []string{"0", "16", "24", "32"} {
			pattern := regexp.QuoteMeta(string(stackObject[1])) + `,\s+` + offset + `(?:\D|$)`
			if !regexp.MustCompile(pattern).Match(statepoint) {
				t.Fatalf("STATEPOINT does not reuse alloca offset %s: %s", offset, statepoint)
			}
		}
	}

	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-filetype=obj", goallcIR, "-o", goallcObject)
	symbol := findLLVMABISymbol(t, readLLVMABIObject(t, goallcObject),
		"p.localAcrossSafepoints")
	checkLLVMABISymbol(t, "GoALLC", symbol, llvmABICase{
		name: "localAcrossSafepoints", args: 16,
	})
	checkLLVMABISourceStackMaps(t, "GoALLC", symbol,
		88,
		[][]int{nil, nil},
		[][]int{nil, {5, 7, 8, 9}},
		[]int32{-1, 1, 0},
		[]int32{1, 1, 1, 1, 0})

	// Native Go may additionally describe this address-taken value as a
	// StackObject. The GoALLC qualification above intentionally requires the
	// conservative LocalsPointerMaps representation first; StackObjects can be
	// added later as a lifetime-precision optimization.
	nativeSymbol := findLLVMABISymbol(t, readLLVMABIObject(t, nativeObject),
		"p.localAcrossSafepoints")
	if got := llvmABIStackMapBitmaps(t, nativeSymbol, "locals_pointer_maps"); !reflect.DeepEqual(got, [][]int{nil, {0, 2, 3, 4}}) {
		t.Fatalf("native LocalsPointerMaps=%v, want [[ ] [0 2 3 4]]", got)
	}
}

func llvmAllocaIRFunction(t *testing.T, ir []byte, name string) []byte {
	t.Helper()
	start := regexp.MustCompile(`(?m)^define [^{\n]*@` + regexp.QuoteMeta(name) +
		`[^{\n]*\{`).FindIndex(ir)
	if start == nil {
		t.Fatalf("IR has no function %s", name)
	}
	body := ir[start[0]:]
	if end := bytes.Index(body, []byte("\n}")); end >= 0 {
		body = body[:end+2]
	}
	return body
}
