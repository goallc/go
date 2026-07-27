package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolFlag(t *testing.T) {
	args := []string{"-p=example.com/p", "-o", "out.a", "-shared"}
	for _, test := range []struct {
		name string
		want string
		ok   bool
	}{
		{"-p", "example.com/p", true},
		{"-o", "out.a", true},
		{"-goversion", "", false},
	} {
		got, ok := toolFlag(args, test.name)
		if got != test.want || ok != test.ok {
			t.Errorf("toolFlag(%q) = %q, %v; want %q, %v", test.name, got, ok, test.want, test.ok)
		}
	}
}

func TestDefaultTriple(t *testing.T) {
	for _, test := range []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "aarch64-apple-darwin-goobj"},
		{"linux", "amd64", "x86_64-unknown-linux-goobj"},
	} {
		got, err := defaultTriple(test.goos, test.goarch)
		if err != nil || got != test.want {
			t.Errorf("defaultTriple(%q, %q) = %q, %v; want %q", test.goos, test.goarch, got, err, test.want)
		}
	}
	if _, err := defaultTriple("darwin", "amd64"); err == nil {
		t.Error("defaultTriple accepted unsupported target")
	}
}

func TestAppendArchiveMember(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "p.a")
	member := filepath.Join(dir, "member.o")
	if err := os.WriteFile(archive, []byte("!<arch>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(member, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	appendArchiveMember(archive, "member.o", member)
	got, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 8+60+3+1 {
		t.Fatalf("archive length = %d, want %d", len(got), 8+60+3+1)
	}
	if got[8+60+3] != 0 {
		t.Fatalf("archive padding = %d, want 0", got[8+60+3])
	}
}

func TestArchiveGoObjectHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.a")
	payload := []byte("go object darwin arm64 go1.27 X:one,two\n\nexport data")
	header := []byte("!<arch>\n" + formatArchiveHeader("__.PKGDEF", len(payload)))
	if err := os.WriteFile(path, append(header, payload...), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := archiveGoObjectHeader(path)
	if err != nil || goObjExperiments(got) != "one,two" {
		t.Fatalf("archiveGoObjectHeader() = %q, %v; want X:one,two", got, err)
	}
}

func TestDecorateGoObj(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obj.o")
	if err := os.WriteFile(path, []byte("go object darwin arm64 go1.27\n\n!\n\x00go"), 0o600); err != nil {
		t.Fatal(err)
	}
	decorateGoObj(path, true)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "go object darwin arm64 go1.27\nmain\n\n!\n\x00go"
	if string(got) != want {
		t.Fatalf("wrapped object = %q, want %q", got, want)
	}
}
