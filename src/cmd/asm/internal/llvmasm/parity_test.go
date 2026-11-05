// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"bytes"
	"cmd/internal/bio"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"fmt"
	"internal/buildcfg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func nativeGoObject(t *testing.T, goos, arch, source string) *goobj.Reader {
	t.Helper()
	return writeNativeGoObject(t, parse(t, goos, arch, source, true))
}

func writeNativeGoObject(t *testing.T, ctxt *obj.Link) *goobj.Reader {
	t.Helper()
	ctxt.NumberSyms()
	path := filepath.Join(t.TempDir(), "native.o")
	w, err := bio.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	obj.WriteObjFile(ctxt, w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return goobj.NewReaderFromBytes(data, false)
}

func auxiliary(t *testing.T, r *goobj.Reader, name string, kind uint8) uint32 {
	t.Helper()
	for _, aux := range r.Auxs(findSymbol(t, r, name)) {
		if aux.Type() == kind {
			return resolveSym(t, r, aux.Sym())
		}
	}
	t.Fatalf("%s has no auxiliary kind %d", name, kind)
	return 0
}

func TestGoObjDwarfParity(t *testing.T) {
	old := buildcfg.Experiment.Dwarf5
	defer func() { buildcfg.Experiment.Dwarf5 = old }()
	for _, dwarf5 := range []bool{false, true} {
		buildcfg.Experiment.Dwarf5 = dwarf5
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(fmt.Sprintf("%s/dwarf5=%t", arch, dwarf5), func(t *testing.T) {
				source := "TEXT probe<ABIInternal>(SB),4,$32-0\nCALL target<ABIInternal>(SB)\nRET\nTEXT wrapper<ABIInternal>(SB),36,$0\nRET\nTEXT private<>(SB),4,$0\nRET\n"
				want := nativeGoObject(t, "linux", arch, source)
				got := readObject(t, emit(t, "linux", arch, source, "obj"))
				for _, name := range []string{"probe", "wrapper", "private"} {
					for _, kind := range []uint8{goobj.AuxDwarfInfo, goobj.AuxDwarfLines} {
						w, g := auxiliary(t, want, name, kind), auxiliary(t, got, name, kind)
						if !bytes.Equal(got.Data(g), want.Data(w)) {
							t.Fatalf("%s auxiliary %d bytes differ:\ngot %x\nwant %x", name, kind, got.Data(g), want.Data(w))
						}
						if got.NReloc(g) != want.NReloc(w) {
							t.Fatalf("%s auxiliary %d relocation count differs", name, kind)
						}
						for i := 0; i < int(want.NReloc(w)); i++ {
							a, b := got.Reloc(g, i), want.Reloc(w, i)
							if a.Off() != b.Off() || a.Siz() != b.Siz() || a.Type() != b.Type() || a.Add() != b.Add() || got.Sym(resolveSym(t, got, a.Sym())).Name(got) != want.Sym(resolveSym(t, want, b.Sym())).Name(want) {
								t.Fatalf("%s auxiliary %d relocation %d differs", name, kind, i)
							}
						}
					}
				}
			})
		}
	}
}

func referenceIdentity(t *testing.T, r *goobj.Reader, ref goobj.SymRef) string {
	t.Helper()
	if ref == (goobj.SymRef{}) {
		return "<nil>"
	}
	if ref.PkgIdx == goobj.PkgIdxBuiltin {
		name, abi := goobj.BuiltinName(int(ref.SymIdx))
		return fmt.Sprintf("%s<%d>", name, abi)
	}
	sym := r.Sym(resolveSym(t, r, ref))
	return fmt.Sprintf("%s<%d>", sym.Name(r), sym.ABI())
}

