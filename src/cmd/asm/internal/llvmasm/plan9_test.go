// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

//go:build !compiler_bootstrap

package llvmasm

import (
	"bytes"
	"cmd/asm/internal/arch"
	"cmd/asm/internal/asm"
	"cmd/asm/internal/lex"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"internal/buildcfg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func plan9Input(t *testing.T, goos, goarch, source string) (*obj.Link, Options) {
	t.Helper()
	oldOS, oldArch := buildcfg.GOOS, buildcfg.GOARCH
	buildcfg.GOOS, buildcfg.GOARCH = goos, goarch
	defer func() { buildcfg.GOOS, buildcfg.GOARCH = oldOS, oldArch }()
	a := arch.Set(goarch, false)
	ctxt := obj.Linknew(a.LinkArch)
	ctxt.Pkgpath, ctxt.IsAsm = "runtime", true
	ctxt.DiagFunc = func(format string, args ...any) { t.Errorf(format, args...) }
	a.Init(ctxt)
	p := asm.NewParser(ctxt, a, lex.NewTokenizer("carrier.s", strings.NewReader(source), nil))
	p.CapturePlan9 = true
	first, ok := p.Parse()
	if !ok {
		t.Fatal("parse failed")
	}
	obj.PrepareText(ctxt, &obj.Plist{Firstpc: first}, nil)
	return ctxt, Options{GOOS: goos, GOARCH: goarch, Output: "obj", Plan9: p.Plan9}
}

func TestPlan9BitcodeRoundTrip(t *testing.T) {
	for _, ar := range []string{"amd64", "arm64"} {
		t.Run(ar, func(t *testing.T) {
			body := "MOVQ data(SB), AX\nRET\n"
			if ar == "arm64" {
				body = "MOVD data(SB), R0\nRET\n"
			}
			source := "DATA data+0(SB)/8,$42\nGLOBL data(SB),8,$8\nTEXT answer<ABIInternal>(SB),4,$0\n" + body
			ctxt, opts := plan9Input(t, "linux", ar, source)
			opts.Output = "bc"
			bitcode, err := Emit(ctxt, opts)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "carrier.bc")
			if err = os.WriteFile(path, bitcode, 0600); err != nil {
				t.Fatal(err)
			}
			// A saved cross-target carrier must not inherit the materializer's
			// platform-default experiments (for example Darwin vs Linux DWARF5).
			oldExperiments := buildcfg.Experiment
			buildcfg.Experiment.Dwarf5 = !buildcfg.Experiment.Dwarf5
			data, err := EmitPlan9File(path)
			buildcfg.Experiment = oldExperiments
			if err != nil {
				t.Fatal(err)
			}
			native := parse(t, "linux", ar, source, true).Text[0]
			r := readObject(t, data)
			if !bytes.Equal(r.Data(findSymbol(t, r, "answer")), native.P) {
				t.Fatal("bitcode round trip changed code")
			}

			// Symbol renaming must affect the real operand, even though source spelling
			// remains unchanged in the carrier's Plan 9 body and original-name table.
			context := llvm.NewContext()
			defer context.Dispose()
			module, err := context.ParseBitcodeFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer module.Dispose()
			module.NamedGlobal("data").SetName("renamed_data")
			data, err = materializePlan9Module(module)
			if err != nil {
				t.Fatal(err)
			}
			r = readObject(t, data)
			idx := findSymbol(t, r, "answer")
			if got := r.Sym(resolveSym(t, r, r.Reloc(idx, 0).Sym())).Name(r); got != "renamed_data" {
				t.Fatalf("renamed reference targets %s", got)
			}

			optimized := filepath.Join(t.TempDir(), "optimized.bc")
			opt := llvmTool(t, "opt")
			if out, err := exec.Command(opt, "-passes=lto-pre-link<O2>,lto<O2>", path, "-o", optimized).CombinedOutput(); err != nil {
				t.Fatalf("opt: %v\n%s", err, out)
			}
			data, err = EmitPlan9File(optimized)
			if err != nil {
				t.Fatal(err)
			}
			r = readObject(t, data)
			if !bytes.Equal(r.Data(findSymbol(t, r, "answer")), native.P) {
				t.Fatal("LTO pipeline changed carrier code")
			}

			opts.Output = "ir"
			ir, err := Emit(ctxt, opts)
			if err != nil {
				t.Fatal(err)
			}
			ir = append(ir, []byte("\ndefine goabiinternal i64 @caller() \"go-nosplit\" \"frame-pointer\"=\"all\" {\n %v = call goabiinternal i64 @answer()\n %r = add i64 %v, 1\n ret i64 %r\n}\n")...)
			inputIR := filepath.Join(t.TempDir(), "caller.ll")
			if err = os.WriteFile(inputIR, ir, 0600); err != nil {
				t.Fatal(err)
			}
			outputIR := filepath.Join(t.TempDir(), "caller-opt.ll")
			if out, err := exec.Command(opt, "-S", "-passes=lto-pre-link<O2>,lto<O2>", inputIR, "-o", outputIR).CombinedOutput(); err != nil {
				t.Fatalf("opt caller: %v\n%s", err, out)
			}
			optimizedIR, err := os.ReadFile(outputIR)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(optimizedIR, []byte("ret i64")) {
				t.Fatal("LTO lost the assembly return contract")
			}
			data, err = EmitPlan9File(outputIR)
			if err != nil {
				t.Fatal(err)
			}
			r = readObject(t, data)
			findSymbol(t, r, "caller")
		})
	}
}

