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

	os.Args = []string{"compile", "-enablellvm=false", "-V=full"}
	if got, want := compileVersionFlagFullSuffix("compiler-build-id"), " buildID=compiler-build-id"; got != want {
		t.Fatalf("compileVersionFlagFullSuffix() = %q, want %q", got, want)
	}
}

func TestLLVMVersionEnabled(t *testing.T) {
	tests := []struct {
		args    []string
		enabled bool
	}{
		{args: nil, enabled: true},
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

func TestLLVMVersionBuildIDUsesContent(t *testing.T) {
	got := llvmVersionBuildID("action1/main1/pkg/content", "backend")
	if got != llvmVersionBuildID("action2/main2/pkg/content", "backend") {
		t.Fatal("action IDs changed the compiler content identity")
	}
	if got == llvmVersionBuildID("action1/main1/pkg/changed", "backend") ||
		got == llvmVersionBuildID("action1/main1/pkg/content", "changed") {
		t.Fatal("compiler or backend content did not change the identity")
	}
}
