// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"cmd/internal/quoted"
	"encoding/json"
	"fmt"
	"go/build/constraint"
	"internal/testenv"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

const llvmTestToolexecEnv = "GOALLC_TEST_TOOLEXEC"

const llvmDefaultCaseTimeoutSeconds = 60

const llvmBlacklistReasonRequirement = "timeout, OOM, unsupported defer/recover, or slow CI case"

func llvmCaseTimeoutSeconds(recipeTimeout int) int {
	if recipeTimeout == 0 || recipeTimeout > llvmDefaultCaseTimeoutSeconds {
		return llvmDefaultCaseTimeoutSeconds
	}
	return recipeTimeout
}

func TestLLVMCaseTimeoutSeconds(t *testing.T) {
	for _, tc := range []struct {
		recipeTimeout int
		want          int
	}{
		{0, 60},
		{30, 30},
		{60, 60},
		{600, 60},
	} {
		if got := llvmCaseTimeoutSeconds(tc.recipeTimeout); got != tc.want {
			t.Errorf("llvmCaseTimeoutSeconds(%d) = %d, want %d", tc.recipeTimeout, got, tc.want)
		}
	}
}

type llvmTestSet struct {
	Whitelist         map[string]string            `json:"whitelist"`
	Graylist          map[string]string            `json:"graylist"`
	Blacklist         map[string]string            `json:"blacklist"`
	PlatformGraylist  map[string]map[string]string `json:"platform_graylist,omitempty"`
	PlatformBlacklist map[string]map[string]string `json:"platform_blacklist,omitempty"`
}

type llvmTestPolicy struct {
	Codegen llvmTestSet `json:"codegen"`
	Runtime llvmTestSet `json:"runtime"`
}

type llvmTestClass uint8

const (
	llvmTestUnclassified llvmTestClass = iota
	llvmTestWhite
	llvmTestGray
	llvmTestBlack
)

type llvmTestCase struct {
	suite string
	class llvmTestClass
}

type llvmTestCounts struct {
	passed  int
	failed  int
	skipped int
}

type llvmTestMode struct {
	cases    map[string]llvmTestCase
	toolexec string
	mu       sync.Mutex
	gray     map[string]*llvmTestCounts
}

func newLLVMTestMode(t *testing.T, common testCommon) *llvmTestMode {
	t.Helper()
	platform := goos + "/" + goarch
	switch platform {
	case "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skipf("LLVM GoObj is not configured for %s", platform)
	}
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		t.Skip("LLVM execution tests do not support cross compilation")
	}
	configureLLVMTestToolchain(t)

	policy := readLLVMTestPolicy(t, common.gorootTestDir)
	if err := applyLLVMPlatformPolicy("codegen", platform, &policy.Codegen); err != nil {
		t.Fatal(err)
	}
	if err := applyLLVMPlatformPolicy("runtime", platform, &policy.Runtime); err != nil {
		t.Fatal(err)
	}
	codegenCandidates := llvmTestCandidates(t, common.gorootTestDir, []string{"codegen"}, "asmcheck")
	runtimeCandidates := llvmTestCandidates(t, common.gorootTestDir, dirs, "run")
	for name := range llvmTestCandidates(t, common.gorootTestDir, dirs, "runoutput") {
		runtimeCandidates[name] = true
	}
	validateLLVMTestSet(t, common.gorootTestDir, "codegen", codegenCandidates, policy.Codegen, true)
	validateLLVMTestSet(t, common.gorootTestDir, "runtime", runtimeCandidates, policy.Runtime, false)
	logLLVMTestPolicy(t, "codegen", codegenCandidates, policy.Codegen)
	logLLVMTestPolicy(t, "runtime", runtimeCandidates, policy.Runtime)
	logLLVMBlacklist(t, "codegen", codegenCandidates, policy.Codegen)
	logLLVMBlacklist(t, "runtime", runtimeCandidates, policy.Runtime)

	mode := &llvmTestMode{
		cases: make(map[string]llvmTestCase, len(codegenCandidates)+len(runtimeCandidates)),
		gray: map[string]*llvmTestCounts{
			"codegen": {},
			"runtime": {},
		},
	}
	for name := range codegenCandidates {
		mode.cases[name] = llvmTestCase{suite: "codegen", class: classifyLLVMTest(t, policy.Codegen, name)}
	}
	for name := range runtimeCandidates {
		mode.cases[name] = llvmTestCase{suite: "runtime", class: classifyLLVMTest(t, policy.Runtime, name)}
	}
	mode.toolexec = llvmToolexec(t, "default<O2>")
	return mode
}

