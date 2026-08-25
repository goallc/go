// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"internal/testenv"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

const llvmStdlibPolicyEnv = "GOALLC_RUN_LLVM_STDLIB"

// These tests exercise metadata, debugger-call injection, or signal semantics
// that the LLVM backend does not model yet. Keep the exclusions at the LLVM
// qualification boundary so the upstream tests continue to specify native Go
// behavior unchanged. Subtest patterns retain the passing panicwrap, panic,
// and unsafe-point rejection coverage beside the excluded cases.
const llvmRuntimeSkip = `^(TestCallersNilPointerPanic|TestDebugCall|TestDebugCallGC|TestDebugCallGrowStack|TestDebugCallLarge|TestDebugCallPanic|TestGCInfo|TestTracebackArgs|TestTracebackElision)$|^TestStackWrapperStackPanic$/^sigpanic$|^TestTracebackSystem$/^trap$`

// The amd64 test additionally requires the native compiler's INT3 function
// alignment filler. LLVM deliberately emits NOP padding instead.
const llvmRuntimeAMD64Skip = `|^TestFunctionAlignmentTraceback$`

// These tests assert native pcln and inline-stack shapes that LLVM GoObj does
// not fully reproduce yet. Keep the heap-profile stack-growth coverage in
// TestMemoryProfiler and TestGenericsHashKeyInPprofBuilder enabled.
const llvmPprofSkip = `^(TestCPUProfileRecursion|TestGenericsInlineLocations|TestProfilerStackDepth)$|^TestTryAdd$/^recursion_chain_inline$`

// LLVM currently reports the inlined TestAll call sites rather than the
// original testPrint call sites expected by the file-line assertions.
const llvmLogSkip = `^TestAll$`

// LLVM GoObj does not yet preserve the symbol boundaries used by the FIPS
// integrity check. The algorithm, self-test, and service-indicator tests remain
// enabled; only the object-layout assertion is excluded.
const llvmFIPS140TestSkip = `^TestIntegrityCheckInfo$`

type llvmStdlibTestSet struct {
	Blacklist         map[string]string            `json:"blacklist"`
	PlatformBlacklist map[string]map[string]string `json:"platform_blacklist,omitempty"`
}

type llvmStdlibPolicy struct {
	Packages llvmStdlibTestSet `json:"packages"`
}

func readLLVMStdlibPolicy(t *testing.T) llvmStdlibPolicy {
	t.Helper()
	filename := filepath.Join(testenv.GOROOT(t), "test", "llvm_stdlib_packages.json")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var policy llvmStdlibPolicy
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&policy); err != nil {
		t.Fatalf("parse llvm_stdlib_packages.json: %v", err)
	}
	return policy
}

func llvmStdlibPackages(t *testing.T) map[string]bool {
	t.Helper()
	cmd := testenv.Command(t, llvmStdlibGoTool(t), "list", "std")
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=", "GOROOT="+testenv.GOROOT(t))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list standard library packages: %v\n%s", err, out)
	}
	packages := make(map[string]bool)
	for _, name := range strings.Fields(string(out)) {
		packages[name] = true
	}
	return packages
}

func llvmStdlibGoTool(t *testing.T) string {
	t.Helper()
	if goTool == "" {
		goTool = testenv.GoToolPath(t)
	}
	return goTool
}

func effectiveLLVMStdlibBlacklist(set llvmStdlibTestSet, platform string) map[string]string {
	effective := make(map[string]string, len(set.Blacklist)+len(set.PlatformBlacklist[platform]))
	for name, reason := range set.Blacklist {
		effective[name] = reason
	}
	for name, reason := range set.PlatformBlacklist[platform] {
		effective[name] = reason
	}
	return effective
}

