// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"internal/testenv"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const llvmDefaultCaseTimeoutSeconds = 300

const llvmBlacklistReasonRequirement = "known unsupported capability, timeout, OOM, or slow CI case"

func llvmCaseTimeoutSeconds(recipeTimeout int) int {
	if recipeTimeout == 0 || recipeTimeout > llvmDefaultCaseTimeoutSeconds {
		return llvmDefaultCaseTimeoutSeconds
	}
	return recipeTimeout
}

func testLLVMCaseTimeoutSeconds(t *testing.T) {
	for _, tc := range []struct {
		recipeTimeout int
		want          int
	}{
		{0, 300},
		{30, 30},
		{60, 60},
		{120, 120},
		{300, 300},
		{600, 300},
	} {
		if got := llvmCaseTimeoutSeconds(tc.recipeTimeout); got != tc.want {
			t.Errorf("llvmCaseTimeoutSeconds(%d) = %d, want %d", tc.recipeTimeout, got, tc.want)
		}
	}
}

type llvmPolicySet struct {
	Blacklist         map[string]string            `json:"blacklist"`
	PlatformBlacklist map[string]map[string]string `json:"platform_blacklist,omitempty"`
}

type llvmTestPolicy struct {
	Codegen llvmPolicySet `json:"codegen"`
	Run     llvmPolicySet `json:"run"`
}

type llvmTestMode struct {
	cases map[string]bool
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
	if err := applyLLVMPlatformPolicy("run", platform, &policy.Run); err != nil {
		t.Fatal(err)
	}
	codegenCandidates := llvmTestCandidates(t, common.gorootTestDir, []string{"codegen"}, "asmcheck")
	runCandidates := llvmTestCandidates(t, common.gorootTestDir, dirs, "run")
	for name := range llvmTestCandidates(t, common.gorootTestDir, dirs, "runoutput") {
		runCandidates[name] = true
	}
	validateLLVMTestSet(t, "codegen", codegenCandidates, policy.Codegen)
	validateLLVMTestSet(t, "run", runCandidates, policy.Run)
	logLLVMTestPolicy(t, "codegen", codegenCandidates, policy.Codegen)
	logLLVMTestPolicy(t, "run", runCandidates, policy.Run)
	logLLVMBlacklist(t, "codegen", codegenCandidates, policy.Codegen)
	logLLVMBlacklist(t, "run", runCandidates, policy.Run)

	mode := &llvmTestMode{
		cases: make(map[string]bool, len(codegenCandidates)+len(runCandidates)),
	}
	for name := range codegenCandidates {
		if !isLLVMTestBlacklisted(t, policy.Codegen, name) {
			mode.cases[name] = true
		}
	}
	for name := range runCandidates {
		if !isLLVMTestBlacklisted(t, policy.Run, name) {
			mode.cases[name] = true
		}
	}
	warmLLVMExecutionRuntime(t, "")
	return mode
}

func warmLLVMExecutionRuntime(t *testing.T, cache string) {
	t.Helper()
	t.Log("warming the LLVM-compiled runtime before parallel execution tests")
	cmd := testenv.Command(t, goTool, "install",
		"-gcflags=all=-enablellvm",
		"runtime",
	)
	cmd.Env = append(os.Environ(),
		"GOENV=off",
		"GOFLAGS=",
		"GOROOT="+testenv.GOROOT(t),
	)
	if cache != "" {
		cmd.Env = append(cmd.Env, "GOCACHE="+cache)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("warming LLVM runtime: %v\n%s", err, out)
	}
}

func readLLVMPolicyFile(t *testing.T, filename string, policy any) {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(policy); err != nil {
		t.Fatalf("parse %s: %v", filepath.Base(filename), err)
	}
}

func readLLVMTestPolicy(t *testing.T, gorootTestDir string) llvmTestPolicy {
	t.Helper()
	var policy llvmTestPolicy
	readLLVMPolicyFile(t, filepath.Join(gorootTestDir, "llvm_tests.json"), &policy)
	return policy
}

