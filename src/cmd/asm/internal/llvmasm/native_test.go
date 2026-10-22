// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"bytes"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"debug/elf"
	"debug/macho"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type nativeReloc struct {
	offset uint64
	kind   uint32
	symbol string
}

type nativeObject struct {
	text, metadata             []byte
	textRelocs, metadataRelocs []nativeReloc
}

func readNative(t *testing.T, data []byte, format, name string, size int) nativeObject {
	t.Helper()
	var result nativeObject
	if format == "elf" {
		f, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		syms, err := f.Symbols()
		if err != nil {
			t.Fatal(err)
		}
		var textSection elf.SectionIndex
		for _, sym := range syms {
			if sym.Name != name {
				continue
			}
			if int(sym.Size) != size {
				t.Fatalf("ELF symbol size=%d, want %d", sym.Size, size)
			}
			textSection = sym.Section
			section, err := f.Sections[sym.Section].Data()
			if err != nil {
				t.Fatal(err)
			}
			result.text = section[sym.Value : sym.Value+sym.Size]
		}
		meta := f.Section(".go.asm")
		if meta == nil {
			t.Fatal("missing native Go metadata section")
		}
		result.metadata, err = meta.Data()
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range f.Sections {
			if section.Type != elf.SHT_RELA {
				continue
			}
			target := f.Sections[section.Info]
			if target != meta && elf.SectionIndex(section.Info) != textSection {
				continue
			}
			raw, err := section.Data()
			if err != nil {
				t.Fatal(err)
			}
			for len(raw) >= 24 {
				off, info := binary.LittleEndian.Uint64(raw), binary.LittleEndian.Uint64(raw[8:])
				index := int(info >> 32)
				if index == 0 || index > len(syms) {
					t.Fatal("bad ELF relocation symbol")
				}
				r := nativeReloc{off, uint32(info), syms[index-1].Name}
				if target == meta {
					result.metadataRelocs = append(result.metadataRelocs, r)
				} else {
					result.textRelocs = append(result.textRelocs, r)
				}
				raw = raw[24:]
			}
		}
	} else {
		f, err := macho.NewFile(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if f.Symtab == nil {
			t.Fatal("missing Mach-O symbol table")
		}
		var textSection *macho.Section
		for _, sym := range f.Symtab.Syms {
			if sym.Name != "_"+name {
				continue
			}
			textSection = f.Sections[int(sym.Sect)-1]
			section, err := textSection.Data()
			if err != nil {
				t.Fatal(err)
			}
			off := sym.Value - textSection.Addr
			if off+uint64(size) > uint64(len(section)) {
				t.Fatal("truncated Mach-O function")
			}
			result.text = section[off : off+uint64(size)]
		}
		meta := f.Section("__go_asm")
		if meta == nil {
			t.Fatal("missing native Go metadata section")
		}
		result.metadata, err = meta.Data()
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range f.Sections {
			if section != textSection && section != meta {
				continue
			}
			for _, rel := range section.Relocs {
				sym := ""
				if rel.Extern {
					sym = strings.TrimPrefix(f.Symtab.Syms[rel.Value].Name, "_")
				}
				r := nativeReloc{uint64(rel.Addr), uint32(rel.Type), sym}
				if section == meta {
					result.metadataRelocs = append(result.metadataRelocs, r)
				} else {
					result.textRelocs = append(result.textRelocs, r)
				}
			}
		}
	}
	if result.text == nil {
		t.Fatalf("missing native function %q", name)
	}
	return result
}

// Ignore only relocation bits, retaining opcodes/registers/addressing modes.
func unrelocatedCode(t *testing.T, code []byte, relocs []obj.Reloc) []byte {
	t.Helper()
	code = bytes.Clone(code)
	mask := func(off int32, bits uint32) {
		word := binary.LittleEndian.Uint32(code[off:])
		binary.LittleEndian.PutUint32(code[off:], word&^bits)
	}
	for _, r := range relocs {
		switch r.Type {
		case objabi.R_CALLIND:
		case objabi.R_ADDRARM64, objabi.R_ARM64_PCREL, objabi.R_ARM64_PCREL_LDST8, objabi.R_ARM64_PCREL_LDST16, objabi.R_ARM64_PCREL_LDST32, objabi.R_ARM64_PCREL_LDST64, objabi.R_ARM64_GOTPCREL, objabi.R_ARM64_GOT:
			mask(r.Off, 0x60ffffe0)   // ADRP immlo/immhi.
			mask(r.Off+4, 0x003ffc00) // ADD/load/store imm12.
		case objabi.R_CALLARM64:
			mask(r.Off, 0x03ffffff)
		case objabi.R_ADDR, objabi.R_PCREL, objabi.R_CALL, objabi.R_GOTPCREL, objabi.R_TLS_LE:
			clear(code[int(r.Off) : int(r.Off)+int(r.Siz)])
		default:
			t.Fatalf("test needs relocation mask for %s", r.Type)
		}
	}
	return code
}

func TestFixedNativeObjects(t *testing.T) {
	for _, format := range []string{"elf", "macho"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(format+"/"+arch, func(t *testing.T) {
				goos := "linux"
				if format == "macho" {
					goos = "darwin"
				}
				body := "CMPQ AX,$0\nJEQ done\nADOXQ AX,BX\nMOVQ data(SB),AX\nADDL $7,data(SB)\nCALL target<ABIInternal>(SB)\ndone:\nPCDATA $0,$-1\nPCALIGN $16\nJMP target<ABIInternal>(SB)\n"
				if arch == "arm64" {
					body = "CBZ R0,done\nSHA512H V0.D2,V1,V2\nMOVD data(SB),R0\nMOVD $data+8(SB),R1\nCALL target<ABIInternal>(SB)\ndone:\nPCDATA $0,$-1\nPCALIGN $16\nRET\n"
				}
				source := "DATA data+0(SB)/8,$7\nGLOBL data(SB),8,$16\nTEXT test<ABIInternal>(SB),4,$32-16\nFUNCDATA $1,maps(SB)\nFUNCDATA $3,maps2(SB)\nPCDATA $0,$-2\n" + body
				ctxt := parse(t, goos, arch, source, false)
				data, err := Emit(ctxt, Options{GOOS: goos, GOARCH: arch, ObjectFormat: format, Output: "obj"})
				if err != nil {
					t.Fatal(err)
				}
				fn := ctxt.Text[0]
				native := parse(t, goos, arch, source, true).Text[0]
				obj := readNative(t, data, format, "test", len(native.P))
				got, want := unrelocatedCode(t, obj.text, native.R), unrelocatedCode(t, native.P, native.R)
				if !bytes.Equal(got, want) {
					t.Fatalf("native encoding changed:\ngot  %x\nwant %x", got, want)
				}
				call := uint32(elf.R_X86_64_PLT32)
				if arch == "arm64" {
					call = uint32(elf.R_AARCH64_CALL26)
				}
				if format == "macho" {
					call = uint32(macho.X86_64_RELOC_BRANCH)
					if arch == "arm64" {
						call = uint32(macho.ARM64_RELOC_BRANCH26)
					}
				}
				found := false
				for _, r := range obj.textRelocs {
					if r.symbol == "target" && r.kind == call {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing native call relocation: %+v", obj.textRelocs)
				}
				checkNativeMetadata(t, obj, fn, nil)
			})
		}
	}
}

func TestNativeTLS(t *testing.T) {
	for _, test := range []struct {
		name, goos, format string
		shared             bool
	}{
		{"elf_local_exec", "linux", "elf", false},
		{"elf_initial_exec", "linux", "elf", true},
		{"macho_go_slot", "darwin", "macho", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "TEXT tlsprobe<ABIInternal>(SB),516,$0\nMOVQ TLS,AX\nMOVQ 0(AX)(TLS*1),CX\nRET\n"
			ctxt := parseShared(t, test.goos, "amd64", source, false, test.shared)
			data, err := Emit(ctxt, Options{GOOS: test.goos, GOARCH: "amd64", ObjectFormat: test.format, Output: "obj"})
			if err != nil {
				t.Fatal(err)
			}
			fn := ctxt.Text[0]
			object := readNative(t, data, test.format, "tlsprobe", len(fn.P))
			if len(fn.R) != 1 {
				t.Fatalf("expected a TLS relocation, got %+v", fn.R)
			}
			rel := fn.R[0]
			if test.format == "macho" {
				if len(object.textRelocs) != 0 {
					t.Fatalf("Go's fixed TLS slot became a Mach-O relocation: %+v", object.textRelocs)
				}
				if got := int32(binary.LittleEndian.Uint32(object.text[rel.Off:])); got != int32(objabi.DarwinAMD64TLSOffset+rel.Add) {
					t.Fatalf("Go TLS slot=%d", got)
				}
			} else {
				want := uint32(elf.R_X86_64_TPOFF32)
				if test.shared {
					want = uint32(elf.R_X86_64_GOTTPOFF)
				}
				if len(object.textRelocs) != 1 || object.textRelocs[0].kind != want || object.textRelocs[0].symbol != "runtime.tlsg" {
					t.Fatalf("unexpected ELF TLS relocation: %+v", object.textRelocs)
				}
			}
			checkNativeMetadata(t, object, fn, nil)
		})
	}
}

