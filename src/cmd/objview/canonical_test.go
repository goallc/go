// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmd/internal/archive"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"internal/testenv"
)

func TestCanonicalGoObjectAndArchive(t *testing.T) {
	archivePath := compileFixture(t)
	doc, err := parseCanonicalFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Archive {
		t.Fatal("compiler -pack output was not recognized as an archive")
	}
	if len(doc.Members) < 2 {
		t.Fatalf("archive contains %d members, want at least package definition and Go object", len(doc.Members))
	}

	object := onlyGoObject(t, doc)
	fn := findSymbol(t, object, "example.com/fixture.MetadataFixture")
	if fn.Function == nil || fn.Function.Info == nil {
		t.Fatalf("%s has no decoded FuncInfo", fn.Name)
	}
	for _, kind := range []string{"pcsp", "pcfile", "pcline", "pcinline"} {
		if findPCTable(fn.Function.PCTables, kind) == nil {
			t.Errorf("%s has no %s table", fn.Name, kind)
		}
	}
	stackMapIndex := findPCTable(fn.Function.PCData, "stack_map_index")
	if stackMapIndex == nil || len(stackMapIndex.Ranges) == 0 {
		t.Errorf("%s has no PCDATA stack map index", fn.Name)
	}
	if fn.Function.PCQuantum <= 0 {
		t.Errorf("%s has invalid PC quantum %d", fn.Name, fn.Function.PCQuantum)
	}
	if len(fn.Function.StackMapQueries) == 0 {
		t.Errorf("%s has no normalized stack-map queries", fn.Name)
	}
	for _, query := range fn.Function.StackMapQueries {
		if query.ReturnPC == 0 || query.LookupPC != query.ReturnPC-1 {
			t.Errorf("bad normalized stack-map query: %+v", query)
		}
		if stackMapIndex != nil &&
			query.StackMapIndex != lookupPCValue(stackMapIndex.Ranges, query.LookupPC) {
			t.Errorf("query does not match raw PCDATA ranges: %+v", query)
		}
	}
	indirect := findSymbol(t, object, "example.com/fixture.IndirectFixture")
	if indirect.Function == nil || len(indirect.Function.StackMapQueries) == 0 {
		t.Fatalf("%s has no normalized indirect-call query", indirect.Name)
	}
	foundIndirect := false
	for _, query := range indirect.Function.StackMapQueries {
		if query.RelocationType != "R_CALLIND" {
			continue
		}
		foundIndirect = true
		if query.DecodeError != "" || query.InstructionSize == 0 || query.ReturnPC == 0 {
			t.Errorf("bad indirect-call query: %+v", query)
		}
	}
	if !foundIndirect {
		t.Errorf("%s has no R_CALLIND query", indirect.Name)
	}
	for _, kind := range []string{"args_pointer_maps", "locals_pointer_maps"} {
		fd := findFuncData(fn.Function.FuncData, kind)
		if fd == nil || fd.StackMap == nil {
			t.Errorf("%s has no decoded %s", fn.Name, kind)
		}
	}
	stackObjects := findFuncData(fn.Function.FuncData, "stack_objects")
	if stackObjects == nil || len(stackObjects.StackObjects) == 0 {
		t.Fatalf("%s has no decoded stack objects", fn.Name)
	}
	if stackObjects.StackObjects[0].GCData == nil {
		t.Errorf("%s stack object has no resolved GC bitmap relocation", fn.Name)
	}

	first, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	again, err := parseCanonicalFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.MarshalIndent(again, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical JSON is not deterministic")
	}

	rawPath := extractRawGoObject(t, archivePath)
	raw, err := parseCanonicalFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Archive {
		t.Fatal("standalone Go object was reported as an archive")
	}
	if len(raw.Members) != 1 || raw.Members[0].Name != "" {
		t.Fatalf("standalone Go object has unexpected members: %+v", raw.Members)
	}
	if !reflect.DeepEqual(object, raw.Members[0].GoObject) {
		t.Fatal("archive member and standalone Go object decoded differently")
	}
}

func TestCanonicalRejectsDamagedInput(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.o")
		if err := os.WriteFile(path, []byte("go objec"), 0o666); err != nil {
			t.Fatal(err)
		}
		if _, err := parseCanonicalFile(path); err == nil {
			t.Fatal("truncated input unexpectedly parsed")
		}
	})

	t.Run("block offset", func(t *testing.T) {
		rawPath := extractRawGoObject(t, compileFixture(t))
		data, err := os.ReadFile(rawPath)
		if err != nil {
			t.Fatal(err)
		}
		magic := bytes.Index(data, []byte("\x00go120ld"))
		if magic < 0 {
			t.Fatal("Go object binary magic not found")
		}
		const firstBlockOffset = len("\x00go120ld") + 8 + 4
		binary.LittleEndian.PutUint32(data[magic+firstBlockOffset:], ^uint32(0))
		path := filepath.Join(t.TempDir(), "bad-offset.o")
		if err := os.WriteFile(path, data, 0o666); err != nil {
			t.Fatal(err)
		}
		_, err = parseCanonicalFile(path)
		if err == nil || !strings.Contains(err.Error(), "offset") {
			t.Fatalf("bad block offset error = %v", err)
		}
	})
}

