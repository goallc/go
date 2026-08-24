// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmd/internal/archive"
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"encoding/json"
	"internal/testenv"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

type importObjviewTarget struct {
	Package  string `json:"package"`
	Name     string `json:"name"`
	Kind     string `json:"pkg_kind"`
	SymIndex uint32 `json:"sym_index"`
}

type importObjviewObject struct {
	Autolib []struct {
		Package     string `json:"package"`
		Fingerprint string `json:"fingerprint"`
	} `json:"autolib"`
	Packages   []string `json:"packages"`
	References []struct {
		Name  string `json:"name"`
		Class string `json:"class"`
	} `json:"references"`
	Symbols []struct {
		Index       uint32   `json:"index"`
		Name        string   `json:"name"`
		Kind        string   `json:"kind"`
		FlagNames   []string `json:"flag_names"`
		Auxiliaries []struct {
			Type   string              `json:"type"`
			Target importObjviewTarget `json:"target"`
		} `json:"aux"`
		Relocations []struct {
			Size   uint8               `json:"size"`
			Type   string              `json:"type"`
			Target importObjviewTarget `json:"target"`
		} `json:"relocations"`
	} `json:"symbols"`
}

func TestLLVMReflectMethodReachability(t *testing.T) {
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

	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}, " ")
	cache := t.TempDir()
	packagePath := "cmd/llvmtoolexec/testdata/reflectmethod"
	packageArg := "./src/" + packagePath
	executable := filepath.Join(t.TempDir(), "reflectmethod")
	buildFixture := testenv.Command(
		t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		"-ldflags=-w",
		"-o", executable,
		packageArg,
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building LLVM reflect-method fixture: %v\n%s", err, out)
	}
	if out, err := testenv.Command(t, executable).CombinedOutput(); err != nil {
		t.Fatalf("running LLVM reflect-method fixture: %v\n%s", err, out)
	}

	listFixture := testenv.Command(
		t, goTool, "list", "-export", "-f={{.Export}}",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		packageArg,
	)
	listFixture.Dir = root
	listFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	archiveOutput, err := listFixture.CombinedOutput()
	if err != nil {
		t.Fatalf("locating LLVM reflect-method archive: %v\n%s", err, archiveOutput)
	}
	object := readImportObjview(t, goTool, strings.TrimSpace(string(archiveOutput)))
	for _, symbol := range object.Symbols {
		if symbol.Name != "main.main" && symbol.Name != packagePath+".main" {
			continue
		}
		for _, flag := range symbol.FlagNames {
			if flag == "reflect_method" {
				return
			}
		}
		t.Fatalf("main symbol flags = %v, want reflect_method", symbol.FlagNames)
	}
	t.Fatal("LLVM reflect-method object has no main function symbol")
}

func readImportObjview(t *testing.T, goTool, archivePath string) *importObjviewObject {
	t.Helper()
	objview := testenv.Command(t, goTool, "tool", "objview", "-format=json", archivePath)
	output, err := objview.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool objview rejected %s: %v\n%s", archivePath, err, output)
	}
	var view struct {
		Members []struct {
			GoObject *importObjviewObject `json:"go_object"`
		} `json:"members"`
	}
	if err := json.Unmarshal(output, &view); err != nil {
		t.Fatalf("decoding objview JSON for %s: %v\n%s", archivePath, err, output)
	}
	for _, member := range view.Members {
		if member.GoObject != nil {
			return member.GoObject
		}
	}
	t.Fatalf("objview JSON for %s has no Go object member", archivePath)
	return nil
}

func importObjviewRefs(object *importObjviewObject, names map[string]string) map[string]importObjviewTarget {
	refs := make(map[string]importObjviewTarget)
	for _, symbol := range object.Symbols {
		for _, reloc := range symbol.Relocations {
			if _, ok := names[reloc.Target.Name]; ok {
				refs[reloc.Target.Name] = reloc.Target
			}
		}
	}
	return refs
}