func testLLVMPolicy(t *testing.T) {
	t.Run("testdir", func(t *testing.T) {
		t.Run("configuration", testLLVMTestPolicy)
		t.Run("recipe", testParseTestRecipe)
		t.Run("case-timeout", testLLVMCaseTimeoutSeconds)
		t.Run("platform-policy", testApplyLLVMPlatformPolicy)
		t.Run("blacklist-match", testIsLLVMTestBlacklisted)
		t.Run("blacklist-reason", testValidLLVMBlacklistReason)
	})
	t.Run("stdlib", func(t *testing.T) {
		t.Run("configuration", testLLVMStdlibPolicy)
		t.Run("effective-blacklist", testEffectiveLLVMStdlibBlacklist)
		t.Run("runtime-async-preemption", testLLVMRuntimeAsyncPreemptionQualification)
		t.Run("pprof-bad-pointer", testLLVMPprofBadPointerQualification)
	})
}

func testLLVMTestPolicy(t *testing.T) {
	gorootTestDir := filepath.Join(testenv.GOROOT(t), "test")
	if _, err := os.Stat(filepath.Join(gorootTestDir, "llvm_tests.json")); err != nil {
		t.Skipf("LLVM test policy is not installed: %v", err)
	}
	codegenCandidates := llvmTestCandidates(t, gorootTestDir, []string{"codegen"}, "asmcheck")
	runCandidates := llvmTestCandidates(t, gorootTestDir, dirs, "run")
	for name := range llvmTestCandidates(t, gorootTestDir, dirs, "runoutput") {
		runCandidates[name] = true
	}

	base := readLLVMTestPolicy(t, gorootTestDir)
	platforms := map[string]bool{runtime.GOOS + "/" + runtime.GOARCH: true}
	for platform := range base.Codegen.PlatformBlacklist {
		platforms[platform] = true
	}
	for platform := range base.Run.PlatformBlacklist {
		platforms[platform] = true
	}
	for platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			policy := readLLVMTestPolicy(t, gorootTestDir)
			if err := applyLLVMPlatformPolicy("codegen", platform, &policy.Codegen); err != nil {
				t.Fatal(err)
			}
			if err := applyLLVMPlatformPolicy("run", platform, &policy.Run); err != nil {
				t.Fatal(err)
			}
			validateLLVMTestSet(t, "codegen", codegenCandidates, policy.Codegen)
			validateLLVMTestSet(t, "run", runCandidates, policy.Run)
		})
	}
}

func effectiveLLVMBlacklist(set llvmPolicySet, platform string) map[string]string {
	effective := make(map[string]string, len(set.Blacklist)+len(set.PlatformBlacklist[platform]))
	for name, reason := range set.Blacklist {
		effective[name] = reason
	}
	for name, reason := range set.PlatformBlacklist[platform] {
		effective[name] = reason
	}
	return effective
}

