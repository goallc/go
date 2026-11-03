// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticDependenciesComeFromPayload(t *testing.T) {
	old := goallcLLVMDir
	t.Cleanup(func() { goallcLLVMDir = old })
	goallcLLVMDir = filepath.Join(t.TempDir(), "payload with spaces")
	lib := filepath.Join(goallcLLVMDir, "lib")
	if err := os.MkdirAll(lib, 0755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(lib, "libzstd.a")
	if err := os.WriteFile(archive, []byte("archive"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"-lzstd", filepath.Join(t.TempDir(), "old", "libzstd.a")} {
		if got := goallcLLVMStaticSystemArchive(input); got != archive {
			t.Errorf("%q resolved to %q, want bundled %q", input, got, archive)
		}
	}
	for _, input := range []string{"-lz", "-lzstd", "-lxml2"} {
		if isGoallcLLVMSystemLibrary(input) {
			t.Errorf("third-party dependency %q may not remain dynamic", input)
		}
	}
	if got := goallcLLVMStaticSystemArchive("-lmissing"); got != "" {
		t.Fatalf("missing payload archive resolved outside payload: %q", got)
	}
}