func TestLLVMImportedPackageReferences(t *testing.T) {
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

	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}, " ")
	cache := t.TempDir()
	packagePath := "cmd/llvmtoolexec/testdata/importrefs"
	packageArg := "./src/" + packagePath
	executable := filepath.Join(t.TempDir(), "importrefs")
	buildFixture := testenv.Command(
		t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		"-o", executable,
		packageArg,
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building LLVM import fixture: %v\n%s", err, out)
	}

	runFixture := testenv.Command(t, executable)
	runFixture.Env = append(os.Environ(), "GOALLC_IMPORT_VALUE=ok")
	if out, err := runFixture.CombinedOutput(); err != nil {
		t.Fatalf("running LLVM import fixture: %v\n%s", err, out)
	}

	listFixture := testenv.Command(
		t, goTool, "list", "-export", "-f={{.Export}}",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		packageArg,
	)
	listFixture.Dir = root
	listFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	archiveOutput, err := listFixture.CombinedOutput()
	if err != nil {
		t.Fatalf("locating LLVM import archive: %v\n%s", err, archiveOutput)
	}
	archivePath := strings.TrimSpace(string(archiveOutput))
	if archivePath == "" {
		t.Fatal("go list returned an empty LLVM import archive path")
	}

	object := readImportObjview(t, goTool, archivePath)
	hasFlag := func(flags []string, want string) bool {
		for _, flag := range flags {
			if flag == want {
				return true
			}
		}
		return false
	}
	var sawDupokFunction, sawLocalGlobal bool
	for _, symbol := range object.Symbols {
		if strings.HasSuffix(symbol.Name, ".box[int]") && hasFlag(symbol.FlagNames, "dupok") {
			sawDupokFunction = true
		}
		if strings.HasPrefix(symbol.Name, ".goallc.anon.") && hasFlag(symbol.FlagNames, "local") {
			sawLocalGlobal = true
		}
	}
	if !sawDupokFunction {
		t.Error("generic wrapper did not derive GoObj dupok from weak LLVM linkage")
	}
	if !sawLocalGlobal {
		t.Error("anonymous global did not derive GoObj local from internal LLVM linkage")
	}

	nativeList := testenv.Command(
		t, goTool, "list", "-export", "-f={{.Export}}", packageArg,
	)
	nativeList.Dir = root
	nativeList.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	nativeArchiveOutput, err := nativeList.CombinedOutput()
	if err != nil {
		t.Fatalf("locating native import archive: %v\n%s", err, nativeArchiveOutput)
	}
	nativeArchivePath := strings.TrimSpace(string(nativeArchiveOutput))
	if nativeArchivePath == "" {
		t.Fatal("go list returned an empty native import archive path")
	}
	nativeObject := readImportObjview(t, goTool, nativeArchivePath)

	imports := make(map[string]string)
	for _, imp := range object.Autolib {
		imports[imp.Package] = imp.Fingerprint
	}
	nativeImports := make(map[string]string)
	for _, imp := range nativeObject.Autolib {
		nativeImports[imp.Package] = imp.Fingerprint
	}
	for _, want := range []string{"encoding/hex", "fmt", "os", "reflect", packagePath + "/dep"} {
		if imports[want] == "" {
			t.Errorf("Autolib does not contain %q: %v", want, imports)
		} else if imports[want] != nativeImports[want] {
			t.Errorf("Autolib fingerprint for %q = %s, native %s",
				want, imports[want], nativeImports[want])
		}
	}
	if len(object.Packages) == 0 || object.Packages[0] != "" {
		t.Errorf("PkgIdx has no dummy index 0: %v", object.Packages)
	}
	linknameRefs := make(map[string]string)
	for _, ref := range object.References {
		linknameRefs[ref.Name] = ref.Class
	}
	for _, name := range []string{"reflect.(*rtype).Elem", "reflect.(*rtype).Kind"} {
		if got := linknameRefs[name]; got != "nonpackage_reference" {
			t.Errorf("reference class for imported linkname %s = %q, want nonpackage_reference", name, got)
		}
	}

	wantRefs := map[string]string{
		"fmt.Sprintf": "fmt",
		"os.Getenv":   "os",
		"cmd/llvmtoolexec/testdata/importrefs/dep.Value": packagePath + "/dep",
	}
	refs := importObjviewRefs(object, wantRefs)
	nativeRefs := importObjviewRefs(nativeObject, wantRefs)
	for name, wantPackage := range wantRefs {
		ref, ok := refs[name]
		if !ok {
			t.Errorf("no package-indexed relocation for %s", name)
			continue
		}
		if ref.Kind != "imported" || ref.Package != wantPackage {
			t.Errorf("%s target = kind %q package %q, want imported package %q",
				name, ref.Kind, ref.Package, wantPackage)
		}
		nativeRef, ok := nativeRefs[name]
		if !ok {
			t.Errorf("native object has no relocation for %s", name)
		} else if ref.SymIndex != nativeRef.SymIndex {
			t.Errorf("%s SymIdx = %d, native %d", name, ref.SymIndex, nativeRef.SymIndex)
		}
	}

	markerTypes := map[string]bool{
		"R_USEIFACE":       true,
		"R_USEIFACEMETHOD": true,
		"R_USENAMEDMETHOD": true,
	}
	markerCounts := make(map[string]int)
	nativeMarkerCounts := make(map[string]int)
	for _, symbol := range object.Symbols {
		for _, reloc := range symbol.Relocations {
			if !markerTypes[reloc.Type] {
				continue
			}
			markerCounts[reloc.Type]++
			if reloc.Size != 0 {
				t.Errorf("%s marker has storage size %d", reloc.Type, reloc.Size)
			}
			if reloc.Type == "R_USENAMEDMETHOD" && reloc.Target.Kind != "self" {
				t.Errorf("R_USENAMEDMETHOD target kind = %q, want self", reloc.Target.Kind)
			}
		}
	}
	for _, symbol := range nativeObject.Symbols {
		for _, reloc := range symbol.Relocations {
			if markerTypes[reloc.Type] {
				nativeMarkerCounts[reloc.Type]++
			}
		}
	}
	for markerType, nativeCount := range nativeMarkerCounts {
		if markerCounts[markerType] != nativeCount {
			t.Errorf("%s count = %d, native %d", markerType, markerCounts[markerType], nativeCount)
		}
	}
	if markerCounts["R_USENAMEDMETHOD"] == 0 {
		t.Error("generic/reflect fixture produced no R_USENAMEDMETHOD marker")
	}
}

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
	if !hasLLVMCompileFlags([]string{"-enablellvm"}) {
		t.Fatal("LLVM compiler selection was not recognized")
	}
	for _, args := range [][]string{
		nil,
		{"-enablellvm=false"},
	} {
		if hasLLVMCompileFlags(args) {
			t.Fatalf("disabled LLVM compiler flags %q were recognized", args)
		}
	}
}

