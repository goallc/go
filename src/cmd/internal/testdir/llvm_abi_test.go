// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"cmd/internal/goobj"
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
			Symbols    []llvmABISymbol `json:"symbols"`
			References []llvmABISymbol `json:"references"`
		} `json:"go_object"`
	} `json:"members"`
}

type llvmABIReference struct {
	PkgKind  string `json:"pkg_kind"`
	SymIndex uint32 `json:"sym_index"`
	Name     string `json:"name"`
}

type llvmABISymbol struct {
	Class       string   `json:"class"`
	ClassIndex  uint32   `json:"class_index"`
	Name        string   `json:"name"`
	ABI         uint16   `json:"abi"`
	Flags       uint64   `json:"flags"`
	FlagNames   []string `json:"flag_names"`
	Relocations []struct {
		Type   string           `json:"type"`
		Target llvmABIReference `json:"target"`
	} `json:"relocations"`
	Function *struct {
		Info *struct {
			Args   uint32 `json:"args"`
			Locals uint32 `json:"locals"`
		} `json:"info"`
		PCTables []struct {
			Kind   string `json:"kind"`
			Ranges []struct {
				Value int32 `json:"value"`
			} `json:"ranges"`
		} `json:"pc_tables"`
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
			StackObjects []struct {
				Offset   int32 `json:"offset"`
				Size     int32 `json:"size"`
				PtrBytes int32 `json:"ptr_bytes"`
				GCData   *struct {
					Name string `json:"name"`
					ABI  uint16 `json:"abi"`
				} `json:"gcdata"`
			} `json:"stack_objects"`
		} `json:"funcdata"`
		StackMapQueries []struct {
			StackMapIndex int32  `json:"stack_map_index"`
			DecodeError   string `json:"decode_error"`
		} `json:"stack_map_queries"`
	} `json:"function"`
}

type llvmABICase struct {
	name            string
	args            uint32
	pointerBits     []int
	nativeArgsMaps  [][]int
	goallcArgsMaps  [][]int
	nativeStackMaps []int32
	goallcStackMaps []int32
	nativeQueryMaps [][]int
	goallcQueryMaps [][]int
	checkFullMaps   bool
	nativeLocals    uint32
	goallcLocals    uint32
	nativeLocalMaps [][]int
	goallcLocalMaps [][]int
	nativeQueries   []int32
	goallcQueries   []int32
}

func runLLVMABIDifferentialTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		runLLVMAArch64ABIDifferentialTest(t, gorootTestDir)
	case "linux/amd64":
		runLLVMAMD64ArgsPointerMapDifferentialTest(t, gorootTestDir)
	default:
		t.Skip("exact ABI differential expectations are currently qualified on darwin/arm64 and linux/amd64")
	}
}

