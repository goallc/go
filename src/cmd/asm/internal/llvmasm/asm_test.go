// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"bytes"
	"cmd/asm/internal/arch"
	"cmd/asm/internal/asm"
	"cmd/asm/internal/lex"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"encoding/binary"
	"internal/buildcfg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func parse(t *testing.T, goos, goarch, source string, native bool) *obj.Link {
	t.Helper()
	oldOS, oldArch := buildcfg.GOOS, buildcfg.GOARCH
	buildcfg.GOOS, buildcfg.GOARCH = goos, goarch
	defer func() { buildcfg.GOOS, buildcfg.GOARCH = oldOS, oldArch }()
	architecture := arch.Set(goarch, false)
	ctxt := obj.Linknew(architecture.LinkArch)
	ctxt.Pkgpath = "runtime"
	ctxt.IsAsm = true
	ctxt.DiagFunc = func(format string, args ...any) { t.Errorf(format, args...) }
	architecture.Init(ctxt)
	path := filepath.Join(t.TempDir(), "input.s")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	parser := asm.NewParser(ctxt, architecture, lex.NewLexer(path))
	first, ok := parser.Parse()
	if !ok {
		t.Fatal("Plan 9 assembly did not parse")
	}
	plist := &obj.Plist{Firstpc: first}
	if native {
		obj.Flushplist(ctxt, plist, nil)
	} else {
		obj.PrepareText(ctxt, plist, nil)
	}
	if ctxt.Errors != 0 {
		t.Fatal("Plan 9 preprocessing failed")
	}
	return ctxt
}

func emit(t *testing.T, goos, goarch, source, output string) []byte {
	t.Helper()
	ctxt := parse(t, goos, goarch, source, false)
	for _, fn := range ctxt.Text {
		if len(fn.P) != 0 {
			t.Fatal("LLVM path already encoded native instructions")
		}
	}
	data, err := Emit(ctxt, Options{GOOS: goos, GOARCH: goarch, Output: output})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNativeInstructionParity(t *testing.T) {
	tests := []struct{ name, arch, body string }{
		{"amd64_integer", "amd64", "MOVQ AX, BX\nADDQ BX, AX\nCMPQ AX, BX\nMOVBQSX BL, CX\nRET"},
		{"amd64_stack", "amd64", "MOVQ a+0(FP), AX\nADDQ b+8(FP), AX\nMOVQ AX, ret+16(FP)\nRET"},
		{"arm64_integer", "arm64", "ADD R1, R0, R0\nSUB $1, R0, R0\nMOVW R0, R1\nCMP R1, R0\nRET"},
		{"arm64_stack", "arm64", "MOVD a+0(FP), R0\nMOVD R0, ret+16(FP)\nRET"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "TEXT test<ABIInternal>(SB),4,$0-24\n" + test.body + "\n"
			native := parse(t, "linux", test.arch, source, true)
			data := emit(t, "linux", test.arch, source, "obj")
			r := readObject(t, data)
			idx := findSymbol(t, r, "test")
			text := r.Data(idx)
			nativeCode := native.Text[0].P
			// The Go arm64 encoder pads every function to 16 bytes.
			if test.arch == "arm64" {
				last := native.Text[0].Func().Text
				for last.Link != nil {
					last = last.Link
				}
				nativeCode = nativeCode[:last.Pc+4]
			}
			if !bytes.Equal(text, nativeCode) {
				t.Fatalf("LLVM code %x, Go assembler code %x", text, native.Text[0].P)
			}
		})
	}
}

func TestSymbolicReferencesAndData(t *testing.T) {
	for _, goarch := range []string{"amd64", "arm64"} {
		t.Run(goarch, func(t *testing.T) {
			body := "MOVQ data(SB), AX\nCALL target(SB)\nRET\n"
			if goarch == "arm64" {
				body = "MOVD data(SB), R0\nCALL target(SB)\nRET\n"
			}
			source := "DATA data+0(SB)/8,$7\nGLOBL data(SB),8,$8\nTEXT test<ABIInternal>(SB),4,$0-0\n" + body
			ir := string(emit(t, "linux", goarch, source, "ir"))
			for _, want := range []string{"naked noinline", "asm sideeffect", "ptr @data", `@"target<ABI0>"`, "unreachable"} {
				if !strings.Contains(ir, want) {
					t.Errorf("IR missing %q", want)
				}
			}
			data := emit(t, "linux", goarch, source, "obj")
			r := readObject(t, data)
			idx := findSymbol(t, r, "test")
			if r.NReloc(idx) < 2 {
				t.Fatal("symbolic data/call references lost")
			}
			target := findSymbol(t, r, "target")
			if r.Sym(target).ABI() != 0 {
				t.Fatal("ABI0 call reference was lost")
			}
			global := findSymbol(t, r, "data")
			if binary.LittleEndian.Uint64(r.Data(global)) != 7 {
				t.Fatal("DATA initializer lost")
			}
		})
	}
}

