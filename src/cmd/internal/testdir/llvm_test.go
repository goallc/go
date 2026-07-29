// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdir_test

import (
	"bytes"
	"cmd/internal/quoted"
	"encoding/json"
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
	Whitelist map[string]string `json:"whitelist"`
	Blacklist map[string]string `json:"blacklist"`
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

		t.Run("fail-closed", runLLVMMemoryOpFailClosedTests)
	})
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
	cmd = exec.Command(fileCheck, "--check-prefix=LLVM", source)
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
	cmd = exec.Command(fileCheck, "--check-prefix=LLVM-OPT", source)
	cmd.Stdin = bytes.NewReader(optimizedIR)
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("optimized LLVM FileCheck failed: %v\n%s", err, out)
	}
}

func runLLVMMemoryOpFailClosedTests(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "ZeroWithPointers",
			source: "package p\nfunc zero(dst *[2]*int) { *dst = [2]*int{} }\n",
			want:   "Zero of pointer-containing type [2]*int requires write-barrier lowering before LLVM",
		},
		{
			name:   "MoveWithPointers",
			source: "package p\nfunc move(dst, src *[2]*int) { *dst = *src }\n",
			want:   "Move of pointer-containing type [2]*int requires write-barrier lowering before LLVM",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "fail.go")
			if err := os.WriteFile(source, []byte(tc.source), 0o666); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "fail.a")
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
			if err == nil {
				t.Fatalf("LLVM compilation unexpectedly succeeded")
			}
			if !bytes.Contains(out, []byte(tc.want)) {
				t.Fatalf("LLVM compilation error does not contain %q:\n%s", tc.want, out)
			}
			if _, err := os.Stat(archive + ".ll"); !os.IsNotExist(err) {
				t.Fatalf("failed LLVM compilation left IR output: %v", err)
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
	cmd := []string{goTool, "build", t.goGcflags(), "-toolexec=" + toolexec, "-o", exe}
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
	value, err := quoted.Join([]string{wrapper, "-llc=" + llc, "-package=main"})
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
