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

	os.Args = []string{"compile", "-V=full"}
	if got, want := compileVersionFlagFullSuffix("compiler-build-id"), " buildID=compiler-build-id"; got != want {
		t.Fatalf("compileVersionFlagFullSuffix() = %q, want %q", got, want)
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
