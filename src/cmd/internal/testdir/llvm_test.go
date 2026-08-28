// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/build/constraint"
	"internal/testenv"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
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

func TestLLVMCaseTimeoutSeconds(t *testing.T) {
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

type llvmTestSet struct {
	Blacklist         map[string]string            `json:"blacklist"`
	PlatformBlacklist map[string]map[string]string `json:"platform_blacklist,omitempty"`
}

type llvmTestPolicy struct {
	Codegen llvmTestSet `json:"codegen"`
	Run     llvmTestSet `json:"run"`
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

func applyLLVMPlatformPolicy(name, platform string, set *llvmTestSet) error {
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

	for filename, reason := range set.PlatformBlacklist[platform] {
		set.Blacklist[filename] = reason
	}
	return nil
}

func TestApplyLLVMPlatformPolicy(t *testing.T) {
	set := llvmTestSet{
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
		set  llvmTestSet
		want string
	}{
		{
			name: "empty platform",
			set: llvmTestSet{
				PlatformBlacklist: map[string]map[string]string{"": {"test.go": "unsupported: reason"}},
			},
			want: "empty platform",
		},
		{
			name: "empty reason",
			set: llvmTestSet{
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"test.go": " "}},
			},
			want: "has no reason",
		},
		{
			name: "blacklist ordinary failure",
			set: llvmTestSet{
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

func TestIsLLVMTestBlacklisted(t *testing.T) {
	set := llvmTestSet{
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

func validateLLVMTestSet(t *testing.T, name string, candidates map[string]bool, set llvmTestSet) {
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

func TestValidLLVMBlacklistReason(t *testing.T) {
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

func isLLVMTestBlacklisted(t *testing.T, set llvmTestSet, filename string) bool {
	t.Helper()
	for pattern := range set.Blacklist {
		if llvmPathMatch(t, pattern, filename) {
			return true
		}
	}
	return false
}

func logLLVMTestPolicy(t *testing.T, name string, candidates map[string]bool, set llvmTestSet) {
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

func logLLVMBlacklist(t *testing.T, name string, candidates map[string]bool, set llvmTestSet) {
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

func sortedLLVMBlacklistedTests(t *testing.T, candidates map[string]bool, set llvmTestSet) []string {
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
	if !bytes.Contains(src, []byte("// LLVM")) {
		return nil
	}
	irBytes, err := os.ReadFile(archive + ".ll")
	if err != nil {
		return err
	}
	fileCheck := llvmToolPath(t, "FileCheck", "GOALLC_FILECHECK")
	cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM", src), source)
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("FileCheck failed: %v\n%s", err, out)
	}

	if bytes.Contains(src, []byte("// LLVM-OPT")) {
		optimizedIR, err := os.ReadFile(archive + ".opt.ll")
		if err != nil {
			return err
		}
		cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM-OPT", src), source)
		cmd.Stdin = bytes.NewReader(optimizedIR)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("optimized LLVM FileCheck failed: %v\n%s", err, out)
		}
	}

	if bytes.Contains(src, []byte("// LLVM-ASM")) {
		cmd = exec.Command(goTool, "tool", "objdump", archive)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
		assembly, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("LLVM object disassembly failed: %v\n%s", err, assembly)
		}
		cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM-ASM", src), source)
		cmd.Stdin = bytes.NewReader(assembly)
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("LLVM assembly FileCheck failed: %v\n%s", err, out)
		}
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
	// select opt, llc, a pass plugin, or toolexec; cmd/compile owns the complete
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