func compareSymbol(t *testing.T, got, want *goobj.Reader, name string) {
	t.Helper()
	g, w := findSymbol(t, got, name), findSymbol(t, want, name)
	a, b := got.Sym(g), want.Sym(w)
	if a.ABI() != b.ABI() || a.Type() != b.Type() || a.Flag() != b.Flag() || a.Flag2() != b.Flag2() || a.Siz() != b.Siz() {
		t.Errorf("%s symbol got ABI=%d type=%d flags=%d/%d size=%d; want ABI=%d type=%d flags=%d/%d size=%d", name, a.ABI(), a.Type(), a.Flag(), a.Flag2(), a.Siz(), b.ABI(), b.Type(), b.Flag(), b.Flag2(), b.Siz())
	}
	// GoObj may omit a trailing zero-filled portion of a data symbol; LLVM
	// globals materialize it. Compare the complete logical data at equal size.
	gd, wd := bytes.TrimRight(got.Data(g), "\x00"), bytes.TrimRight(want.Data(w), "\x00")
	if !bytes.Equal(gd, wd) {
		t.Errorf("%s data differs: got %x, want %x", name, gd, wd)
	}
	if got.NReloc(g) != want.NReloc(w) {
		t.Errorf("%s relocation count differs", name)
		return
	}
	for i := 0; i < int(want.NReloc(w)); i++ {
		a, b := got.Reloc(g, i), want.Reloc(w, i)
		gs, ws := referenceIdentity(t, got, a.Sym()), referenceIdentity(t, want, b.Sym())
		if a.Off() != b.Off() || a.Siz() != b.Siz() || a.Type() != b.Type() || a.Add() != b.Add() || gs != ws {
			t.Errorf("%s reloc %d got off=%d size=%d type=%d add=%d target=%s; want off=%d size=%d type=%d add=%d target=%s", name, i, a.Off(), a.Siz(), a.Type(), a.Add(), gs, b.Off(), b.Siz(), b.Type(), b.Add(), ws)
		}
	}
}

func TestGoObjSymbolAndRelocationParity(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		for _, arch := range []string{"amd64", "arm64"} {
			for _, mode := range []string{"normal", "shared", "dynlink", "linkshared"} {
				t.Run(goos+"/"+arch+"/"+mode, func(t *testing.T) {
					body := "MOVQ data(SB),AX\nMOVQ $data+8(SB),CX\nMOVSD $1.23456,X0\nCALL target(SB)\nPCDATA $0,$-1\nPCALIGN $16\nRET\n"
					if arch == "arm64" {
						body = "MOVD data(SB),R0\nMOVD $data+8(SB),R1\nFMOVD $1.23456,F0\nCALL target(SB)\nPCDATA $0,$-1\nPCALIGN $16\nRET\n"
					}
					source := "DATA data+0(SB)/8,$7\nGLOBL data(SB),16,$16\nDATA pointer+0(SB)/8,$data+8(SB)\nGLOBL pointer(SB),8,$8\nGLOBL bss<>(SB),16,$64\nTEXT probe(SB),4,$32-16\nFUNCDATA $1,maps(SB)\nFUNCDATA $3,maps2(SB)\nPCDATA $0,$-2\n" + body + "TEXT target(SB),4,$0\nRET\nTEXT private<>(SB),4,$0\nRET\nTEXT main·main(SB),4,$0\nRET\n"
					parse := func(native bool) *obj.Link {
						return parseMode(t, goos, arch, source, native, mode == "shared", mode == "dynlink", mode == "linkshared")
					}
					want := writeNativeGoObject(t, parse(true))
					data, err := Emit(parse(false), Options{GOOS: goos, GOARCH: arch, Output: "obj"})
					if err != nil {
						t.Fatal(err)
					}
					got := readObject(t, data)
					if got.Flags() != want.Flags() {
						t.Errorf("object flags got %d want %d", got.Flags(), want.Flags())
					}
					for _, name := range []string{"probe", "target", "private", "main.main", "data", "pointer", "bss", "$f64.3ff3c0c1fc8f3238"} {
						compareSymbol(t, got, want, name)
					}
					// Files are parsed in independent temporary directories. Compare their
					// basenames and the tables' values, not the temporary directory names.
					if got.NFile() != want.NFile() {
						t.Fatalf("file count got %d want %d", got.NFile(), want.NFile())
					}
					for i := 0; i < want.NFile(); i++ {
						if filepath.Base(got.File(i)) != filepath.Base(want.File(i)) {
							t.Fatalf("file %d differs", i)
						}
					}
					for _, name := range []string{"probe", "target", "private"} {
						groups := func(r *goobj.Reader) map[uint8][]string {
							m := map[uint8][]string{}
							for _, aux := range r.Auxs(findSymbol(t, r, name)) {
								var key string
								if aux.Type() == goobj.AuxFuncdata {
									key = referenceIdentity(t, r, aux.Sym())
								} else {
									key = fmt.Sprintf("%x", r.Data(resolveSym(t, r, aux.Sym())))
								}
								m[aux.Type()] = append(m[aux.Type()], key)
							}
							return m
						}
						g, w := groups(got), groups(want)
						for kind, entries := range w {
							if strings.Join(g[kind], ";") != strings.Join(entries, ";") {
								t.Errorf("%s auxiliary %d got %v want %v", name, kind, g[kind], entries)
							}
						}
						for kind := range g {
							if _, ok := w[kind]; !ok {
								t.Errorf("%s unexpected auxiliary kind %d", name, kind)
							}
						}
					}
				})
			}
		}
	}
}