func TestBranchAndPCData(t *testing.T) {
	for _, goarch := range []string{"amd64", "arm64"} {
		t.Run(goarch, func(t *testing.T) {
			body := "CMPQ AX, $0\nJEQ done\nADDQ $1, AX\ndone:\nRET\n"
			if goarch == "arm64" {
				body = "CBZ R0, done\nADD $1, R0, R0\ndone:\nRET\n"
			}
			source := "TEXT test<ABIInternal>(SB),4,$0\nPCDATA $0,$-2\n" + body
			data := emit(t, "linux", goarch, source, "obj")
			r := readObject(t, data)
			idx := findSymbol(t, r, "test")
			found := false
			for _, aux := range r.Auxs(idx) {
				if aux.Type() == goobj.AuxPcdata {
					table := r.Data(resolveSym(t, r, aux.Sym()))
					v, n := binary.Varint(table)
					if n <= 0 || v != -1 {
						t.Fatalf("PCDATA unsafe point starts with %d, want -2", v-1)
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatal("PCDATA was lost")
			}
		})
	}
}

func TestRejectUnsupportedOperand(t *testing.T) {
	ctxt := parse(t, "linux", "arm64", "TEXT test<ABIInternal>(SB),4,$0\nVADD V0.B16,V1.B16,V2.B16\nRET\n", false)
	if _, err := Emit(ctxt, Options{GOOS: "linux", GOARCH: "arm64", Output: "ir"}); err == nil || !strings.Contains(err.Error(), "unsupported arm64 instruction") {
		t.Fatalf("got %v", err)
	}
}

func readObject(t *testing.T, data []byte) *goobj.Reader {
	t.Helper()
	off := bytes.Index(data, []byte("\n!\n"))
	if off < 0 {
		t.Fatal("missing Go object header")
	}
	r := goobj.NewReaderFromBytes(data[off+3:], true)
	if r == nil || !r.FromAssembly() {
		t.Fatal("invalid Go assembly object")
	}
	return r
}

func findSymbol(t *testing.T, r *goobj.Reader, name string) uint32 {
	t.Helper()
	n := r.NSym() + r.NHashed64def() + r.NHasheddef() + r.NNonpkgdef() + r.NNonpkgref()
	for i := 0; i < n; i++ {
		if r.Sym(uint32(i)).Name(r) == name {
			return uint32(i)
		}
	}
	t.Fatalf("missing symbol %s", name)
	return 0
}

func resolveSym(t *testing.T, r *goobj.Reader, ref goobj.SymRef) uint32 {
	t.Helper()
	i := ref.SymIdx
	switch ref.PkgIdx {
	case goobj.PkgIdxSelf:
	case goobj.PkgIdxHashed64:
		i += uint32(r.NSym())
	case goobj.PkgIdxHashed:
		i += uint32(r.NSym() + r.NHashed64def())
	case goobj.PkgIdxNone:
		i += uint32(r.NSym() + r.NHashed64def() + r.NHasheddef())
	default:
		t.Fatalf("external reference %v", ref)
	}
	return i
}

// A real Go caller supplies ABI wrappers and argument stack maps. The assembler
// emits GoObj through LLVM, and the unchanged Go linker builds the executable.
func TestHostExecution(t *testing.T) {
	if (runtime.GOOS != "darwin" && runtime.GOOS != "linux") || (runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64") {
		t.Skip("unsupported host")
	}
	dir := t.TempDir()
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	assembler := filepath.Join(dir, "asm")
	cmd := exec.Command(goTool, "build", "-gcflags=all=-enablellvm=false", "-o", assembler, "cmd/asm")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build assembler: %v\n%s", err, out)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("tools", "#!/bin/sh\nif [ \"${1##*/}\" = asm ]; then shift; exec '"+strings.ReplaceAll(assembler, "'", "'\\''")+"' \"$@\"; fi\nexec \"$@\"\n")
	write("go.mod", "module asmprobe\n\ngo 1.26\n")
	write("main.go", `package main
import ("runtime"; "strings"; "fmt")
func add(a,b int64) int64
//go:noescape
func keep(p *int) int
func narrow(x uint64) int64
func immediate() uint64
//go:noinline
func grow(n int) int {
 var frame [256]byte
 frame[n%256] = byte(n)
 if n > 0 { return grow(n-1)+int(frame[n%256]) }
 runtime.GC()
 var pcs [100]uintptr
 count := runtime.Callers(0, pcs[:])
 frames := runtime.CallersFrames(pcs[:count])
 found := false
 for { f, more := frames.Next(); if strings.HasSuffix(f.Function,".keep") { found=true }; if !more { break } }
 if !found { panic("assembly frame missing from traceback") }
 return int(frame[0])
}
func forceGC() { grow(50) }
func main() {
 if immediate()!=0xffffffff || narrow(0x80000000)!=-2147483648 || narrow(0xffffffff)!=-1 || add(17,25)!=42 || add(-7,3)!=-4 { panic("ABI0 arithmetic") }
 done := make(chan bool)
 for i:=0;i<20;i++ { go func(){ x:=123; if keep(&x)!=123 { panic("pointer stack map") }; done<-true }() }
 for i:=0;i<20;i++ { <-done }
 fmt.Println("OK")
}
`)
	amd64Body := `TEXT ·immediate(SB),NOSPLIT,$0-8
 MOVL $-1,AX
 MOVQ AX,ret+0(FP)
 RET
TEXT ·narrow(SB),NOSPLIT,$0-16
 MOVQ x+0(FP),AX
 MOVLQSX AX,AX
 MOVQ AX,ret+8(FP)
 RET
TEXT ·add(SB),NOSPLIT,$0-24
 MOVQ a+0(FP),AX
 ADDQ b+8(FP),AX
 MOVQ AX,ret+16(FP)
 RET
TEXT ·keep(SB),0,$32-16
 NO_LOCAL_POINTERS
 CALL ·forceGC(SB)
 MOVQ p+0(FP),AX
 MOVQ (AX),AX
 MOVQ AX,ret+8(FP)
 RET
`
	arm64Body := `TEXT ·immediate(SB),NOSPLIT,$0-8
 MOVW $-1,R0
 MOVD R0,ret+0(FP)
 RET
TEXT ·narrow(SB),NOSPLIT,$0-16
 MOVD x+0(FP),R0
 MOVW R0,R0
 MOVD R0,ret+8(FP)
 RET
TEXT ·add(SB),NOSPLIT,$0-24
 MOVD a+0(FP),R0
 MOVD b+8(FP),R1
 ADD R1,R0,R0
 MOVD R0,ret+16(FP)
 RET
TEXT ·keep(SB),0,$32-16
 NO_LOCAL_POINTERS
 CALL ·forceGC(SB)
 MOVD p+0(FP),R0
 MOVD (R0),R0
 MOVD R0,ret+8(FP)
 RET
`
	for arch, body := range map[string]string{"amd64": amd64Body, "arm64": arm64Body} {
		write("asm_"+arch+".s", "#include \"textflag.h\"\n#include \"funcdata.h\"\n"+body)
	}
	targets := []string{runtime.GOARCH}
	// Opt in after checking that this host can execute x86-64 binaries.
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" && os.Getenv("GOALLC_ASM_TEST_AMD64") == "1" {
		targets = append(targets, "amd64")
	}
	for _, arch := range targets {
		for _, backend := range []string{"false", "true"} {
			t.Logf("execute %s/%s enablellvm=%s", runtime.GOOS, arch, backend)
			cmd := exec.Command(goTool, "run", "-gcflags=all=-enablellvm=false", "-toolexec="+filepath.Join(dir, "tools"), "-asmflags=asmprobe=-enablellvm="+backend, ".")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOARCH="+arch, "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil || string(out) != "OK\n" {
				t.Fatalf("run %s enablellvm=%s: %v\n%s", arch, backend, err, out)
			}
		}
	}
}

func llvmTool(t *testing.T, name string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "pkg", "goallc-llvm-payload"))
	if err != nil {
		t.Skip("LLVM payload tools are unavailable")
	}
	path := filepath.Join(strings.TrimSpace(string(payload)), "bin", name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("LLVM tool %s is unavailable", name)
	}
	return path
}

