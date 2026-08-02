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
	"runtime"
	"sort"
	"strings"
	"testing"
)

type llvmTestSet struct {
	Whitelist         map[string]string            `json:"whitelist"`
	Blacklist         map[string]string            `json:"blacklist"`
	PlatformBlacklist map[string]map[string]string `json:"platform_blacklist,omitempty"`
}

type llvmTestPolicy struct {
	Codegen llvmTestSet `json:"codegen"`
	Runtime llvmTestSet `json:"runtime"`
}

// runLLVMTests reuses the existing GOROOT/test inputs. The whitelist contains
// tests that must pass today; the blacklist classifies known unsupported tests.
// Whitelist entries take precedence over broad blacklist patterns, so enabling
// another test is a one-line policy change plus its LLVM IR checks.
func runLLVMTests(t *testing.T, common testCommon) {
	t.Run("LLVM", func(t *testing.T) {
		switch runtime.GOOS + "/" + runtime.GOARCH {
		case "darwin/arm64", "linux/amd64":
		default:
			t.Skipf("LLVM GoObj is not configured for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		policy := readLLVMTestPolicy(t, common.gorootTestDir)
		platform := runtime.GOOS + "/" + runtime.GOARCH
		if err := applyLLVMPlatformBlacklist("codegen", platform, &policy.Codegen); err != nil {
			t.Fatal(err)
		}
		if err := applyLLVMPlatformBlacklist("runtime", platform, &policy.Runtime); err != nil {
			t.Fatal(err)
		}
		codegenCandidates := llvmTestCandidates(t, common.gorootTestDir, []string{"codegen"}, "asmcheck")
		runtimeCandidates := llvmTestCandidates(t, common.gorootTestDir, dirs, "run")
		validateLLVMTestSet(t, common.gorootTestDir, "codegen", codegenCandidates, policy.Codegen, true)
		validateLLVMTestSet(t, common.gorootTestDir, "runtime", runtimeCandidates, policy.Runtime, false)

		t.Logf("LLVM codegen whitelist: %d/%d files", len(policy.Codegen.Whitelist), len(codegenCandidates))
		t.Logf("LLVM runtime whitelist: %d/%d files", len(policy.Runtime.Whitelist), len(runtimeCandidates))

		t.Run("codegen", func(t *testing.T) {
			names := sortedLLVMWhitelist(policy.Codegen.Whitelist)
			for _, name := range names {
				t.Run(name, func(t *testing.T) {
					runLLVMCodegenTest(t, common.gorootTestDir, name)
				})
			}
		})

		t.Run("abi-differential", func(t *testing.T) {
			runLLVMABIDifferentialTest(t, common.gorootTestDir)
		})

		t.Run("alloca-statepoint", func(t *testing.T) {
			runLLVMAllocaStatepointTest(t, common.gorootTestDir)
		})

		t.Run("writebarrier-helpers", runLLVMWriteBarrierHelperTest)

		t.Run("compile-only-regressions", func(t *testing.T) {
			for _, name := range []string{
				"cmp.go",
				"typeparam/issue47684c.go",
			} {
				t.Run(name, func(t *testing.T) {
					runLLVMCompileOnlyRegression(t, common.gorootTestDir, name)
				})
			}
		})

		t.Run("runtime", func(t *testing.T) {
			names := sortedLLVMWhitelist(policy.Runtime.Whitelist)
			for _, name := range names {
				t.Run(name, func(t *testing.T) {
					dir, file := path.Split(name)
					tc := test{
						testCommon: common,
						T:          t,
						dir:        strings.TrimSuffix(dir, "/"),
						goFile:     file,
						llvm:       true,
					}
					if err := tc.run(); err != nil {
						t.Fatal(err)
					}
				})
			}
		})

		t.Run("writebarrier-ir", runLLVMWriteBarrierIRTests)
	})
}

func runLLVMCompileOnlyRegression(t *testing.T, gorootTestDir, name string) {
	t.Helper()
	toolexec := llvmToolexec(t)
	exe := filepath.Join(t.TempDir(), "test.exe")
	cmd := exec.Command(goTool, "build",
		"-gcflags=all="+os.Getenv("GO_GCFLAGS"),
		"-gcflags=-enablellvm",
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

func applyLLVMPlatformBlacklist(name, platform string, set *llvmTestSet) error {
	for target, entries := range set.PlatformBlacklist {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("LLVM %s platform blacklist has an empty platform", name)
		}
		for filename, reason := range entries {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("LLVM %s platform blacklist entry %q for %s has no reason", name, filename, target)
			}
			if _, ok := set.Whitelist[filename]; !ok {
				return fmt.Errorf("LLVM %s platform blacklist entry %q for %s is not in the common whitelist", name, filename, target)
			}
		}
	}

	for filename := range set.PlatformBlacklist[platform] {
		delete(set.Whitelist, filename)
	}
	return nil
}

func TestApplyLLVMPlatformBlacklist(t *testing.T) {
	set := llvmTestSet{
		Whitelist: map[string]string{
			"common.go": "common",
			"linux.go":  "linux",
			"darwin.go": "darwin",
		},
		PlatformBlacklist: map[string]map[string]string{
			"linux/amd64":  {"linux.go": "linux limitation"},
			"darwin/arm64": {"darwin.go": "darwin limitation"},
		},
	}
	if err := applyLLVMPlatformBlacklist("runtime", "linux/amd64", &set); err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Whitelist["linux.go"]; ok {
		t.Fatal("current-platform exclusion remained in the effective whitelist")
	}
	for _, filename := range []string{"common.go", "darwin.go"} {
		if _, ok := set.Whitelist[filename]; !ok {
			t.Errorf("applyLLVMPlatformBlacklist removed %q for another platform", filename)
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
				Whitelist:         map[string]string{"test.go": "test"},
				PlatformBlacklist: map[string]map[string]string{"": {"test.go": "reason"}},
			},
			want: "empty platform",
		},
		{
			name: "empty reason",
			set: llvmTestSet{
				Whitelist:         map[string]string{"test.go": "test"},
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"test.go": " "}},
			},
			want: "has no reason",
		},
		{
			name: "not in common whitelist",
			set: llvmTestSet{
				Whitelist:         map[string]string{"test.go": "test"},
				PlatformBlacklist: map[string]map[string]string{"linux/amd64": {"missing.go": "reason"}},
			},
			want: "is not in the common whitelist",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := applyLLVMPlatformBlacklist("runtime", "linux/amd64", &tc.set)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("applyLLVMPlatformBlacklist error = %v, want substring %q", err, tc.want)
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
	for filename := range set.Whitelist {
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

	for pattern := range set.Blacklist {
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

	for filename := range candidates {
		if _, ok := set.Whitelist[filename]; ok {
			continue
		}
		classified := false
		for pattern := range set.Blacklist {
			if llvmPathMatch(t, pattern, filename) {
				classified = true
				break
			}
		}
		if !classified {
			t.Errorf("LLVM %s test %q is in neither whitelist nor blacklist", name, filename)
			failed = true
		}
	}
	if failed {
		t.FailNow()
	}
}

func llvmPathMatch(t *testing.T, pattern, filename string) bool {
	t.Helper()
	matched, err := path.Match(pattern, filename)
	if err != nil {
		t.Fatalf("invalid LLVM test blacklist pattern %q: %v", pattern, err)
	}
	if matched || strings.Contains(pattern, "/") {
		return matched
	}
	matched, err = path.Match(pattern, path.Base(filename))
	if err != nil {
		t.Fatalf("invalid LLVM test blacklist pattern %q: %v", pattern, err)
	}
	return matched
}

func sortedLLVMWhitelist(entries map[string]string) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func runLLVMCodegenTest(t *testing.T, gorootTestDir, name string) {
	t.Helper()
	source := filepath.Join(gorootTestDir, filepath.FromSlash(name))
	src, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	header, _, _ := strings.Cut(string(src), "\npackage")
	if ok, why := shouldTest(header, goos, goarch); !ok {
		t.Skip(why)
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
		t.Fatalf("LLVM compilation failed: %v\n%s", err, out)
	}
	irBytes, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	opt := llvmToolPath(t, "opt", "GOALLC_OPT")
	cmd = exec.Command(opt, "-passes=verify", "-disable-output")
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("LLVM verifier failed: %v\n%s", err, out)
	}
	fileCheck := llvmToolPath(t, "FileCheck", "GOALLC_FILECHECK")
	cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM", src), source)
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("FileCheck failed: %v\n%s", err, out)
	}

	if !bytes.Contains(src, []byte("// LLVM-OPT")) {
		return
	}
	cmd = exec.Command(opt, "-passes=default<O2>", "-S")
	cmd.Stdin = bytes.NewReader(irBytes)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	optimizedIR, err := cmd.Output()
	if err != nil {
		t.Fatalf("LLVM optimization failed: %v\n%s", err, stderr.Bytes())
	}
	cmd = exec.Command(fileCheck, "--check-prefixes="+llvmFileCheckPrefixes("LLVM-OPT", src), source)
	cmd.Stdin = bytes.NewReader(optimizedIR)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("optimized LLVM FileCheck failed: %v\n%s", err, out)
	}
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

func (t test) runLLVMCase(tempDir string, flags, args []string, runcmd runCmd) error {
	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		t.Skip("LLVM execution tests do not support cross compilation")
	}
	toolexec := llvmToolexec(t.T)
	exe := filepath.Join(tempDir, "test.exe")
	// GoObj DWARF emission is not implemented by the LLVM backend yet. Disable
	// debug data explicitly so runtime qualification measures compile, link, and
	// execution support instead of failing in the linker's DWARF writer.
	cmd := []string{goTool, "build", t.goGcflags(), "-gcflags=-enablellvm", "-ldflags=-w", "-toolexec=" + toolexec, "-o", exe}
	cmd = append(cmd, flags...)
	cmd = append(cmd, t.goFileName())
	if _, err := runcmd(cmd...); err != nil {
		return err
	}
	out, err := runcmd(append([]string{exe}, args...)...)
	if err != nil {
		return err
	}
	return t.checkExpectedOutput(out)
}

func llvmToolexec(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(goTool, "tool", "-n", "llvmtoolexec").CombinedOutput()
	if err != nil {
		t.Skipf("llvmtoolexec is unavailable: %v\n%s", err, out)
	}
	wrapper := strings.TrimSpace(string(out))

	llc := llvmToolPath(t, "llc", "GOALLC_LLC")
	value, err := quoted.Join([]string{wrapper, "-llc=" + llc})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func llvmToolPath(t *testing.T, name, envName string) string {
	t.Helper()
	var candidates []string
	if candidate := os.Getenv(envName); candidate != "" {
		candidates = append(candidates, candidate)
	}
	if root := os.Getenv("GOALLC_LLVM_DIR"); root != "" {
		candidates = append(candidates, filepath.Join(root, "bin", name))
	}
	candidates = append(candidates, filepath.Join(testenv.GOROOT(t), "llvm", "bin", name))

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skipf("%s is unavailable; set %s or install the LLVM payload under GOROOT/llvm", name, envName)
	return ""
}