func (mode *llvmTestMode) recordResult(t *testing.T, tc llvmTestCase, runErr error) {
	t.Helper()
	if tc.class != llvmTestGray {
		return
	}
	mode.mu.Lock()
	counts := mode.gray[tc.suite]
	switch {
	case t.Skipped():
		counts.skipped++
	case runErr != nil:
		counts.failed++
	default:
		counts.passed++
	}
	mode.mu.Unlock()

	switch {
	case t.Skipped():
		t.Logf("LLVM %s graylist result: SKIP", tc.suite)
	case runErr != nil:
		t.Logf("LLVM %s graylist result: FAIL (allowed)", tc.suite)
	default:
		t.Logf("LLVM %s graylist result: PASS", tc.suite)
	}
}

func (mode *llvmTestMode) logSummary(t *testing.T) {
	t.Helper()
	mode.mu.Lock()
	defer mode.mu.Unlock()
	for _, suite := range []string{"codegen", "runtime"} {
		counts := mode.gray[suite]
		t.Logf("LLVM %s graylist summary: %d passed, %d failed (allowed), %d skipped",
			suite, counts.passed, counts.failed, counts.skipped)
	}
}

func runLLVMInfrastructureTests(t *testing.T, common testCommon) {
	t.Run("abi-differential", func(t *testing.T) {
		runLLVMABIDifferentialTest(t, common.gorootTestDir)
	})
	t.Run("alloca-statepoint", func(t *testing.T) {
		runLLVMAllocaStatepointTest(t, common.gorootTestDir)
	})
	t.Run("caller-state", func(t *testing.T) {
		runLLVMCallerStateTest(t, common.gorootTestDir)
	})
	t.Run("getg-abi0-fail-closed", runLLVMGetGABI0FailClosedTest)
	t.Run("nosplit", runLLVMNoSplitTest)
	t.Run("nowritebarrierrec", runLLVMNoWriteBarrierRecTest)
	t.Run("writebarrier-helpers", runLLVMWriteBarrierHelperTest)
	t.Run("compile-only-regressions", func(t *testing.T) {
		for _, name := range []string{"cmp.go", "typeparam/issue47684c.go"} {
			t.Run(name, func(t *testing.T) {
				runLLVMCompileOnlyRegression(t, common.gorootTestDir, name)
			})
		}
	})
	t.Run("writebarrier-ir", runLLVMWriteBarrierIRTests)
}

func runLLVMNoSplitTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "nosplit.go")
	program := `package p

func use(*[32]uintptr)

//go:nosplit
//go:noinline
func NoSplit(pointer *int) *int {
	var words [32]uintptr
	use(&words)
	return pointer
}

//go:noinline
func Split(pointer *int) *int {
	var words [32]uintptr
	use(&words)
	return pointer
}
`
	if err := os.WriteFile(source, []byte(program), 0o666); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "nosplit.a")
	runLLVMABICommand(t, nil, goTool, "tool", "compile",
		"-p=p", "-enablellvm", "-llvmironly", "-o", archive, source)
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	attributeLine := func(name string) []byte {
		definition := regexp.MustCompile(`(?m)^define goabiinternal .*@` + regexp.QuoteMeta(name) + `\([^\n]*\) #([0-9]+)`).FindSubmatch(ir)
		if definition == nil {
			t.Fatalf("LLVM IR has no attributed definition for %s\n%s", name, ir)
		}
		pattern := regexp.MustCompile(`(?m)^attributes #` + string(definition[1]) + ` = \{.*$`)
		line := pattern.Find(ir)
		if line == nil {
			t.Fatalf("LLVM IR has no attribute group for %s\n%s", name, ir)
		}
		return line
	}
	noSplitAttrs := attributeLine("p.NoSplit")
	if !bytes.Contains(noSplitAttrs, []byte(`"go-nosplit"`)) ||
		!bytes.Contains(noSplitAttrs, []byte(`noinline`)) {
		t.Fatalf("LLVM nosplit attributes do not select the nosplit prologue policy: %s", noSplitAttrs)
	}
	splitAttrs := attributeLine("p.Split")
	if bytes.Contains(splitAttrs, []byte(`"go-nosplit"`)) {
		t.Fatalf("LLVM split attributes do not select the stack-growth prologue policy: %s", splitAttrs)
	}
	if bytes.Contains(ir, []byte(`"go-stack-growth-statepoint"`)) {
		t.Fatal("LLVM IR still contains the obsolete stack-growth attribute")
	}

	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmABIPassPlugin(t, llc)
	object := filepath.Join(dir, "nosplit.o")
	runLLVMABICommand(t, nil, llc, "-load-pass-plugin="+plugin,
		"-verify-machineinstrs", "-filetype=obj", "-o", object, archive+".ll")
	document := readLLVMABIObject(t, object)
	noSplit := findLLVMABISymbol(t, document, "p.NoSplit")
	if !slices.Contains(noSplit.FlagNames, "nosplit") {
		t.Fatalf("p.NoSplit GoObj flags %v do not contain nosplit", noSplit.FlagNames)
	}
	if llvmABIHasRelocationTo(document, noSplit, "runtime.morestack_noctxt") {
		t.Fatal("p.NoSplit unexpectedly calls runtime.morestack_noctxt")
	}
	noSplitArgs := llvmABIArgsPointerBitmaps(t, noSplit)
	if len(noSplitArgs) == 0 || !slices.Equal(noSplitArgs[0], []int{0}) {
		t.Fatalf("p.NoSplit entry ArgsPointerMaps = %v, want pointer bit 0", noSplitArgs)
	}
	split := findLLVMABISymbol(t, document, "p.Split")
	if !llvmABIHasRelocationTo(document, split, "runtime.morestack_noctxt") {
		t.Fatal("p.Split has no runtime.morestack_noctxt relocation")
	}
	splitArgs := llvmABIArgsPointerBitmaps(t, split)
	if len(splitArgs) == 0 || !slices.Equal(splitArgs[0], []int{0}) {
		t.Fatalf("p.Split entry ArgsPointerMaps = %v, want pointer bit 0", splitArgs)
	}
}