func TestPlan9Carrier(t *testing.T) {
	for _, ar := range []string{"amd64", "arm64"} {
		t.Run(ar, func(t *testing.T) {
			body := "MOVQ data(SB), AX\nCMPQ AX, $0\nJEQ done\nCALL target(SB)\ndone:\nPCDATA $0,$-1\nRET\n"
			if ar == "arm64" {
				body = "MOVD data(SB), R0\nCBZ R0, done\nCALL target(SB)\ndone:\nPCDATA $0,$-1\nRET\n"
			}
			source := "DATA data+0(SB)/8,$7\nGLOBL data(SB),8,$8\nGLOBL unrelated<>(SB),8,$8\nGLOBL maps(SB),8,$8\nTEXT test<ABIInternal>(SB),4,$32-0\nFUNCDATA $1,maps(SB)\nPCDATA $0,$-2\n" + body
			ctxt, opts := plan9Input(t, "linux", ar, source)
			opts.Output = "ir"
			ir, err := Emit(ctxt, opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(ctxt.Text[0].P) != 0 {
				t.Fatal("IR emission encoded the function early")
			}
			for _, want := range []string{"go-plan9-context", "asm sideeffect", "ptr @data", `@"target<ABI0>"`} {
				if !bytes.Contains(ir, []byte(want)) {
					t.Fatalf("IR missing %s\n%s", want, ir)
				}
			}
			f, err := lowerPlan9(ctxt, opts)
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"TEXT", "DATA", "GLOBL", "PCDATA", "FUNCDATA", ".goobj", ".byte"} {
				if strings.Contains(f[0].body, bad) {
					t.Fatalf("inline body contains %s: %s", bad, f[0].body)
				}
			}
			for _, ref := range f[0].refs {
				if ref.sym.Name == "unrelated" {
					t.Fatal("carrier retains an unrelated global")
				}
			}
			opts.Output = "obj"
			data, err := Emit(ctxt, opts)
			if err != nil {
				t.Fatal(err)
			}
			r := readObject(t, data)
			idx := findSymbol(t, r, "test")
			native := parse(t, "linux", ar, source, true).Text[0]
			if !bytes.Equal(r.Data(idx), native.P) {
				t.Fatalf("bytes differ:\nLLVM %x\nGo   %x", r.Data(idx), native.P)
			}
			if int(r.NReloc(idx)) != len(native.R) {
				t.Fatalf("relocation count %d want %d", r.NReloc(idx), len(native.R))
			}
			for i, want := range native.R {
				got := r.Reloc(idx, i)
				if got.Off() != want.Off || got.Siz() != want.Siz || got.Type() != uint16(want.Type) || got.Add() != want.Add {
					t.Fatalf("relocation %d differs: got %v want %v", i, got, want)
				}
				if want.Sym != nil && r.Sym(resolveSym(t, r, got.Sym())).Name(r) != want.Sym.Name {
					t.Fatalf("relocation target changed")
				}
			}
			quantum := uint64(1)
			if ar == "arm64" {
				quantum = 4
			}
			foundSP, foundPC := false, false
			for _, aux := range r.Auxs(idx) {
				if aux.Type() == goobj.AuxPcsp {
					foundSP = true
					table := r.Data(resolveSym(t, r, aux.Sym()))
					for pc := uint64(0); pc < uint64(native.Size); pc += quantum {
						if got, want := pcValue(t, table, pc, quantum), pcValue(t, native.Func().Pcln.Pcsp.P, pc, quantum); got != want {
							t.Fatalf("PCSP at %d: %d want %d", pc, got, want)
						}
					}
				}
				if aux.Type() == goobj.AuxPcdata {
					foundPC = true
					table := r.Data(resolveSym(t, r, aux.Sym()))
					for pc := uint64(0); pc < uint64(native.Size); pc += quantum {
						if got, want := pcValue(t, table, pc, quantum), pcValue(t, native.Func().Pcln.Pcdata[0].P, pc, quantum); got != want {
							t.Fatalf("PCDATA at %d: %d want %d", pc, got, want)
						}
					}
				}
			}
			if !foundSP || !foundPC {
				t.Fatal("missing PC metadata")
			}
		})
	}
}

func TestPlan9EncoderCoverage(t *testing.T) {
	for _, test := range []struct{ arch, body string }{
		{"amd64", "ADCXQ DX, DX\nVADDPD Z13, Z11, K3, Z14\n"},
		{"arm64", "CASALD R5, (R6), R7\nSHA512H V2.D2, V1, V0\n"},
	} {
		t.Run(test.arch, func(t *testing.T) {
			source := "TEXT encoded<ABIInternal>(SB),4,$0\n" + test.body + "RET\n"
			ctxt, options := plan9Input(t, "linux", test.arch, source)
			data, err := Emit(ctxt, options)
			if err != nil {
				t.Fatal(err)
			}
			r := readObject(t, data)
			native := parse(t, "linux", test.arch, source, true).Text[0]
			if !bytes.Equal(r.Data(findSymbol(t, r, "encoded")), native.P) {
				t.Fatal("target encoding changed")
			}
		})
	}
}

func TestPlan9StackSlotNamesAreNotGlobals(t *testing.T) {
	source := "TEXT result(SB),4,$0-8\nMOVQ $42, AX\nMOVQ AX, ret+0(FP)\nRET\n"
	ctxt, options := plan9Input(t, "linux", "amd64", source)
	options.Output = "ir"
	data, err := Emit(ctxt, options)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("@ret")) {
		t.Fatal("FP slot name became an LLVM global")
	}
	options.Output = "obj"
	if _, err = Emit(ctxt, options); err != nil {
		t.Fatal(err)
	}
}
