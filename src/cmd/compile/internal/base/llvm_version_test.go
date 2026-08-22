// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"os"
	"testing"
)

func TestCompileVersionFlagFullSuffixUsesBuildID(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	for _, test := range []struct {
		name     string
		args     []string
		external string
	}{
		{name: "native", args: []string{"compile", "-V=full"}},
		{name: "external LLVM", args: []string{"compile", "-enablellvm", "-V=full"}, external: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			os.Args = test.args
			t.Setenv("GOALLC_EXTERNAL_BACKEND", test.external)
			if got, want := compileVersionFlagFullSuffix("compiler-build-id"), " buildID=compiler-build-id"; got != want {
				t.Fatalf("compileVersionFlagFullSuffix() = %q, want %q", got, want)
			}
		})
	}
}

func TestLLVMVersionEnabled(t *testing.T) {
	tests := []struct {
		args    []string
		enabled bool
	}{
		{args: nil},
		{args: []string{"-enablellvm"}, enabled: true},
		{args: []string{"-enablellvm=false"}},
		{args: []string{"-enablellvm=false", "-enablellvm"}, enabled: true},
	}
	for _, test := range tests {
		if enabled := llvmVersionEnabled(test.args); enabled != test.enabled {
			t.Errorf("llvmVersionEnabled(%q) = %v, want %v", test.args, enabled, test.enabled)
		}
	}
}