func runLLVMNoWriteBarrierRecTest(t *testing.T) {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "nowritebarrier.a")
	source := filepath.Join(testenv.GOROOT(t), "test", "nowritebarrier.go")
	cmd := exec.Command(goTool, "tool", "compile",
		"-p=runtime",
		"-+",
		"-C",
		"-e",
		"-d=ssa/check/on",
		"-enablellvm",
		"-llvmironly",
		"-o", archive,
		source,
	)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("LLVM accepted prohibited runtime write barriers\n%s", out)
	}
	for _, want := range []string{
		"write barrier prohibited by caller; b2",
		"write barrier prohibited by caller; d3",
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("LLVM //go:nowritebarrierrec check missed %q: %v\n%s", want, err, out)
		}
	}
}

func runLLVMGetGABI0FailClosedTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(testenv.GOROOT(t), "src", "cmd", "internal", "testdir", "testdata", "llvm_getg_abi0.go")
	archive := filepath.Join(dir, "getg.a")
	cmd := exec.Command(goTool, "tool", "compile",
		"-std",
		"-+",
		"-p=runtime",
		"-enablellvm",
		"-llvmironly",
		"-o", archive,
		source,
	)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=", "GOOS=linux", "GOARCH=amd64")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("LLVM accepted amd64 ABI0 OpGetG; want fail-closed\n%s", out)
	}
	want := "GetG is unsupported for LLVM amd64 ABI ABI0; ABI0 must load g from TLS"
	if !bytes.Contains(out, []byte(want)) {
		t.Fatalf("LLVM amd64 ABI0 OpGetG failed with the wrong diagnostic: %v\n%s\nwant substring %q", err, out, want)
	}
}

func runLLVMCompileOnlyRegression(t *testing.T, gorootTestDir, name string) {
	t.Helper()
	toolexec := llvmToolexec(t, "")
	exe := filepath.Join(t.TempDir(), "test.exe")
	cmd := exec.Command(goTool, "build",
		"-gcflags=all="+os.Getenv("GO_GCFLAGS"),
		"-gcflags=-enablellvm -llvmironly",
		"-ldflags=-w",
		"-toolexec="+toolexec,
		"-o", exe,
		filepath.Join(gorootTestDir, filepath.FromSlash(name)),
	)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("LLVM compile-only regression %s failed: %v\n%s", name, err, out)
	}
}

func readLLVMTestPolicy(t *testing.T, gorootTestDir string) llvmTestPolicy {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gorootTestDir, "llvm_tests.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy llvmTestPolicy
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&policy); err != nil {
		t.Fatalf("parse llvm_tests.json: %v", err)
	}
	return policy
}

func TestLLVMTestPolicy(t *testing.T) {
	gorootTestDir := filepath.Join(testenv.GOROOT(t), "test")
	if _, err := os.Stat(filepath.Join(gorootTestDir, "llvm_tests.json")); err != nil {
		t.Skipf("LLVM test policy is not installed: %v", err)
	}
	codegenCandidates := llvmTestCandidates(t, gorootTestDir, []string{"codegen"}, "asmcheck")
	runtimeCandidates := llvmTestCandidates(t, gorootTestDir, dirs, "run")
	for name := range llvmTestCandidates(t, gorootTestDir, dirs, "runoutput") {
		runtimeCandidates[name] = true
	}

	base := readLLVMTestPolicy(t, gorootTestDir)
	platforms := map[string]bool{runtime.GOOS + "/" + runtime.GOARCH: true}
	for platform := range base.Codegen.PlatformGraylist {
		platforms[platform] = true
	}
	for platform := range base.Codegen.PlatformBlacklist {
		platforms[platform] = true
	}
	for platform := range base.Runtime.PlatformGraylist {
		platforms[platform] = true
	}
	for platform := range base.Runtime.PlatformBlacklist {
		platforms[platform] = true
	}
	for platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			policy := readLLVMTestPolicy(t, gorootTestDir)
			if err := applyLLVMPlatformPolicy("codegen", platform, &policy.Codegen); err != nil {
				t.Fatal(err)
			}
			if err := applyLLVMPlatformPolicy("runtime", platform, &policy.Runtime); err != nil {
				t.Fatal(err)
			}
			validateLLVMTestSet(t, gorootTestDir, "codegen", codegenCandidates, policy.Codegen, true)
			validateLLVMTestSet(t, gorootTestDir, "runtime", runtimeCandidates, policy.Runtime, false)
		})
	}
}

