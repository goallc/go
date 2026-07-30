// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type llvmABIDocument struct {
	Members []struct {
		GoObject *struct {
			Symbols []llvmABISymbol `json:"symbols"`
		} `json:"go_object"`
	} `json:"members"`
}

type llvmABISymbol struct {
	Name     string `json:"name"`
	ABI      uint16 `json:"abi"`
	Function *struct {
		Info *struct {
			Args uint32 `json:"args"`
		} `json:"info"`
		PCData []struct {
			Kind   string `json:"kind"`
			Ranges []struct {
				Value int32 `json:"value"`
			} `json:"ranges"`
		} `json:"pcdata"`
		FuncData []struct {
			Kind     string `json:"kind"`
			StackMap *struct {
				Count   int32 `json:"count"`
				NumBits int32 `json:"num_bits"`
				Bitmaps []struct {
					SetBits []int `json:"set_bits"`
				} `json:"bitmaps"`
			} `json:"stack_map"`
		} `json:"funcdata"`
		StackMapQueries []struct {
			StackMapIndex int32  `json:"stack_map_index"`
			DecodeError   string `json:"decode_error"`
		} `json:"stack_map_queries"`
	} `json:"function"`
}

type llvmABICase struct {
	name        string
	args        uint32
	pointerBits []int
}

func runLLVMABIDifferentialTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("exact ABI differential expectations are currently qualified on darwin/arm64")
	}

	dir := t.TempDir()
	source := filepath.Join(gorootTestDir, "abi", "llvm_args_results.go")
	nativeObject := filepath.Join(dir, "native.o")
	llvmArchive := filepath.Join(dir, "goallc.a")
	llvmIR := llvmArchive + ".ll"
	goallcObject := filepath.Join(dir, "goallc.o")

	nativeAssembly := runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-S", "-l", "-p=main", "-importcfg="+stdlibImportcfgFile(),
		"-o", nativeObject, source)
	if !bytes.Contains(nativeAssembly, []byte("main.mixedABI")) ||
		!bytes.Contains(nativeAssembly, []byte("main.bothOverflow")) {
		t.Fatalf("native -S output does not contain representative ABI functions")
	}

	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=main", "-importcfg="+stdlibImportcfgFile(),
		"-enablellvm", "-llvmironly", "-o", llvmArchive, source)
	ir, err := os.ReadFile(llvmIR)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range [][]byte{
		[]byte("define goabiinternal"),
		[]byte(`"go_results_tuple"`),
		[]byte(`gc "goallc"`),
		[]byte(`"go-stack-growth-statepoint"`),
	} {
		if !bytes.Contains(ir, needle) {
			t.Fatalf("GoALLC IR does not contain %q", needle)
		}
	}

	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, ir, opt, "-passes=verify", "-disable-output")

	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	rewrittenIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin,
		"-goallc-pass-plugin-emit-ir",
		"-filetype=null", "-o", "-", llvmIR)
	for _, needle := range [][]byte{
		[]byte(`"gc-live"(`),
		[]byte("@llvm.experimental.gc.relocate"),
		[]byte("extractvalue %main.stackAggregate"),
	} {
		if !bytes.Contains(rewrittenIR, needle) {
			t.Fatalf("rewritten GoALLC IR does not contain %q", needle)
		}
	}
	for _, line := range bytes.Split(rewrittenIR, []byte("\n")) {
		const gcLive = `"gc-live"(`
		start := bytes.Index(line, []byte(gcLive))
		if start < 0 {
			continue
		}
		operands := line[start+len(gcLive):]
		if end := bytes.IndexByte(operands, ')'); end >= 0 {
			operands = operands[:end]
		}
		if bytes.Contains(operands, []byte("{")) ||
			bytes.Contains(operands, []byte("[")) ||
			bytes.Contains(operands, []byte("<")) ||
			bytes.Contains(operands, []byte("%main.stackAggregate")) {
			t.Fatalf("aggregate survived in gc-live: %s", line)
		}
	}
	checkLLVMABIStatepointTupleAttrs(t, rewrittenIR,
		"overflowResults", "stackAggregateResult", "bothOverflow")
	runLLVMABICommand(t, rewrittenIR, opt,
		"-load-pass-plugin="+plugin, "-passes=verify", "-disable-output", "-")

	goallcAssembly := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-filetype=asm", llvmIR, "-o", "-")
	for _, name := range []string{"main.mixedABI", "main.growPointer", "main.bothOverflow"} {
		if !bytes.Contains(goallcAssembly, []byte(name)) {
			t.Fatalf("GoALLC assembly does not contain %s", name)
		}
	}
	checkLLVMABIAssembly(t, nativeAssembly, goallcAssembly)

	runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-filetype=obj", llvmIR, "-o", goallcObject)

	native := readLLVMABIObject(t, nativeObject)
	goallc := readLLVMABIObject(t, goallcObject)
	cases := []llvmABICase{
		{name: "checkpoint", args: 8, pointerBits: []int{0}},
		{name: "mixedABI", args: 152, pointerBits: []int{2, 4, 18}},
		{name: "growPointer", args: 16, pointerBits: []int{0}},
		{name: "overflowResults", args: 48, pointerBits: []int{2, 3, 4, 5}},
		{name: "stackAggregateResult", args: 40, pointerBits: []int{4}},
		{name: "bothOverflow", args: 168, pointerBits: []int{2, 6, 20}},
		{name: "requireAggregate", args: 40, pointerBits: []int{4}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nativeSymbol := findLLVMABISymbol(t, native, "main."+tc.name)
			goallcSymbol := findLLVMABISymbol(t, goallc, "main."+tc.name)
			checkLLVMABISymbol(t, "native", nativeSymbol, tc)
			checkLLVMABISymbol(t, "GoALLC", goallcSymbol, tc)

			if nativeSymbol.Function.Info.Args != goallcSymbol.Function.Info.Args {
				t.Fatalf("FuncInfo args differ: native=%d GoALLC=%d",
					nativeSymbol.Function.Info.Args, goallcSymbol.Function.Info.Args)
			}
			nativeBits := llvmABIEntryPointerBits(t, nativeSymbol)
			goallcBits := llvmABIEntryPointerBits(t, goallcSymbol)
			if !reflect.DeepEqual(nativeBits, goallcBits) {
				t.Fatalf("entry ArgsPointerMaps differ: native=%v GoALLC=%v",
					nativeBits, goallcBits)
			}
		})
	}

}

