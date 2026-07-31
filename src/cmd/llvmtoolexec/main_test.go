package main

import (
	"bytes"
	"cmd/internal/archive"
	"internal/testenv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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

func TestCompileInvocationClassification(t *testing.T) {
	if !isFullVersion([]string{"-enablellvm", "-V=full"}) {
		t.Fatal("-V=full was not recognized")
	}
	if isCompileAction([]string{"-V=full"}) {
		t.Fatal("version probe was classified as a compile action")
	}
	if !isCompileAction([]string{"-p=main", "-o", "out.a", "main.go"}) {
		t.Fatal("compile with output was not recognized")
	}
}

func TestBoolToolFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"absent", nil, false},
		{"bare", []string{"-enablellvm"}, true},
		{"true", []string{"-enablellvm=true"}, true},
		{"false", []string{"-enablellvm=false"}, false},
		{"last false", []string{"-enablellvm", "-enablellvm=false"}, false},
		{"last true", []string{"-enablellvm=false", "-enablellvm"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := boolToolFlag(test.args, "-enablellvm"); got != test.want {
				t.Fatalf("boolToolFlag(%q) = %v, want %v", test.args, got, test.want)
			}
		})
	}
}

func TestBackendIdentityTracksContentsNotTimestamp(t *testing.T) {
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "wrapper")
	llc := filepath.Join(dir, "llc")
	plugin := filepath.Join(dir, "plugin")
	library := filepath.Join(dir, "libLLVM")
	for path, content := range map[string]string{
		wrapper: "wrapper-v1",
		llc:     "llc-v1",
		plugin:  "plugin-v1",
		library: "libLLVM-v1",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := backendIdentity([]byte("compile version devel buildID=native\n"), wrapper, llc, plugin, library)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(plugin, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := backendIdentity([]byte("compile version devel buildID=native\n"), wrapper, llc, plugin, library)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("timestamp-only change altered identity: %q != %q", got, want)
	}
	for _, path := range []string{llc, plugin, library} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)+"-v2"), 0o600); err != nil {
			t.Fatal(err)
		}
		changed, err := backendIdentity([]byte("compile version devel buildID=native\n"), wrapper, llc, plugin, library)
		if err != nil {
			t.Fatal(err)
		}
		if changed == want {
			t.Fatalf("changing %s did not alter identity", filepath.Base(path))
		}
		if err := os.WriteFile(path, []byte(filepath.Base(path)+"-v1"), 0o600); err != nil {
			t.Fatal(err)
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

func TestLLVMIndirectCallStackCheck(t *testing.T) {
	llc := os.Getenv("GOALLC_LLC")
	plugin := os.Getenv("GOALLC_PASS_PLUGIN")
	if llc == "" || plugin == "" {
		t.Skip("requires GOALLC_LLC and GOALLC_PASS_PLUGIN")
	}
	testenv.MustHaveGoBuild(t)

	root := testenv.GOROOT(t)
	goTool := testenv.GoToolPath(t)
	wrapper := filepath.Join(t.TempDir(), "llvmtoolexec")
	buildWrapper := testenv.Command(t, goTool, "build", "-o", wrapper, "./src/cmd/llvmtoolexec")
	buildWrapper.Dir = root
	if out, err := buildWrapper.CombinedOutput(); err != nil {
		t.Fatalf("building llvmtoolexec: %v\n%s", err, out)
	}

	executable := filepath.Join(t.TempDir(), "indirect")
	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}, " ")
	buildFixture := testenv.Command(
		t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags=-enablellvm",
		"-ldflags=-w -debugnosplit",
		"-o", executable,
		"./src/cmd/llvmtoolexec/testdata/indirect",
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	out, err := buildFixture.CombinedOutput()
	if err != nil {
		t.Fatalf("building LLVM indirect-call fixture: %v\n%s", err, out)
	}
	if !regexp.MustCompile(`(?m)^nosplit: main\.invoke<\d+> \+\d+ -> indirect$`).Match(out) {
		t.Fatalf("linker stackcheck did not consume R_CALLIND for main.invoke:\n%s", out)
	}

	runFixture := testenv.Command(t, executable)
	if out, err := runFixture.CombinedOutput(); err != nil {
		t.Fatalf("running LLVM indirect-call fixture: %v\n%s", err, out)
	}

	archive := filepath.Join(t.TempDir(), "indirect.a")
	source := filepath.Join(root, "src", "cmd", "llvmtoolexec", "testdata", "indirect", "main.go")
	compileFixture := testenv.Command(
		t, goTool, "tool", "compile",
		"-p=main", "-enablellvm", "-llvmironly",
		"-o", archive, source,
	)
	if out, err := compileFixture.CombinedOutput(); err != nil {
		t.Fatalf("compiling LLVM indirect-call fixture: %v\n%s", err, out)
	}
	object := filepath.Join(t.TempDir(), "indirect.o")
	runLLC := testenv.Command(
		t, llc,
		"-load-pass-plugin="+plugin,
		"-filetype=obj",
		"-o", object,
		archive+".ll",
	)
	if out, err := runLLC.CombinedOutput(); err != nil {
		t.Fatalf("writing indirect-call GoObj: %v\n%s", err, out)
	}
	objdump := testenv.Command(t, goTool, "tool", "objdump", object)
	objdumpOutput, err := objdump.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool objdump rejected LLVM GoObj: %v\n%s", err, objdumpOutput)
	}
	if !strings.Contains(string(objdumpOutput), "TEXT main.invoke(SB)") ||
		!strings.Contains(string(objdumpOutput), "R_CALLIND") {
		t.Fatalf("go tool objdump omitted indirect-call metadata:\n%s", objdumpOutput)
	}
}