func applyLLVMPlatformPolicy(name, platform string, set *llvmTestSet) error {
	if set.Graylist == nil {
		set.Graylist = make(map[string]string)
	}
	if set.Blacklist == nil {
		set.Blacklist = make(map[string]string)
	}
	for target, entries := range set.PlatformGraylist {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("LLVM %s platform graylist has an empty platform", name)
		}
		for filename, reason := range entries {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("LLVM %s platform graylist entry %q for %s has no reason", name, filename, target)
			}
			if _, ok := set.Whitelist[filename]; !ok {
				return fmt.Errorf("LLVM %s platform graylist entry %q for %s is not in the common whitelist", name, filename, target)
			}
		}
	}
	for target, entries := range set.PlatformBlacklist {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("LLVM %s platform blacklist has an empty platform", name)
		}
		for filename, reason := range entries {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("LLVM %s platform blacklist entry %q for %s has no reason", name, filename, target)
			}
			if !validLLVMBlacklistReason(reason) {
				return fmt.Errorf("LLVM %s platform blacklist entry %q for %s is not a %s", name, filename, target, llvmBlacklistReasonRequirement)
			}
		}
	}

	for filename, reason := range set.PlatformGraylist[platform] {
		delete(set.Whitelist, filename)
		set.Graylist[filename] = reason
	}
	for filename, reason := range set.PlatformBlacklist[platform] {
		delete(set.Whitelist, filename)
		set.Blacklist[filename] = reason
	}
	return nil
}

