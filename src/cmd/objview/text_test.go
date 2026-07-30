// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmd/internal/archive"
	"cmd/internal/goobj"
	"encoding/base64"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"internal/testenv"
)

func TestSelectOutputFormat(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		json      bool
		formatSet bool
		want      string
		wantErr   string
	}{
		{name: "default", format: "text", want: "text"},
		{name: "text", format: "text", formatSet: true, want: "text"},
		{name: "raw", format: "raw", formatSet: true, want: "raw"},
		{name: "json format", format: "json", formatSet: true, want: "json"},
		{name: "json compatibility", format: "text", json: true, want: "json"},
		{name: "json explicit", format: "json", json: true, formatSet: true, want: "json"},
		{name: "conflict", format: "text", json: true, formatSet: true, wantErr: "conflicts"},
		{name: "unknown", format: "yaml", formatSet: true, wantErr: "invalid -format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectOutputFormat(tt.format, tt.json, tt.formatSet)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("selectOutputFormat error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("selectOutputFormat = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatBitmap(t *testing.T) {
	tests := []struct {
		name    string
		numBits int
		setBits []int
		want    string
	}{
		{name: "zero width", want: "-"},
		{name: "all zero", numBits: 4, want: "0000"},
		{name: "bit index order", numBits: 4, setBits: []int{0, 2}, want: "1010"},
		{name: "high bit", numBits: 4, setBits: []int{3}, want: "0001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBitmap(tt.numBits, tt.setBits); got != tt.want {
				t.Fatalf("formatBitmap(%d, %v) = %q, want %q", tt.numBits, tt.setBits, got, tt.want)
			}
		})
	}
}

func TestCommandFormatCompatibility(t *testing.T) {
	archive := compileFixture(t)
	defaultText := objviewCommandOutput(t, archive, "text-default")
	explicitText := objviewCommandOutput(t, archive, "text-explicit")
	if !bytes.Equal(defaultText, explicitText) {
		t.Fatal("default text output differs from -format=text")
	}
	raw := objviewCommandOutput(t, archive, "raw-explicit")
	if !bytes.HasPrefix(raw, []byte("GOOBJRAW 2 ")) {
		t.Fatalf("explicit raw output has unexpected header: %q", raw[:min(len(raw), 40)])
	}
	legacyJSON := objviewCommandOutput(t, archive, "json-legacy")
	explicitJSON := objviewCommandOutput(t, archive, "json-explicit")
	if !bytes.Equal(legacyJSON, explicitJSON) {
		t.Fatal("-json output differs from -format=json")
	}
}

func TestRawFormat(t *testing.T) {
	path := compileFixture(t)
	var first, second bytes.Buffer
	if err := writeRawFile(&first, path); err != nil {
		t.Fatal(err)
	}
	if err := writeRawFile(&second, path); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("raw output is not deterministic")
	}
	for _, want := range []string{
		"GOOBJRAW 2 archive=true",
		"kind=package_definition",
		"kind=go_object",
		"OFFSET    HEX BYTES",
		"| INTERPRETATION",
		"00000000  00 67 6f",
		"-- block[0] autolib",
		`name="example.com/fixture.MetadataFixture"`,
		"-- block[14] relocation",
		"reloc[0]",
		"-- block[15] aux",
		"aux[0]",
		"-- block[16] data",
	} {
		if !strings.Contains(first.String(), want) {
			t.Errorf("raw output is missing %q", want)
		}
	}
	if strings.Contains(first.String(), "\nTEXT ") {
		t.Fatal("raw output unexpectedly contains disassembly")
	}
	if strings.Contains(first.String(), "limitation[") {
		t.Fatal("raw output unexpectedly contains a limitations list")
	}
}

func TestObjviewCommandHelper(t *testing.T) {
	mode := os.Getenv("GOOBJVIEW_HELPER")
	if mode == "" {
		return
	}
	path := os.Getenv("GOOBJVIEW_HELPER_INPUT")
	args := []string{"objview"}
	switch mode {
	case "text-default":
	case "text-explicit":
		args = append(args, "-format=text")
	case "raw-explicit":
		args = append(args, "-format=raw")
	case "json-legacy":
		args = append(args, "-json")
	case "json-explicit":
		args = append(args, "-format=json")
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
	args = append(args, path)
	os.Args = args
	flag.CommandLine = flag.NewFlagSet("objview", flag.ExitOnError)
	outputFormat = flag.String("format", "text", "output format: text, raw, or json")
	jsonOutput = flag.Bool("json", false, "print canonical JSON (alias for -format=json)")
	main()
	os.Exit(0)
}

func objviewCommandOutput(t *testing.T, path, mode string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestObjviewCommandHelper$")
	cmd.Env = append(os.Environ(),
		"GOOBJVIEW_HELPER="+mode,
		"GOOBJVIEW_HELPER_INPUT="+path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("objview helper %s: %v\n%s", mode, err, output)
	}
	return output
}

func TestTextNativeGolden(t *testing.T) {
	tests := []struct {
		name, goos, goarch string
	}{
		{name: "AArch64", goos: "darwin", goarch: "arm64"},
		{name: "X86", goos: "linux", goarch: "amd64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeTextFile(&out, compileFixtureFor(t, tt.goos, tt.goarch)); err != nil {
				t.Fatal(err)
			}
			got := textGoldenSnapshot(out.String())
			testdata := os.Getenv("GOOBJVIEW_TESTDATA")
			if testdata == "" {
				testdata = "testdata"
			}
			golden := filepath.Join(testdata, "text-"+tt.goarch+".golden")
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("%s text golden mismatch:\n--- want ---\n%s--- got ---\n%s", tt.goarch, want, got)
			}
		})
	}
}