func TestLTOCarrier(t *testing.T) {
	opt, llc := llvmTool(t, "opt"), llvmTool(t, "llc")
	for _, goarch := range []string{"amd64", "arm64"} {
		t.Run(goarch, func(t *testing.T) {
			body := "MOVQ $42, AX\nRET\n"
			if goarch == "arm64" {
				body = "MOVD $42, R0\nRET\n"
			}
			source := "TEXT asm_result<ABIInternal>(SB),4,$0\n" + body
			dir := t.TempDir()
			input := filepath.Join(dir, "input.ll")
			optimized := filepath.Join(dir, "optimized.ll")
			object := filepath.Join(dir, "output.o")
			ir := string(emit(t, "linux", goarch, source, "ir"))
			// An opaque-pointer call can have the Go declaration's real prototype even
			// though assembly itself has no typed formal arguments or IR return value.
			ir += "\ndefine goabiinternal i64 @caller() \"go-nosplit\" \"frame-pointer\"=\"all\" {\n %v = call goabiinternal i64 @asm_result()\n %r = add i64 %v, 1\n ret i64 %r\n}\n"
			if err := os.WriteFile(input, []byte(ir), 0600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(opt, "-S", "-passes=lto-pre-link<O2>,lto<O2>", input, "-o", optimized).CombinedOutput(); err != nil {
				t.Fatalf("LTO: %v\n%s", err, out)
			}
			data, err := os.ReadFile(optimized)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte("ret i64")) || !bytes.Contains(data, []byte("asm sideeffect")) {
				t.Fatalf("LTO lost naked assembly return contract:\n%s", data)
			}
			if out, err := exec.Command(llc, "-filetype=obj", optimized, "-o", object).CombinedOutput(); err != nil {
				t.Fatalf("LTO codegen: %v\n%s", err, out)
			}
		})
	}
}