func TestApplyLLVMPlatformPolicy(t *testing.T) {
	set := llvmTestSet{
		Whitelist: map[string]string{
			"common.go": "common",
			"linux.go":  "linux",
			"darwin.go": "darwin",
			"oom.go":    "normally supported",
		},
		PlatformGraylist: map[string]map[string]string{
			"linux/amd64":  {"linux.go": "linux limitation"},
			"darwin/arm64": {"darwin.go": "darwin limitation"},
		},
		PlatformBlacklist: map[string]map[string]string{
			"linux/amd64": {"oom.go": "OOM: exceeds runner memory"},
		},
	}
	if err := applyLLVMPlatformPolicy("runtime", "linux/amd64", &set); err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Whitelist["linux.go"]; ok {
		t.Fatal("current-platform graylist entry remained in the effective whitelist")
	}
	if _, ok := set.Graylist["linux.go"]; !ok {
		t.Fatal("current-platform entry was not moved to the effective graylist")
	}
	if _, ok := set.Blacklist["oom.go"]; !ok {
		t.Fatal("current-platform entry was not moved to the effective blacklist")
	}
	for _, filename := range []string{"common.go", "darwin.go"} {
		if _, ok := set.Whitelist[filename]; !ok {
			t.Errorf("applyLLVMPlatformPolicy removed %q for another platform", filename)
		}
	}

	tests := []struct {
		name string
		set  llvmTestSet
		want string
	}{
		{
			name: "empty platform",
			set: llvmTestSet{
				Whitelist:        map[string]string{"test.go": "test"},
				PlatformGraylist: map[string]map[string]string{"": {"test.go": "reason"}},
			},
			want: "empty platform",
		},
		{
			name: "empty reason",
			set: llvmTestSet{
				Whitelist:        map[string]string{"test.go": "test"},
				PlatformGraylist: map[string]map[string]string{"linux/amd64": {"test.go": " "}},
			},
			want: "has no reason",
		},
		{
			name: "not in common whitelist",
			set: llvmTestSet{
				Whitelist:        map[string]string{"test.go": "test"},
				PlatformGraylist: map[string]map[string]string{"linux/amd64": {"missing.go": "reason"}},
			},
			want: "is not in the common whitelist",
		},
		{
			name: "blacklist ordinary failure",
			set: llvmTestSet{
				Whitelist:         map[string]string{"test.go": "test"},
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"test.go": "ordinary failure"}},
			},
			want: "is not a timeout, OOM, unsupported defer/recover, or slow CI case",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := applyLLVMPlatformPolicy("runtime", "linux/amd64", &tc.set)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("applyLLVMPlatformPolicy error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestClassifyLLVMTest(t *testing.T) {
	set := llvmTestSet{
		Whitelist: map[string]string{"white.go": "must pass", "black.go": "normally white"},
		Graylist:  map[string]string{"*": "run speculatively"},
		Blacklist: map[string]string{"black.go": "timeout: does not terminate"},
	}
	for _, tc := range []struct {
		name string
		want llvmTestClass
	}{
		{"white.go", llvmTestWhite},
		{"gray.go", llvmTestGray},
		{"black.go", llvmTestBlack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyLLVMTest(t, set, tc.name); got != tc.want {
				t.Fatalf("classifyLLVMTest(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func llvmTestCandidates(t *testing.T, gorootTestDir string, scanDirs []string, wantAction string) map[string]bool {
	t.Helper()
	candidates := make(map[string]bool)
	for _, dir := range scanDirs {
		entries, err := os.ReadDir(filepath.Join(gorootTestDir, dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			name := path.Join(dir, entry.Name())
			if llvmTestAction(t, filepath.Join(gorootTestDir, filepath.FromSlash(name))) == wantAction {
				candidates[name] = true
			}
		}
	}
	return candidates
}

func llvmTestAction(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	for src := string(data); src != ""; {
		var line string
		line, src, _ = strings.Cut(src, "\n")
		if constraint.IsGoBuild(line) || constraint.IsPlusBuild(line) || strings.TrimSpace(line) == "" {
			continue
		}
		action := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		fields, err := splitQuoted(action)
		if err != nil {
			t.Fatalf("%s: invalid test recipe: %v", filename, err)
		}
		if len(fields) == 0 {
			return ""
		}
		return fields[0]
	}
	return ""
}

func validateLLVMTestSet(t *testing.T, gorootTestDir, name string, candidates map[string]bool, set llvmTestSet, requireChecks bool) {
	t.Helper()
	failed := false
	for filename, reason := range set.Whitelist {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("LLVM %s whitelist entry %q has no reason", name, filename)
			failed = true
		}
		if !candidates[filename] {
			t.Errorf("LLVM %s whitelist entry %q is not a %s test", name, filename, name)
			failed = true
		}
		if requireChecks {
			source, err := os.ReadFile(filepath.Join(gorootTestDir, filepath.FromSlash(filename)))
			if err != nil {
				t.Errorf("read LLVM %s whitelist entry %q: %v", name, filename, err)
				failed = true
			} else if !bytes.Contains(source, []byte("// LLVM")) {
				t.Errorf("LLVM %s whitelist entry %q has no FileCheck directives", name, filename)
				failed = true
			}
		}
	}

	for _, entries := range []struct {
		class string
		items map[string]string
	}{
		{"graylist", set.Graylist},
		{"blacklist", set.Blacklist},
	} {
		for pattern, reason := range entries.items {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("LLVM %s %s pattern %q has no reason", name, entries.class, pattern)
				failed = true
			}
			if entries.class == "blacklist" && !validLLVMBlacklistReason(reason) {
				t.Errorf("LLVM %s blacklist pattern %q is not a %s", name, pattern, llvmBlacklistReasonRequirement)
				failed = true
			}
			matched := false
			for filename := range candidates {
				if llvmPathMatch(t, pattern, filename) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("LLVM %s %s pattern %q matches no tests", name, entries.class, pattern)
				failed = true
			}
		}
	}

	for filename := range candidates {
		if classifyLLVMTest(t, set, filename) == llvmTestUnclassified {
			t.Errorf("LLVM %s test %q is not classified as white, gray, or black", name, filename)
			failed = true
		}
	}
	if failed {
		t.FailNow()
	}
}

func validLLVMBlacklistReason(reason string) bool {
	reason = strings.ToLower(reason)
	return strings.Contains(reason, "timeout") ||
		strings.Contains(reason, "out of memory") ||
		strings.Contains(reason, "oom") ||
		strings.Contains(reason, "slow") ||
		strings.Contains(reason, "defer") ||
		strings.Contains(reason, "recover")
}

func TestValidLLVMBlacklistReason(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   bool
	}{
		{"timeout: does not terminate", true},
		{"OOM during LLVM code generation", true},
		{"slow: exceeds the one-minute CI budget", true},
		{"unsupported defer/recover execution", true},
		{"ordinary lowering failure", false},
	} {
		if got := validLLVMBlacklistReason(tc.reason); got != tc.want {
			t.Errorf("validLLVMBlacklistReason(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}

func classifyLLVMTest(t *testing.T, set llvmTestSet, filename string) llvmTestClass {
	t.Helper()
	for pattern := range set.Blacklist {
		if llvmPathMatch(t, pattern, filename) {
			return llvmTestBlack
		}
	}
	if _, ok := set.Whitelist[filename]; ok {
		return llvmTestWhite
	}
	for pattern := range set.Graylist {
		if llvmPathMatch(t, pattern, filename) {
			return llvmTestGray
		}
	}
	return llvmTestUnclassified
}

func logLLVMTestPolicy(t *testing.T, name string, candidates map[string]bool, set llvmTestSet) {
	t.Helper()
	counts := make(map[llvmTestClass]int)
	for filename := range candidates {
		counts[classifyLLVMTest(t, set, filename)]++
	}
	t.Logf("LLVM %s policy: %d white, %d gray, %d black (%d files)",
		name, counts[llvmTestWhite], counts[llvmTestGray], counts[llvmTestBlack], len(candidates))
}

func logLLVMBlacklist(t *testing.T, name string, candidates map[string]bool, set llvmTestSet) {
	t.Helper()
	for _, filename := range sortedLLVMTests(t, candidates, set, llvmTestBlack) {
		reason := ""
		for pattern, candidateReason := range set.Blacklist {
			if llvmPathMatch(t, pattern, filename) {
				reason = candidateReason
				break
			}
		}
		t.Logf("LLVM %s blacklist result: NOT RUN test=%q reason=%q", name, filename, reason)
	}
}

func llvmPathMatch(t *testing.T, pattern, filename string) bool {
	t.Helper()
	matched, err := path.Match(pattern, filename)
	if err != nil {
		t.Fatalf("invalid LLVM test policy pattern %q: %v", pattern, err)
	}
	if matched || strings.Contains(pattern, "/") {
		return matched
	}
	matched, err = path.Match(pattern, path.Base(filename))
	if err != nil {
		t.Fatalf("invalid LLVM test policy pattern %q: %v", pattern, err)
	}
	return matched
}

func sortedLLVMTests(t *testing.T, candidates map[string]bool, set llvmTestSet, class llvmTestClass) []string {
	t.Helper()
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		if classifyLLVMTest(t, set, name) == class {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func runLLVMCodegenTest(t *testing.T, source string) error {
	t.Helper()
	src, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	archive := filepath.Join(t.TempDir(), "codegen.a")
	cmd := exec.Command(goTool, "tool", "compile",
		"-p=codegen",
		"-importcfg="+stdlibImportcfgFile(),
		"-enablellvm",
		"-llvmironly",
		"-c=16",
		"-o", archive,
		source,
	)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("LLVM compilation failed: %v\n%s", err, out)
	}
	irBytes, err := os.ReadFile(archive + ".ll")
	if err != nil {
		return err
	}
	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	cmd = exec.Command(opt, "-passes=verify", "-disable-output")
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("LLVM verifier failed: %v\n%s", err, out)
	}
	fileCheck := llvmToolPath(t, "FileCheck", "GOALLC_FILECHECK")
	cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM", src), source)
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("FileCheck failed: %v\n%s", err, out)
	}

	if !bytes.Contains(src, []byte("// LLVM-OPT")) {
		return nil
	}
	cmd = exec.Command(opt, "-passes=default<O2>", "-S")
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	optimizedIR, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("LLVM optimization failed: %v\n%s", err, stderr.Bytes())
	}
	cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM-OPT", src), source)
	cmd.Stdin = bytes.NewReader(optimizedIR)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("optimized LLVM FileCheck failed: %v\n%s", err, out)
	}
	return nil
}

func llvmFileCheckPrefixes(base string, source []byte) string {
	prefixes := []string{base}
	var architecturePrefix string
	switch runtime.GOARCH {
	case "amd64":
		architecturePrefix = base + "-AMD64"
	case "arm64":
		architecturePrefix = base + "-ARM64"
	}
	if architecturePrefix != "" &&
		(bytes.Contains(source, []byte(architecturePrefix+":")) ||
			bytes.Contains(source, []byte(architecturePrefix+"-"))) {
		prefixes = append(prefixes, architecturePrefix)
	}
	return strings.Join(prefixes, ",")
}

func runLLVMWriteBarrierIRTests(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "ZeroWithPointers",
			source: "package p\nfunc zero(dst *[2]*int) { *dst = [2]*int{} }\n",
			want:   []string{"call goabiinternal void @runtime.wbZero(ptr", "@llvm.memset.inline", `"gc-leaf-function"`},
		},
		{
			name:   "MoveWithPointers",
			source: "package p\nfunc move(dst, src *[2]*int) { *dst = *src }\n",
			want:   []string{"call goabiinternal void @runtime.wbMove(ptr", "@llvm.memmove", `"gc-leaf-function"`},
		},
		{
			name:   "DeletePointer",
			source: "package p\nfunc delete(dst **int) { *dst = nil }\n",
			want:   []string{"@llvm.go.gc.write.barrier"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "fail.go")
			if err := os.WriteFile(source, []byte(tc.source), 0o666); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "writebarrier.a")
			cmd := exec.Command(goTool, "tool", "compile",
				"-p=p",
				"-importcfg="+stdlibImportcfgFile(),
				"-enablellvm",
				"-llvmironly",
				"-o", archive,
				source,
			)
			cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("LLVM compilation failed: %v\n%s", err, out)
			}
			ir, err := os.ReadFile(archive + ".ll")
			if err != nil {
				t.Fatal(err)
			}
			want := append(tc.want, `"go-async-unsafe"`)
			for _, want := range want {
				if !bytes.Contains(ir, []byte(want)) {
					t.Fatalf("LLVM write-barrier IR does not contain %s\n%s", want, ir)
				}
			}
			if bytes.Contains(ir, []byte(".rawarg = ptrtoint")) {
				t.Fatalf("LLVM write-barrier IR still coerces pointer arguments to raw uintptr\n%s", ir)
			}
			if bytes.Contains(ir, []byte("llvm.go.gc.unsafe.point")) {
				t.Fatalf("LLVM write-barrier IR still contains unsafe-point marker calls\n%s", ir)
			}
		})
	}
}

