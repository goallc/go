// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmlto

import (
	"bytes"
	"cmd/internal/archive"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArchiveReplacement(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "input.a"), filepath.Join(dir, "output.a")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	a, err := archive.New(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ name, data string }{{"__.PKGDEF", "export data"}, {"asm.o", "assembly bytes"}, {irMember, "IR bytes"}} {
		a.AddEntry(archive.EntryNativeObj, e.name, 0, 0, 0, 0644, int64(len(e.data)), strings.NewReader(e.data))
	}
	f.Close()
	if err := rewriteArchive(src, dst, []byte("new object")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"export data", "assembly bytes", "new object"} {
		if !bytes.Contains(b, []byte(keep)) {
			t.Errorf("lost %q", keep)
		}
	}
	for _, drop := range []string{"old object", "IR bytes", irMember} {
		if bytes.Contains(b, []byte(drop)) {
			t.Errorf("retained %q", drop)
		}
	}
	original, err := readIR(src)
	if err != nil || string(original) != "IR bytes" {
		t.Fatalf("input modified: %q %v", original, err)
	}
}

func TestArchivePreservesMembers(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "input.a"), filepath.Join(dir, "output.a")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	a, err := archive.New(f)
	if err != nil {
		t.Fatal(err)
	}
	// Odd member sizes exercise archive padding. Metadata and order must also
	// survive, including duplicate assembly member names accepted by Go pack.
	for _, e := range []struct{ name, data string }{
		{"__.PKGDEF", "export data"}, {"asm.o", "first asm"},
		{irMember, "IR bytes!"}, {"asm.o", "other asm"},
	} {
		a.AddEntry(archive.EntryNativeObj, e.name, 7, 12, 34, 0755, int64(len(e.data)), strings.NewReader(e.data))
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rewriteArchive(src, dst, []byte("new object!")); err != nil {
		t.Fatal(err)
	}
	readMembers := func(path string) (names []string, preserved [][]byte) {
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
		for _, e := range a.Entries {
			if e.Name == irMember {
				names = append(names, "_go_.o")
				continue
			}
			names = append(names, e.Name)
			if e.Name == "_go_.o" {
				continue
			}
			raw, err := io.ReadAll(io.NewSectionReader(f, e.Offset-archive.HeaderSize, archive.HeaderSize+e.Size+(e.Size&1)))
			if err != nil {
				t.Fatal(err)
			}
			preserved = append(preserved, raw)
		}
		return
	}
	wantNames, wantMembers := readMembers(src)
	gotNames, gotMembers := readMembers(dst)
	if !reflect.DeepEqual(wantNames, gotNames) || !reflect.DeepEqual(wantMembers, gotMembers) {
		t.Fatalf("archive members changed: names %v -> %v, bytes %q -> %q", wantNames, gotNames, wantMembers, gotMembers)
	}
}

func TestInvalidArchiveMembers(t *testing.T) {
	for _, test := range []struct {
		name    string
		members []string
		readIR  bool
		want    string
	}{
		{"missing-bitcode", []string{"__.PKGDEF"}, false, "found 0"},
		{"duplicate-object", []string{"_go_.o", "_go_.o"}, false, "found 2"},
		{"duplicate-ir", []string{irMember, irMember}, true, "duplicate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			src, dst := filepath.Join(dir, "input.a"), filepath.Join(dir, "output.a")
			f, err := os.Create(src)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := archive.New(f); err != nil {
				t.Fatal(err)
			}
			for _, name := range test.members {
				if err := writeArchiveEntry(f, name, 9, strings.NewReader("abcdefghi")); err != nil {
					t.Fatal(err)
				}
			}
			f.Close()
			if test.readIR {
				_, err = readIR(src)
			} else {
				err = rewriteArchive(src, dst, nil)
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
			if _, err := os.Stat(dst); !os.IsNotExist(err) {
				t.Fatalf("invalid archive created an output: %v", err)
			}
		})
	}
}

type failingWriter struct{ remaining int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrShortWrite
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestArchiveWriteErrors(t *testing.T) {
	for _, limit := range []int{0, archive.HeaderSize, archive.HeaderSize + 3} {
		err := writeArchiveEntry(&failingWriter{limit}, irMember, 3, strings.NewReader("abc"))
		if !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("write limit %d: got %v", limit, err)
		}
	}
	if err := writeArchiveEntry(io.Discard, irMember, 3, strings.NewReader("a")); err == nil {
		t.Fatal("accepted truncated member")
	}
}