func pcValue(t *testing.T, data []byte, pc, quantum uint64) int64 {
	t.Helper()
	value, end := int64(-1), uint64(0)
	for first := true; len(data) != 0; first = false {
		delta, n := binary.Varint(data)
		if n <= 0 {
			t.Fatal("invalid PC table value")
		}
		data = data[n:]
		if delta == 0 && !first {
			break
		}
		value += delta
		advance, n := binary.Uvarint(data)
		if n <= 0 {
			t.Fatal("invalid PC table offset")
		}
		data = data[n:]
		end += advance * quantum
		if pc < end {
			return value
		}
	}
	t.Fatalf("PC %d beyond table", pc)
	return 0
}

func TestFrameMetadata(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			source := "TEXT test<ABIInternal>(SB),4,$32-16\nFUNCDATA $1, maps(SB)\nCALL target<ABIInternal>(SB)\nRET\n"
			native := parse(t, "linux", arch, source, true).Text[0]
			r := readObject(t, emit(t, "linux", arch, source, "obj"))
			i := findSymbol(t, r, "test")
			if r.Sym(i).Flag()&goobj.SymFlagLeaf != 0 {
				t.Fatal("inline assembly call incorrectly classified as leaf")
			}
			if r.Sym(i).Flag()&goobj.SymFlagNoSplit == 0 {
				t.Fatal("NOSPLIT lost")
			}
			quantum := uint64(1)
			if arch == "arm64" {
				quantum = 4
			}
			funcdata := 0
			for _, aux := range r.Auxs(i) {
				switch aux.Type() {
				case goobj.AuxFuncInfo:
					data := r.Data(resolveSym(t, r, aux.Sym()))
					if int32(binary.LittleEndian.Uint32(data)) != native.Func().Args || int32(binary.LittleEndian.Uint32(data[4:])) != native.Func().Locals {
						t.Fatalf("frame differs from native: %x", data[:8])
					}
				case goobj.AuxPcsp:
					data := r.Data(resolveSym(t, r, aux.Sym()))
					for pc := uint64(0); pc < uint64(r.Sym(i).Siz()); pc += quantum {
						want, got := pcValue(t, native.Func().Pcln.Pcsp.P, pc, quantum), pcValue(t, data, pc, quantum)
						if want != got {
							t.Fatalf("PCSP at %d: got %d want %d", pc, got, want)
						}
					}
				case goobj.AuxFuncdata:
					if funcdata == 0 && aux.Sym() != (goobj.SymRef{}) {
						t.Fatal("absent args map was synthesized")
					}
					if funcdata == 1 && r.Sym(resolveSym(t, r, aux.Sym())).Name(r) != "maps" {
						t.Fatal("FUNCDATA reference lost")
					}
					funcdata++
				}
			}
			if funcdata != 2 {
				t.Fatalf("got %d FUNCDATA slots", funcdata)
			}
		})
	}
}

func TestStaticSymbolsAndDataKinds(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			source := `DATA pointer+0(SB)/8,$bss(SB)
GLOBL pointer(SB),0,$8
GLOBL bss(SB),0,$64
GLOBL noptr(SB),16,$64
TEXT local<>(SB),4,$0-0
 RET
TEXT exported(SB),4,$0-0
 CALL local<>(SB)
 RET
`
			r := readObject(t, emit(t, "linux", arch, source, "obj"))
			if r.Sym(findSymbol(t, r, "local")).ABI() != goobj.SymABIstatic {
				t.Fatal("static function became externally visible")
			}
			for name, want := range map[string]objabi.SymKind{"pointer": objabi.SDATA, "bss": objabi.SBSS, "noptr": objabi.SNOPTRBSS} {
				if got := r.Sym(findSymbol(t, r, name)).Type(); got != uint8(want) {
					t.Errorf("%s kind=%d want %d", name, got, want)
				}
			}
			if r.NReloc(findSymbol(t, r, "pointer")) != 1 {
				t.Fatal("DATA pointer relocation lost")
			}
		})
	}
}
