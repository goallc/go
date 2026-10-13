// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package main

import (
	"bytes"
	"cmd/internal/archive"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestArchiveReplacement(t *testing.T) {
	dir := t.TempDir()
	src, dst, obj := filepath.Join(dir, "input.a"), filepath.Join(dir, "output.a"), filepath.Join(dir, "new.o")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	a, err := archive.New(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ name, data string }{{"__.PKGDEF", "export data"}, {"_go_.o", "old object"}, {"asm.o", "assembly bytes"}, {irMember, "IR bytes"}} {
		a.AddEntry(archive.EntryNativeObj, e.name, 0, 0, 0, 0644, int64(len(e.data)), strings.NewReader(e.data))
	}
	f.Close()
	if err := os.WriteFile(obj, []byte("new object"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rewriteArchive(src, dst, obj); err != nil {
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
	src, dst, obj := filepath.Join(dir, "input.a"), filepath.Join(dir, "output.a"), filepath.Join(dir, "new.o")
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
		{"__.PKGDEF", "export data"}, {"_go_.o", "old object!"}, {"asm.o", "first asm"},
		{irMember, "IR bytes!"}, {"asm.o", "other asm"},
	} {
		a.AddEntry(archive.EntryNativeObj, e.name, 7, 12, 34, 0755, int64(len(e.data)), strings.NewReader(e.data))
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(obj, []byte("new object!"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rewriteArchive(src, dst, obj); err != nil {
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
		{"missing-object", []string{irMember}, false, "found 0"},
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
				err = rewriteArchive(src, dst, "unused.o")
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

func TestSupportedOptions(t *testing.T) {
	for _, args := range [][]string{
		nil, {"-enablellvm", "-llvm-keep-ir"}, {"-enablellvm=false", "-enablellvm=true"},
		{"-N", "-N=0", "-l"}, {"-shared=false", "-race=false", "-llvm-opt-passes=auto"},
	} {
		if err := validateCompileArgs(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"-enablellvm=false"}, {"--enablellvm=false"}, {"-llvm-keep-ir=false"}, {"-N"}, {"-N=1"}, {"--N"},
		{"-shared"}, {"-dynlink"}, {"-linkshared"}, {"-race"}, {"-msan"}, {"-asan"},
		{"-llvm-opt-passes=none"}, {"--llvm-opt-passes=none"}, {"-llvm-opt-passes", ""}, {"-linkobj", "separate.o"},
	} {
		if err := validateCompileArgs(args); err == nil {
			t.Errorf("accepted incompatible compiler options %v", args)
		}
	}
	for _, args := range [][]string{{"-buildmode=exe", "main.a"}, {"-race=false", "main.a"}} {
		if err := validateLinkArgs(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{nil, {"-buildmode=pie", "main.a"}, {"-buildmode", "c-archive", "main.a"}, {"-linkshared", "main.a"}, {"-race", "main.a"}} {
		if err := validateLinkArgs(args); err == nil {
			t.Errorf("accepted incompatible linker options %v", args)
		}
	}
	args := []string{"-importcfg=first", "-importcfg", "last"}
	if got := argValue(args, "-importcfg"); got != "last" {
		t.Fatalf("effective importcfg = %q, want last", got)
	}
	replaceArg(args, "-importcfg", "replacement")
	if got := argValue(args, "-importcfg"); got != "replacement" {
		t.Fatalf("effective importcfg after replacement = %q", got)
	}
}

func TestCacheIdentity(t *testing.T) {
	dir := t.TempDir()
	bin, lib := filepath.Join(dir, "bin"), filepath.Join(dir, "lib")
	for _, path := range []string{bin, lib} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	llcPath, library := filepath.Join(bin, "llc"), filepath.Join(lib, "libLLVM.so.23")
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(llcPath, "a")
	write(library, "bc")
	libraries, err := llvmLibraries(llcPath, filepath.Join(lib, "plugin.so"))
	if err != nil || len(libraries) == 0 {
		t.Fatalf("shared libraries not found: %v, %v", libraries, err)
	}
	paths := append([]string{llcPath}, libraries...)
	identity := func(selection, version string) string {
		t.Helper()
		id, err := cacheIdentity(paths, selection, []byte(version))
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	base := identity("all", "compile version")
	if identity("example.com/lto/...", "compile version") == base || identity("all", "new compiler version") == base {
		t.Fatal("package selection or compiler version did not change cache identity")
	}
	write(library, "bd")
	if identity("all", "compile version") == base {
		t.Fatal("shared LLVM library update did not change cache identity")
	}
	// Same concatenated bytes, different file boundaries.
	write(llcPath, "ab")
	write(library, "c")
	if identity("all", "compile version") == base {
		t.Fatal("cache identity does not frame artifact contents")
	}
}

func TestFullLTO(t *testing.T) {
	if (runtime.GOOS != "darwin" && runtime.GOOS != "linux") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Skip("FullLTO runtime fixture supports darwin/linux on amd64/arm64")
	}
	helper := os.Getenv("GOALLC_LTO_BACKEND")
	llcPath := os.Getenv("GOALLC_LLC")
	pluginPath := os.Getenv("GOALLC_PASS_PLUGIN")
	if helper == "" || llcPath == "" || pluginPath == "" {
		t.Skip("set GOALLC_LTO_BACKEND, GOALLC_LLC and GOALLC_PASS_PLUGIN")
	}
	goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
	dir := t.TempDir()
	driver := filepath.Join(dir, "llvmlto")
	run := func(cwd string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
		if args[0] != goTool {
			cmd.Env = append(cmd.Env, "GOGC=1")
		}
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", args[0], err, b)
		}
		return string(b)
	}
	run("", goTool, "build", "-gcflags=all=-enablellvm=false", "-o", driver, "cmd/llvmlto")
	fixture, err := filepath.Abs("testdata/runtime")
	if err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(dir, "baseline")
	run(fixture, goTool, "build", "-buildmode=exe", "-gcflags=all=-enablellvm", "-gcflags=example.com/lto=-enablellvm -l", "-o", baseline, ".")
	want := run("", baseline)
	if !strings.Contains(want, "LTO RUNTIME OK") {
		t.Fatal(want)
	}
	for _, p := range []string{driver, helper, llcPath, pluginPath} {
		if strings.ContainsAny(p, " \t\n\"'") {
			t.Fatal("test tool paths must not contain whitespace or quotes")
		}
	}
	wrapper := strings.Join([]string{driver, "-packages=example.com/lto/...", "-keep", "-backend=" + helper, "-llc=" + llcPath, "-plugin=" + pluginPath}, " ")
	output := filepath.Join(dir, "lto")
	// A per-test link ID prevents a cached executable from hiding the LTO
	// work directory. It leaves dependencies eligible for the Go build cache.
	linkID := "lto-" + filepath.Base(filepath.Dir(dir))
	log := run(fixture, goTool, "build", "-buildmode=exe", "-ldflags=-buildid="+linkID, "-gcflags=all=-enablellvm", "-gcflags=example.com/lto=-enablellvm -l", "-toolexec="+wrapper, "-o", output, ".")
	if got := run("", output); got != want {
		t.Fatalf("LTO output %q, baseline %q", got, want)
	}
	var work string
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "LTO_WORK=") {
			work = strings.TrimPrefix(line, "LTO_WORK=")
		}
	}
	if work == "" {
		t.Fatalf("LTO did not run: %s", log)
	}
	defer os.RemoveAll(work)
	before, err := filepath.Glob(filepath.Join(work, "*.input.ll"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := filepath.Glob(filepath.Join(work, "optimized", "*.ll"))
	if err != nil {
		t.Fatal(err)
	}
	hasCall := func(files []string, name string) bool {
		for _, file := range files {
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			s := string(b)
			start := strings.Index(s, "define goabiinternal void @main.main(")
			if start < 0 {
				continue
			}
			s = s[start:]
			if end := strings.Index(s, "\n}"); end >= 0 {
				s = s[:end]
			}
			for _, line := range strings.Split(s, "\n") {
				if strings.Contains(line, "call ") && strings.Contains(line, name) {
					return true
				}
			}
		}
		return false
	}
	if !hasCall(before, "example.com/lto/dep.Add") {
		t.Fatal("fixture did not preserve the pre-LTO cross-package call")
	}
	if hasCall(after, "example.com/lto/dep.Add") {
		t.Fatal("LLVM did not eliminate the cross-package call")
	}
	if !hasCall(after, "example.com/lto/dep.Opaque") {
		t.Fatal("LLVM did not preserve the //go:noinline call")
	}
	// Force a new link action while reusing cached dependency fat archives.
	// Changing ldflags may rebuild main, which embeds the build settings.
	cacheLog := run(fixture, goTool, "build", "-buildmode=exe", "-x", "-gcflags=all=-enablellvm", "-gcflags=example.com/lto=-enablellvm -l", "-ldflags=-buildid="+linkID+"-cache-probe", "-toolexec="+wrapper, "-o", output, ".")
	cacheLinked := false
	for _, line := range strings.Split(cacheLog, "\n") {
		if strings.Contains(line, "/compile ") && !strings.Contains(line, " -V=full") && !strings.Contains(line, " -p main ") {
			t.Fatalf("cache probe unexpectedly compiled a dependency: %s", line)
		}
		if strings.HasPrefix(line, "LTO_WORK=") {
			cacheLinked = true
			defer os.RemoveAll(strings.TrimPrefix(line, "LTO_WORK="))
		}
	}
	if !cacheLinked {
		t.Fatal("cache test did not execute a fresh link action")
	}
	if got := run("", output); got != want {
		t.Fatalf("cache-hit output: %q", got)
	}
}