func TestWithLLVMExternalCodegen(t *testing.T) {
	args := []string{"-enablellvm", "-o", "out.a"}
	got := withLLVMExternalCodegen(args)
	if !boolToolFlag(got, "-llvm-external-codegen") {
		t.Fatalf("external codegen protocol was not added: %q", got)
	}
	if got[0] != "-llvm-external-codegen" {
		t.Fatalf("external codegen protocol must precede source arguments: %q", got)
	}
	if boolToolFlag(args, "-llvm-external-codegen") {
		t.Fatalf("withLLVMExternalCodegen modified its input: %q", args)
	}
}

func TestCodegenLLCArgsMachinePassPolicy(t *testing.T) {
	for _, test := range []struct {
		name      string
		enableLSR bool
		want      string
	}{
		{
			name: "defaults",
			want: "-load-pass-plugin=plugin -trap-unreachable -disable-machine-cse -force-loop-cold-block -disable-lsr -filetype=obj input.ll -o output.o",
		},
		{
			name:      "LSR opt-in",
			enableLSR: true,
			want:      "-load-pass-plugin=plugin -trap-unreachable -disable-machine-cse -force-loop-cold-block -filetype=obj input.ll -o output.o",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(codegenLLCArgs("plugin", "input.ll", "output.o", test.enableLSR), " ")
			if got != test.want {
				t.Fatalf("codegenLLCArgs() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNativePackageOverride(t *testing.T) {
	packages := stringSetFlag{"runtime_test": {}}
	args := []string{
		"-p", "runtime_test", "-enablellvm", "-llvm-external-codegen=true",
		"-o", "out.a", "callers_test.go",
	}
	if !useNativeCompiler(args, packages) {
		t.Fatal("exact native package was not recognized")
	}
	native := withoutLLVMCompileFlags(args)
	if hasLLVMCompileFlags(native) {
		t.Fatalf("LLVM selection survived native override: %q", native)
	}
	if got, ok := toolFlag(native, "-p"); !ok || got != "runtime_test" {
		t.Fatalf("native override changed package flag to %q, %v", got, ok)
	}
	if useNativeCompiler([]string{"-p=runtime", "-enablellvm", "-llvm-external-codegen"}, packages) {
		t.Fatal("native package override matched a different package")
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

func TestResolveOpt(t *testing.T) {
	dir := t.TempDir()
	llc := filepath.Join(dir, "llc")
	opt := filepath.Join(dir, "opt")
	for _, path := range []string{llc, opt} {
		if err := os.WriteFile(path, nil, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	got, err := resolveOpt(llc, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != opt {
		t.Fatalf("resolveOpt next to llc = %q, want %q", got, opt)
	}

	explicit := filepath.Join(t.TempDir(), "custom-opt")
	if err := os.WriteFile(explicit, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err = resolveOpt(llc, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Fatalf("resolveOpt explicit path = %q, want %q", got, explicit)
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
		"-p=main", "-enablellvm", "-llvm-external-codegen",
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

func TestLLVMPCLNInlineTraceback(t *testing.T) {
	llc := os.Getenv("GOALLC_LLC")
	opt := os.Getenv("GOALLC_OPT")
	plugin := os.Getenv("GOALLC_PASS_PLUGIN")
	if llc == "" || opt == "" || plugin == "" {
		t.Skip("requires GOALLC_LLC, GOALLC_OPT, and GOALLC_PASS_PLUGIN")
	}
	testenv.MustHaveGoBuild(t)

	root := testenv.GOROOT(t)
	goTool := testenv.GoToolPath(t)
	packagePath := "cmd/llvmtoolexec/testdata/pclninline"
	source := filepath.Join(root, "src", packagePath, "main.go")

	// Verify the source-semantic graph independently of final code layout.
	archive := filepath.Join(t.TempDir(), "pclninline.a")
	compile := testenv.Command(t, goTool, "tool", "compile",
		"-p=main", "-enablellvm", "-llvm-external-codegen", "-o", archive, source)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compiling PCLN inline fixture to LLVM IR: %v\n%s", err, out)
	}
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DICompileUnit(language: DW_LANG_Go",
		"emissionKind: FullDebug",
		"!goobj.debug.funcs",
		"inlinedAt:",
	} {
		if !bytes.Contains(ir, []byte(want)) {
			t.Fatalf("LLVM PCLN metadata does not contain %q:\n%s", want, ir)
		}
	}
	verify := testenv.Command(t, opt, "-passes=verify", "-disable-output", archive+".ll")
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verifying PCLN inline LLVM IR: %v\n%s", err, out)
	}

	wrapper := filepath.Join(t.TempDir(), "llvmtoolexec")
	buildWrapper := testenv.Command(t, goTool, "build", "-o", wrapper, "./src/cmd/llvmtoolexec")
	buildWrapper.Dir = root
	if out, err := buildWrapper.CombinedOutput(); err != nil {
		t.Fatalf("building llvmtoolexec: %v\n%s", err, out)
	}

	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
		"-opt=" + opt,
		"-opt-passes=default<O2>",
	}, " ")
	executable := filepath.Join(t.TempDir(), "pclninline")
	buildFixture := testenv.Command(t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		"-ldflags=-w",
		"-o", executable,
		"./src/"+packagePath,
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building PCLN inline fixture: %v\n%s", err, out)
	}

	output, runErr := testenv.Command(t, executable).CombinedOutput()
	if runErr == nil {
		t.Fatalf("PCLN inline fixture did not panic:\n%s", output)
	}
	trace := string(output)
	if !strings.Contains(trace, "panic: pcln-inline") {
		t.Fatalf("unexpected PCLN inline fixture failure: %v\n%s", runErr, output)
	}
	patterns := []string{
		`(?m)^main\.capture\(\)\n\t.*pclninline/main\.go:23 \+0x[0-9a-f]+$`,
		`(?m)^main\.inner\(\.\.\.\)\n\t.*pclninline/main\.go:8$`,
		`(?m)^main\.middle\(\.\.\.\)\n\t.*pclninline/main\.go:12$`,
		`(?m)^main\.outer\.func1(?:#[^\n]+#)?\(\.\.\.\)\n\t.*pclninline/main\.go:17$`,
		`(?m)^main\.outer\(\.\.\.\)\n\t.*pclninline/main\.go:18$`,
		`(?m)^main\.main\(\)\n\t.*pclninline/main\.go:27 \+0x[0-9a-f]+$`,
	}
	last := -1
	for _, pattern := range patterns {
		location := regexp.MustCompile(pattern).FindStringIndex(trace)
		if location == nil {
			t.Fatalf("traceback does not match %q:\n%s", pattern, output)
		}
		if location[0] <= last {
			t.Fatalf("inline frames are out of order:\n%s", output)
		}
		last = location[0]
	}
}

func TestLLVMDWARFDebugInfo(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("final executable inspection currently uses debug/macho")
	}
	llc := os.Getenv("GOALLC_LLC")
	opt := os.Getenv("GOALLC_OPT")
	plugin := os.Getenv("GOALLC_PASS_PLUGIN")
	if llc == "" || opt == "" || plugin == "" {
		t.Skip("requires GOALLC_LLC, GOALLC_OPT, and GOALLC_PASS_PLUGIN")
	}
	testenv.MustHaveGoBuild(t)

	root := testenv.GOROOT(t)
	goTool := testenv.GoToolPath(t)
	packagePath := "cmd/llvmtoolexec/testdata/dwarfdebug"
	packageArg := "./src/" + packagePath
	source := filepath.Join(root, "src", packagePath, "main.go")

	// Verify the frontend's source-semantic graph independently of final layout.
	archive := filepath.Join(t.TempDir(), "dwarfdebug.a")
	compile := testenv.Command(t, goTool, "tool", "compile",
		"-p=main", "-enablellvm", "-llvm-external-codegen", "-o", archive, source)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compiling DWARF fixture to LLVM IR: %v\n%s", err, out)
	}
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DICompileUnit(language: DW_LANG_Go",
		"emissionKind: FullDebug",
		"DICompositeType(tag: DW_TAG_structure_type, name: \"main.pair\"",
		"DILocalVariable(name: \"local\"",
		"#dbg_declare",
		"!goobj.debug.vars",
		"inlinedAt:",
	} {
		if !bytes.Contains(ir, []byte(want)) {
			t.Fatalf("LLVM debug metadata does not contain %q:\n%s", want, ir)
		}
	}
	verify := testenv.Command(t, opt, "-passes=verify", "-disable-output", archive+".ll")
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("verifying DWARF fixture LLVM IR: %v\n%s", err, out)
	}

	wrapper := filepath.Join(t.TempDir(), "llvmtoolexec")
	buildWrapper := testenv.Command(t, goTool, "build", "-o", wrapper, "./src/cmd/llvmtoolexec")
	buildWrapper.Dir = root
	if out, err := buildWrapper.CombinedOutput(); err != nil {
		t.Fatalf("building llvmtoolexec: %v\n%s", err, out)
	}
	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
		"-opt=" + opt,
		"-opt-passes=default<O2>",
	}, " ")
	cache := t.TempDir()
	executable := filepath.Join(t.TempDir(), "dwarfdebug")
	buildFixture := testenv.Command(t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		"-ldflags=-compressdwarf=false",
		"-o", executable, packageArg)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building DWARF fixture: %v\n%s", err, out)
	}
	if out, err := testenv.Command(t, executable).CombinedOutput(); err != nil {
		t.Fatalf("running DWARF fixture: %v\n%s", err, out)
	}

	listFixture := testenv.Command(t, goTool, "list", "-export", "-f={{.Export}}",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm", packageArg)
	listFixture.Dir = root
	listFixture.Env = append(os.Environ(), "GOCACHE="+cache)
	archiveOutput, err := listFixture.CombinedOutput()
	if err != nil {
		t.Fatalf("locating DWARF fixture archive: %v\n%s", err, archiveOutput)
	}
	object := readImportObjview(t, goTool, strings.TrimSpace(string(archiveOutput)))
	carrierKinds := make(map[string]int)
	relocationTypes := make(map[string]int)
	mainHasInfo, mainHasLines := false, false
	for _, symbol := range object.Symbols {
		if strings.HasPrefix(symbol.Name, ".debug_") {
			t.Errorf("ordinary LLVM DWARF section leaked into GoObj: %s", symbol.Name)
		}
		carrierKinds[symbol.Kind]++
		for _, reloc := range symbol.Relocations {
			relocationTypes[reloc.Type]++
		}
		if symbol.Name == "main.main" {
			for _, aux := range symbol.Auxiliaries {
				switch aux.Type {
				case "dwarf_info":
					mainHasInfo = true
				case "dwarf_lines":
					mainHasLines = true
				}
			}
		}
	}
	for _, kind := range []string{"SDWARFFCN", "SDWARFABSFCN", "SDWARFLINES"} {
		if carrierKinds[kind] == 0 {
			t.Errorf("GoObj has no %s carrier: %v", kind, carrierKinds)
		}
	}
	if !mainHasInfo || !mainHasLines {
		t.Errorf("main.main DWARF aux = info:%v lines:%v", mainHasInfo, mainHasLines)
	}
	for _, typ := range []string{"R_ADDR", "R_DWARFSECREF", "R_USETYPE"} {
		if relocationTypes[typ] == 0 {
			t.Errorf("GoObj DWARF has no %s relocation: %v", typ, relocationTypes)
		}
	}

	machoFile, err := macho.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer machoFile.Close()
	dwarfData, err := machoFile.DWARF()
	if err != nil {
		t.Fatal(err)
	}
	checkDwarfDebugFixture(t, dwarfData)
}

func TestLLVMDWARF5DebugInfo(t *testing.T) {
	llc := os.Getenv("GOALLC_LLC")
	opt := os.Getenv("GOALLC_OPT")
	plugin := os.Getenv("GOALLC_PASS_PLUGIN")
	if llc == "" || opt == "" || plugin == "" {
		t.Skip("requires GOALLC_LLC, GOALLC_OPT, and GOALLC_PASS_PLUGIN")
	}
	testenv.MustHaveGoBuild(t)

	root := testenv.GOROOT(t)
	goTool := testenv.GoToolPath(t)
	packagePath := "cmd/llvmtoolexec/testdata/dwarfdebug"
	packageArg := "./src/" + packagePath
	wrapper := filepath.Join(t.TempDir(), "llvmtoolexec")
	buildWrapper := testenv.Command(t, goTool, "build", "-o", wrapper, "./src/cmd/llvmtoolexec")
	buildWrapper.Dir = root
	if out, err := buildWrapper.CombinedOutput(); err != nil {
		t.Fatalf("building llvmtoolexec: %v\n%s", err, out)
	}
	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
		"-opt=" + opt,
		"-opt-passes=default<O2>",
	}, " ")
	cache := t.TempDir()
	buildEnv := append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "GOCACHE="+cache)
	executable := filepath.Join(t.TempDir(), "dwarfdebug-linux")
	buildFixture := testenv.Command(t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm",
		"-ldflags=-compressdwarf=false",
		"-o", executable, packageArg)
	buildFixture.Dir = root
	buildFixture.Env = buildEnv
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building Linux DWARF5 fixture: %v\n%s", err, out)
	}

	listFixture := testenv.Command(t, goTool, "list", "-export", "-f={{.Export}}",
		"-toolexec="+toolexec,
		"-gcflags="+packagePath+"=-enablellvm", packageArg)
	listFixture.Dir = root
	listFixture.Env = buildEnv
	archiveOutput, err := listFixture.CombinedOutput()
	if err != nil {
		t.Fatalf("locating Linux DWARF5 fixture archive: %v\n%s", err, archiveOutput)
	}
	object := readImportObjview(t, goTool, strings.TrimSpace(string(archiveOutput)))
	carrierKinds := make(map[string]int)
	relocationTypes := make(map[string]int)
	mainHasInfo, mainHasRanges, mainHasLines := false, false, false
	for _, symbol := range object.Symbols {
		if strings.HasPrefix(symbol.Name, ".debug_") {
			t.Errorf("ordinary LLVM DWARF section leaked into GoObj: %s", symbol.Name)
		}
		carrierKinds[symbol.Kind]++
		for _, reloc := range symbol.Relocations {
			relocationTypes[reloc.Type]++
		}
		if symbol.Name == "main.main" {
			for _, aux := range symbol.Auxiliaries {
				switch aux.Type {
				case "dwarf_info":
					mainHasInfo = true
				case "dwarf_ranges":
					mainHasRanges = true
				case "dwarf_lines":
					mainHasLines = true
				}
			}
		}
	}
	for _, kind := range []string{"SDWARFFCN", "SDWARFABSFCN", "SDWARFRANGE", "SDWARFLINES"} {
		if carrierKinds[kind] == 0 {
			t.Errorf("GoObj has no %s carrier: %v", kind, carrierKinds)
		}
	}
	if !mainHasInfo || !mainHasRanges || !mainHasLines {
		t.Errorf("main.main DWARF aux = info:%v ranges:%v lines:%v", mainHasInfo, mainHasRanges, mainHasLines)
	}
	for _, typ := range []string{"R_DWTXTADDR_U4", "R_DWARFSECREF", "R_USETYPE"} {
		if relocationTypes[typ] == 0 {
			t.Errorf("GoObj DWARF5 has no %s relocation: %v", typ, relocationTypes)
		}
	}
	if relocationTypes["R_ADDRCUOFF"] != 0 {
		t.Errorf("GoObj DWARF5 unexpectedly uses R_ADDRCUOFF: %v", relocationTypes)
	}

	elfFile, err := elf.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer elfFile.Close()
	dwarfData, err := elfFile.DWARF()
	if err != nil {
		t.Fatal(err)
	}
	checkDwarfDebugFixture(t, dwarfData)
}