func TestTextMetadataNames(t *testing.T) {
	if got := textPCDataName(99); got != "PCDATA_99" {
		t.Fatalf("unknown PCDATA name = %q", got)
	}
	if got := textFuncDataName(99); got != "FUNCDATA_99" {
		t.Fatalf("unknown FUNCDATA name = %q", got)
	}
}

func TestTextMultiObjectArchive(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	dir := t.TempDir()
	objects := []struct {
		name, goos, goarch string
	}{
		{name: "arm64.o", goos: "darwin", goarch: "arm64"},
		{name: "amd64.o", goos: "linux", goarch: "amd64"},
	}
	var paths []string
	for _, object := range objects {
		raw, err := os.ReadFile(extractRawGoObject(t, compileFixtureFor(t, object.goos, object.goarch)))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, object.name)
		if err := os.WriteFile(path, raw, 0o666); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	mixed := filepath.Join(dir, "mixed.a")
	args := append([]string{"tool", "pack", "c", mixed}, paths...)
	cmd := testenv.Command(t, testenv.GoToolPath(t), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pack mixed archive: %v\n%s", err, output)
	}

	var out bytes.Buffer
	if err := writeTextFile(&out, mixed); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`GOOBJ "arm64.o" darwin/arm64`,
		`GOOBJ "amd64.o" linux/amd64`,
		"94000000",
		"ffd1",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("mixed-archive text is missing %q", want)
		}
	}
}

func TestTextRejectsLegacyLLVMMetadataCarrier(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "multiple-calls.goobj.base64"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy-llvm.goobj")
	if err := os.WriteFile(path, data, 0o666); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err = writeTextFile(&out, path)
	if err == nil ||
		!strings.Contains(err.Error(), `function "different_pointer_sets_across_calls"`) ||
		!strings.Contains(err.Error(), "pcsp uses none carrier") {
		t.Fatalf("legacy LLVM text error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("text emitted %d bytes before rejecting legacy LLVM metadata", out.Len())
	}
}

func TestTextRejectsDamagedMetadata(t *testing.T) {
	t.Run("PC table", func(t *testing.T) {
		path := mutateFunctionAuxData(t, compileFixtureFor(t, "darwin", "arm64"),
			"example.com/fixture.MetadataFixture", goobj.AuxPcsp, 0,
			func(data []byte) {
				data[len(data)-1] = 0x80
			})
		assertTextError(t, path, "MetadataFixture", "pcsp", "unexpected EOF")
	})

	t.Run("stack map", func(t *testing.T) {
		path := mutateFunctionAuxData(t, compileFixtureFor(t, "darwin", "arm64"),
			"example.com/fixture.MetadataFixture", goobj.AuxFuncdata, 0,
			func(data []byte) {
				data[0] = 100
				data[1], data[2], data[3] = 0, 0, 0
			})
		assertTextError(t, path, "MetadataFixture", "FUNCDATA_ArgsPointerMaps", "stack map")
	})
}

func TestTextRejectsUnknownArchitectureAndInstruction(t *testing.T) {
	t.Run("architecture", func(t *testing.T) {
		path := compileFixtureFor(t, "darwin", "arm64")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.ReplaceAll(data, []byte(" arm64 "), []byte(" bogus "))
		bad := filepath.Join(t.TempDir(), "unknown-arch.a")
		if err := os.WriteFile(bad, data, 0o666); err != nil {
			t.Fatal(err)
		}
		assertTextError(t, bad, "unsupported architecture", "bogus")
	})

	t.Run("instruction", func(t *testing.T) {
		path := mutateFunctionData(t, compileFixtureFor(t, "darwin", "arm64"),
			"example.com/fixture.touch", func(data []byte) {
				copy(data, []byte{0xff, 0xff, 0xff, 0xff})
			})
		assertTextError(t, path, `function "example.com/fixture.touch"`, "PC +0x0", "unknown instruction")
	})
}

func assertTextError(t *testing.T, path string, substrings ...string) {
	t.Helper()
	var out bytes.Buffer
	err := writeTextFile(&out, path)
	if err == nil {
		t.Fatal("text unexpectedly succeeded")
	}
	for _, substring := range substrings {
		if !strings.Contains(err.Error(), substring) {
			t.Errorf("text error %q is missing %q", err, substring)
		}
	}
	if out.Len() != 0 {
		t.Errorf("text emitted %d bytes before failing", out.Len())
	}
}

func mutateFunctionAuxData(t *testing.T, path, function string, auxType uint8, auxIndex int, mutate func([]byte)) string {
	t.Helper()
	return mutateGoObject(t, path, func(r *goobj.Reader) (uint32, int) {
		functionIndex := findGoObjSymbolIndex(t, r, function)
		index := 0
		for i := 0; i < r.NAux(functionIndex); i++ {
			aux := r.Aux(functionIndex, i)
			if aux.Type() != auxType {
				continue
			}
			if index == auxIndex {
				return localSymIndex(t, r, aux.Sym()), r.DataSize(localSymIndex(t, r, aux.Sym()))
			}
			index++
		}
		t.Fatalf("%s has no aux type %d index %d", function, auxType, auxIndex)
		return 0, 0
	}, mutate)
}

func mutateFunctionData(t *testing.T, path, function string, mutate func([]byte)) string {
	t.Helper()
	return mutateGoObject(t, path, func(r *goobj.Reader) (uint32, int) {
		index := findGoObjSymbolIndex(t, r, function)
		return index, r.DataSize(index)
	}, mutate)
}

func mutateGoObject(t *testing.T, path string, locate func(*goobj.Reader) (uint32, int), mutate func([]byte)) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
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
		object := data[entry.Obj.Offset : entry.Obj.Offset+entry.Obj.Size]
		r := goobj.NewReaderFromBytes(object, false)
		if r == nil {
			t.Fatal("invalid Go object")
		}
		index, size := locate(r)
		start := int(entry.Obj.Offset) + int(r.DataOff(index))
		if size <= 0 || start < 0 || start+size > len(data) {
			t.Fatalf("mutation range [%d,%d) is outside %d-byte archive", start, start+size, len(data))
		}
		mutate(data[start : start+size])
		bad := filepath.Join(t.TempDir(), "damaged.a")
		if err := os.WriteFile(bad, data, 0o666); err != nil {
			t.Fatal(err)
		}
		return bad
	}
	t.Fatal("archive contains no Go object")
	return ""
}

