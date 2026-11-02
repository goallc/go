// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmlto

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestThinLTODebugPackages(t *testing.T) {
	goTool := thinLTOToolchain(t)
	output := filepath.Join(t.TempDir(), "program")
	cmd := exec.Command(goTool, "build", "-lto=thin", "-ldflags=-compressdwarf=false", "-o", output, ".")
	cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/runtime")
	cmd.Env = thinLTOEnv()
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	var data *dwarf.Data
	var err error
	if runtime.GOOS == "darwin" {
		f, openErr := macho.Open(output)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer f.Close()
		data, err = f.DWARF()
	} else {
		f, openErr := elf.Open(output)
		if openErr != nil {
			t.Fatal(openErr)
		}
		defer f.Close()
		data, err = f.DWARF()
	}
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"main": "main", "runtime": "runtime", "example.com/lto/dep": "dep"}
	reader := data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		if entry == nil {
			break
		}
		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}
		path, _ := entry.Val(dwarf.AttrName).(string)
		if name, ok := want[path]; ok {
			const attrGoPackageName = dwarf.Attr(0x2905)
			if got := entry.Val(attrGoPackageName); got != name {
				t.Fatalf("CU %s package name = %v, want %s", path, got, name)
			}
			delete(want, path)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing logical compilation units: %v", want)
	}
}

func thinLTOToolchain(t *testing.T) string {
	t.Helper()
	if os.Getenv("GOALLC_TEST_LTO") != "1" {
		t.Skip("set GOALLC_TEST_LTO=1 with an LLVM-enabled toolchain")
	}
	if (runtime.GOOS != "darwin" && runtime.GOOS != "linux") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Skip("LTO runtime fixture supports darwin/linux on amd64/arm64")
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}

func thinLTOEnv() []string {
	return append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
}

func TestThinLTOFIPS(t *testing.T) {
	goTool := thinLTOToolchain(t)
	for _, mode := range []string{"exe", "pie"} {
		t.Run(mode, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "fips.test")
			cmd := exec.Command(goTool, "test", "-c", "-lto=thin", "-buildmode="+mode, "-o", output, "crypto/internal/fips140test")
			cmd.Env = thinLTOEnv()
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("FIPS build: %v\n%s", err, data)
			}
			// Check the intact binary, rejection of a modified checksum, and
			// the placement of FIPS and non-FIPS symbols in the final image.
			cmd = exec.Command(output, "-test.run=^TestIntegrityCheck", "-test.v", "-test.count=1")
			cmd.Env = append(thinLTOEnv(), "GODEBUG=fips140=on")
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("FIPS integrity: %v\n%s", err, data)
			}
		})
	}
}

// Both successful standard-library references and forbidden main-package
// pulls must retain their original package identity across ThinLTO imports.
func TestThinLTOLinkname(t *testing.T) {
	goTool := thinLTOToolchain(t)
	for _, tc := range []struct {
		source  string
		allowed bool
	}{
		{"ok.go", true}, {"textvar", true}, {"fastrand.go", true},
		{"coro.go", false}, {"coro_var_unused.go", false}, {"builtin.go", false},
	} {
		for _, flags := range []string{"", "-w"} {
			t.Run(tc.source+"/"+flags, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "program")
				source := filepath.Join(runtime.GOROOT(), "src/cmd/link/testdata/linkname", tc.source)
				cmd := exec.Command(goTool, "build", "-lto=thin", "-ldflags="+flags, "-o", output, source)
				cmd.Env = thinLTOEnv()
				data, err := cmd.CombinedOutput()
				if tc.allowed {
					if err != nil {
						t.Fatalf("valid linkname rejected: %v\n%s", err, data)
					}
					cmd = exec.Command(output)
					cmd.Env = append(thinLTOEnv(), "GOGC=1")
					if data, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("runtime: %v\n%s", err, data)
					}
				} else if err == nil || !strings.Contains(string(data), "invalid reference to") {
					t.Fatalf("want linkname rejection, got %v\n%s", err, data)
				}
			})
		}
	}
}