func checkDwarfDebugFixture(t *testing.T, dwarfData *dwarf.Data) {
	t.Helper()
	reader := dwarfData.Reader()
	var mainCU *dwarf.Entry
	inMain := false
	subprograms := make(map[string]dwarf.Offset)
	inlineOrigins := make([]dwarf.Offset, 0, 3)
	formalParameters := make(map[string]bool)
	hasLocal, unavailableLocations := false, 0
	for {
		entry, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		if entry == nil {
			break
		}
		if entry.Tag == dwarf.TagCompileUnit {
			name, _ := entry.Val(dwarf.AttrName).(string)
			if inMain && name != "main" {
				break
			}
			inMain = name == "main"
			if inMain {
				mainCU = entry
			}
			continue
		}
		if !inMain {
			continue
		}
		name, _ := entry.Val(dwarf.AttrName).(string)
		switch entry.Tag {
		case dwarf.TagSubprogram:
			if name != "" {
				subprograms[name] = entry.Offset
			}
		case dwarf.TagInlinedSubroutine:
			origin, ok := entry.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
			if !ok || entry.Val(dwarf.AttrCallFile) == nil || entry.Val(dwarf.AttrCallLine) == nil {
				t.Errorf("inlined_subroutine lacks origin/call site: %#v", entry)
			} else {
				inlineOrigins = append(inlineOrigins, origin)
			}
			ranges, err := dwarfData.Ranges(entry)
			if err != nil || len(ranges) == 0 {
				t.Errorf("inlined_subroutine ranges = %v, %v", ranges, err)
			}
		case dwarf.TagFormalParameter:
			formalParameters[name] = entry.Val(dwarf.AttrType) != nil
			if location, ok := entry.Val(dwarf.AttrLocation).([]byte); ok && len(location) == 0 {
				unavailableLocations++
			}
		case dwarf.TagVariable:
			if name == "local" && entry.Val(dwarf.AttrType) != nil {
				hasLocal = true
			}
			if location, ok := entry.Val(dwarf.AttrLocation).([]byte); ok && len(location) == 0 {
				unavailableLocations++
			}
		}
	}
	for _, name := range []string{
		"main.outer", "main.middle", "main.inner", "main.observe",
		"main.split", "main.arrayWord", "main.main",
	} {
		if subprograms[name] == 0 {
			t.Errorf("final DWARF has no %s subprogram", name)
		}
	}
	for _, name := range []string{"x", "p", "~r0", "lo", "hi"} {
		if !formalParameters[name] {
			t.Errorf("final DWARF has no typed %s parameter", name)
		}
	}
	if !hasLocal || unavailableLocations < 2 {
		t.Errorf("final DWARF local=%v unavailable locations=%d", hasLocal, unavailableLocations)
	}
	wantOrigins := map[dwarf.Offset]bool{
		subprograms["main.outer"]:  true,
		subprograms["main.middle"]: true,
		subprograms["main.inner"]:  true,
	}
	for _, origin := range inlineOrigins {
		delete(wantOrigins, origin)
	}
	if len(wantOrigins) != 0 {
		t.Errorf("inline abstract origins missing: %v", wantOrigins)
	}
	if mainCU == nil {
		t.Fatal("final DWARF has no main compile unit")
	}
	lineReader, err := dwarfData.LineReader(mainCU)
	if err != nil {
		t.Fatal(err)
	}
	lines := make(map[int]bool)
	for {
		var line dwarf.LineEntry
		err := lineReader.Next(&line)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if line.File != nil && strings.HasSuffix(line.File.Name, "/dwarfdebug/main.go") {
			lines[line.Line] = true
		}
	}
	for _, line := range []int{20, 26, 30, 35} {
		if !lines[line] {
			t.Errorf("final DWARF line table lacks main.go:%d: %v", line, lines)
		}
	}
}