func validateLLVMStdlibPolicy(t *testing.T, packages map[string]bool, set llvmStdlibTestSet, currentPlatform string) {
	t.Helper()
	failed := false
	for name, reason := range set.Blacklist {
		if name == "*" || strings.ContainsAny(name, "*?[\\") {
			t.Errorf("LLVM standard library blacklist entry %q is not an exact package", name)
			failed = true
		}
		if !packages[name] {
			t.Errorf("LLVM standard library blacklist entry %q is not a standard library package", name)
			failed = true
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("LLVM standard library blacklist entry %q has no reason", name)
			failed = true
		}
	}
	for platform, entries := range set.PlatformBlacklist {
		if strings.TrimSpace(platform) == "" {
			t.Error("LLVM standard library platform blacklist has an empty platform")
			failed = true
		}
		for name, reason := range entries {
			if name == "*" || strings.ContainsAny(name, "*?[\\") {
				t.Errorf("LLVM standard library platform blacklist entry %q for %s is not an exact package", name, platform)
				failed = true
			}
			if platform == currentPlatform && !packages[name] {
				t.Errorf("LLVM standard library platform blacklist entry %q for %s is not a standard library package", name, platform)
				failed = true
			}
			if strings.TrimSpace(reason) == "" {
				t.Errorf("LLVM standard library platform blacklist entry %q for %s has no reason", name, platform)
				failed = true
			}
			if _, ok := set.Blacklist[name]; ok {
				t.Errorf("LLVM standard library package %q appears in both common and %s blacklists", name, platform)
				failed = true
			}
		}
	}
	if failed {
		t.FailNow()
	}
}

func TestLLVMStdlibPolicy(t *testing.T) {
	packages := llvmStdlibPackages(t)
	validateLLVMStdlibPolicy(t, packages, readLLVMStdlibPolicy(t).Packages, runtime.GOOS+"/"+runtime.GOARCH)
}

func TestEffectiveLLVMStdlibBlacklist(t *testing.T) {
	set := llvmStdlibTestSet{
		Blacklist: map[string]string{"bytes": "known failure"},
		PlatformBlacklist: map[string]map[string]string{
			"linux/amd64": {"cmp": "platform failure"},
		},
	}
	effective := effectiveLLVMStdlibBlacklist(set, "linux/amd64")
	for name, want := range map[string]string{"bytes": "known failure", "cmp": "platform failure"} {
		if got := effective[name]; got != want {
			t.Errorf("effective blacklist reason for %q = %q, want %q", name, got, want)
		}
	}
	if _, ok := set.Blacklist["cmp"]; ok {
		t.Fatal("platform selection modified the common blacklist")
	}
}

func llvmStdlibCapabilitySkip(name, goarch string) string {
	switch name {
	case "crypto/internal/fips140test":
		return llvmFIPS140TestSkip
	case "log":
		return llvmLogSkip
	case "runtime":
		skip := llvmRuntimeSkip
		if goarch == "amd64" {
			skip += llvmRuntimeAMD64Skip
		}
		return skip
	case "runtime/pprof":
		return llvmPprofSkip
	default:
		return ""
	}
}

func TestLLVMRuntimeAsyncPreemptionQualification(t *testing.T) {
	skip := regexp.MustCompile(llvmRuntimeSkip + llvmRuntimeAMD64Skip)
	for _, name := range []string{
		"TestPreemption",
		"TestPreemptionGC",
		"TestAsyncPreempt",
		"TestUnsafePoint",
	} {
		if skip.MatchString(name) {
			t.Errorf("qualified LLVM runtime test %q is skipped", name)
		}
	}
}

func TestLLVMPprofBadPointerQualification(t *testing.T) {
	skip := regexp.MustCompile(llvmPprofSkip)
	for _, name := range []string{
		"TestMemoryProfiler/proto",
		"TestGenericsHashKeyInPprofBuilder",
	} {
		if skip.MatchString(name) {
			t.Errorf("qualified LLVM runtime/pprof test %q is skipped", name)
		}
	}
}

