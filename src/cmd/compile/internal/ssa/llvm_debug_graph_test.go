// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"bytes"
	"cmd/internal/objabi"
	"fmt"
	"internal/testenv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLVMNativeDebugVariables(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	source := filepath.Join(t.TempDir(), "p.go")
	const code = `package p
type Named [2]int
//go:noinline
func Arrays(x Named, y [2]int) (result Named) { result = Named(y); result[0] += x[0]; return }
//go:noinline
func Generic[T ~[2]int](x T) (result T) { local := x; return local }
var Instantiated = Generic[Named]
func InlineResult(x int) (result int) { return x + 1 }
func Caller(x int) int { return InlineResult(x) + InlineResult(x+1) }
`
	if err := os.WriteFile(source, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		for _, mode := range []string{"optimized", "noopt", "no-inline-dies"} {
			t.Run(arch+"/"+mode, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "p.a")
				args := []string{"tool", "compile", "-enablellvm", "-p=p", "-llvm-keep-ir", "-o", output}
				if mode == "noopt" {
					args = append(args, "-N", "-l")
				} else if mode == "no-inline-dies" {
					args = append(args, "-gendwarfinl=0")
				}
				cmd := testenv.Command(t, testenv.GoToolPath(t), append(args, source)...)
				cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("compile: %v\n%s", err, out)
				}
				for _, suffix := range []string{".ll", ".opt.ll"} {
					data, err := os.ReadFile(output + suffix)
					if err != nil {
						t.Fatal(err)
					}
					ir := string(data)
					if strings.Contains(ir, "!goobj.debug.vars") {
						t.Errorf("%s: separate variable table remains", suffix)
					}
					for _, want := range []string{`DW_TAG_array_type, name: "p.Named"`, `DW_TAG_array_type, name: "[2]int"`, `!"go.return", !"1"`, `!"go.dictindex"`} {
						if !strings.Contains(ir, want) {
							t.Errorf("%s: missing %s", suffix, want)
						}
					}
				}
				r := readDwarfTestObject(t, output)
				foundReturn := false
				for i := uint32(0); i < uint32(r.NSym()+r.NHashed64def()+r.NHasheddef()+r.NNonpkgdef()); i++ {
					if objabi.SymKind(r.Sym(i).Type()) == objabi.SDWARFFCN && bytes.Contains(r.Data(i), []byte("result\x00")) {
						foundReturn = true
					}
				}
				if !foundReturn {
					t.Error("object lost named return variables")
				}
			})
		}
	}
}

func TestLLVMNativeDebugPackage(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	for _, arch := range []string{"amd64", "arm64"} {
		for _, dwarf := range []bool{false, true} {
			for _, empty := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/dwarf=%t/empty=%t", arch, dwarf, empty), func(t *testing.T) {
					dir := t.TempDir()
					source := filepath.Join(dir, "p.go")
					code := "package sourcepkg\n"
					if !empty {
						code += "var Global int\nfunc F(x int) int { return x + Global }\n"
					}
					if err := os.WriteFile(source, []byte(code), 0600); err != nil {
						t.Fatal(err)
					}
					const pkgPath = "example.com/with.dot"
					output := filepath.Join(dir, "p.a")
					cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile", "-enablellvm", "-p="+pkgPath, fmt.Sprintf("-dwarf=%t", dwarf), "-llvm-keep-ir", "-o", output, source)
					cmd.Env = append(cmd.Environ(), "GOOS=linux", "GOARCH="+arch)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("compile: %v\n%s", err, out)
					}
					for _, suffix := range []string{".ll", ".opt.ll"} {
						data, err := os.ReadFile(output + suffix)
						if err != nil {
							t.Fatal(err)
						}
						if bytes.Contains(data, []byte("!goobj.debug.config")) || !bytes.Contains(data, []byte(`!DIModule(scope: null, name: "sourcepkg"`)) {
							t.Errorf("%s: missing native package module or obsolete config remains", suffix)
						}
					}
					r := readDwarfTestObject(t, output)
					count := 0
					for i := uint32(0); i < uint32(r.NSym()+r.NHashed64def()+r.NHasheddef()+r.NNonpkgdef()); i++ {
						if r.Sym(i).Name(r) == "go:cuinfo.packagename."+objabi.PathToPrefix(pkgPath) {
							count++
							if string(r.Data(i)) != "sourcepkg" {
								t.Errorf("package name = %q", r.Data(i))
							}
						}
					}
					if count != 1 {
						t.Errorf("package-name carrier count = %d, want 1", count)
					}
				})
			}
		}
	}
}
