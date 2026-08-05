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
	goallcStackObject    int32
}

var llvmAllocaChecks = map[string]llvmAllocaArchitectureChecks{
	"darwin/arm64": {
		betweenCallsPattern:  `(?s)\bbl\s+p\.mutateLocal\n(.*?)\bbl\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*(?:str|stp)\b`,
		goallcLocals:         88,
		goallcStackObject:    -40,
	},
	"linux/amd64": {
		betweenCallsPattern:  `(?s)\bcallq\s+p\.mutateLocal\n(.*?)\bcallq\s+p\.safepoint`,
		restoredStorePattern: `(?m)^\s*mov[a-z]*\s+[^,\n]+,\s*-[0-9]+\(%rbp\)`,
		goallcLocals:         96,
		goallcStackObject:    -48,
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

	// LLVM compilation runs before native register allocation turns
	// OpKeepAlive of a stack address into OpVarLive. Preserve the value use
	// explicitly so statepoint liveness extends through the preceding calls.
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
		`call void \(\.\.\.\) @llvm\.fake\.use\(ptr %v[0-9]+\)`)
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
	// StackObjects are function-wide, so the plugin initializes their four
	// pointer leaves at entry. VarDef separately starts the source lifetime
	// before the frontend's whole-object zero initialization.
	if !bytes.Contains(rewrittenFunction, []byte("call void @llvm.lifetime.start")) {
		t.Fatalf("stack-object lifetime marker was not preserved\n%s", rewrittenFunction)
	}
	if got, want := bytes.Count(rewrittenFunction, []byte("store ptr null")), 4; got != want {
		t.Fatalf("StackObject entry pointer initializers=%d, want %d\n%s", got, want, rewrittenFunction)
	}
	entryInitialize := bytes.Index(rewrittenFunction, []byte("store ptr null"))
	lifetimeStart := bytes.Index(rewrittenFunction, []byte("call void @llvm.lifetime.start"))
	zeroInitialize := bytes.Index(rewrittenFunction, []byte("call void @llvm.memset.inline"))
	if entryInitialize < 0 || lifetimeStart < 0 || zeroInitialize < 0 ||
		entryInitialize >= lifetimeStart || lifetimeStart >= zeroInitialize {
		t.Fatalf("VarDef lifetime does not contain the frontend zero initialization\n%s", rewrittenFunction)
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
		checks.goallcLocals,
		[][]int{nil},
		[][]int{nil},
		[]int32{-1, 0},
		[]int32{0, 0, 0, 0, 0})

	goallcStackObjects := llvmABIStackObjects(t, symbol)
	if got, want := len(goallcStackObjects), 1; got != want {
		t.Fatalf("GoALLC StackObjects=%v, want one object", goallcStackObjects)
	}
	if object := goallcStackObjects[0]; object.Offset != checks.goallcStackObject || object.Size != 40 ||
		object.PtrBytes != 40 || object.GCData == nil ||
		object.GCData.Name != "runtime.gcbits.1d00000000000000" || object.GCData.ABI != 0 {
		t.Fatalf("GoALLC StackObject=%+v, want offset=%d, size=40, ptrbytes=40, and ABI0 runtime.gcbits.1d00000000000000", object, checks.goallcStackObject)
	}

	// StackObjects describe the function-wide identity and layout of the
	// address-taken object. Its direct gc-live alloca address is an activity and
	// relocation signal only; it must not duplicate the object's pointer fields
	// in LocalsPointerMaps. Native Go currently emits a static locals bitmap for
	// this fixture, which remains useful evidence of the native metadata shape
	// but is not GoALLC's StackObject/deopt contract.
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