func llvmToolexec(t *testing.T, optPasses string) string {
	t.Helper()
	wrapper := llvmToolexecPath(t)

	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	plugin := llvmPassPluginPath(t)
	args := []string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}
	if optPasses != "" {
		opt := llvmToolPath(t, "opt", "GOALLC_OPT")
		args = append(args, "-opt="+opt, "-opt-passes="+optPasses)
	}
	value, err := quoted.Join(args)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

type llvmTestToolchain struct {
	root      string
	llc       string
	opt       string
	fileCheck string
	plugin    string
	wrapper   string
}

func configureLLVMTestToolchain(t *testing.T) llvmTestToolchain {
	t.Helper()
	root, configured := llvmTestPayloadRoot(t)
	if root == "" {
		t.Skip("LLVM payload is unavailable; set GOALLC_LLVM_DIR or build Go with -llvm-dir")
	}

	llvmConfig := llvmPayloadExecutable(t, root, "llvm-config")
	version := llvmCommandOutput(t, llvmConfig, "--version")
	if version != "23" && !strings.HasPrefix(version, "23.") {
		t.Fatalf("selected LLVM payload %q has version %q, want LLVM 23", root, version)
	}
	prefix := llvmCommandOutput(t, llvmConfig, "--prefix")
	if !sameLLVMTestPath(prefix, root) {
		t.Fatalf("selected LLVM payload prefix mismatch: root %q, llvm-config --prefix %q", root, prefix)
	}

	tools := llvmTestToolchain{
		root:      root,
		llc:       llvmPayloadTool(t, root, "llc", "GOALLC_LLC"),
		opt:       llvmPayloadTool(t, root, "opt", "GOALLC_OPT"),
		fileCheck: llvmPayloadTool(t, root, "FileCheck", "GOALLC_FILECHECK"),
		plugin:    llvmPayloadPlugin(t, root),
		wrapper:   buildLLVMTestToolexec(t),
	}

	// Freeze every consumer to the validated payload. In particular, do not let
	// an individual subtest silently fall back to a stale GOROOT/llvm tree.
	t.Setenv("GOALLC_LLVM_DIR", tools.root)
	t.Setenv("GOALLC_LLC", tools.llc)
	t.Setenv("GOALLC_OPT", tools.opt)
	t.Setenv("GOALLC_FILECHECK", tools.fileCheck)
	t.Setenv("GOALLC_PASS_PLUGIN", tools.plugin)
	t.Setenv(llvmTestToolexecEnv, tools.wrapper)
	t.Logf("LLVM test toolchain: go=%s wrapper=%s payload=%s llc=%s opt=%s FileCheck=%s plugin=%s runtime-pipeline=default<O2>",
		goTool, tools.wrapper, tools.root, tools.llc, tools.opt, tools.fileCheck, tools.plugin)
	if configured != "" {
		t.Logf("LLVM payload selected by %s", configured)
	}
	return tools
}

