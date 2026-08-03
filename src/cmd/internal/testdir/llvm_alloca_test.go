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
	"strconv"
	"testing"
)

type llvmAllocaArchitectureChecks struct {
	betweenCallsPattern  string
	restoredStorePattern string
}

var llvmAllocaChecks = map[string]llvmAllocaArchitectureChecks{
	"darwin/arm64": {
		betweenCallsPattern:  `(?s)\bbl\s+p\.mutateLocal\n(.*?)\bbl\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*(?:str|stp)\b`,
	},
	"linux/amd64": {
		betweenCallsPattern:  `(?s)\bcallq\s+p\.mutateLocal\n(.*?)\bcallq\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*mov[a-z]*\s+[^,\n]+,\s*-[0-9]+\(%rbp\)`,
	},
}

func runLLVMAllocaStatepointTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	checks, ok := llvmAllocaChecks[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		t.Skip("exact alloca statepoint frame and stack-map expectations are qualified on darwin/arm64 and linux/amd64")
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

	// Four ordinary call sites describe the same 40-byte alloca as memory in a
	// deopt suffix. Its four pointer leaves are bits 0, 2, 3, and 4 (0b11101).
	// They must not become SSA roots or be written back after a mutating callee.
	if bytes.Contains(rewrittenFunction, []byte("llvm.statepoint.fixed_stack_home")) {
		t.Fatalf("obsolete fixed-stack-home metadata survived\n%s", rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("@llvm.experimental.gc.statepoint")), 4; got != want {
		t.Fatalf("ordinary statepoints=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction, []byte(`"deopt"(`)), 4; got != want {
		t.Fatalf("alloca deopt records=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("i64 40, i64 8, i64 8, i64 5, i64 64, i64 1, i64 29")), 4; got != want {
		t.Fatalf("alloca bitmap payloads=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if bytes.Contains(rewrittenFunction, []byte(`"gc-live"`)) ||
		bytes.Contains(rewrittenFunction, []byte("@llvm.experimental.gc.relocate")) {
		t.Fatalf("alloca leaves became SSA roots\n%s", rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction, []byte("store ptr null")), 4; got != want {
		t.Fatalf("alloca pointer initializers=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	runLLVMABICommand(t, rewrittenIR, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")

	machineIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-stop-after=finalize-isel",
		"-o", "-", goallcIR)
	machineFunction := llvmABIMachineFunction(t, machineIR, "p.localAcrossSafepoints")
	// The IR requires 8-byte alignment. SelectionDAG may strengthen this when
	// expanding the inline memset (for example, to 16 bytes on AArch64), so
	// require at least the source alignment rather than one exact value.
	machineAllocas := regexp.MustCompile(`(?m)^\s+- \{ id: ([0-9]+), name: ([^,\s]+), type: default, offset: -?[0-9]+, size: 40, alignment: ([0-9]+),`).
		FindAllSubmatch(machineFunction, -1)
	if len(machineAllocas) != 1 {
		t.Fatalf("MIR 40-byte pointer alloca count=%d, want 1\n%s",
			len(machineAllocas), machineFunction)
	}
	alignment, err := strconv.Atoi(string(machineAllocas[0][3]))
	if err != nil || alignment < 8 || alignment&(alignment-1) != 0 {
		t.Fatalf("MIR pointer alloca alignment=%q, want a power of two of at least 8\n%s",
			machineAllocas[0][3], machineFunction)
	}
	stackObject := []byte("%stack." + string(machineAllocas[0][1]) + "." + string(machineAllocas[0][2]))
	ordinaryStatepoints := regexp.MustCompile(`(?m)^.*STATEPOINT.*%stack\.[0-9]+.*$`).
		FindAll(machineFunction, -1)
	if got, want := len(ordinaryStatepoints), 4; got != want {
		t.Fatalf("MIR alloca-backed STATEPOINT count=%d, want %d\n%s",
			got, want, machineFunction)
	}
	for _, statepoint := range ordinaryStatepoints {
		for _, constant := range []string{"1195461697", "1398033231", "40", "29", "1095519299"} {
			if !bytes.Contains(statepoint, []byte(constant)) {
				t.Fatalf("STATEPOINT lost alloca contract constant %s: %s", constant, statepoint)
			}
		}
		if !bytes.Contains(statepoint, stackObject) {
			t.Fatalf("STATEPOINT does not reference pointer alloca %s: %s",
				stackObject, statepoint)
		}
		for _, object := range regexp.MustCompile(`%stack\.[^,\s]+`).FindAll(statepoint, -1) {
			if !bytes.Equal(object, stackObject) {
				t.Fatalf("STATEPOINT allocated a separate root spill %s beside %s: %s",
					object, stackObject, statepoint)
			}
		}
		pattern := `0,\s+` + regexp.QuoteMeta(string(stackObject)) + `,\s+0`
		if !regexp.MustCompile(pattern).Match(statepoint) {
			t.Fatalf("STATEPOINT does not carry the direct alloca frame base: %s", statepoint)
		}
	}

	goallcAssembly := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-verify-machineinstrs",
		"-filetype=asm", "-o", "-", goallcIR)
	betweenCalls := regexp.MustCompile(checks.betweenCallsPattern).FindSubmatch(goallcAssembly)
	if len(betweenCalls) != 2 {
		t.Fatalf("GoALLC assembly has no adjacent mutateLocal/safepoint calls\n%s", goallcAssembly)
	}
	if regexp.MustCompile(checks.restoredStorePattern).Match(betweenCalls[1]) {
		t.Fatalf("GoALLC restored an alloca field after mutateLocal:\n%s", betweenCalls[1])
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

	goallcStackObjects := llvmABIStackObjects(t, symbol)
	if got, want := len(goallcStackObjects), 1; got != want {
		t.Fatalf("GoALLC StackObjects=%v, want one object", goallcStackObjects)
	}
	if object := goallcStackObjects[0]; object.Offset >= 0 || object.Size != 40 ||
		object.PtrBytes != 40 || object.GCData == nil ||
		object.GCData.Name != "runtime.gcbits.1d00000000000000" || object.GCData.ABI != 0 {
		t.Fatalf("GoALLC StackObject=%+v, want negative offset, size=40, ptrbytes=40, and ABI0 runtime.gcbits.1d00000000000000", object)
	}

	// StackObjects describe address-taken object identity and layout. They are
	// not themselves roots, so the initial implementation also keeps the
	// conservative per-safepoint LocalsPointerMaps bits until precise static
	// object liveness is implemented.
	nativeSymbol := findLLVMABISymbol(t, readLLVMABIObject(t, nativeObject),
		"p.localAcrossSafepoints")
	if got := llvmABIStackMapBitmaps(t, nativeSymbol, "locals_pointer_maps"); !reflect.DeepEqual(got, [][]int{nil, {0, 2, 3, 4}}) {
		t.Fatalf("native LocalsPointerMaps=%v, want [[ ] [0 2 3 4]]", got)
	}
	nativeStackObjects := llvmABIStackObjects(t, nativeSymbol)
	if got, want := len(nativeStackObjects), 1; got != want {
		t.Fatalf("native StackObjects=%v, want one object", nativeStackObjects)
	}
}

func llvmABIStackObjects(t *testing.T, symbol llvmABISymbol) []struct {
	Offset   int32 `json:"offset"`
	Size     int32 `json:"size"`
	PtrBytes int32 `json:"ptr_bytes"`
	GCData   *struct {
		Name string `json:"name"`
		ABI  uint16 `json:"abi"`
	} `json:"gcdata"`
} {
	t.Helper()
	if symbol.Function == nil {
		t.Fatalf("symbol %s has no function metadata", symbol.Name)
	}
	for _, data := range symbol.Function.FuncData {
		if data.Kind == "stack_objects" {
			return data.StackObjects
		}
	}
	return nil
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