func TestLLVMInitTaskOrder(t *testing.T) {
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

	executable := filepath.Join(t.TempDir(), "inittask")
	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}, " ")
	buildFixture := testenv.Command(
		t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags=cmd/llvmtoolexec/testdata/inittask=-enablellvm",
		"-ldflags=-w",
		"-o", executable,
		"./src/cmd/llvmtoolexec/testdata/inittask",
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building LLVM inittask fixture: %v\n%s", err, out)
	}

	runFixture := testenv.Command(t, executable)
	if out, err := runFixture.CombinedOutput(); err != nil {
		t.Fatalf("running LLVM inittask fixture: %v\n%s", err, out)
	}
}

func TestLLVMExplicitNilCheckRuntime(t *testing.T) {
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

	executable := filepath.Join(t.TempDir(), "nilcheck")
	toolexec := strings.Join([]string{
		wrapper,
		"-llc=" + llc,
		"-pass-plugin=" + plugin,
	}, " ")
	buildFixture := testenv.Command(
		t, goTool, "build",
		"-toolexec="+toolexec,
		"-gcflags=cmd/llvmtoolexec/testdata/nilcheck/p=-enablellvm",
		"-ldflags=-w",
		"-o", executable,
		"./src/cmd/llvmtoolexec/testdata/nilcheck",
	)
	buildFixture.Dir = root
	buildFixture.Env = append(os.Environ(), "GOCACHE="+t.TempDir())
	if out, err := buildFixture.CombinedOutput(); err != nil {
		t.Fatalf("building LLVM nil-check fixture: %v\n%s", err, out)
	}

	runFixture := testenv.Command(t, executable)
	if out, err := runFixture.CombinedOutput(); err != nil {
		t.Fatalf("running LLVM nil-check fixture: %v\n%s", err, out)
	}
}