func llvmTestPayloadRoot(t *testing.T) (root, configured string) {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv("GOALLC_LLVM_DIR")); value != "" {
		return llvmAbsolutePath(t, value, "GOALLC_LLVM_DIR"), "GOALLC_LLVM_DIR"
	}
	if value := strings.TrimSpace(os.Getenv("GOALLC_LLC")); value != "" {
		llc := llvmRegularFile(t, value, "GOALLC_LLC", true)
		return filepath.Dir(filepath.Dir(llc)), "GOALLC_LLC"
	}

	goroot := testenv.GOROOT(t)
	payloadConfig := filepath.Join(goroot, "pkg", "goallc-llvm-payload")
	if data, err := os.ReadFile(payloadConfig); err == nil {
		value := strings.TrimSpace(string(data))
		if value == "" {
			t.Fatalf("LLVM payload configuration %s is empty", payloadConfig)
		}
		return llvmAbsolutePath(t, value, payloadConfig), payloadConfig
	} else if !os.IsNotExist(err) {
		t.Fatalf("reading LLVM payload configuration %s: %v", payloadConfig, err)
	}

	legacy := filepath.Join(goroot, "llvm")
	if _, err := os.Stat(filepath.Join(legacy, "bin", "llvm-config")); err == nil {
		return legacy, "GOROOT/llvm"
	} else if !os.IsNotExist(err) {
		t.Fatalf("checking legacy LLVM payload %s: %v", legacy, err)
	}
	return "", ""
}