// Native dep objects and assembly retain their original references to the
// ThinLTO leaf package's symbol indices.
func TestThinLTOMixedArchive(t *testing.T) {
	goTool := thinLTOToolchain(t)
	output := filepath.Join(t.TempDir(), "program")
	cmd := exec.Command(goTool, "build", "-lto=thin", "-gcflags=example.com/lto/dep=-enablellvm=false", "-o", output, ".")
	cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/runtime")
	cmd.Env = thinLTOEnv()
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	cmd = exec.Command(output)
	cmd.Env = append(thinLTOEnv(), "GOGC=1")
	if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "LTO RUNTIME OK") {
		t.Fatalf("runtime: %v\n%s", err, data)
	}
}

func TestLTOPIE(t *testing.T) {
	goTool := thinLTOToolchain(t)
	output := filepath.Join(t.TempDir(), "program")
	cmd := exec.Command(goTool, "build", "-lto=thin", "-buildmode=pie", "-o", output, ".")
	cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/runtime")
	cmd.Env = thinLTOEnv()
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PIE build: %v\n%s", err, data)
	}
	cmd = exec.Command(output)
	cmd.Env = append(thinLTOEnv(), "GOGC=1")
	if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "LTO RUNTIME OK") {
		t.Fatalf("PIE runtime: %v\n%s", err, data)
	}
}

func TestLTOCgo(t *testing.T) {
	goTool := thinLTOToolchain(t)
	testenv.MustHaveCGO(t)
	for _, mode := range []string{"auto", "external"} {
		t.Run(mode, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "program")
			cmd := exec.Command(goTool, "build", "-lto=thin", "-ldflags=-linkmode="+mode, "-o", output, ".")
			cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/cgo")
			cmd.Env = append(thinLTOEnv(), "CGO_ENABLED=1")
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("cgo build: %v\n%s", err, data)
			}
			cmd = exec.Command(output)
			cmd.Env = append(thinLTOEnv(), "GOGC=1")
			if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "LTO CGO OK") {
				t.Fatalf("cgo runtime: %v\n%s", err, data)
			}
		})
	}
}

// A package-indexed zero definition can be replaced by an initialized
// linkname definition, as with runtime._cgo_clearenv on Linux. Ordinary Go
// objects must still resolve the declaring package's original symbol index.
func TestLTOIndexedBSS(t *testing.T) {
	goTool := thinLTOToolchain(t)
	for _, mode := range []string{"ordinary", "thin", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "program")
			args := []string{"build", "-o", output}
			if mode == "ordinary" {
				args = append(args, "-gcflags=all=-enablellvm=false")
			} else {
				args = append(args, "-lto=thin")
				if mode == "mixed" {
					args = append(args, "-gcflags=example.com/lto-bss/observer=-enablellvm=false")
				}
			}
			cmd := exec.Command(goTool, append(args, ".")...)
			cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/bss")
			cmd.Env = thinLTOEnv()
			if data, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, data)
			}
			cmd = exec.Command(output)
			if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "LTO INDEXED BSS OK") {
				t.Fatalf("runtime: %v\n%s", err, data)
			}
		})
	}
}

func TestThinLTORuntime(t *testing.T) {
	goTool := thinLTOToolchain(t)
	output := filepath.Join(t.TempDir(), "program")
	cmd := exec.Command(goTool, "build", "-lto=thin", "-gcflags=example.com/lto/...=-l", "-o", output, ".")
	cmd.Dir = filepath.Join(runtime.GOROOT(), "src/cmd/internal/llvmlto/testdata/runtime")
	cmd.Env = thinLTOEnv()
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	cmd = exec.Command(output)
	cmd.Env = append(thinLTOEnv(), "GOGC=1")
	if data, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(data), "LTO RUNTIME OK") {
		t.Fatalf("runtime: %v\n%s", err, data)
	}
}