func findGoObjSymbolIndex(t *testing.T, r *goobj.Reader, name string) uint32 {
	t.Helper()
	count := uint32(r.NSym() + r.NHashed64def() + r.NHasheddef() + r.NNonpkgdef())
	for i := uint32(0); i < count; i++ {
		if r.Sym(i).Name(r) == name {
			return i
		}
	}
	t.Fatalf("symbol %q not found", name)
	return 0
}

func localSymIndex(t *testing.T, r *goobj.Reader, ref goobj.SymRef) uint32 {
	t.Helper()
	switch ref.PkgIdx {
	case goobj.PkgIdxSelf:
		return ref.SymIdx
	case goobj.PkgIdxHashed64:
		return uint32(r.NSym()) + ref.SymIdx
	case goobj.PkgIdxHashed:
		return uint32(r.NSym()+r.NHashed64def()) + ref.SymIdx
	case goobj.PkgIdxNone:
		return uint32(r.NSym()+r.NHashed64def()+r.NHasheddef()) + ref.SymIdx
	default:
		t.Fatalf("symbol reference %+v is not local", ref)
		return 0
	}
}

var (
	goldenPC       = regexp.MustCompile(`(\+0x[0-9a-f]+) 0x[0-9a-f]+`)
	goldenX86Jump  = regexp.MustCompile(`\b(JBE|JE|JMP|CALL) 0x[0-9a-f]+`)
	goldenTextPath = regexp.MustCompile(`(TEXT [^\n]+\(SB\)) [^\n]+`)
)

func textGoldenSnapshot(text string) string {
	text = goldenTextPath.ReplaceAllString(text, "$1 fixture.go")
	text = goldenPC.ReplaceAllString(text, "$1 PC")
	text = goldenX86Jump.ReplaceAllString(text, "$1 <target>")

	var out strings.Builder
	selected := false
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if strings.HasPrefix(line, "GOOBJ ") {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(line, "TEXT ") {
			selected = strings.Contains(line, ".MetadataFixture(SB)") ||
				strings.Contains(line, ".IndirectFixture(SB)")
		}
		if !selected {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			line = "  " + trimmed
		}
		instruction := strings.HasPrefix(trimmed, "fixture.go:")
		if !instruction ||
			strings.Contains(line, "|") ||
			strings.Contains(line, "R_CALL") ||
			strings.Contains(line, "padding(") {
			out.WriteString(strings.TrimRight(line, " "))
			out.WriteByte('\n')
		}
	}
	return out.String()
}