func llvmPayloadTool(t *testing.T, root, name, envName string) string {
	t.Helper()
	expected := llvmPayloadExecutable(t, root, name)
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		configured := llvmRegularFile(t, value, envName, true)
		if !sameLLVMTestFile(configured, expected) {
			t.Fatalf("%s=%q does not belong to selected LLVM payload %q (expected %q)", envName, configured, root, expected)
		}
	}
	return expected
}

func llvmPayloadExecutable(t *testing.T, root, name string) string {
	t.Helper()
	return llvmRegularFile(t, filepath.Join(root, "bin", name), "LLVM payload", true)
}

func llvmPayloadPlugin(t *testing.T, root string) string {
	t.Helper()
	name := "GoALLCStatepoints.so"
	if runtime.GOOS == "darwin" {
		name = "GoALLCStatepoints.dylib"
	}
	expected := llvmRegularFile(t, filepath.Join(root, "lib", name), "LLVM payload", false)
	if value := strings.TrimSpace(os.Getenv("GOALLC_PASS_PLUGIN")); value != "" {
		configured := llvmRegularFile(t, value, "GOALLC_PASS_PLUGIN", false)
		if !sameLLVMTestFile(configured, expected) {
			t.Fatalf("GOALLC_PASS_PLUGIN=%q does not belong to selected LLVM payload %q (expected %q)", configured, root, expected)
		}
	}
	return expected
}

func llvmToolexecPath(t *testing.T) string {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv(llvmTestToolexecEnv)); value != "" {
		return llvmRegularFile(t, value, llvmTestToolexecEnv, true)
	}
	out, err := exec.Command(goTool, "tool", "-n", "llvmtoolexec").CombinedOutput()
	if err != nil {
		t.Fatalf("resolving llvmtoolexec from selected Go tool %q: %v\n%s", goTool, err, out)
	}
	return llvmRegularFile(t, strings.TrimSpace(string(out)), "go tool llvmtoolexec", true)
}

func buildLLVMTestToolexec(t *testing.T) string {
	t.Helper()
	wrapper := filepath.Join(t.TempDir(), "llvmtoolexec")
	cmd := exec.Command(goTool, "build", "-o", wrapper, "cmd/llvmtoolexec")
	cmd.Dir = testenv.GOROOT(t)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building llvmtoolexec from the selected Go checkout: %v\n%s", err, out)
	}
	return llvmRegularFile(t, wrapper, "fresh llvmtoolexec", true)
}

func llvmCommandOutput(t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running %s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func llvmAbsolutePath(t *testing.T, value, source string) string {
	t.Helper()
	path, err := filepath.Abs(value)
	if err != nil {
		t.Fatalf("resolving %s path %q: %v", source, value, err)
	}
	return filepath.Clean(path)
}

func llvmRegularFile(t *testing.T, value, source string, executable bool) string {
	t.Helper()
	path := llvmAbsolutePath(t, value, source)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("checking %s file %q: %v", source, path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s file %q is not a regular file", source, path)
	}
	if executable && info.Mode()&0o111 == 0 {
		t.Fatalf("%s file %q is not executable", source, path)
	}
	return path
}

func sameLLVMTestPath(a, b string) bool {
	a, errA := filepath.EvalSymlinks(filepath.Clean(a))
	b, errB := filepath.EvalSymlinks(filepath.Clean(b))
	return errA == nil && errB == nil && a == b
}

func sameLLVMTestFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
}

func llvmToolPath(t *testing.T, name, envName string) string {
	t.Helper()
	root, _ := llvmTestPayloadRoot(t)
	if root == "" {
		t.Skipf("%s is unavailable; set GOALLC_LLVM_DIR or build Go with -llvm-dir", name)
	}
	return llvmPayloadTool(t, root, name, envName)
}

func llvmPassPluginPath(t *testing.T) string {
	t.Helper()
	root, _ := llvmTestPayloadRoot(t)
	if root == "" {
		t.Skip("GoALLC pass plugin is unavailable; set GOALLC_LLVM_DIR or build Go with -llvm-dir")
	}
	return llvmPayloadPlugin(t, root)
}
