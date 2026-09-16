// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testenv

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// GoCompilerUsesLLVM reports whether the go command's compiler uses LLVM by
// default. This describes compiler invocations made by a test, not the backend
// that built the test binary. Explicit per-command -enablellvm flags override it.
func GoCompilerUsesLLVM(t testing.TB) bool {
	t.Helper()
	MustHaveGoBuild(t)
	enabled, err := goCompilerUsesLLVM()
	if err != nil {
		t.Fatal(err)
	}
	return enabled
}

var goCompilerUsesLLVM = sync.OnceValues(func() (bool, error) {
	goTool, err := goTool()
	if err != nil {
		return false, err
	}
	cmd := exec.Command(goTool, "tool", "compile", "-V=full")
	cmd.Env = origEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("query compiler backend: %v\n%s", err, out)
	}
	// The compiler's LLVM build identity includes its payload and plugin.
	return strings.Contains(string(out), " buildID=goallc-"), nil
})

// SkipIfLLVM skips a test whose expectations require the native Go backend.
func SkipIfLLVM(t testing.TB, reason string) {
	t.Helper()
	if GoCompilerUsesLLVM(t) {
		t.Skip("LLVM backend: " + reason)
	}
}