func TestCanonicalAMD64IndirectCall(t *testing.T) {
	doc, err := parseCanonicalFile(compileFixtureFor(t, "linux", "amd64"))
	if err != nil {
		t.Fatal(err)
	}
	fn := findSymbol(t, onlyGoObject(t, doc), "example.com/fixture.IndirectFixture")
	if fn.Function == nil || fn.Function.PCQuantum != 1 {
		t.Fatalf("amd64 function PC quantum = %v", fn.Function)
	}
	for _, query := range fn.Function.StackMapQueries {
		if query.RelocationType == "R_CALLIND" {
			if query.DecodeError != "" || query.InstructionSize == 0 ||
				query.LookupPC != query.ReturnPC-1 {
				t.Fatalf("bad amd64 indirect-call query: %+v", query)
			}
			return
		}
	}
	t.Fatal("amd64 function has no R_CALLIND query")
}

func TestCanonicalArchiveMemberKinds(t *testing.T) {
	archivePath := compileFixture(t)
	dir := t.TempDir()
	native := filepath.Join(dir, "native.o")
	sentinel := filepath.Join(dir, "preferlinkext")
	if err := os.WriteFile(native, []byte("NATIVE00"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "pack", "r",
		archivePath, native, sentinel)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add archive members: %v\n%s", err, output)
	}
	doc, err := parseCanonicalFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	kinds := make(map[string]bool)
	for _, member := range doc.Members {
		kinds[member.Kind] = true
	}
	for _, kind := range []string{"package_definition", "go_object", "native_object", "sentinel"} {
		if !kinds[kind] {
			t.Errorf("archive has no %s member", kind)
		}
	}
}

func TestLLVMGoObject(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "multiple-calls.goobj.base64"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "multiple-calls.goobj")
	if err := os.WriteFile(path, data, 0o666); err != nil {
		t.Fatal(err)
	}

	// The canonical parser owns LLVM-emitted PC metadata in the non-package
	// definition class. It must not depend on cmd/internal/objfile accepting
	// this GoALLC-specific representation.
	doc, err := parseCanonicalFile(path)
	if err != nil {
		t.Fatalf("canonical parse: %v", err)
	}
	object := onlyGoObject(t, doc)
	fn := findSymbol(t, object, "different_pointer_sets_across_calls")
	if fn.Function == nil {
		t.Fatal("LLVM text symbol has no decoded function metadata")
	}
	for _, kind := range []string{"pcfile", "pcline"} {
		table := findPCTable(fn.Function.PCTables, kind)
		if table == nil || table.Error != "" || len(table.Ranges) == 0 {
			t.Fatalf("LLVM %s table was not decoded: %+v", kind, table)
		}
	}
	pcfile := findPCTable(fn.Function.PCTables, "pcfile")
	if pcfile.Ranges[0].File != "llvm-ir" {
		t.Fatalf("LLVM pcfile range = %+v, want llvm-ir", pcfile.Ranges[0])
	}
	references := make(map[string]bool)
	for _, ref := range object.References {
		references[ref.Name] = true
	}
	for _, name := range []string{"first_callee", "second_callee", "runtime.morestack_noctxt"} {
		if !references[name] {
			t.Errorf("LLVM object has no reference to %q", name)
		}
	}

	var raw bytes.Buffer
	if err := writeRawFile(&raw, path); err != nil {
		t.Fatalf("raw output: %v", err)
	}
	for _, want := range []string{
		"GOOBJRAW 2 archive=false members=1",
		"OFFSET    HEX BYTES",
		"| INTERPRETATION",
		"00000000  00 67 6f",
		`name="different_pointer_sets_across_calls"`,
		"-- block[16] data",
		`function="different_pointer_sets_across_calls"`,
		`name="runtime.morestack_noctxt"`,
	} {
		if !strings.Contains(raw.String(), want) {
			t.Errorf("raw output is missing %q", want)
		}
	}
}

func compileFixture(t *testing.T) string {
	return compileFixtureFor(t, "", "")
}

func compileFixtureFor(t *testing.T, goos, goarch string) string {
	t.Helper()
	testenv.MustHaveGoBuild(t)
	out := filepath.Join(t.TempDir(), "fixture.a")
	cmd := testenv.Command(t, testenv.GoToolPath(t), "tool", "compile",
		"-pack", "-p", "example.com/fixture", "-o", out,
		filepath.Join("testdata", "fixture.go"))
	if goos != "" {
		cmd.Env = append(cmd.Environ(), "GOOS="+goos, "GOARCH="+goarch)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile fixture: %v\n%s", err, output)
	}
	return out
}

func extractRawGoObject(t *testing.T, archivePath string) string {
	t.Helper()
	f, err := os.Open(archivePath)
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
		data := make([]byte, entry.Size)
		if _, err := f.ReadAt(data, entry.Offset); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "_go_.o")
		if err := os.WriteFile(out, data, 0o666); err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Fatal("archive contains no Go object")
	return ""
}

func onlyGoObject(t *testing.T, doc *canonicalDocument) *canonicalGoObject {
	t.Helper()
	var found *canonicalGoObject
	for i := range doc.Members {
		if doc.Members[i].GoObject == nil {
			continue
		}
		if found != nil {
			t.Fatal("document contains more than one Go object")
		}
		found = doc.Members[i].GoObject
	}
	if found == nil {
		t.Fatal("document contains no Go object")
	}
	return found
}

func findSymbol(t *testing.T, object *canonicalGoObject, name string) *canonicalSymbol {
	t.Helper()
	for i := range object.Symbols {
		if object.Symbols[i].Name == name {
			return &object.Symbols[i]
		}
	}
	t.Fatalf("symbol %q not found", name)
	return nil
}

func findPCTable(tables []canonicalPCTable, kind string) *canonicalPCTable {
	for i := range tables {
		if tables[i].Kind == kind {
			return &tables[i]
		}
	}
	return nil
}

func findFuncData(all []canonicalFuncData, kind string) *canonicalFuncData {
	for i := range all {
		if all[i].Kind == kind {
			return &all[i]
		}
	}
	return nil
}