// This independent reader validates serialized field boundaries and checks the
// actual object section, including pointer relocations, against native Go data.
type metadataReader struct {
	t    *testing.T
	data []byte
	off  int
}

func (r *metadataReader) take(n int) []byte {
	r.t.Helper()
	if n < 0 || n > len(r.data)-r.off {
		r.t.Fatal("truncated native metadata")
	}
	p := r.data[r.off : r.off+n]
	r.off += n
	return p
}
func (r *metadataReader) u32() uint32  { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *metadataReader) u64() uint64  { return binary.LittleEndian.Uint64(r.take(8)) }
func (r *metadataReader) blob() []byte { return r.take(int(r.u32())) }
func (r *metadataReader) align()       { r.take((-r.off) & 7) }
func (r *metadataReader) ptr() int     { r.align(); off := r.off; r.take(8); return off }

func checkNativeMetadata(t *testing.T, object nativeObject, fn *obj.LSym, renamed map[string]string) {
	t.Helper()
	r := metadataReader{t: t, data: object.metadata}
	if string(r.take(8)) != "GoAsm\x00\x01\x00" {
		t.Fatal("bad native metadata magic/version")
	}
	if r.u64() != uint64(len(r.data)) {
		t.Fatal("bad metadata length")
	}
	r.u32() // Module flags.
	files, symbols := r.u32(), r.u32()
	if r.u32() != 0 {
		t.Fatal("nonzero reserved header field")
	}
	for i := 0; i < 4; i++ {
		if len(r.blob()) == 0 && i < 3 {
			t.Fatal("missing module identity")
		}
	}
	for i := uint32(0); i < files; i++ {
		r.blob()
	}
	refAt := map[int]string{}
	for _, rel := range object.metadataRelocs {
		refAt[int(rel.offset)] = rel.symbol
	}
	found := false
	for i := uint32(0); i < symbols; i++ {
		r.align()
		start := r.off
		length := r.u64()
		address := r.ptr()
		r.ptr() // Optional Go type descriptor.
		size := r.u64()
		r.u32()
		r.u32()
		r.u32()
		r.u32() // Kind, alignment, attributes, ABI.
		name := string(r.blob())
		nreloc := r.u32()
		if name == fn.Name && nreloc != uint32(len(fn.R)) {
			t.Fatal("original Go relocation count changed")
		}
		for n := uint32(0); n < nreloc; n++ {
			off, kind, size := r.u32(), r.u32(), r.u32()
			if r.u32() != 0 {
				t.Fatal("nonzero relocation reserved field")
			}
			add, target := int64(r.u64()), r.ptr()
			if name != fn.Name {
				continue
			}
			want := fn.R[n]
			if off != uint32(want.Off) || kind != uint32(want.Type) || size != uint32(want.Siz) || add != want.Add {
				t.Fatal("original Go relocation changed")
			}
			if want.Sym == nil {
				if _, ok := refAt[target]; ok {
					t.Fatal("target-less Go relocation acquired a target")
				}
			} else {
				expected := want.Sym.Name
				if replacement, ok := renamed[expected]; ok {
					expected = replacement
				}
				if refAt[target] != expected {
					t.Fatalf("Go relocation target=%q, want %q", refAt[target], expected)
				}
			}
		}
		isFunc := r.u32()
		if name != fn.Name {
			r.take(int(length) - (r.off - start))
			continue
		}
		found = true
		if refAt[address] != fn.Name || size != uint64(fn.Size) || isFunc != 1 {
			t.Fatalf("bad function metadata reference: %q size=%d kind=%d", refAt[address], size, isFunc)
		}
		f := fn.Func()
		if int32(r.u32()) != f.Args || int32(r.u32()) != f.Locals {
			t.Fatal("frame metadata changed")
		}
		if r.u32() != uint32(f.FuncID) || r.u32() != uint32(f.FuncFlag) || int32(r.u32()) != f.StartLine {
			t.Fatal("function attributes changed")
		}
		for _, table := range []*obj.LSym{f.Pcln.Pcsp, f.Pcln.Pcfile, f.Pcln.Pcline, f.Pcln.Pcinline} {
			var want []byte
			if table != nil {
				want = table.P
			}
			if !bytes.Equal(r.blob(), want) {
				t.Fatal("PC table differs from Go's encoded table")
			}
		}
		for n := r.u32(); n > 0; n-- {
			if r.u32() >= files {
				t.Fatal("file index outside module file table")
			}
		}
		if n := r.u32(); n != uint32(len(f.Pcln.Pcdata)) {
			t.Fatalf("PCDATA count=%d", n)
		}
		for _, table := range f.Pcln.Pcdata {
			var want []byte
			if table != nil {
				want = table.P
			}
			if !bytes.Equal(r.blob(), want) {
				t.Fatal("PCDATA differs from Go")
			}
		}
		if n := r.u32(); n != uint32(len(f.Pcln.Funcdata)) {
			t.Fatalf("FUNCDATA count=%d", n)
		}
		for _, data := range f.Pcln.Funcdata {
			off := r.ptr()
			if data == nil {
				if _, ok := refAt[off]; ok {
					t.Fatal("absent FUNCDATA has relocation")
				}
			} else if refAt[off] != data.Name {
				t.Fatalf("FUNCDATA reference=%q, want %q", refAt[off], data.Name)
			}
		}
		r.align()
		if r.off-start != int(length) {
			t.Fatal("function record length mismatch")
		}
	}
	if !found || r.off != len(r.data) {
		t.Fatal("incomplete native metadata")
	}
}