// Read the linked executable, not just the carrier, so missing DIE ownership,
// address relocations and the line table's file mapping are observable.
func checkLinkedAssemblyDWARF(t *testing.T, path, file string) {
	t.Helper()
	var data *dwarf.Data
	var err error
	if runtime.GOOS == "darwin" {
		f, e := macho.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		data, err = f.DWARF()
	} else {
		f, e := elf.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		data, err = f.DWARF()
	}
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{"main.add": false, "main.keep": false}
	reader := data.Reader()
	var lines *dwarf.LineReader
	for {
		entry, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		if entry == nil {
			break
		}
		if entry.Tag == dwarf.TagCompileUnit {
			lines, err = data.LineReader(entry)
			if err != nil {
				t.Fatal(err)
			}
		}
		if entry.Tag != dwarf.TagSubprogram || lines == nil {
			continue
		}
		name, _ := entry.Val(dwarf.AttrName).(string)
		if _, ok := found[name]; !ok {
			continue
		}
		ranges, err := data.Ranges(entry)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range ranges {
			var line dwarf.LineEntry
			if err := lines.SeekPC(r[0], &line); err != nil {
				t.Fatalf("%s line lookup: %v", name, err)
			}
			if line.File != nil && filepath.Base(line.File.Name) == file && line.Line > 0 && !line.EndSequence {
				found[name] = true
			}
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("linked DWARF has no %s subprogram with source rows in %s", name, file)
		}
	}
}

func TestGoObjUnrelocatedADRPParity(t *testing.T) {
	source := "TEXT probe<ABIInternal>(SB),516,$0\nADRP 1(PC),R0\nRET\n"
	want := nativeGoObject(t, "linux", "arm64", source)
	got := readObject(t, emit(t, "linux", "arm64", source, "obj"))
	compareSymbol(t, got, want, "probe")
}

func TestAssemblyCLIParity(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("unsupported host")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("unsupported architecture")
	}
	dir := t.TempDir()
	assembler := filepath.Join(dir, "asm")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-gcflags=all=-enablellvm=false", "-o", assembler, "cmd/asm")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build assembler: %v\n%s", err, out)
	}
	input := filepath.Join(dir, "input.s")
	body := "MOVQ $VALUE,AX\nRET\n"
	if runtime.GOARCH == "arm64" {
		body = "MOVD $VALUE,R0\nRET\n"
	}
	source := "#include \"flags.h\"\nTEXT probe(SB),NOSPLIT,$0\n" + body
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "flags.h"), []byte("#define NOSPLIT 4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"-S"}, {"-S", "-S"}, {"-S", "-S", "-S"}, {"-gensymabis"}} {
		t.Run(strings.Join(flags, "_"), func(t *testing.T) {
			var want []byte
			for _, backend := range []string{"false", "true"} {
				output := filepath.Join(dir, "output.o")
				args := []string{"-enablellvm=" + backend, "-I", dir, "-D", "VALUE=42", "-trimpath", dir, "-p", "probe", "-o", output}
				args = append(args, flags...)
				args = append(args, input)
				cmd := exec.Command(assembler, args...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("asm: %v\n%s", err, out)
				}
				if flags[0] == "-gensymabis" {
					out, err = os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
				}
				if len(out) == 0 {
					t.Fatal("empty assembler listing")
				}
				if backend == "false" {
					want = out
				} else if !bytes.Equal(out, want) {
					t.Fatalf("LLVM diagnostic output differs:\ngot:\n%s\nwant:\n%s", out, want)
				}
			}
		})
	}
}

