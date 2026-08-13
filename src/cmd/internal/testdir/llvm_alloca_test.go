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
	goallcLocals         uint32
	goallcPointerBits    []int
	goallcPCData         []int32
	goallcQueries        []int32
}

var llvmAllocaChecks = map[string]llvmAllocaArchitectureChecks{
	"darwin/arm64": {
		betweenCallsPattern:  `(?s)\bbl\s+p\.mutateLocal\n(.*?)\bbl\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*(?:str|stp)\b`,
		goallcLocals:         88,
		goallcPointerBits:    []int{5, 7, 8, 9},
		goallcPCData:         []int32{-1, 1, -1},
		goallcQueries:        []int32{-1, 1, 1, 1, 1},
	},
	"linux/amd64": {
		betweenCallsPattern:  `(?s)\bcallq\s+p\.mutateLocal\n(.*?)\bcallq\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*mov[a-z]*\s+[^,\n]+,\s*-[0-9]+\(%rbp\)`,
		goallcLocals:         88,
		goallcPointerBits:    []int{5, 7, 8, 9},
		goallcPCData:         []int32{-1, 1, -1},
		goallcQueries:        []int32{1, 1, 1, 1, -1},
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
		`call void @llvm\.lifetime\.start\.p0\(ptr %v[0-9]+\)`,
		`call void @llvm\.memset\.inline\.p0\.i64\(ptr align 8 %v[0-9]+, i8 0, i64 40`,
		`call goabiinternal void @p\.mutateLocal`,
		`call goabiinternal void @p\.safepoint`,
	} {
		if !regexp.MustCompile(pattern).Match(inputIR) &&
			!regexp.MustCompile(pattern).Match(inputFunction) {
			t.Fatalf("input alloca IR does not match %q\n%s", pattern, inputFunction)
		}
	}
	parameterInputFunction := llvmAllocaIRFunction(t, inputIR,
		"p.parameterAcrossSafepoints")
	for _, pattern := range []string{
		`define goabiinternal void @p\.parameterAcrossSafepoints\(%p\.pointerLocal %value\)`,
		`alloca %p\.pointerLocal, align 8`,
		`call void @llvm\.lifetime\.start\.p0\(ptr %v[0-9]+\)`,
		`store %p\.pointerLocal %value, ptr %v[0-9]+, align 8`,
	} {
		if !regexp.MustCompile(pattern).Match(parameterInputFunction) {
			t.Fatalf("input parameter-home IR does not match %q\n%s",
				pattern, parameterInputFunction)
		}
	}
	stackParameterInputFunction := llvmAllocaIRFunction(t, inputIR,
		"p.stackParameterAcrossSafepoints")
	for _, pattern := range []string{
		`define goabiinternal void @p\.stackParameterAcrossSafepoints\(\[2 x ptr\] %value\)`,
		`alloca \[2 x ptr\], align 8`,
		`call void @llvm\.lifetime\.start\.p0\(ptr %v[0-9]+\)`,
		`store \[2 x ptr\] %value, ptr %v[0-9]+, align 8`,
	} {
		if !regexp.MustCompile(pattern).Match(stackParameterInputFunction) {
			t.Fatalf("input stack-parameter-home IR does not match %q\n%s",
				pattern, stackParameterInputFunction)
		}
	}

	// LLVM compilation runs before native register allocation turns
	// OpKeepAlive of a stack address into OpVarLive. Preserve the value in the
	// go.keepalive operand bundle so statepoint liveness extends through the
	// preceding calls.
	keepAliveArchive := filepath.Join(dir, "keepalive.a")
	keepAliveSource := filepath.Join(gorootTestDir, "fixedbugs", "issue30476.go")
	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=main", "-importcfg="+stdlibImportcfgFile(), "-enablellvm",
		"-llvmironly", "-o", keepAliveArchive, keepAliveSource)
	keepAliveIR, err := os.ReadFile(keepAliveArchive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	keepAliveFunction := llvmAllocaIRFunction(t, keepAliveIR, "main.main")
	keepAlivePattern := regexp.MustCompile(`(?s)` +
		`call goabiinternal void @runtime\.GC\(\).*` +
		`call goabiinternal void @runtime\.GC\(\).*` +
		`call goabiinternal void @runtime\.GC\(\).*` +
		`call void @llvm\.donothing\(\) \[ "go\.keepalive"\(ptr %v[0-9]+\) \]`)
	if !keepAlivePattern.Match(keepAliveFunction) {
		t.Fatalf("OpKeepAlive is not preserved after the GC calls\n%s", keepAliveFunction)
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
	// The alloca address is an explicit gc-live root whose relocate is a
	// rematerialized frame index; its contents are never written back.
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
	if got, want := bytes.Count(rewrittenFunction, []byte(`"gc-live"(ptr `)), 4; got != want {
		t.Fatalf("alloca gc-live roots=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction,
		[]byte("@llvm.experimental.gc.relocate")), 4; got != want {
		t.Fatalf("alloca relocate references=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	// The original VarDef starts the alloca's source lifetime and completely
	// initializes it before the first call where it is GC-live. Merely carrying
	// the layout at every statepoint must not add an earlier initialization or
	// widen the object's GC activity.
	lifetimeStart := bytes.Index(rewrittenFunction, []byte("call void @llvm.lifetime.start"))
	zeroInitialize := bytes.Index(rewrittenFunction, []byte("call void @llvm.memset.inline"))
	if lifetimeStart < 0 || zeroInitialize < 0 ||
		bytes.Count(rewrittenFunction, []byte("call void @llvm.memset.inline")) != 1 ||
		bytes.Contains(rewrittenFunction, []byte("store ptr null")) ||
		lifetimeStart >= zeroInitialize {
		t.Fatalf("VarDef lifetime and source initialization are misordered or duplicated\n%s", rewrittenFunction)
	}
	runLLVMABICommand(t, rewrittenIR, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")

	machineIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-stop-after=finalize-isel",
		"-o", "-", goallcIR)
	machineFunction := llvmABIMachineFunction(t, machineIR, "p.localAcrossSafepoints")
	// The IR requires 8-byte alignment. SelectionDAG may strengthen this when
	// expanding the frontend's inline memset (for example, to 16 bytes on AArch64), so
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
		for _, constant := range []string{"1195461697", "1095520067", "40", "29", "1095519299"} {
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
	for _, tc := range []struct {
		name string
		size int
	}{
		{"p.parameterAcrossSafepoints", 40},
		{"p.stackParameterAcrossSafepoints", 16},
	} {
		parameterMachineFunction := llvmABIMachineFunction(t, machineIR, tc.name)
		parameterHomes := regexp.MustCompile(`(?m)^\s+- \{ id: ([0-9]+), type: (?:default|spill-slot), offset: [0-9]+, size: `+strconv.Itoa(tc.size)+`, alignment: 8,`).
			FindAllSubmatch(parameterMachineFunction, -1)
		if len(parameterHomes) != 1 {
			t.Fatalf("MIR %s fixed-home count=%d, want 1\n%s",
				tc.name, len(parameterHomes), parameterMachineFunction)
		}
		if !bytes.Contains(parameterMachineFunction, []byte("stack:           []")) {
			t.Fatalf("MIR %s retained a separate local parameter alloca\n%s",
				tc.name, parameterMachineFunction)
		}
		parameterHome := []byte("%fixed-stack." + string(parameterHomes[0][1]))
		parameterStatepoints := regexp.MustCompile(`(?m)^.*STATEPOINT.*$`).
			FindAll(parameterMachineFunction, -1)
		if got, want := len(parameterStatepoints), 3; got != want {
			t.Fatalf("MIR %s STATEPOINT count=%d, want %d\n%s",
				tc.name, got, want, parameterMachineFunction)
		}
		for _, statepoint := range parameterStatepoints {
			if !bytes.Contains(statepoint, parameterHome) {
				t.Fatalf("STATEPOINT does not reference %s fixed home %s: %s",
					tc.name, parameterHome, statepoint)
			}
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
		checks.goallcLocals,
		[][]int{nil, nil},
		[][]int{nil, checks.goallcPointerBits},
		checks.goallcPCData,
		checks.goallcQueries)

	goallcStackObjects := llvmABIStackObjects(t, symbol)
	if len(goallcStackObjects) != 0 {
		t.Fatalf("GoALLC all-callsite-live object emitted StackObjects=%v", goallcStackObjects)
	}

	parameterCases := []struct {
		name     string
		size     int32
		bits     []int
		gcSymbol string
	}{
		{"p.parameterAcrossSafepoints", 40, []int{0, 2, 3, 4}, "runtime.gcbits.1d00000000000000"},
		{"p.stackParameterAcrossSafepoints", 16, []int{0, 1}, "runtime.gcbits.0300000000000000"},
	}
	for _, tc := range parameterCases {
		parameterSymbol := findLLVMABISymbol(t, readLLVMABIObject(t, goallcObject),
			tc.name)
		parameterObjects := llvmABIStackObjects(t, parameterSymbol)
		if got, want := len(parameterObjects), 1; got != want {
			t.Fatalf("GoALLC %s StackObjects=%v, want one object",
				tc.name, parameterObjects)
		}
		parameterObject := parameterObjects[0]
		if parameterObject.Offset != 0 || parameterObject.Size != tc.size ||
			parameterObject.PtrBytes != tc.size || parameterObject.GCData == nil ||
			parameterObject.GCData.Name != tc.gcSymbol {
			t.Fatalf("GoALLC %s has malformed argp-relative StackObject=%v",
				tc.name, parameterObject)
		}
		if got, want := llvmABIStackMapBitmaps(t, parameterSymbol, "args_pointer_maps"),
			[][]int{tc.bits, nil}; !reflect.DeepEqual(got, want) {
			t.Fatalf("GoALLC %s ArgsPointerMaps=%v, want %v", tc.name, got, want)
		}
		if got := llvmABIStackMapBitmaps(t, parameterSymbol, "locals_pointer_maps"); !reflect.DeepEqual(got, [][]int{nil, nil}) {
			t.Fatalf("GoALLC %s LocalsPointerMaps=%v, want [[] []]", tc.name, got)
		}
	}

	// A matching direct gc-live alloca expands the deopt layout into the
	// callsite's LocalsPointerMaps. This fixture is live at every ordinary
	// statepoint, so it does not need the fallback function-level StackObject.
	// Native Go emits the equivalent pointer fields with a different frame-bit
	// bias and retains source-level StackObjects metadata.
	nativeSymbol := findLLVMABISymbol(t, readLLVMABIObject(t, nativeObject),
		"p.localAcrossSafepoints")
	if got := llvmABIStackMapBitmaps(t, nativeSymbol, "locals_pointer_maps"); !reflect.DeepEqual(got, [][]int{nil, {0, 2, 3, 4}}) {
		t.Fatalf("native LocalsPointerMaps=%v, want [[ ] [0 2 3 4]]", got)
	}
	nativeStackObjects := llvmABIStackObjects(t, nativeSymbol)
	if got, want := len(nativeStackObjects), 1; got != want {
		t.Fatalf("native StackObjects=%v, want one object", nativeStackObjects)
	}
	for _, tc := range parameterCases {
		nativeParameterSymbol := findLLVMABISymbol(t,
			readLLVMABIObject(t, nativeObject), tc.name)
		nativeParameterObjects := llvmABIStackObjects(t, nativeParameterSymbol)
		if got, want := len(nativeParameterObjects), 1; got != want {
			t.Fatalf("native %s StackObjects=%v, want one object",
				tc.name, nativeParameterObjects)
		}
		nativeParameterObject := nativeParameterObjects[0]
		if nativeParameterObject.Offset != 0 ||
			nativeParameterObject.Size != tc.size ||
			nativeParameterObject.PtrBytes != tc.size ||
			nativeParameterObject.GCData == nil ||
			nativeParameterObject.GCData.Name != tc.gcSymbol {
			t.Fatalf("native %s has malformed argp-relative StackObject=%v",
				tc.name, nativeParameterObject)
		}
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
