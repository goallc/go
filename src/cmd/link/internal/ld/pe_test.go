// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"bytes"
	"cmd/internal/buildid"
	"fmt"
	"internal/testenv"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPEExternalContentIdentity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires PE external linking")
	}
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	dir := t.TempDir()
	source, exe := filepath.Join(dir, "main.go"), filepath.Join(dir, "main.exe")
	if err := os.WriteFile(source, []byte("package main\nimport \"C\"\nfunc main() {}\n"), 0666); err != nil {
		t.Fatal(err)
	}
	var want [32]byte
	for i, id := range []string{"action-one", "action-two"} {
		cmd := testenv.Command(t, testenv.GoToolPath(t), "build", "-o", exe, "-ldflags=-linkmode=external -buildid="+id, source)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		data, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		// Normalize the ID wherever it occurs, including the recorded build flags.
		// A secondary linker-generated hash of the ID cannot be normalized this way.
		matches, hash, err := buildid.FindAndHash(bytes.NewReader(data), id, 0)
		if err != nil || len(matches) == 0 {
			t.Fatalf("FindAndHash: %v, matches %v", err, matches)
		}
		if i == 0 {
			want = hash
		} else if hash != want {
			t.Fatal("changing only the action ID changed the PE content identity")
		}
	}
}

func TestPEExternalDataBoundary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires PE external linking")
	}
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	dir := t.TempDir()
	source, exe := filepath.Join(dir, "main.go"), filepath.Join(dir, "main.exe")
	// Vary initialized pointer data across the external COFF section alignment.
	// The BSS and following SEH sections must retain their assigned addresses.
	for n := 1; n <= 4; n++ {
		text := fmt.Sprintf(`package main
import "C"
var x int
var data = [%d]*int{&x}
var bss *int
func main() {
 if bss != nil { panic("BSS was not zeroed") }
 bss = data[0]
 *bss = 42
 if x != 42 { panic("incorrect data relocation") }
}
`, n)
		if err := os.WriteFile(source, []byte(text), 0666); err != nil {
			t.Fatal(err)
		}
		cmd := testenv.Command(t, testenv.GoToolPath(t), "build", "-o", exe, "-ldflags=-linkmode=external", source)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("data length %d: %v\n%s", n, err, out)
		}
		if out, err := testenv.Command(t, exe).CombinedOutput(); err != nil {
			t.Fatalf("data length %d execution: %v\n%s", n, err, out)
		}
	}
}
