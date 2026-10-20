// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"bytes"
	"cmd/internal/archive"
	"cmd/internal/goobj"
	"cmd/internal/objabi"
	"fmt"
	"internal/testenv"
	"os"
	"path/filepath"
	"testing"
)

// Compare the object contract with native Go, not only the input LLVM IR.
// -dwarf=false still needs minimal function DIEs and line tables, but must
// preserve runtime PC tables without emitting local-variable or inline DIEs.
func TestLLVMDwarfObject(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	source := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(source, []byte(`package p
var Global int
func Add(x int) int { return x + Global }
func Caller(x int) int { return Add(x) }
`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		for _, llvm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/llvm=%t", arch, llvm), func(t *testing.T) {
				var full map[string][]byte
				for _, dwarf := range []bool{true, false} {
					output := filepath.Join(t.TempDir(), "p.a")
					cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile",
						fmt.Sprintf("-enablellvm=%t", llvm), fmt.Sprintf("-dwarf=%t", dwarf),
						"-p=p", "-o", output, source)
					cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("compile: %v\n%s", err, out)
					}
					r := readDwarfTestObject(t, output)
					counts := make(map[objabi.SymKind]int)
					runtimeData := make(map[string][]byte)
					for i := uint32(0); i < uint32(r.NSym()+r.NHashed64def()+r.NHasheddef()+r.NNonpkgdef()); i++ {
						s := r.Sym(i)
						kind := objabi.SymKind(s.Type())
						counts[kind]++
						if kind != objabi.STEXT {
							continue
						}
						name := s.Name(r)
						runtimeData[name+"/text"] = r.Data(i)
						auxTypes := make(map[uint8]int)
						for _, a := range r.Auxs(i) {
							n := auxTypes[a.Type()]
							auxTypes[a.Type()]++
							switch a.Type() {
							case goobj.AuxPcsp, goobj.AuxPcfile, goobj.AuxPcline, goobj.AuxPcinline, goobj.AuxPcdata:
								runtimeData[fmt.Sprintf("%s/%d/%d", name, a.Type(), n)] = dwarfTestAuxData(t, r, a.Sym())
							}
						}
						for _, typ := range []uint8{goobj.AuxDwarfInfo, goobj.AuxDwarfLines, goobj.AuxPcfile, goobj.AuxPcline} {
							if auxTypes[typ] != 1 {
								t.Errorf("dwarf=%t %s: aux type %d count = %d, want 1", dwarf, name, typ, auxTypes[typ])
							}
						}
						if name == "p.Caller" && auxTypes[goobj.AuxPcinline] != 1 {
							t.Errorf("dwarf=%t: missing runtime inline table", dwarf)
						}
					}
					for _, kind := range []objabi.SymKind{objabi.SDWARFFCN, objabi.SDWARFLINES} {
						if counts[kind] != 2 {
							t.Errorf("dwarf=%t: %s count = %d, want 2", dwarf, kind, counts[kind])
						}
					}
					if counts[objabi.SDWARFVAR] != 1 {
						t.Errorf("dwarf=%t: global-variable DIE count = %d, want 1", dwarf, counts[objabi.SDWARFVAR])
					}
					if got := counts[objabi.SDWARFABSFCN] != 0; got != dwarf {
						t.Errorf("dwarf=%t: abstract inline DIE present = %t", dwarf, got)
					}
					if dwarf {
						full = runtimeData
						continue
					}
					if len(runtimeData) != len(full) {
						t.Errorf("runtime data entries = %d, want %d", len(runtimeData), len(full))
					}
					for key, want := range full {
						if !bytes.Equal(runtimeData[key], want) {
							t.Errorf("-dwarf=false changed runtime data %s", key)
						}
					}
				}
			})
		}
	}
}

func TestLLVMDwarfLink(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveExec(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	code := `package main
import "runtime"
func capture(pcs []uintptr) int { return runtime.Callers(1, pcs) }
func main() {
    var pcs [8]uintptr
    frames := runtime.CallersFrames(pcs[:capture(pcs[:])])
    frame, _ := frames.Next()
    if frame.Function != "main.capture" || frame.File == "" || frame.Line <= 0 {
        panic("lost runtime source or inline frame with -dwarf=false")
    }
}
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, llvm := range []bool{false, true} {
		t.Run(fmt.Sprintf("llvm=%t", llvm), func(t *testing.T) {
			exe := filepath.Join(dir, fmt.Sprintf("main-%t.exe", llvm))
			cmd := testenv.Command(t, testenv.GoToolPath(t), "build",
				fmt.Sprintf("-gcflags=-enablellvm=%t -dwarf=false", llvm),
				"-ldflags=-w=false", "-o", exe, source)
			cmd.Env = append(cmd.Environ(), "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, out)
			}
			if out, err := testenv.Command(t, exe).CombinedOutput(); err != nil {
				t.Fatalf("run: %v\n%s", err, out)
			}
		})
	}
}

func readDwarfTestObject(t *testing.T, path string) *goobj.Reader {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	a, err := archive.Parse(f, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range a.Entries {
		if entry.Type != archive.EntryGoObj {
			continue
		}
		data := make([]byte, entry.Obj.Size)
		if _, err := f.ReadAt(data, entry.Obj.Offset); err != nil {
			t.Fatal(err)
		}
		r := goobj.NewReaderFromBytes(data, true)
		if r == nil {
			t.Fatal("invalid Go object")
		}
		return r
	}
	t.Fatal("archive contains no Go object")
	return nil
}

func dwarfTestAuxData(t *testing.T, r *goobj.Reader, ref goobj.SymRef) []byte {
	t.Helper()
	index := ref.SymIdx
	switch ref.PkgIdx {
	case goobj.PkgIdxSelf:
	case goobj.PkgIdxHashed64:
		index += uint32(r.NSym())
	case goobj.PkgIdxHashed:
		index += uint32(r.NSym() + r.NHashed64def())
	case goobj.PkgIdxNone:
		index += uint32(r.NSym() + r.NHashed64def() + r.NHasheddef())
	default:
		t.Fatalf("unexpected PC table reference %+v", ref)
	}
	return r.Data(index)
}