func TestLLVMStdlib(t *testing.T) {
	if os.Getenv(llvmStdlibPolicyEnv) != "1" {
		t.Skipf("set %s=1 to run the LLVM standard library package policy", llvmStdlibPolicyEnv)
	}
	testenv.MustHaveGoBuild(t)
	platform := runtime.GOOS + "/" + runtime.GOARCH
	switch platform {
	case "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skipf("LLVM GoObj is not configured for %s", platform)
	}

	packages := llvmStdlibPackages(t)
	policySet := readLLVMStdlibPolicy(t).Packages
	validateLLVMStdlibPolicy(t, packages, policySet, platform)
	blacklist := effectiveLLVMStdlibBlacklist(policySet, platform)
	configureLLVMTestToolchain(t)

	candidates := make([]string, 0, len(packages)-len(blacklist))
	blacklisted := make([]string, 0, len(blacklist))
	for name := range packages {
		if _, ok := blacklist[name]; ok {
			blacklisted = append(blacklisted, name)
		} else {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	sort.Strings(blacklisted)
	mode := "long"
	if testing.Short() {
		mode = "short"
	}
	t.Logf("LLVM standard library default-test policy: mode=%s, %d tested, %d blacklisted (%d packages)", mode, len(candidates), len(blacklisted), len(packages))
	for _, name := range blacklisted {
		t.Logf("LLVM stdlib blacklist result: NOT RUN package=%q reason=%q", name, blacklist[name])
	}

	// The target package and in-process LLVM pipeline are part of cmd/go's action IDs,
	// so one isolated cache can safely serve every package in this policy run.
	// A single all= pattern compiles the test package, generated test main,
	// runtime, and the complete dependency closure with LLVM. The compiler and
	// its recorded payload remain fixed for the lifetime of the test process.
	cache := t.TempDir()
	warmLLVMExecutionRuntime(t, cache)
	parallelism := runtime.NumCPU()
	limit := make(chan struct{}, parallelism)
	t.Logf("LLVM standard library package parallelism: %d (%d CPUs)", parallelism, runtime.NumCPU())
	for _, name := range candidates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			limit <- struct{}{}
			defer func() { <-limit }()
			testTimeout := "2m"
			processTimeout := 5 * time.Minute
			if name == "runtime" {
				// The runtime stress tests need more CPU time when they run
				// alongside other full-LLVM package builds.
				testTimeout = "15m"
				processTimeout = 30 * time.Minute
			} else if name == "runtime/pprof" {
				testTimeout = "5m"
				processTimeout = 15 * time.Minute
			} else if name == "encoding/json/v2" {
				// Its tests are fast once linked, but a cold full-LLVM
				// compilation under package-level contention can exceed
				// fifteen minutes.
				processTimeout = 30 * time.Minute
			}
			args := []string{
				"test",
				// The outer semaphore supplies package-level build parallelism.
				// -p does not change GOMAXPROCS or -test.parallel in the test
				// binary, so runtime concurrency remains fully exercised.
				"-p=1",
				"-count=1",
				"-timeout=" + testTimeout,
				"-gcflags=all=-enablellvm",
			}
			if testing.Short() {
				args = append(args, "-short")
			}
			if name == "runtime" {
				// LLVM GoObj does not yet emit complete
				// per-function DWARF, GC, and traceback metadata, or support
				// the remaining debugger-call and signal tests.
				args = append(args, "-ldflags=-w")
			}
			if skip := llvmStdlibCapabilitySkip(name, runtime.GOARCH); skip != "" {
				args = append(args, "-skip="+skip)
				t.Logf("LLVM capability-boundary skips: %s", skip)
			}
			args = append(args, name)
			ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), processTimeout)
			defer cancel()
			cmd := testenv.CommandContext(t, ctx, llvmStdlibGoTool(t), args...)
			cmd.Env = append(os.Environ(),
				"GOENV=off",
				"GOFLAGS=",
				"GOROOT="+testenv.GOROOT(t),
				"GOCACHE="+cache,
			)
			out, err := cmd.CombinedOutput()
			ctxErr := ctx.Err()
			if err != nil {
				if ctxErr != nil {
					t.Fatalf("LLVM stdlib result: TIMEOUT package=%q: %v\n%s", name, ctxErr, out)
				}
				t.Fatalf("LLVM stdlib result: FAIL package=%q: %v\n%s", name, err, out)
			}
			t.Logf("LLVM stdlib result: PASS package=%q", name)
		})
	}
}