func runLLVMAArch64ABIDifferentialTest(t *testing.T, gorootTestDir string) {
	t.Helper()
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
		[]byte(" goret("),
		[]byte(` "goretindex"="`),
		[]byte(`"go_results_tuple"`),
		[]byte(`gc "goallc"`),
	} {
		if !bytes.Contains(ir, needle) {
			t.Fatalf("GoALLC IR does not contain %q", needle)
		}
	}
	if bytes.Contains(ir, []byte(`"go-stack-growth-statepoint"`)) {
		t.Fatal("GoALLC IR still contains the obsolete stack-growth attribute")
	}
	for _, tc := range []struct {
		name      string
		required  [][]byte
		forbidden [][]byte
	}{
		{
			name: "main.memoryOnlyPointerResult",
			required: [][]byte{
				[]byte(`ptr goret([2 x ptr]) align 8 "goretindex"="0" %.result0`),
				[]byte(`getelementptr i8, ptr %.result0`),
				[]byte(`store ptr %first`),
				[]byte(`store ptr %second`),
			},
			forbidden: [][]byte{[]byte("alloca"), []byte("memmove")},
		},
		{
			name: "main.initializedStackResult",
			required: [][]byte{
				[]byte(`ptr goret(ptr) align 8 "goretindex"="16" %.result16`),
				[]byte(`store ptr %pointer, ptr %.result16`),
			},
			forbidden: [][]byte{[]byte("alloca"), []byte("memmove")},
		},
		{
			name: "main.deferredNamedStackResult",
			required: [][]byte{
				[]byte(`ptr goret([2 x ptr]) align 8 "goretindex"="16" %.result16`),
				[]byte(`getelementptr i8, ptr %.result16`),
			},
			forbidden: [][]byte{
				[]byte("!goallc.defer_result"),
				[]byte("memmove.p0.p0.i64(ptr align 8 %.result16"),
				[]byte("%open.defer.result16"),
			},
		},
		{
			name: "main.escapedNamedStackResult",
			required: [][]byte{
				[]byte(`ptr goret([2 x ptr]) align 8 "goretindex"="16" %.result16`),
				[]byte(`call goabiinternal ptr @runtime.mallocgcSmallScanNoHeaderSC`),
				[]byte(`call void @llvm.memmove.p0.p0.i64(ptr align 8 %.result16`),
			},
		},
	} {
		body := llvmABIIRFunction(t, ir, tc.name)
		for _, required := range tc.required {
			if !bytes.Contains(body, required) {
				t.Fatalf("%s does not contain %q\n%s", tc.name, required, body)
			}
		}
		for _, forbidden := range tc.forbidden {
			if bytes.Contains(body, forbidden) {
				t.Fatalf("%s still contains %q\n%s", tc.name, forbidden, body)
			}
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
	for _, pattern := range []string{
		`(?s)define goabiinternal \{ ptr, i64 \} @main\.liveScalarStackArgument.*?ptr byval\(ptr\) align 8 %pointer.*?load ptr, ptr %pointer.*?"gc-live"\(ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
		`(?s)define goabiinternal \{ ptr, ptr, i64 \} @main\.livePointerSequenceStackArguments.*?ptr byval\(ptr\) align 8 %first.*?ptr byval\(ptr\) align 8 %second.*?load ptr, ptr %first.*?load ptr, ptr %second.*?"gc-live"\(ptr %[[:alnum:]$._-]+, ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
		`(?s)define goabiinternal \{ ptr, ptr, i64 \} @main\.livePointerAggregateStackArgument.*?ptr byval\(%main\.pointerStackAggregate\) align 8 %value.*?load %main\.pointerStackAggregate, ptr %value.*?"gc-live"\(ptr %[[:alnum:]$._-]+, ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
		`(?s)define goabiinternal void @main\.callMemoryOnlyPointerResult\(ptr %fn, ptr %first, ptr %second, ptr goret\(\[2 x ptr\]\) align 8 "goretindex"="0" %\.result0\).*?@llvm\.experimental\.gc\.statepoint.*?ptr goret\(\[2 x ptr\]\) align 8 "goretindex"="0" %[[:alnum:]$._-]+, ptr nest %fn.*?"gc-live"\(ptr %\.result0, ptr %[[:alnum:]$._-]+\).*?ret void`,
		`(?s)define goabiinternal \{ i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64, i64 \} @main\.pointerAggregateBothOverflow.*?ptr byval\(%main\.pointerStackAggregate\) align 8 %value.*?ptr goret\(ptr\) align 8 "goretindex"="16" %\.result16.*?ptr goret\(ptr\) align 8 "goretindex"="17" %\.result17.*?load %main\.pointerStackAggregate, ptr %value.*?"gc-live"\(ptr %[[:alnum:]$._-]+, ptr %\.result17, ptr %[[:alnum:]$._-]+, ptr %\.result16\).*?gc\.relocate`,
	} {
		if !regexp.MustCompile(pattern).Match(rewrittenIR) {
			t.Fatalf("rewritten GoALLC ABI IR does not match %q", pattern)
		}
	}
	checkLLVMABIStatepointTupleAttrs(t, rewrittenIR,
		"liveScalarStackArgument", "livePointerSequenceStackArguments",
		"livePointerAggregateStackArgument", "overflowResults",
		"stackAggregateResult", "bothOverflow", "pointerAggregateBothOverflow")
	checkLLVMABIStatepointGoRetAttrs(t, rewrittenIR, map[string][]string{
		"memoryOnlyPointerResult":      {"0"},
		"overflowResults":              {"16", "17"},
		"initializedStackResult":       {"16"},
		"deferredNamedStackResult":     {"16"},
		"escapedNamedStackResult":      {"16"},
		"stackAggregateResult":         {"15"},
		"bothOverflow":                 {"16", "17"},
		"pointerAggregateBothOverflow": {"16", "17"},
		"stackResultsAfterGrowth":      {"16"},
	})
	runLLVMABICommand(t, rewrittenIR, opt,
		"-load-pass-plugin="+plugin, "-passes=verify", "-disable-output", "-")

	// Match the production optimization pipeline before checking the final
	// lowering. Loaded pointers deliberately use ordinary statepoint spill slots;
	// this test does not require the optional fixed-home reuse optimization.
	optimizedLLVMIR := llvmArchive + ".opt.ll"
	runLLVMABICommand(t, nil, opt, "-passes=default<O2>", "-S", llvmIR,
		"-o", optimizedLLVMIR)

	machineIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-stop-after=finalize-isel",
		"-o", "-", optimizedLLVMIR)
	for _, pattern := range []string{
		`(?s)name:\s+main\.liveScalarStackArgument.*?fixedStack:.*?offset:\s+8.*?size:\s+8.*?isAliased:\s+true.*?stack:\s+.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*1, 8, %stack\.[0-9]+, 0.*?LDRXui\s+%stack\.[0-9]+`,
		`(?s)name:\s+main\.livePointerSequenceStackArguments.*?fixedStack:.*?offset:\s+24.*?offset:\s+8.*?stack:\s+.*?size:\s+8.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*1, 8, %stack\.[0-9]+, 0[^\n]*1, 8, %stack\.[0-9]+, 0.*?LDRXui\s+%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+`,
		`(?s)name:\s+main\.livePointerAggregateStackArgument.*?fixedStack:.*?offset:\s+8.*?size:\s+24.*?isAliased:\s+true.*?stack:\s+.*?size:\s+8.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*1, 8, %stack\.[0-9]+, 0[^\n]*1, 8, %stack\.[0-9]+, 0.*?LDRXui\s+%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+`,
		`(?s)name:\s+main\.pointerAggregateBothOverflow.*?fixedStack:.*?offset:\s+8.*?size:\s+24.*?isAliased:\s+true.*?stack:\s+.*?size:\s+8.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*1, 8, %stack\.[0-9]+, 0[^\n]*1, 8, %stack\.[0-9]+, 0.*?LDRXui\s+%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+`,
	} {
		if !regexp.MustCompile(pattern).Match(machineIR) {
			t.Fatalf("GoALLC ABI MIR does not match %q", pattern)
		}
	}

	goallcAssembly := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-filetype=asm", optimizedLLVMIR, "-o", "-")
	for _, name := range []string{
		"main.mixedABI", "main.liveScalarStackArgument",
		"main.livePointerSequenceStackArguments",
		"main.livePointerAggregateStackArgument",
		"main.growPointer", "main.bothOverflow",
		"main.pointerAggregateBothOverflow",
	} {
		if !bytes.Contains(goallcAssembly, []byte(name)) {
			t.Fatalf("GoALLC assembly does not contain %s", name)
		}
	}
	checkLLVMABIAssembly(t, nativeAssembly, goallcAssembly)

	runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-filetype=obj", optimizedLLVMIR, "-o", goallcObject)

	native := readLLVMABIObject(t, nativeObject)
	goallc := readLLVMABIObject(t, goallcObject)
	cases := []llvmABICase{
		{
			name: "checkpoint", args: 8, pointerBits: []int{0},
			nativeArgsMaps:  [][]int{{0}, nil},
			goallcArgsMaps:  [][]int{{0}, nil, nil},
			nativeStackMaps: []int32{-1, 0, 1, -1},
			goallcStackMaps: []int32{-1, 1, -1, 2},
		},
		{
			name: "mixedABI", args: 152, pointerBits: []int{2, 4, 18},
			nativeArgsMaps:  [][]int{{2, 4, 18}, nil},
			goallcArgsMaps:  [][]int{{2, 4, 18}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1, 1},
			goallcQueryMaps: [][]int{{2, 4, 18}, nil, nil, nil, nil},
		},
		{
			name: "liveScalarStackArgument", args: 136, pointerBits: []int{0},
			nativeArgsMaps:  [][]int{{0}, nil},
			goallcArgsMaps:  [][]int{{0}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
			checkFullMaps:   true,
			nativeLocals:    8,
			goallcLocals:    40,
			nativeLocalMaps: [][]int{nil, nil},
			goallcLocalMaps: [][]int{nil, {3}},
			nativeQueries:   []int32{0, -1},
			goallcQueries:   []int32{-1, 1},
		},
		{
			name: "livePointerSequenceStackArguments", args: 152, pointerBits: []int{0, 2},
			nativeArgsMaps:  [][]int{{0, 2}, nil},
			goallcArgsMaps:  [][]int{{0, 2}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
			checkFullMaps:   true,
			nativeLocals:    8,
			goallcLocals:    40,
			nativeLocalMaps: [][]int{nil, nil},
			goallcLocalMaps: [][]int{nil, {2, 3}},
			nativeQueries:   []int32{0, -1},
			goallcQueries:   []int32{-1, 1},
		},
		{
			name: "livePointerAggregateStackArgument", args: 136, pointerBits: []int{0, 2},
			nativeArgsMaps:  [][]int{{0, 2}, nil},
			goallcArgsMaps:  [][]int{{0, 2}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
			checkFullMaps:   true,
			nativeLocals:    8,
			goallcLocals:    40,
			nativeLocalMaps: [][]int{nil, nil},
			goallcLocalMaps: [][]int{nil, {2, 3}},
			nativeQueries:   []int32{0, -1},
			goallcQueries:   []int32{-1, 1},
		},
		{
			name: "growPointer", args: 16, pointerBits: []int{0},
			nativeArgsMaps:  [][]int{{0}, nil},
			goallcArgsMaps:  [][]int{{0}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
		},
		{
			name: "memoryOnlyPointerResult", args: 32, pointerBits: []int{2, 3},
			nativeArgsMaps:  [][]int{{2, 3}, nil},
			goallcArgsMaps:  [][]int{{2, 3}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
		},
		{
			name: "overflowResults", args: 48, pointerBits: []int{2, 3, 4, 5},
			nativeArgsMaps:  [][]int{{2, 3, 4, 5}, nil},
			goallcArgsMaps:  [][]int{{2, 3, 4, 5}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
		},
		{
			name: "initializedStackResult", args: 16, pointerBits: []int{1},
			nativeArgsMaps:  [][]int{{1}, nil},
			goallcArgsMaps:  [][]int{{1}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
		},
		{
			name: "stackAggregateResult", args: 40, pointerBits: []int{4},
			nativeArgsMaps:  [][]int{{4}, nil},
			goallcArgsMaps:  [][]int{{4}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
		},
		{
			name: "bothOverflow", args: 168, pointerBits: []int{2, 6, 20},
			nativeArgsMaps:  [][]int{{2, 6, 20}, nil},
			goallcArgsMaps:  [][]int{{2, 6, 20}, nil, nil},
			nativeStackMaps: []int32{-1, 0, 1, -1},
			goallcStackMaps: []int32{-1, 1, -1, 2},
		},
		{
			name: "pointerAggregateBothOverflow", args: 152, pointerBits: []int{0, 2},
			nativeArgsMaps:  [][]int{{0, 2}, nil},
			goallcArgsMaps:  [][]int{{0, 2}, nil},
			nativeStackMaps: []int32{-1, 0, -1},
			goallcStackMaps: []int32{-1, 1, -1},
			checkFullMaps:   true,
			nativeLocals:    8,
			goallcLocals:    152,
			nativeLocalMaps: [][]int{nil, nil},
			goallcLocalMaps: [][]int{nil, {16, 17}},
			nativeQueries:   []int32{0, -1},
			goallcQueries:   []int32{-1, 1},
		},
		{
			name: "requireAggregate", args: 40, pointerBits: []int{4},
			nativeArgsMaps:  [][]int{{4}, nil},
			goallcArgsMaps:  [][]int{{4}, nil, nil},
			nativeStackMaps: []int32{-1, 0, 1, -1},
			goallcStackMaps: []int32{-1, 1, -1, 2},
			goallcQueryMaps: [][]int{{4}, nil, nil, nil},
		},
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
			if tc.checkFullMaps {
				checkLLVMABISourceStackMaps(t, "native", nativeSymbol,
					tc.nativeLocals, tc.nativeArgsMaps, tc.nativeLocalMaps,
					tc.nativeStackMaps, tc.nativeQueries)
				checkLLVMABISourceStackMaps(t, "GoALLC", goallcSymbol,
					tc.goallcLocals, tc.goallcArgsMaps, tc.goallcLocalMaps,
					tc.goallcStackMaps, tc.goallcQueries)
			} else {
				checkLLVMABIStackMaps(t, "native", nativeSymbol,
					tc.nativeArgsMaps, tc.nativeStackMaps)
				checkLLVMABIStackMaps(t, "GoALLC", goallcSymbol,
					tc.goallcArgsMaps, tc.goallcStackMaps)
			}
			if tc.nativeQueryMaps != nil {
				checkLLVMABIStackMapQueryBitmaps(t, "native", nativeSymbol,
					tc.nativeQueryMaps)
			}
			if tc.goallcQueryMaps != nil {
				checkLLVMABIStackMapQueryBitmaps(t, "GoALLC", goallcSymbol,
					tc.goallcQueryMaps)
			}
		})
	}

	t.Run("source-args-pointer-maps", func(t *testing.T) {
		runLLVMABIArgsPointerMapSourceTest(t, gorootTestDir, llc, opt, plugin)
	})
	t.Run("machine-args-pointer-maps", func(t *testing.T) {
		runLLVMABIArgsPointerMapMachineTest(t, filepath.Dir(gorootTestDir), llc, plugin)
	})
}

func runLLVMAMD64ArgsPointerMapDifferentialTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(gorootTestDir, "abi", "llvm_args_pointer_maps.go")
	nativeObject := filepath.Join(dir, "native.o")
	goallcArchive := filepath.Join(dir, "goallc.a")
	goallcIR := goallcArchive + ".ll"
	goallcObject := filepath.Join(dir, "goallc.o")

	nativeAssembly := runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-S", "-l", "-p=p", "-o", nativeObject, source)
	for _, pattern := range []string{
		`TEXT\s+p\.initializedPointerResult\(SB\), ABIInternal, \$8-72`,
		`(?s)TEXT\s+p\.initializedPointerResult.*?MOVQ\s+AX,\s+p\.pointer\+80\(SP\).*?CALL\s+p\.safepoint\(SB\).*?MOVQ\s+AX,\s+p\.result\+72\(SP\)`,
		`(?s)TEXT\s+p\.partiallyInitializedAggregateResult.*?MOVQ\s+AX,\s+p\.first\+88\(SP\).*?MOVQ\s+BX,\s+p\.second\+96\(SP\).*?MOVQ\s+AX,\s+p\.result\+64\(SP\).*?MOVQ\s+AX,\s+p\.result\+80\(SP\)`,
		`(?s)TEXT\s+p\.liveScalarStackArgument.*?CALL\s+p\.safepoint\(SB\).*?MOVQ\s+p\.pointer\+72\(SP\),\s+AX`,
		`(?s)TEXT\s+p\.liveAggregateStackArgument.*?CALL\s+p\.safepoint\(SB\).*?MOVQ\s+p\.value\+56\(SP\),\s+AX.*?MOVQ\s+p\.value\+72\(SP\),\s+BX`,
	} {
		if !regexp.MustCompile(pattern).Match(nativeAssembly) {
			t.Fatalf("native amd64 assembly does not match %q", pattern)
		}
	}

	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=p", "-enablellvm", "-llvmironly", "-o", goallcArchive, source)
	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", goallcIR)
	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	// Keep direct llc invocations aligned with cmd/llvmtoolexec. In particular,
	// result-slot addresses after statepoints must not be CSE'd with addresses
	// materialized before the statepoint.
	llcCodegenArgs := func(args ...string) []string {
		return append([]string{
			"-load-pass-plugin=" + plugin,
			"-trap-unreachable",
			"-disable-machine-cse",
			"-disable-lsr",
		}, args...)
	}
	rewrittenIR := runLLVMABICommand(t, nil, llc,
		llcCodegenArgs("-goallc-pass-plugin-emit-ir",
			"-filetype=null", "-o", "-", goallcIR)...)
	for _, pattern := range []string{
		`(?s)define goabiinternal ptr @p\.liveScalarStackArgument.*?ptr byval\(ptr\) align 8 %pointer.*?load ptr, ptr %pointer.*?"gc-live"\(ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
		`(?s)define goabiinternal \{ ptr, ptr \} @p\.liveAggregateStackArgument.*?ptr byval\(%p\.pointerAggregate\) align 8 %value.*?load %p\.pointerAggregate, ptr %value.*?"gc-live"\(ptr %[[:alnum:]$._-]+, ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
	} {
		if !regexp.MustCompile(pattern).Match(rewrittenIR) {
			t.Fatalf("rewritten amd64 IR does not match %q", pattern)
		}
	}
	runLLVMABICommand(t, rewrittenIR, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")
	// Match the production pipeline before checking ordinary statepoint spills.
	// Loaded pointers are not reused from byval homes in this change.
	optimizedGoallcIR := goallcArchive + ".opt.ll"
	runLLVMABICommand(t, nil, opt, "-passes=default<O2>", "-S", goallcIR,
		"-o", optimizedGoallcIR)

	machineIR := runLLVMABICommand(t, nil, llc,
		llcCodegenArgs("-stop-after=prolog-epilog",
			"-o", "-", optimizedGoallcIR)...)
	morestackName, ok := goobj.BuiltinSymbolName("runtime.morestack_noctxt", 0)
	if !ok {
		t.Fatal("runtime.morestack_noctxt ABI0 is absent from the builtin table")
	}
	morestackPattern := `(?m)^.*CALL64pcrel32 &"?` + regexp.QuoteMeta(morestackName+"<ABI0>") + `"?[^\n]*$`
	machinePatterns := map[string][]string{
		"p.initializedPointerResult": {
			`STATEPOINT -[0-9]+,[^\n]*\$rsp, 0,`,
		},
		"p.partiallyInitializedAggregateResult": {
			`STATEPOINT -[0-9]+,[^\n]*\$rsp, 0,[^\n]*\$rsp, 8,`,
		},
		"p.liveScalarStackArgument": {
			`STATEPOINT -[0-9]+,[^\n]*\$rsp, 0,`,
		},
		"p.liveAggregateStackArgument": {
			`STATEPOINT -[0-9]+,[^\n]*\$rsp, 0,[^\n]*\$rsp, 8,`,
		},
	}
	for name, patterns := range machinePatterns {
		body := llvmABIMachineFunction(t, machineIR, name)
		if !regexp.MustCompile(morestackPattern).Match(body) {
			t.Fatalf("%s PEI MIR has no builtin-encoded ABI0 morestack call\n%s", name, body)
		}
		if regexp.MustCompile(`(?m)^.*STATEPOINT[^\n]*runtime\.morestack_noctxt[^\n]*$`).Match(body) {
			t.Fatalf("%s PEI MIR still represents morestack as a statepoint\n%s", name, body)
		}
		for _, pattern := range patterns {
			if !regexp.MustCompile(pattern).Match(body) {
				t.Fatalf("%s PEI MIR does not match %q\n%s", name, pattern, body)
			}
		}
	}

	runLLVMABICommand(t, nil, llc,
		llcCodegenArgs("-filetype=obj", optimizedGoallcIR, "-o", goallcObject)...)
	// Check the instructions from the GoObj artifact used below, rather than
	// llc's diagnostic assembly-text output.
	goallcDisassembly := runLLVMABICommand(t, nil, goTool, "tool", "objdump", goallcObject)
	for _, pattern := range []string{
		`(?s)TEXT p\.initializedPointerResult.*?R_CALL:p\.safepoint.*?MOVQ\s+0\(SP\), AX.*?LEAQ\s+0x48\(BP\), CX.*?MOVQ\s+AX, 0\(CX\)`,
		`(?s)TEXT p\.partiallyInitializedAggregateResult.*?R_CALL:p\.safepoint.*?R_CALL:p\.safepoint.*?MOVQ\s+0x8\(SP\), AX.*?LEAQ\s+0x40\(BP\), CX.*?MOVQ\s+0\(SP\), DX.*?MOVQ\s+DX, 0\(CX\).*?MOVQ\s+AX, 0x10\(CX\)`,
		`(?s)TEXT p\.liveScalarStackArgument.*?R_CALL:p\.safepoint.*?MOVQ\s+0\(SP\), AX`,
		`(?s)TEXT p\.liveAggregateStackArgument.*?R_CALL:p\.safepoint.*?MOVQ\s+0x8\(SP\), BX.*?MOVQ\s+0\(SP\), AX`,
	} {
		if !regexp.MustCompile(pattern).Match(goallcDisassembly) {
			t.Fatalf("GoALLC amd64 object disassembly does not match %q\n%s", pattern, goallcDisassembly)
		}
	}

	native := readLLVMABIObject(t, nativeObject)
	goallc := readLLVMABIObject(t, goallcObject)
	type amd64Case struct {
		name          string
		args          uint32
		entryBits     []int
		goallcLocals  uint32
		goallcArgs    [][]int
		goallcMaps    [][]int
		nativePCData  []int32
		goallcPCData  []int32
		nativeQueries []int32
		goallcQueries []int32
		nativePCSP    []int32
		goallcPCSP    []int32
	}
	cases := []amd64Case{
		{
			name: "initializedPointerResult", args: 72, entryBits: []int{8},
			goallcLocals: 16, goallcArgs: [][]int{{8}, nil},
			goallcMaps:   [][]int{nil, {1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{1, -1},
			nativePCSP: []int32{0, 8, 0}, goallcPCSP: []int32{0, 8, 16, 8, 0},
		},
		{
			name: "partiallyInitializedAggregateResult", args: 88,
			entryBits: []int{9, 10}, goallcLocals: 24,
			goallcArgs: [][]int{{9, 10}, nil}, goallcMaps: [][]int{nil, {1, 2}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, 0, -1}, goallcQueries: []int32{1, 1, -1},
			nativePCSP: []int32{0, 8, 0}, goallcPCSP: []int32{0, 8, 24, 8, 0},
		},
		{
			name: "liveScalarStackArgument", args: 136, entryBits: []int{7},
			goallcLocals: 16, goallcArgs: [][]int{{7}, nil}, goallcMaps: [][]int{nil, {1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{1, -1},
			nativePCSP: []int32{0, 8, 0}, goallcPCSP: []int32{0, 8, 16, 8, 0},
		},
		{
			name: "liveAggregateStackArgument", args: 136, entryBits: []int{5, 7},
			goallcLocals: 24, goallcArgs: [][]int{{5, 7}, nil}, goallcMaps: [][]int{nil, {1, 2}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{1, -1},
			nativePCSP: []int32{0, 8, 0}, goallcPCSP: []int32{0, 8, 24, 8, 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nativeSymbol := findLLVMABISymbol(t, native, "p."+tc.name)
			goallcSymbol := findLLVMABISymbol(t, goallc, "p."+tc.name)
			base := llvmABICase{name: tc.name, args: tc.args, pointerBits: tc.entryBits}
			checkLLVMABISymbol(t, "native", nativeSymbol, base)
			checkLLVMABISymbol(t, "GoALLC", goallcSymbol, base)
			checkLLVMABISourceStackMaps(t, "native", nativeSymbol,
				8, [][]int{tc.entryBits, nil}, [][]int{nil, nil},
				tc.nativePCData, tc.nativeQueries)
			checkLLVMABISourceStackMaps(t, "GoALLC", goallcSymbol,
				tc.goallcLocals, tc.goallcArgs, tc.goallcMaps,
				tc.goallcPCData, tc.goallcQueries)
			if got := llvmABIPCTableRanges(nativeSymbol, "pcsp"); !reflect.DeepEqual(got, tc.nativePCSP) {
				t.Fatalf("native pcsp=%v, want %v", got, tc.nativePCSP)
			}
			if got := llvmABIPCTableRanges(goallcSymbol, "pcsp"); !reflect.DeepEqual(got, tc.goallcPCSP) {
				t.Fatalf("GoALLC pcsp=%v, want %v", got, tc.goallcPCSP)
			}
		})
	}
}

func llvmABIMachineFunction(t *testing.T, machineIR []byte, name string) []byte {
	t.Helper()
	startPattern := regexp.MustCompile(`(?m)^name:\s+` + regexp.QuoteMeta(name) + `\s*$`)
	start := startPattern.FindIndex(machineIR)
	if start == nil {
		t.Fatalf("MIR has no machine function %s", name)
	}
	body := machineIR[start[0]:]
	if end := bytes.Index(body, []byte("\n...")); end >= 0 {
		body = body[:end]
	}
	return body
}

func llvmABIIRFunction(t *testing.T, ir []byte, name string) []byte {
	t.Helper()
	start := regexp.MustCompile(`(?m)^define [^\n]*@` + regexp.QuoteMeta(name) + `\(`).FindIndex(ir)
	if start == nil {
		t.Fatalf("IR has no function %s", name)
	}
	body := ir[start[0]:]
	if end := bytes.Index(body, []byte("\n}")); end >= 0 {
		return body[:end+2]
	}
	t.Fatalf("IR definition for %s has no end", name)
	return nil
}

func llvmABIPCTableRanges(symbol llvmABISymbol, kind string) []int32 {
	for _, table := range symbol.Function.PCTables {
		if table.Kind != kind {
			continue
		}
		values := make([]int32, 0, len(table.Ranges))
		for _, r := range table.Ranges {
			values = append(values, r.Value)
		}
		return values
	}
	return nil
}

func runLLVMABIArgsPointerMapSourceTest(t *testing.T, gorootTestDir, llc, opt, plugin string) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(gorootTestDir, "abi", "llvm_args_pointer_maps.go")
	nativeObject := filepath.Join(dir, "native.o")
	goallcArchive := filepath.Join(dir, "goallc.a")
	goallcIR := goallcArchive + ".ll"
	goallcObject := filepath.Join(dir, "goallc.o")

	nativeAssembly := runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-S", "-l", "-p=p", "-o", nativeObject, source)
	for _, pattern := range []string{
		`(?s)TEXT\s+p\.initializedPointerResult.*?CALL\s+p\.safepoint\(SB\).*?MOVD\s+R0,\s+p\.result\(FP\)`,
		`(?s)TEXT\s+p\.liveScalarStackArgument.*?CALL\s+p\.safepoint\(SB\).*?MOVD\s+p\.pointer\(FP\),\s+R0`,
		`(?s)TEXT\s+p\.liveAggregateStackArgument.*?CALL\s+p\.safepoint\(SB\).*?MOVD\s+p\.value\(FP\),\s+R0.*?MOVD\s+p\.value\+16\(FP\),\s+R1`,
	} {
		if !regexp.MustCompile(pattern).Match(nativeAssembly) {
			t.Fatalf("native assembly does not match %q", pattern)
		}
	}

	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-l", "-p=p", "-enablellvm", "-llvmironly", "-o", goallcArchive, source)
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", goallcIR)
	rewrittenIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-goallc-pass-plugin-emit-ir",
		"-filetype=null", "-o", "-", goallcIR)
	for _, pattern := range []string{
		`(?s)define goabiinternal ptr @p\.liveScalarStackArgument.*?ptr byval\(ptr\) align 8 %pointer.*?load ptr, ptr %pointer.*?"gc-live"\(ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
		`(?s)define goabiinternal \{ ptr, ptr \} @p\.liveAggregateStackArgument.*?ptr byval\(%p\.pointerAggregate\) align 8 %value.*?load %p\.pointerAggregate, ptr %value.*?"gc-live"\(ptr %[[:alnum:]$._-]+, ptr %[[:alnum:]$._-]+\).*?gc\.relocate`,
	} {
		if !regexp.MustCompile(pattern).Match(rewrittenIR) {
			t.Fatalf("rewritten source IR does not match %q", pattern)
		}
	}
	runLLVMABICommand(t, rewrittenIR, opt, "-load-pass-plugin="+plugin,
		"-passes=verify", "-disable-output", "-")
	optimizedGoallcIR := goallcArchive + ".opt.ll"
	runLLVMABICommand(t, nil, opt, "-passes=default<O2>", "-S", goallcIR,
		"-o", optimizedGoallcIR)

	machineIR := runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin, "-stop-after=finalize-isel",
		"-o", "-", optimizedGoallcIR)
	for _, pattern := range []string{
		`(?s)name:\s+p\.liveScalarStackArgument.*?fixedStack:.*?offset:\s+8.*?isAliased:\s+true.*?stack:\s+.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+`,
		`(?s)name:\s+p\.liveAggregateStackArgument.*?fixedStack:.*?offset:\s+8.*?size:\s+24.*?isAliased:\s+true.*?stack:\s+.*?size:\s+8.*?size:\s+8.*?LDRXui\s+%fixed-stack\.[0-9]+.*?LDRXui\s+%fixed-stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STRXui[^\n]*%stack\.[0-9]+.*?STATEPOINT[^\n]*%stack\.[0-9]+[^\n]*%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+.*?LDRXui\s+%stack\.[0-9]+`,
	} {
		if !regexp.MustCompile(pattern).Match(machineIR) {
			t.Fatalf("source MIR does not match %q", pattern)
		}
	}
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-filetype=obj", optimizedGoallcIR, "-o", goallcObject)

	native := readLLVMABIObject(t, nativeObject)
	goallc := readLLVMABIObject(t, goallcObject)
	type sourceCase struct {
		name          string
		args          uint32
		entryBits     []int
		nativeLocals  uint32
		goallcLocals  uint32
		nativeArgs    [][]int
		goallcArgs    [][]int
		nativeMaps    [][]int
		goallcMaps    [][]int
		nativePCData  []int32
		goallcPCData  []int32
		nativeQueries []int32
		goallcQueries []int32
	}
	cases := []sourceCase{
		{
			name: "initializedPointerResult", args: 16, entryBits: []int{1},
			nativeLocals: 8, goallcLocals: 24,
			nativeArgs: [][]int{{1}, nil}, goallcArgs: [][]int{{1}, nil},
			nativeMaps: [][]int{nil, nil}, goallcMaps: [][]int{nil, {1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{-1, 1},
		},
		{
			name: "partiallyInitializedAggregateResult", args: 40, entryBits: []int{3, 4},
			nativeLocals: 8, goallcLocals: 24,
			nativeArgs: [][]int{{3, 4}, nil}, goallcArgs: [][]int{{3, 4}, nil},
			nativeMaps: [][]int{nil, nil}, goallcMaps: [][]int{nil, {0, 1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, 0, -1}, goallcQueries: []int32{-1, 1, 1},
		},
		{
			name: "liveScalarStackArgument", args: 136, entryBits: []int{0},
			nativeLocals: 8, goallcLocals: 24,
			nativeArgs: [][]int{{0}, nil}, goallcArgs: [][]int{{0}, nil},
			nativeMaps: [][]int{nil, nil}, goallcMaps: [][]int{nil, {1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{-1, 1},
		},
		{
			name: "liveAggregateStackArgument", args: 136, entryBits: []int{0, 2},
			nativeLocals: 8, goallcLocals: 24,
			nativeArgs: [][]int{{0, 2}, nil}, goallcArgs: [][]int{{0, 2}, nil},
			nativeMaps: [][]int{nil, nil}, goallcMaps: [][]int{nil, {0, 1}},
			nativePCData: []int32{-1, 0, -1}, goallcPCData: []int32{-1, 1, -1},
			nativeQueries: []int32{0, -1}, goallcQueries: []int32{-1, 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nativeSymbol := findLLVMABISymbol(t, native, "p."+tc.name)
			goallcSymbol := findLLVMABISymbol(t, goallc, "p."+tc.name)
			base := llvmABICase{name: tc.name, args: tc.args, pointerBits: tc.entryBits}
			checkLLVMABISymbol(t, "native", nativeSymbol, base)
			checkLLVMABISymbol(t, "GoALLC", goallcSymbol, base)
			checkLLVMABISourceStackMaps(t, "native", nativeSymbol,
				tc.nativeLocals, tc.nativeArgs, tc.nativeMaps, tc.nativePCData, tc.nativeQueries)
			checkLLVMABISourceStackMaps(t, "GoALLC", goallcSymbol,
				tc.goallcLocals, tc.goallcArgs, tc.goallcMaps, tc.goallcPCData, tc.goallcQueries)
		})
	}
}

func runLLVMABIArgsPointerMapMachineTest(t *testing.T, goroot, llc, plugin string) {
	t.Helper()
	object := filepath.Join(t.TempDir(), "args-pointer-maps.o")
	source := filepath.Join(goroot, "src", "cmd", "internal", "testdir",
		"testdata", "llvm_args_pointer_maps.mir")
	runLLVMABICommand(t, nil, llc,
		"-load-pass-plugin="+plugin,
		"-mtriple=aarch64-apple-darwin-goobj",
		"-start-after=prolog-epilog",
		"-filetype=obj", source, "-o", object)

	symbol := findLLVMABISymbol(t, readLLVMABIObject(t, object), "p.argResult")
	if symbol.Function == nil || symbol.Function.Info == nil {
		t.Fatal("machine ArgsPointerMaps symbol has no FuncInfo")
	}
	if got := symbol.Function.Info.Args; got != 16 {
		t.Fatalf("machine ArgsPointerMaps FuncInfo args=%d, want 16", got)
	}
	if got, want := llvmABIStackMapBitmaps(t, symbol, "args_pointer_maps"),
		[][]int{{1}, {0}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("machine ArgsPointerMaps=%v, want %v", got, want)
	}
	if got, want := llvmABIStackMapBitmaps(t, symbol, "locals_pointer_maps"),
		[][]int{nil, nil}; !reflect.DeepEqual(got, want) {
		t.Fatalf("machine LocalsPointerMaps=%v, want %v", got, want)
	}
	if got, want := llvmABIStackMapRanges(symbol), []int32{-1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("machine PCDATA_StackMapIndex=%v, want %v", got, want)
	}
	var queryIndexes []int32
	for _, query := range symbol.Function.StackMapQueries {
		if query.DecodeError != "" {
			t.Fatalf("machine stack-map query failed: %s", query.DecodeError)
		}
		queryIndexes = append(queryIndexes, query.StackMapIndex)
	}
	if got, want := queryIndexes, []int32{-1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("machine stack-map query indexes=%v, want %v", got, want)
	}
	t.Logf("machine ArgsPointerMaps=%v LocalsPointerMaps=%v PCDATA=%v",
		llvmABIStackMapBitmaps(t, symbol, "args_pointer_maps"),
		llvmABIStackMapBitmaps(t, symbol, "locals_pointer_maps"),
		llvmABIStackMapRanges(symbol))
}

func checkLLVMABIStatepointTupleAttrs(t *testing.T, ir []byte, callees ...string) {
	t.Helper()
	for _, callee := range callees {
		call := regexp.MustCompile(`(?m)^.*@llvm\.experimental\.gc\.statepoint.*@main\.` +
			regexp.QuoteMeta(callee) + `.*#([0-9]+)[^\n]*$`).FindSubmatch(ir)
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

func checkLLVMABIStatepointGoRetAttrs(t *testing.T, ir []byte, callees map[string][]string) {
	t.Helper()
	for callee, indexes := range callees {
		call := regexp.MustCompile(`(?m)^.*@llvm\.experimental\.gc\.statepoint.*@main\.` +
			regexp.QuoteMeta(callee) + `.*$`).Find(ir)
		if call == nil {
			t.Fatalf("rewritten IR has no statepoint call to main.%s", callee)
		}
		for _, index := range indexes {
			pattern := `goret\([^)]*\) align [0-9]+ "goretindex"="` + regexp.QuoteMeta(index) + `"`
			if !regexp.MustCompile(pattern).Match(call) {
				t.Fatalf("statepoint call to main.%s lost goret index %s: %s", callee, index, call)
			}
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
		`TEXT\s+main\.liveScalarStackArgument\(SB\), ABIInternal, \$[0-9]+-136`,
		`TEXT\s+main\.livePointerSequenceStackArguments\(SB\), ABIInternal, \$[0-9]+-152`,
		`TEXT\s+main\.livePointerAggregateStackArgument\(SB\), ABIInternal, \$[0-9]+-136`,
		`TEXT\s+main\.overflowResults\(SB\), ABIInternal, \$[0-9]+-48`,
		`TEXT\s+main\.stackAggregateResult\(SB\), ABIInternal, \$[0-9]+-40`,
		`TEXT\s+main\.bothOverflow\(SB\), ABIInternal, \$[0-9]+-168`,
		`TEXT\s+main\.pointerAggregateBothOverflow\(SB\), ABIInternal, \$[0-9]+-152`,
		`(?s)TEXT\s+main\.liveScalarStackArgument.*?CALL\s+runtime\.GC\(SB\).*?MOVD\s+main\.pointer\(FP\),\s+R0`,
		`(?s)TEXT\s+main\.livePointerSequenceStackArguments.*?CALL\s+runtime\.GC\(SB\).*?MOVD\s+main\.first\(FP\),\s+R0.*?MOVD\s+main\.second\+16\(FP\),\s+R1`,
		`(?s)TEXT\s+main\.livePointerAggregateStackArgument.*?CALL\s+runtime\.GC\(SB\).*?MOVD\s+main\.value\(FP\),\s+R0.*?MOVD\s+main\.value\+16\(FP\),\s+R1`,
		`(?s)TEXT\s+main\.pointerAggregateBothOverflow.*?CALL\s+runtime\.GC\(SB\).*?MOVD\s+main\.value\(FP\),\s+R0.*?MOVD\s+R0,\s+main\.~r16\+24\(FP\).*?MOVD\s+main\.value\+16\(FP\),\s+R0.*?MOVD\s+R0,\s+main\.~r17\+32\(FP\)`,
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
		`(?m)^main\.liveScalarStackArgument:`,
		`(?m)^main\.livePointerSequenceStackArguments:`,
		`(?m)^main\.livePointerAggregateStackArgument:`,
		`(?m)^main\.overflowResults:`,
		`(?m)^main\.stackAggregateResult:`,
		`(?m)^main\.bothOverflow:`,
		`(?m)^main\.pointerAggregateBothOverflow:`,
		`(?s)\bbl\s+main\.overflowResults.*?\bldp\s+x[0-9]+, x[0-9]+, \[sp, #8\]`,
		`(?s)\bbl\s+main\.overflowResults.*?\b(?:mov|stp)\s+[^\n]*x15`,
		`(?s)\bbl\s+main\.stackAggregateResult.*?\b(?:ldp\s+x[0-9]+, x[0-9]+|ldur\s+q[0-9]+), \[sp, #8\].*?\bmov\s+x[0-9]+, x15`,
		`(?s)\bbl\s+main\.bothOverflow.*?\bldr\s+x[0-9]+, \[sp, #32\].*?\bldr\s+x[0-9]+, \[sp, #40\]`,
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

func runLLVMCallerStateTest(t *testing.T, gorootTestDir string) {
	t.Helper()
	dir := t.TempDir()
	archive := filepath.Join(dir, "caller-state.a")
	source := filepath.Join(gorootTestDir, "internal", "runtime", "sys", "inlinegcpc.go")
	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-p=internal/runtime/sys", "-enablellvm", "-llvmironly", "-o", archive, source)
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}

	patterns := []string{
		`(?m)^define goabiinternal i64 @"internal/runtime/sys\.pc"\(\) #[0-9]+`,
		`(?m)^define goabiinternal i64 @"internal/runtime/sys\.sp"\(\) #[0-9]+`,
		`call ptr @llvm\.returnaddress\(i32 0\)`,
		`attributes #[0-9]+ = \{ noinline `,
	}
	switch runtime.GOARCH {
	case "arm64":
		patterns = append(patterns, `call ptr @llvm\.sponentry\(\)`)
	case "amd64":
		patterns = append(patterns, `call ptr @llvm\.addressofreturnaddress\(\)`, `getelementptr i8, ptr .*, i64 8`)
	default:
		t.Fatalf("unsupported caller-state test architecture %s", runtime.GOARCH)
	}
	for _, pattern := range patterns {
		if !regexp.MustCompile(pattern).Match(ir) {
			t.Fatalf("caller-state IR does not match %q\n%s", pattern, ir)
		}
	}

	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", archive+".ll")
	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-filetype=obj", archive+".ll", "-o", filepath.Join(dir, "caller-state.o"))

	appendArchive := filepath.Join(dir, "append.a")
	appendSource := filepath.Join(gorootTestDir, "codegen", "append.go")
	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-p=codegen", "-enablellvm", "-llvmironly", "-o", appendArchive, appendSource)
	appendIR, err := os.ReadFile(appendArchive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(appendIR, []byte("call ptr @llvm.stackaddress.p0()")) {
		t.Fatalf("append IR does not materialize the current logical SP")
	}
	runLLVMABICommand(t, nil, opt, "-passes=verify", "-disable-output", appendArchive+".ll")
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-filetype=obj", appendArchive+".ll", "-o", filepath.Join(dir, "append.o"))
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

func llvmABIRelocationTargetName(document llvmABIDocument, target llvmABIReference) string {
	if target.Name != "" || target.PkgKind != "none" {
		return target.Name
	}
	for _, member := range document.Members {
		if member.GoObject == nil {
			continue
		}
		var nonpackageDefinitions uint32
		for _, symbol := range member.GoObject.Symbols {
			if symbol.Class == "nonpackage" && symbol.ClassIndex >= nonpackageDefinitions {
				nonpackageDefinitions = symbol.ClassIndex + 1
			}
		}
		if target.SymIndex < nonpackageDefinitions {
			continue
		}
		classIndex := target.SymIndex - nonpackageDefinitions
		for _, reference := range member.GoObject.References {
			if reference.Class == "nonpackage_reference" && reference.ClassIndex == classIndex {
				return reference.Name
			}
		}
	}
	return ""
}

func llvmABIHasRelocationTo(document llvmABIDocument, symbol llvmABISymbol, name string) bool {
	for _, relocation := range symbol.Relocations {
		if llvmABIRelocationTargetName(document, relocation.Target) == name {
			return true
		}
	}
	return false
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

func checkLLVMABIStackMaps(t *testing.T, backend string, symbol llvmABISymbol, wantArgs [][]int, wantPCData []int32) {
	t.Helper()
	args := llvmABIArgsPointerBitmaps(t, symbol)
	pcdata := llvmABIStackMapRanges(symbol)
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("%s ArgsPointerMaps=%v, want %v", backend, args, wantArgs)
	}
	if !reflect.DeepEqual(pcdata, wantPCData) {
		t.Fatalf("%s PCDATA_StackMapIndex=%v, want %v", backend, pcdata, wantPCData)
	}
	t.Logf("%s ArgsPointerMaps=%v PCDATA=%v", backend, args, pcdata)
}

func checkLLVMABIStackMapQueryBitmaps(t *testing.T, backend string, symbol llvmABISymbol, want [][]int) {
	t.Helper()
	args := llvmABIArgsPointerBitmaps(t, symbol)
	got := make([][]int, 0, len(symbol.Function.StackMapQueries))
	for _, query := range symbol.Function.StackMapQueries {
		if query.DecodeError != "" {
			t.Fatalf("%s stack-map query failed: %s", backend, query.DecodeError)
		}
		index := query.StackMapIndex
		if index == -1 {
			// runtime.stkframe.getStackMap interprets the entry sentinel as map 0.
			index = 0
		}
		if index < 0 || int(index) >= len(args) {
			t.Fatalf("%s stack-map query index %d is outside %d ArgsPointerMaps",
				backend, query.StackMapIndex, len(args))
		}
		got = append(got, args[index])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s stack-map query bitmaps=%v, want %v", backend, got, want)
	}
	t.Logf("%s stack-map query bitmaps=%v", backend, got)
}

func checkLLVMABISourceStackMaps(t *testing.T, backend string, symbol llvmABISymbol, wantLocals uint32, wantArgs, wantMaps [][]int, wantPCData, wantQueries []int32) {
	t.Helper()
	if got := symbol.Function.Info.Locals; got != wantLocals {
		t.Fatalf("%s FuncInfo locals=%d, want %d", backend, got, wantLocals)
	}
	checkLLVMABIStackMaps(t, backend, symbol, wantArgs, wantPCData)
	if got := llvmABIStackMapBitmaps(t, symbol, "locals_pointer_maps"); !reflect.DeepEqual(got, wantMaps) {
		t.Fatalf("%s LocalsPointerMaps=%v, want %v", backend, got, wantMaps)
	}
	if got := llvmABIStackMapQueryIndexes(symbol); !reflect.DeepEqual(got, wantQueries) {
		t.Fatalf("%s stack-map query indexes=%v, want %v", backend, got, wantQueries)
	}
	t.Logf("%s FuncInfo.locals=%d LocalsPointerMaps=%v stack-map queries=%v",
		backend, wantLocals, wantMaps, wantQueries)
}

func llvmABIEntryPointerBits(t *testing.T, symbol llvmABISymbol) []int {
	t.Helper()
	bitmaps := llvmABIArgsPointerBitmaps(t, symbol)
	return bitmaps[0]
}

func llvmABIArgsPointerBitmaps(t *testing.T, symbol llvmABISymbol) [][]int {
	t.Helper()
	return llvmABIStackMapBitmaps(t, symbol, "args_pointer_maps")
}

func llvmABIStackMapBitmaps(t *testing.T, symbol llvmABISymbol, kind string) [][]int {
	t.Helper()
	for _, data := range symbol.Function.FuncData {
		if data.Kind != kind {
			continue
		}
		if data.StackMap == nil || data.StackMap.Count == 0 ||
			len(data.StackMap.Bitmaps) == 0 {
			t.Fatalf("%s has no decoded %s", symbol.Name, kind)
		}
		bitmaps := make([][]int, 0, len(data.StackMap.Bitmaps))
		for _, bitmap := range data.StackMap.Bitmaps {
			bits := append([]int(nil), bitmap.SetBits...)
			sort.Ints(bits)
			bitmaps = append(bitmaps, bits)
		}
		return bitmaps
	}
	t.Fatalf("%s has no %s", symbol.Name, kind)
	return nil
}

func llvmABIStackMapRanges(symbol llvmABISymbol) []int32 {
	for _, data := range symbol.Function.PCData {
		if data.Kind != "stack_map_index" {
			continue
		}
		values := make([]int32, 0, len(data.Ranges))
		for _, r := range data.Ranges {
			values = append(values, r.Value)
		}
		return values
	}
	return nil
}

func llvmABIStackMapQueryIndexes(symbol llvmABISymbol) []int32 {
	indexes := make([]int32, 0, len(symbol.Function.StackMapQueries))
	for _, query := range symbol.Function.StackMapQueries {
		indexes = append(indexes, query.StackMapIndex)
	}
	return indexes
}