// Exercise the serialized metadata rather than only direct TargetMachine
// emission. Renaming the actual LLVM global must retarget FUNCDATA without
// editing strings, and LTO must keep a static map used only by the carrier.
func TestGoObjStructuredMetadataRoundTrip(t *testing.T) {
	old := buildcfg.Experiment.Dwarf5
	defer func() { buildcfg.Experiment.Dwarf5 = old }()
	opt, llc := llvmTool(t, "opt"), llvmTool(t, "llc")
	for _, dwarf5 := range []bool{false, true} {
		buildcfg.Experiment.Dwarf5 = dwarf5
		for _, goos := range []string{"linux", "darwin"} {
			for _, arch := range []string{"amd64", "arm64"} {
				t.Run(fmt.Sprintf("%s/%s/dwarf5=%t", goos, arch, dwarf5), func(t *testing.T) {
					body := "CALL AX\n"
					if arch == "arm64" {
						body = "CALL (R0)\n"
					}
					source := "DATA maps<>+0(SB)/8,$7\nGLOBL maps<>(SB),8,$8\nTEXT probe<ABIInternal>(SB),4,$32-16\nFUNCDATA $1,maps<>(SB)\nPCDATA $0,$-2\n" + body + "PCDATA $0,$-1\nRET\n"
					want := nativeGoObject(t, goos, arch, source)
					dir := t.TempDir()
					input, renamed := filepath.Join(dir, "input.bc"), filepath.Join(dir, "renamed.bc")
					if err := os.WriteFile(input, emit(t, goos, arch, source, "bc"), 0600); err != nil {
						t.Fatal(err)
					}
					c := llvm.NewContext()
					defer c.Dispose()
					m, err := c.ParseBitcodeFile(input)
					if err != nil {
						t.Fatal(err)
					}
					defer m.Dispose()
					if strings.Contains(m.String(), ".goobj.") || strings.Contains(m.String(), "module asm") {
						t.Fatal("textual GoObj metadata remains")
					}
					maps := m.NamedGlobal("maps")
					if maps.IsNil() {
						t.Fatal("missing static FUNCDATA map")
					}
					maps.SetName("renamed_maps")
					bc := llvm.WriteBitcodeToMemoryBuffer(m)
					defer bc.Dispose()
					if err := os.WriteFile(renamed, bc.Bytes(), 0600); err != nil {
						t.Fatal(err)
					}
					for _, optimize := range []bool{false, true} {
						codegen := renamed
						if optimize {
							codegen = filepath.Join(dir, "optimized.bc")
							if out, err := exec.Command(opt, "-passes=lto-pre-link<O2>,lto<O2>", renamed, "-o", codegen).CombinedOutput(); err != nil {
								t.Fatalf("LTO: %v\n%s", err, out)
							}
						}
						object := filepath.Join(dir, "output.o")
						if out, err := exec.Command(llc, "-O0", "-filetype=obj", codegen, "-o", object).CombinedOutput(); err != nil {
							t.Fatalf("codegen: %v\n%s", err, out)
						}
						data, err := os.ReadFile(object)
						if err != nil {
							t.Fatal(err)
						}
						got := readObject(t, data)
						compareSymbol(t, got, want, "probe")
						for _, kind := range []uint8{goobj.AuxFuncInfo, goobj.AuxPcsp, goobj.AuxPcfile, goobj.AuxPcline, goobj.AuxPcinline, goobj.AuxPcdata, goobj.AuxDwarfInfo, goobj.AuxDwarfLines} {
							g, w := auxiliary(t, got, "probe", kind), auxiliary(t, want, "probe", kind)
							if !bytes.Equal(got.Data(g), want.Data(w)) {
								t.Fatalf("LTO=%t auxiliary %d changed: got %x want %x", optimize, kind, got.Data(g), want.Data(w))
							}
							if got.NReloc(g) != want.NReloc(w) {
								t.Fatalf("auxiliary %d relocation count differs", kind)
							}
							for n := 0; n < int(want.NReloc(w)); n++ {
								a, b := got.Reloc(g, n), want.Reloc(w, n)
								if a.Off() != b.Off() || a.Siz() != b.Siz() || a.Type() != b.Type() || a.Add() != b.Add() || referenceIdentity(t, got, a.Sym()) != referenceIdentity(t, want, b.Sym()) {
									t.Fatalf("auxiliary %d relocation changed", kind)
								}
							}
						}
						var slots []goobj.SymRef
						for _, aux := range got.Auxs(findSymbol(t, got, "probe")) {
							if aux.Type() == goobj.AuxFuncdata {
								slots = append(slots, aux.Sym())
							}
						}
						if len(slots) != 2 || slots[0] != (goobj.SymRef{}) || got.Sym(resolveSym(t, got, slots[1])).Name(got) != "renamed_maps" {
							t.Fatal("sparse or renamed FUNCDATA changed")
						}
						mapData := got.Data(findSymbol(t, got, "renamed_maps"))
						if len(mapData) != 8 || mapData[0] != 7 {
							t.Fatal("FUNCDATA-only definition discarded")
						}
					}
				})
			}
		}
	}
}