func TestLLVMExplicitNilCheckGoObj(t *testing.T) {
	llc := os.Getenv("GOALLC_LLC")
	plugin := os.Getenv("GOALLC_PASS_PLUGIN")
	if llc == "" || plugin == "" {
		t.Skip("requires GOALLC_LLC and GOALLC_PASS_PLUGIN")
	}
	testenv.MustHaveGoBuild(t)

	root := testenv.GOROOT(t)
	goTool := testenv.GoToolPath(t)
	archive := filepath.Join(t.TempDir(), "nilcheckobj.a")
	source := filepath.Join(root, "src", "cmd", "llvmtoolexec", "testdata", "nilcheckobj", "p.go")
	compileFixture := testenv.Command(
		t, goTool, "tool", "compile",
		"-p=cmd/llvmtoolexec/testdata/nilcheckobj",
		"-enablellvm", "-llvm-external-codegen",
		"-o", archive, source,
	)
	if out, err := compileFixture.CombinedOutput(); err != nil {
		t.Fatalf("compiling LLVM nil-check GoObj fixture: %v\n%s", err, out)
	}
	ir, err := os.ReadFile(archive + ".ll")
	if err != nil {
		t.Fatalf("reading LLVM nil-check IR: %v", err)
	}
	panicLoc := regexp.MustCompile(`(?m)^[[:space:]]*(?:tail )?call goabiinternal void @runtime\.panicmem\(\), !dbg !([0-9]+)$`).FindSubmatch(ir)
	if panicLoc == nil {
		t.Fatalf("panicmem call is missing its nil-check debug location:\n%s", ir)
	}
	wantLoc := []byte("!" + string(panicLoc[1]) + " = !DILocation(line: 8,")
	if !bytes.Contains(ir, wantLoc) {
		t.Fatalf("panicmem debug location is not the source dereference line; want %q:\n%s", wantLoc, ir)
	}

	object := filepath.Join(t.TempDir(), "nilcheckobj.o")
	runLLC := testenv.Command(
		t, llc,
		"-load-pass-plugin="+plugin,
		"-verify-machineinstrs",
		"-filetype=obj",
		"-o", object,
		archive+".ll",
	)
	if out, err := runLLC.CombinedOutput(); err != nil {
		t.Fatalf("writing nil-check GoObj: %v\n%s", err, out)
	}

	objview := testenv.Command(t, goTool, "tool", "objview", "-format=text", object)
	output, err := objview.CombinedOutput()
	if err != nil {
		t.Fatalf("go tool objview rejected nil-check GoObj: %v\n%s", err, output)
	}
	text := string(output)
	if !regexp.MustCompile(`(?m)^  [^\n]*CALL [^\n]*runtime\.panicmem[^\n]*\| pcsp=[1-9][0-9]* [^\n]*PCDATA_StackMapIndex=1\([^\n]*\)\n[[:space:]]*\| ordinary safepoint [^\n]* map\[1\]`).MatchString(text) {
		t.Fatalf("panicmem call is missing its frameful ordinary statepoint map:\n%s", output)
	}
}
