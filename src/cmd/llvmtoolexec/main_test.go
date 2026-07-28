package main

import (
	"bytes"
	"cmd/internal/archive"
	"os"
	"path/filepath"
	"strings"
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

func TestResolvePassPluginExplicit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePassPlugin("unused-llc", path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolvePassPlugin explicit path = %q, want %q", got, want)
	}
}

func TestResolvePassPluginNextToLLC(t *testing.T) {
	root := t.TempDir()
	llc := filepath.Join(root, "bin", "llc")
	if err := os.MkdirAll(filepath.Dir(llc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(llc, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	filename, err := passPluginFilename()
	if err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(root, "lib", filename)
	if err := os.MkdirAll(filepath.Dir(plugin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePassPlugin(llc, "")
	if err != nil {
		t.Fatal(err)
	}
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Stat(plugin)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("resolvePassPlugin next to llc = %q, want %q", got, plugin)
	}
}

func TestResolvePassPluginNextToLLCSymlink(t *testing.T) {
	payload := filepath.Join(t.TempDir(), "payload")
	realRoot := filepath.Join(t.TempDir(), "build")
	realLLC := filepath.Join(realRoot, "bin", "llc")
	if err := os.MkdirAll(filepath.Dir(realLLC), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realLLC, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	llc := filepath.Join(payload, "bin", "llc")
	if err := os.MkdirAll(filepath.Dir(llc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realLLC, llc); err != nil {
		t.Fatal(err)
	}
	filename, err := passPluginFilename()
	if err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(payload, "lib", filename)
	if err := os.MkdirAll(filepath.Dir(plugin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePassPlugin(llc, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != plugin {
		t.Fatalf("resolvePassPlugin next to llc symlink = %q, want %q", got, plugin)
	}
}

func TestResolvePassPluginMissing(t *testing.T) {
	root := t.TempDir()
	llc := filepath.Join(root, "bin", "llc")
	if err := os.MkdirAll(filepath.Dir(llc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(llc, nil, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := resolvePassPlugin(llc, "")
	if err == nil || !strings.Contains(err.Error(), "pass plugin not found") {
		t.Fatalf("resolvePassPlugin missing error = %v", err)
	}
}

func TestAppendArchiveMember(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "p.a")
	member := filepath.Join(dir, "member.o")
	arFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	ar, err := archive.New(arFile)
	if err != nil {
		t.Fatal(err)
	}
	ar.AddEntry(archive.EntryPkgDef, "__.PKGDEF", 0, 0, 0, 0o644, 6, bytes.NewReader([]byte("pkgdef")))
	if err := arFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(member, []byte("native-o"), 0o600); err != nil {
		t.Fatal(err)
	}
	appendArchiveMember(archivePath, "_go_.o", member)
	arFile, err = os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer arFile.Close()
	parsed, err := archive.Parse(arFile, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(parsed.Entries), 2; got != want {
		t.Fatalf("archive entries = %d, want %d", got, want)
	}
	if got, want := parsed.Entries[0].Name, "__.PKGDEF"; got != want {
		t.Fatalf("first archive member = %q, want %q", got, want)
	}
	if got, want := parsed.Entries[1].Name, "_go_.o"; got != want {
		t.Fatalf("second archive member = %q, want %q", got, want)
	}
}