func checkLLVMABIStatepointTupleAttrs(t *testing.T, ir []byte, callees ...string) {
	t.Helper()
	for _, callee := range callees {
		call := regexp.MustCompile(`(?m)^.*@llvm\.experimental\.gc\.statepoint.*@main\.` +
			regexp.QuoteMeta(callee) + `.*#([0-9]+)$`).FindSubmatch(ir)
		if len(call) != 2 {
			t.Fatalf("rewritten IR has no attributed statepoint call to main.%s", callee)
		}
		attr := regexp.MustCompile(`(?m)^attributes #` + string(call[1]) +
			` = \{[^\n]*"go_results_tuple"`).Find(ir)
		if attr == nil {
			t.Fatalf("statepoint call to main.%s lost go_results_tuple", callee)
		}
	}
}

func checkLLVMABIAssembly(t *testing.T, native, goallc []byte) {
	t.Helper()

	// The native listing is the ABI reference for frame/argument size. These
	// signatures deliberately exercise input-only, result-only, and combined
	// overflow layouts.
	for _, pattern := range []string{
		`TEXT\s+main\.mixedABI\(SB\), ABIInternal, \$[0-9]+-152`,
		`TEXT\s+main\.overflowResults\(SB\), ABIInternal, \$[0-9]+-48`,
		`TEXT\s+main\.stackAggregateResult\(SB\), ABIInternal, \$[0-9]+-40`,
		`TEXT\s+main\.bothOverflow\(SB\), ABIInternal, \$[0-9]+-168`,
		`(?s)CALL\s+main\.overflowResults\(SB\).*?MOVD\s+R15,.*?LDP\s+8\(RSP\)`,
		`(?s)CALL\s+main\.stackAggregateResult\(SB\).*?LDP\s+8\(RSP\).*?MOVD\s+R15,`,
		`(?s)CALL\s+main\.bothOverflow\(SB\).*?MOVD\s+R15,.*?LDP\s+32\(RSP\)`,
	} {
		if !regexp.MustCompile(pattern).Match(native) {
			t.Fatalf("native assembly does not match %q", pattern)
		}
	}

	// GoALLC may choose different virtual/temporary registers, but the Go ABI
	// stack-result offsets and the register-stack-register result order must
	// agree with the native listing.
	for _, pattern := range []string{
		`(?m)^main\.mixedABI:`,
		`(?m)^main\.overflowResults:`,
		`(?m)^main\.stackAggregateResult:`,
		`(?m)^main\.bothOverflow:`,
		`(?s)\bbl\s+main\.overflowResults.*?\bldp\s+x[0-9]+, x[0-9]+, \[sp, #8\]`,
		`(?s)\bbl\s+main\.overflowResults.*?\b(?:mov|stp)\s+[^\n]*x15`,
		`(?s)\bbl\s+main\.stackAggregateResult.*?\bldp\s+x[0-9]+, x[0-9]+, \[sp, #8\].*?\bmov\s+x[0-9]+, x15`,
		`(?s)\bbl\s+main\.bothOverflow.*?\bldp\s+x[0-9]+, x[0-9]+, \[sp, #32\]`,
		`(?s)\bbl\s+main\.bothOverflow.*?\b(?:mov|stp)\s+[^\n]*x15`,
	} {
		if !regexp.MustCompile(pattern).Match(goallc) {
			t.Fatalf("GoALLC assembly does not match %q\n%s", pattern,
				llvmABIAssemblyExcerpt(goallc, "bl\tmain.overflowResults"))
		}
	}
}