func applyLLVMPlatformPolicy(name, platform string, set *llvmPolicySet) error {
	if set.Blacklist == nil {
		set.Blacklist = make(map[string]string)
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

	set.Blacklist = effectiveLLVMBlacklist(*set, platform)
	return nil
}

func testApplyLLVMPlatformPolicy(t *testing.T) {
	set := llvmPolicySet{
		Blacklist: map[string]string{
			"common.go": "unsupported: common limitation",
		},
		PlatformBlacklist: map[string]map[string]string{
			"linux/amd64": {"oom.go": "OOM: exceeds runner memory"},
		},
	}
	if err := applyLLVMPlatformPolicy("run", "linux/amd64", &set); err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Blacklist["oom.go"]; !ok {
		t.Fatal("current-platform entry was not moved to the effective blacklist")
	}
	if _, ok := set.Blacklist["common.go"]; !ok {
		t.Fatal("common blacklist entry was not retained")
	}

	tests := []struct {
		name string
		set  llvmPolicySet
		want string
	}{
		{
			name: "empty platform",
			set: llvmPolicySet{
				PlatformBlacklist: map[string]map[string]string{"": {"test.go": "unsupported: reason"}},
			},
			want: "empty platform",
		},
		{
			name: "empty reason",
			set: llvmPolicySet{
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"test.go": " "}},
			},
			want: "has no reason",
		},
		{
			name: "blacklist ordinary failure",
			set: llvmPolicySet{
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"test.go": "ordinary failure"}},
			},
			want: "is not a known unsupported capability, timeout, OOM, or slow CI case",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := applyLLVMPlatformPolicy("run", "linux/amd64", &tc.set)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("applyLLVMPlatformPolicy error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func testIsLLVMTestBlacklisted(t *testing.T) {
	set := llvmPolicySet{
		Blacklist: map[string]string{"black.go": "timeout: does not terminate"},
	}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"default.go", false},
		{"black.go", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLLVMTestBlacklisted(t, set, tc.name); got != tc.want {
				t.Fatalf("isLLVMTestBlacklisted(%q) = %v, want %v", tc.name, got, tc.want)
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
	recipe, err := testRecipe(string(data))
	if err != nil {
		t.Fatalf("%s: %v", filename, err)
	}
	fields, err := parseTestRecipe(recipe)
	if err != nil {
		t.Fatalf("%s: %v", filename, err)
	}
	return fields[0]
}

func validateLLVMTestSet(t *testing.T, name string, candidates map[string]bool, set llvmPolicySet) {
	t.Helper()
	failed := false
	for pattern, reason := range set.Blacklist {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("LLVM %s blacklist pattern %q has no reason", name, pattern)
			failed = true
		}
		if !validLLVMBlacklistReason(reason) {
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
			t.Errorf("LLVM %s blacklist pattern %q matches no tests", name, pattern)
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
		strings.Contains(reason, "unsupported")
}

func testValidLLVMBlacklistReason(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   bool
	}{
		{"timeout: does not terminate", true},
		{"OOM during LLVM code generation", true},
		{"slow: exceeds the one-minute CI budget", true},
		{"unsupported: defer/recover execution", true},
		{"ordinary lowering failure", false},
	} {
		if got := validLLVMBlacklistReason(tc.reason); got != tc.want {
			t.Errorf("validLLVMBlacklistReason(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}

func isLLVMTestBlacklisted(t *testing.T, set llvmPolicySet, filename string) bool {
	t.Helper()
	for pattern := range set.Blacklist {
		if llvmPathMatch(t, pattern, filename) {
			return true
		}
	}
	return false
}

func logLLVMTestPolicy(t *testing.T, name string, candidates map[string]bool, set llvmPolicySet) {
	t.Helper()
	black := 0
	for filename := range candidates {
		if isLLVMTestBlacklisted(t, set, filename) {
			black++
		}
	}
	t.Logf("LLVM %s policy: %d enabled, %d blacklisted (%d files)",
		name, len(candidates)-black, black, len(candidates))
}

func logLLVMBlacklist(t *testing.T, name string, candidates map[string]bool, set llvmPolicySet) {
	t.Helper()
	for _, filename := range sortedLLVMBlacklistedTests(t, candidates, set) {
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

func sortedLLVMBlacklistedTests(t *testing.T, candidates map[string]bool, set llvmPolicySet) []string {
	t.Helper()
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		if isLLVMTestBlacklisted(t, set, name) {
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
		"-llvm-keep-ir",
		"-c=16",
		"-o", archive,
		source,
	)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("LLVM compilation failed: %v\n%s", err, out)
	}
	// Existing asmcheck tests without LLVM directives still exercise the LLVM
	// compiler path. Only tests that define LLVM expectations need FileCheck;
	// do not invent LLVM-specific checks for the native assembly directives.
	if !hasLLVMFileCheckPrefix(src, "LLVM") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-OPT") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-ASM") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-OBJVIEW") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-OBJSUMMARY") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-NATIVE-OBJSUMMARY") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-NM") &&
		!hasLLVMFileCheckPrefix(src, "LLVM-LINK") {
		return nil
	}
	fileCheck := llvmToolPath(t, "FileCheck", "GOALLC_FILECHECK")
	if hasLLVMFileCheckPrefix(src, "LLVM") {
		irBytes, err := os.ReadFile(archive + ".ll")
		if err != nil {
			return err
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM", src, irBytes); err != nil {
			return fmt.Errorf("LLVM IR FileCheck failed: %v", err)
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-OPT") {
		optimizedIR, err := os.ReadFile(archive + ".opt.ll")
		if err != nil {
			return err
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM-OPT", src, optimizedIR); err != nil {
			return fmt.Errorf("optimized LLVM IR FileCheck failed: %v", err)
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-ASM") {
		cmd = exec.Command(goTool, "tool", "objdump", archive)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
		assembly, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("LLVM object disassembly failed: %v\n%s", err, assembly)
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM-ASM", src, assembly); err != nil {
			return fmt.Errorf("LLVM assembly FileCheck failed: %v", err)
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-OBJVIEW") {
		output, err := runLLVMCodegenTool(goTool, "tool", "objview", "-format=text", archive)
		if err != nil {
			return fmt.Errorf("LLVM GoObj inspection failed: %v\n%s", err, output)
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM-OBJVIEW", src, output); err != nil {
			return fmt.Errorf("LLVM GoObj FileCheck failed: %v", err)
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-OBJSUMMARY") || hasLLVMFileCheckPrefix(src, "LLVM-NATIVE-OBJSUMMARY") {
		llvmJSON, err := runLLVMCodegenTool(goTool, "tool", "objview", "-format=json", archive)
		if err != nil {
			return fmt.Errorf("LLVM GoObj JSON inspection failed: %v\n%s", err, llvmJSON)
		}
		if hasLLVMFileCheckPrefix(src, "LLVM-OBJSUMMARY") {
			summary, err := llvmObjviewSummary("LLVM", llvmJSON)
			if err != nil {
				return err
			}
			if err := runLLVMFileCheck(fileCheck, source, "LLVM-OBJSUMMARY", src, summary); err != nil {
				return fmt.Errorf("LLVM GoObj summary FileCheck failed: %v", err)
			}
		}
		if hasLLVMFileCheckPrefix(src, "LLVM-NATIVE-OBJSUMMARY") {
			nativeArchive := filepath.Join(t.TempDir(), "codegen-native.a")
			nativeOutput, err := runLLVMCodegenTool(goTool, "tool", "compile",
				"-p=codegen",
				"-importcfg="+stdlibImportcfgFile(),
				"-c=16",
				"-o", nativeArchive,
				source,
			)
			if err != nil {
				return fmt.Errorf("native comparison compilation failed: %v\n%s", err, nativeOutput)
			}
			nativeJSON, err := runLLVMCodegenTool(goTool, "tool", "objview", "-format=json", nativeArchive)
			if err != nil {
				return fmt.Errorf("native GoObj JSON inspection failed: %v\n%s", err, nativeJSON)
			}
			nativeSummary, err := llvmObjviewSummary("NATIVE", nativeJSON)
			if err != nil {
				return err
			}
			llvmSummary, err := llvmObjviewSummary("LLVM", llvmJSON)
			if err != nil {
				return err
			}
			comparison := append(nativeSummary, llvmSummary...)
			if err := runLLVMFileCheck(fileCheck, source, "LLVM-NATIVE-OBJSUMMARY", src, comparison); err != nil {
				return fmt.Errorf("native/LLVM GoObj summary FileCheck failed: %v", err)
			}
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-NM") {
		output, err := runLLVMCodegenTool(goTool, "tool", "nm", archive)
		if err != nil {
			return fmt.Errorf("LLVM GoObj symbol inspection failed: %v\n%s", err, output)
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM-NM", src, output); err != nil {
			return fmt.Errorf("LLVM symbol FileCheck failed: %v", err)
		}
	}

	if hasLLVMFileCheckPrefix(src, "LLVM-LINK") {
		executable := filepath.Join(t.TempDir(), "codegen-link")
		output, err := runLLVMCodegenTool(goTool, "build",
			"-gcflags=-enablellvm",
			"-ldflags=-w -debugnosplit",
			"-o", executable,
			source,
		)
		if err != nil {
			return fmt.Errorf("LLVM link check failed: %v\n%s", err, output)
		}
		if err := runLLVMFileCheck(fileCheck, source, "LLVM-LINK", src, output); err != nil {
			return fmt.Errorf("LLVM linker output FileCheck failed: %v", err)
		}
		if output, err := runLLVMCodegenTool(executable); err != nil {
			return fmt.Errorf("LLVM linked executable failed: %v\n%s", err, output)
		}
	}
	return nil
}

func runLLVMCodegenTool(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	return cmd.CombinedOutput()
}

func runLLVMFileCheck(fileCheck, source, prefix string, sourceBytes, input []byte) error {
	cmd := exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes(prefix, sourceBytes), source)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%s", err, output)
	}
	return nil
}

func hasLLVMFileCheckPrefix(source []byte, base string) bool {
	candidates := []string{base}
	switch runtime.GOARCH {
	case "amd64":
		candidates = append(candidates, base+"-AMD64")
	case "arm64":
		candidates = append(candidates, base+"-ARM64")
	}
	for _, candidate := range candidates {
		if hasLLVMFileCheckCandidate(source, candidate) {
			return true
		}
	}
	return false
}

func hasLLVMFileCheckCandidate(source []byte, candidate string) bool {
	for line := range strings.SplitSeq(string(source), "\n") {
		comment := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "//"))
		if strings.HasPrefix(comment, candidate+":") {
			return true
		}
		if !strings.HasPrefix(comment, candidate+"-") {
			continue
		}
		kind, _, ok := strings.Cut(strings.TrimPrefix(comment, candidate+"-"), ":")
		if ok && llvmFileCheckDirectiveKind(kind) {
			return true
		}
	}
	return false
}

func llvmFileCheckDirectiveKind(kind string) bool {
	switch kind {
	case "DAG", "EMPTY", "LABEL", "NEXT", "NOT", "SAME":
		return true
	}
	if count, ok := strings.CutPrefix(kind, "COUNT-"); ok {
		_, err := strconv.Atoi(count)
		return err == nil
	}
	return false
}

type llvmObjviewDocument struct {
	Members []struct {
		GoObject *llvmObjviewObject `json:"go_object"`
	} `json:"members"`
}

type llvmObjviewObject struct {
	Autolib []struct {
		Package     string `json:"package"`
		Fingerprint string `json:"fingerprint"`
	} `json:"autolib"`
	Packages   []string `json:"packages"`
	References []struct {
		Name  string `json:"name"`
		Class string `json:"class"`
	} `json:"references"`
	Symbols []struct {
		Name      string   `json:"name"`
		Kind      string   `json:"kind"`
		FlagNames []string `json:"flag_names"`
		Aux       []struct {
			Type   string `json:"type"`
			Target struct {
				Package  string `json:"package"`
				Name     string `json:"name"`
				Kind     string `json:"pkg_kind"`
				SymIndex uint32 `json:"sym_index"`
			} `json:"target"`
		} `json:"aux"`
		Relocations []struct {
			Size   uint8  `json:"size"`
			Type   string `json:"type"`
			Target struct {
				Package  string `json:"package"`
				Name     string `json:"name"`
				Kind     string `json:"pkg_kind"`
				SymIndex uint32 `json:"sym_index"`
			} `json:"target"`
		} `json:"relocations"`
	} `json:"symbols"`
}

func llvmObjviewSummary(label string, data []byte) ([]byte, error) {
	var document llvmObjviewDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decoding objview JSON: %w", err)
	}
	var object *llvmObjviewObject
	for _, member := range document.Members {
		if member.GoObject != nil {
			object = member.GoObject
			break
		}
	}
	if object == nil {
		return nil, fmt.Errorf("objview JSON has no Go object member")
	}
	var lines []string
	for _, imp := range object.Autolib {
		lines = append(lines, fmt.Sprintf("%s autolib package=%q fingerprint=%s", label, imp.Package, imp.Fingerprint))
	}
	for index, pkg := range object.Packages {
		lines = append(lines, fmt.Sprintf("%s package index=%d path=%q", label, index, pkg))
	}
	for _, ref := range object.References {
		lines = append(lines, fmt.Sprintf("%s reference name=%q class=%s", label, ref.Name, ref.Class))
	}
	relocationCounts := make(map[string]int)
	for _, symbol := range object.Symbols {
		lines = append(lines, fmt.Sprintf("%s symbol name=%q kind=%s flags=%s", label, symbol.Name, symbol.Kind, strings.Join(symbol.FlagNames, ",")))
		for _, aux := range symbol.Aux {
			lines = append(lines, fmt.Sprintf("%s aux owner=%q type=%s target_kind=%s target_package=%q target_name=%q target_index=%d",
				label, symbol.Name, aux.Type, aux.Target.Kind, aux.Target.Package, aux.Target.Name, aux.Target.SymIndex))
		}
		for _, reloc := range symbol.Relocations {
			relocationCounts[reloc.Type]++
			lines = append(lines, fmt.Sprintf("%s relocation owner=%q type=%s size=%d target_kind=%s target_package=%q target_name=%q target_index=%d",
				label, symbol.Name, reloc.Type, reloc.Size, reloc.Target.Kind, reloc.Target.Package, reloc.Target.Name, reloc.Target.SymIndex))
		}
	}
	for relocation, count := range relocationCounts {
		lines = append(lines, fmt.Sprintf("%s relocation-count type=%s count=%d", label, relocation, count))
	}
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func llvmFileCheckPrefixes(base string, source []byte) string {
	var prefixes []string
	if hasLLVMFileCheckCandidate(source, base) {
		prefixes = append(prefixes, base)
	}
	var architecturePrefix string
	switch runtime.GOARCH {
	case "amd64":
		architecturePrefix = base + "-AMD64"
	case "arm64":
		architecturePrefix = base + "-ARM64"
	}
	if architecturePrefix != "" && hasLLVMFileCheckCandidate(source, architecturePrefix) {
		prefixes = append(prefixes, architecturePrefix)
	}
	return strings.Join(prefixes, ",")
}

func configureLLVMTestToolchain(t *testing.T) {
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

	// Freeze the compiler to the validated payload. Individual tests do not
	// select opt, llc, or a pass plugin; cmd/compile owns the complete
	// optimization and code-generation pipeline. Codegen IR assertions resolve
	// FileCheck lazily from this same payload.
	t.Setenv("GOALLC_LLVM_DIR", root)
	t.Logf("LLVM test toolchain: go=%s payload=%s in-process-pipeline=default<O2>",
		goTool, root)
	if configured != "" {
		t.Logf("LLVM payload selected by %s", configured)
	}
}

func llvmTestPayloadRoot(t *testing.T) (root, configured string) {
	t.Helper()
	if value := strings.TrimSpace(os.Getenv("GOALLC_LLVM_DIR")); value != "" {
		return llvmAbsolutePath(t, value, "GOALLC_LLVM_DIR"), "GOALLC_LLVM_DIR"
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