func TestNativeHostExecution(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("native host linker test requires Darwin or Linux")
	}
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is unavailable")
	}
	format := "elf"
	if runtime.GOOS == "darwin" {
		format = "macho"
	}
	arches := []string{runtime.GOARCH}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" && os.Getenv("GOALLC_ASM_TEST_AMD64") == "1" {
		arches = append(arches, "amd64")
	}
	for _, arch := range arches {
		t.Run(arch, func(t *testing.T) {
			// A no-argument leaf has the same integer return register in the host
			// ABI and Go ABI. This executes native address relocations without
			// pretending that the host linker already consumes Go runtime metadata.
			body := "MOVSD $1.23456,X0\nMOVQ pointer(SB),AX\nMOVQ (AX),AX\nADDQ $1,AX\nRET\n"
			if arch == "arm64" {
				body = "FMOVD $1.23456,F0\nMOVD pointer(SB),R0\nMOVD (R0),R0\nADD $1,R0,R0\nRET\n"
			}
			source := "DATA data+0(SB)/8,$6\nDATA data+8(SB)/8,$41\nGLOBL data(SB),8,$16\nDATA pointer+0(SB)/8,$data+8(SB)\nGLOBL pointer(SB),8,$8\nTEXT probe<ABIInternal>(SB),516,$0\n" + body
			ctxt := parse(t, runtime.GOOS, arch, source, false)
			data, err := Emit(ctxt, Options{GOOS: runtime.GOOS, GOARCH: arch, ObjectFormat: format, Output: "obj"})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			object, main, executable := filepath.Join(dir, "probe.o"), filepath.Join(dir, "main.c"), filepath.Join(dir, "probe")
			if err := os.WriteFile(object, data, 0600); err != nil {
				t.Fatal(err)
			}
			// Each separately emitted object synthesizes the same floating-point
			// constant. Both definitions must remain linkable.
			secondSource := strings.NewReplacer("pointer", "pointer2", "data", "data2", "probe", "probe2").Replace(source)
			secondData, err := Emit(parse(t, runtime.GOOS, arch, secondSource, false), Options{GOOS: runtime.GOOS, GOARCH: arch, ObjectFormat: format, Output: "obj"})
			if err != nil {
				t.Fatal(err)
			}
			secondObject := filepath.Join(dir, "probe2.o")
			if err := os.WriteFile(secondObject, secondData, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(main, []byte("extern long probe(void), probe2(void);\nint main(void) { return probe() != 42 || probe2() != 42; }\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{main, object, secondObject, "-o", executable}
			if runtime.GOOS == "darwin" {
				nativeArch := "arm64"
				if arch == "amd64" {
					nativeArch = "x86_64"
				}
				args = append([]string{"-arch", nativeArch}, args...)
			}
			if out, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
				t.Fatalf("native link: %v\n%s", err, out)
			}
			if out, err := exec.Command(executable).CombinedOutput(); err != nil {
				t.Fatalf("native execution: %v\n%s", err, out)
			}
		})
	}
}

func TestNativeBitcodeAndLTO(t *testing.T) {
	opt, llc := llvmTool(t, "opt"), llvmTool(t, "llc")
	for _, format := range []string{"elf", "macho"} {
		for _, arch := range []string{"amd64", "arm64"} {
			t.Run(format+"/"+arch, func(t *testing.T) {
				goos, body := "linux", "CALL target<ABIInternal>(SB)\nMOVQ $42,AX\nRET\n"
				if format == "macho" {
					goos = "darwin"
				}
				if arch == "arm64" {
					body = "CALL target<ABIInternal>(SB)\nMOVD $42,R0\nRET\n"
				}
				ctxt := parseShared(t, goos, arch, "TEXT answer<ABIInternal>(SB),4,$0\n"+body, false, true)
				bc, err := Emit(ctxt, Options{GOOS: goos, GOARCH: arch, ObjectFormat: format, Output: "bc"})
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				input, ir, optimized, output := filepath.Join(dir, "input.bc"), filepath.Join(dir, "input.ll"), filepath.Join(dir, "optimized.ll"), filepath.Join(dir, "output.o")
				if err := os.WriteFile(input, bc, 0600); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command(llc, "-O0", "-filetype=obj", input, "-o", output).CombinedOutput(); err != nil {
					t.Fatalf("unoptimized bitcode codegen: %v\n%s", err, out)
				}
				unoptimized, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				checkNativeMetadata(t, readNative(t, unoptimized, format, "answer", len(ctxt.Text[0].P)), ctxt.Text[0], nil)
				if out, err := exec.Command(opt, "-S", input, "-o", ir).CombinedOutput(); err != nil {
					t.Fatalf("bitcode reload: %v\n%s", err, out)
				}
				text, err := os.ReadFile(ir)
				if err != nil {
					t.Fatal(err)
				}
				for _, forbidden := range []string{".goobj.", "go-plan9-context", "__goasm_operand_"} {
					if bytes.Contains(text, []byte(forbidden)) {
						t.Fatalf("native carrier contains %q", forbidden)
					}
				}
				if !bytes.Contains(text, []byte(`!"PIC Level", i32 2`)) {
					t.Fatal("bitcode lost PIC relocation mode")
				}
				// Rename only the IR global identifiers. No inline-asm string edits.
				text = bytes.ReplaceAll(text, []byte("@target"), []byte("@renamed_target"))
				text = append(text, []byte("\ndefine goabiinternal i64 @caller() { %v = call goabiinternal i64 @answer()\n %r = add i64 %v, 1\n ret i64 %r }\n")...)
				if err := os.WriteFile(ir, text, 0600); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command(opt, "-S", "-passes=lto-pre-link<O2>,lto<O2>", ir, "-o", optimized).CombinedOutput(); err != nil {
					t.Fatalf("LTO: %v\n%s", err, out)
				}
				text, err = os.ReadFile(optimized)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(text, []byte("ret i64")) {
					t.Fatal("LTO lost assembly return contract")
				}
				if out, err := exec.Command(llc, "-filetype=obj", optimized, "-o", output).CombinedOutput(); err != nil {
					t.Fatalf("standalone llc: %v\n%s", err, out)
				}
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				object := readNative(t, data, format, "answer", len(ctxt.Text[0].P))
				found := false
				for _, rel := range object.textRelocs {
					if rel.symbol == "renamed_target" {
						found = true
					}
				}
				if !found {
					t.Fatalf("renamed reference lost: %+v", object.textRelocs)
				}
				checkNativeMetadata(t, object, ctxt.Text[0], map[string]string{"target": "renamed_target"})
				// The assembler must reject a layout change instead of silently
				// making the original PC tables or local branch bytes stale.
				bad := bytes.Replace(text, []byte(fmt.Sprintf(" != %d\\0A.error \\22Go assembly function size changed", ctxt.Text[0].Size)), []byte(" != 99999\\0A.error \\22Go assembly function size changed"), 1)
				if bytes.Equal(bad, text) {
					t.Fatal("missing function layout assertion")
				}
				if err := os.WriteFile(ir, bad, 0600); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command(llc, "-filetype=obj", ir, "-o", output).CombinedOutput(); err == nil || !bytes.Contains(out, []byte("Go assembly function size changed")) {
					t.Fatalf("bad layout was not rejected: %v\n%s", err, out)
				}
			})
		}
	}
}

func TestNativeTargetMismatch(t *testing.T) {
	for _, options := range []Options{
		{GOOS: "darwin", GOARCH: "arm64", ObjectFormat: "elf", Output: "ir"},
		{GOOS: "linux", GOARCH: "amd64", ObjectFormat: "macho", Output: "bc"},
	} {
		ctxt := parse(t, options.GOOS, options.GOARCH, "TEXT test<ABIInternal>(SB),516,$0\nRET\n", false)
		if _, err := Emit(ctxt, options); err == nil || !strings.Contains(err.Error(), "does not match GOOS") {
			t.Fatalf("incompatible target accepted: %v", err)
		}
	}
}

func TestFixedLayoutRejectsUnrelocatedADRP(t *testing.T) {
	for _, format := range []string{"elf", "macho"} {
		goos := "linux"
		if format == "macho" {
			goos = "darwin"
		}
		ctxt := parse(t, goos, "arm64", "TEXT test<ABIInternal>(SB),516,$0\nADRP 1(PC),R0\nRET\n", false)
		if _, err := Emit(ctxt, Options{GOOS: goos, GOARCH: "arm64", ObjectFormat: format, Output: "ir"}); err == nil || !strings.Contains(err.Error(), "ADRP without a symbolic relocation") {
			t.Fatalf("unrelocated page address accepted for %s: %v", format, err)
		}
	}
}