func llvmABIAssemblyExcerpt(assembly []byte, needle string) []byte {
	start := bytes.Index(assembly, []byte(needle))
	if start < 0 {
		return assembly
	}
	end := start + 1200
	if end > len(assembly) {
		end = len(assembly)
	}
	return assembly[start:end]
}

func runLLVMABICommand(t *testing.T, stdin []byte, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return out
}

func llvmABIPassPlugin(t *testing.T, llc string) string {
	t.Helper()
	if configured := os.Getenv("GOALLC_PASS_PLUGIN"); configured != "" {
		return configured
	}
	suffix := ".so"
	if runtime.GOOS == "darwin" {
		suffix = ".dylib"
	}
	root := filepath.Dir(filepath.Dir(llc))
	for _, libdir := range []string{"lib", "lib64"} {
		path := filepath.Join(root, libdir, "GoALLCStatepoints"+suffix)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	t.Fatalf("GoALLC pass plugin is unavailable next to %s; set GOALLC_PASS_PLUGIN", llc)
	return ""
}

func readLLVMABIObject(t *testing.T, object string) llvmABIDocument {
	t.Helper()
	out := runLLVMABICommand(t, nil, goTool, "tool", "objview", "-json", object)
	var document llvmABIDocument
	if err := json.Unmarshal(out, &document); err != nil {
		t.Fatalf("decode objview output for %s: %v", object, err)
	}
	return document
}

func findLLVMABISymbol(t *testing.T, document llvmABIDocument, name string) llvmABISymbol {
	t.Helper()
	for _, member := range document.Members {
		if member.GoObject == nil {
			continue
		}
		for _, symbol := range member.GoObject.Symbols {
			if symbol.Name == name {
				return symbol
			}
		}
	}
	t.Fatalf("objview output has no symbol %s", name)
	return llvmABISymbol{}
}

func checkLLVMABISymbol(t *testing.T, backend string, symbol llvmABISymbol, tc llvmABICase) {
	t.Helper()
	if symbol.ABI != 1 {
		t.Fatalf("%s symbol ABI=%d, want ABIInternal (1)", backend, symbol.ABI)
	}
	if symbol.Function == nil || symbol.Function.Info == nil {
		t.Fatalf("%s symbol has no FuncInfo", backend)
	}
	if symbol.Function.Info.Args != tc.args {
		t.Fatalf("%s FuncInfo args=%d, want %d", backend, symbol.Function.Info.Args, tc.args)
	}
	if got := llvmABIEntryPointerBits(t, symbol); !reflect.DeepEqual(got, tc.pointerBits) {
		t.Fatalf("%s entry pointer bits=%v, want %v", backend, got, tc.pointerBits)
	}

	haveStackMapIndex := false
	for _, pcdata := range symbol.Function.PCData {
		if pcdata.Kind == "stack_map_index" && len(pcdata.Ranges) != 0 {
			haveStackMapIndex = true
		}
	}
	if !haveStackMapIndex {
		t.Fatalf("%s has no decoded PCDATA_StackMapIndex", backend)
	}
	for _, query := range symbol.Function.StackMapQueries {
		if query.DecodeError != "" {
			t.Fatalf("%s stack-map query failed: %s", backend, query.DecodeError)
		}
	}
}

func llvmABIEntryPointerBits(t *testing.T, symbol llvmABISymbol) []int {
	t.Helper()
	for _, data := range symbol.Function.FuncData {
		if data.Kind != "args_pointer_maps" {
			continue
		}
		if data.StackMap == nil || data.StackMap.Count == 0 ||
			len(data.StackMap.Bitmaps) == 0 {
			t.Fatalf("%s has no decoded ArgsPointerMaps", symbol.Name)
		}
		var entry []int
		for i, bitmap := range data.StackMap.Bitmaps {
			bits := append([]int(nil), bitmap.SetBits...)
			sort.Ints(bits)
			if i == 0 {
				entry = bits
			} else if len(bits) != 0 {
				t.Fatalf("%s has non-entry pointer bits in ArgsPointerMaps[%d]: %v",
					symbol.Name, i, bits)
			}
		}
		return entry
	}
	t.Fatalf("%s has no FUNCDATA_ArgsPointerMaps", symbol.Name)
	return nil
}
