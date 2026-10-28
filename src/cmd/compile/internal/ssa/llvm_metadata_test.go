// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"cmd/internal/goobj"
	"fmt"
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Check ordinary object compilation against native Go. These fields must
// survive even though the LLVM frontend no longer emits redundant flags or
// a module-wide gotype relationship table.
func TestLLVMNativeMetadata(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	source := filepath.Join(t.TempDir(), "p.go")
	const code = `package p
type Node struct { Next *Node; Value int }
var Root Node
var Sink uintptr
//go:noinline
func Call(x uintptr) { Sink = x }
//go:nosplit
//go:noinline
func NoSplit(x uintptr) { Call(x) }
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			for _, enableLLVM := range []bool{false, true} {
				t.Run(fmt.Sprintf("llvm=%t", enableLLVM), func(t *testing.T) {
					output := filepath.Join(t.TempDir(), "p.a")
					args := []string{"tool", "compile", fmt.Sprintf("-enablellvm=%t", enableLLVM), "-p=p", "-o", output}
					if enableLLVM {
						args = append(args, "-llvm-keep-ir")
					}
					cmd := testenv.Command(t, testenv.GoToolPath(t), append(args, source)...)
					cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("compile: %v\n%s", err, out)
					}
					r := readDwarfTestObject(t, output)
					found := map[string]bool{}
					for i := uint32(0); i < uint32(r.NSym()); i++ {
						s := r.Sym(i)
						switch s.Name(r) {
						case "p.NoSplit":
							found["nosplit"] = true
							if !s.NoSplit() {
								t.Error("NoSplit lost its Go object flag")
							}
						case "p.Root":
							for _, aux := range r.Auxs(i) {
								if aux.Type() != goobj.AuxGotype {
									continue
								}
								ref := aux.Sym()
								if ref.PkgIdx != goobj.PkgIdxSelf {
									t.Fatalf("unexpected Root type reference: %+v", ref)
								}
								if name := r.Sym(ref.SymIdx).Name(r); name != "type:p.Node" {
									t.Fatalf("Root gotype = %q", name)
								}
								found["gotype"] = true
							}
						}
					}
					if !found["nosplit"] || !found["gotype"] {
						t.Fatalf("missing object metadata: %v", found)
					}
					if enableLLVM {
						for _, suffix := range []string{".ll", ".opt.ll"} {
							data, err := os.ReadFile(output + suffix)
							if err != nil {
								t.Fatal(err)
							}
							ir := string(data)
							if strings.Contains(ir, "!goobj.gotype =") {
								t.Errorf("%s still has a global gotype table", suffix)
							}
							for _, marker := range []string{`!"goobj.abi"`, `"go-nosplit"`, `!goobj.gotype !`} {
								if !strings.Contains(ir, marker) {
									t.Errorf("%s missing %s", suffix, marker)
								}
							}
						}
					}
				})
			}
		})
	}
}
