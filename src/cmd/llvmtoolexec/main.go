// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// llvm-toolexec is a go build -toolexec wrapper that replaces selected Go
// compiler actions with objects produced by llc from compiler LLVM IR. An
// action is selected when -enablellvm is present in its compiler flags. The
// wrapper adds cmd/compile's internal -llvm-external-codegen protocol flag, so
// the compiler writes LLVM IR and __.PKGDEF but no linker object. Dependents
// retain normal Go export-data handling, and llc supplies the sole _go_.o.
package main

import (
	"cmd/internal/archive"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type stringSetFlag map[string]struct{}

func (f *stringSetFlag) Set(value string) error {
	if value == "" {
		return errors.New("package path must not be empty")
	}
	if *f == nil {
		*f = make(map[string]struct{})
	}
	(*f)[value] = struct{}{}
	return nil
}

func (f *stringSetFlag) String() string {
	if f == nil {
		return ""
	}
	values := make([]string, 0, len(*f))
	for value := range *f {
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

var (
	llcPath        = flag.String("llc", os.Getenv("GOALLC_LLC"), "path to llc")
	optPath        = flag.String("opt", os.Getenv("GOALLC_OPT"), "path to opt")
	optPasses      = flag.String("opt-passes", "", "optional LLVM optimization pipeline to run before llc")
	passPluginPath = flag.String("pass-plugin", os.Getenv("GOALLC_PASS_PLUGIN"), "path to the GoALLC LLVM pass plugin (default next to llc)")
	enableLSR      = flag.Bool("enable-lsr", false, "enable LLVM loop strength reduction (experimental with Go stack pointer maps)")
	keepIR         = flag.Bool("keep-ir", false, "keep the compiler-generated .ll sidecar")
	nativePackages stringSetFlag
)

func init() {
	flag.Var(&nativePackages, "native-package", "compile this exact -p package with the native Go backend even when inherited gcflags select LLVM (repeatable)")
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fatalf("missing Go tool path")
	}

	tool, args := flag.Arg(0), flag.Args()[1:]
	if filepath.Base(tool) != "compile" {
		run(tool, args...)
		return
	}
	if isFullVersion(args) {
		// Version probes do not carry per-package gcflags. Conservatively include
		// the LLVM backend in the wrapper identity so an LLVM action can never
		// reuse an object cached for a different payload.
		printToolIdentity(tool, args, *llcPath, *optPath, *optPasses, *passPluginPath, *enableLSR)
		return
	}
	if !hasLLVMCompileFlags(args) {
		run(tool, args...)
		return
	}
	if useNativeCompiler(args, nativePackages) {
		run(tool, withoutLLVMCompileFlags(args)...)
		return
	}
	if !isCompileAction(args) {
		run(tool, args...)
		return
	}
	llc, err := resolveLLC(*llcPath)
	if err != nil {
		fatalf("%v", err)
	}
	pluginPath, err := resolvePassPlugin(llc, *passPluginPath)
	if err != nil {
		fatalf("%v", err)
	}

	output, ok := toolFlag(args, "-o")
	if !ok || output == "" {
		fatalf("compile invocation has no -o output")
	}
	run(tool, withLLVMExternalCodegen(args)...)

	irPath := output + ".ll"
	if _, err := os.Stat(irPath); err != nil {
		fatalf("compiler did not produce %s: %v", irPath, err)
	}
	llcInput := irPath
	optimizedIRPath := output + ".opt.ll"
	if *optPasses != "" {
		opt, err := resolveOpt(llc, *optPath)
		if err != nil {
			fatalf("%v", err)
		}
		optArgs := []string{"-passes=" + *optPasses}
		if !*enableLSR {
			optArgs = append(optArgs, "-disable-lsr")
		}
		optArgs = append(optArgs, "-S", irPath, "-o", optimizedIRPath)
		run(opt, optArgs...)
		llcInput = optimizedIRPath
	}
	objPath := filepath.Join(filepath.Dir(output), "llvm-goobj.o")
	llcArgs := codegenLLCArgs(pluginPath, llcInput, objPath, *enableLSR)
	run(llc, llcArgs...)
	// cmd/link identifies the package linker object by this archive member
	// name. Unlike the mixed native/LLVM path, this is the sole linker member.
	appendArchiveMember(output, "_go_.o", objPath)
	if err := os.Remove(objPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fatalf("remove %s: %v", objPath, err)
	}
	if !*keepIR {
		if err := os.Remove(irPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fatalf("remove %s: %v", irPath, err)
		}
		if *optPasses != "" {
			if err := os.Remove(optimizedIRPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				fatalf("remove %s: %v", optimizedIRPath, err)
			}
		}
	}
}

func codegenLLCArgs(pluginPath, inputPath, outputPath string, enableLSR bool) []string {
	args := []string{
		"-load-pass-plugin=" + pluginPath,
		"-trap-unreachable",
		// MachineCSE runs after SelectionDAG has lowered statepoints. It can CSE
		// a post-statepoint frame-index LEA with an earlier LEA, extending the
		// earlier virtual register's live range across statepoints whose gc-live
		// operands have already been fixed. This breaks x86 code even without a
		// Go stack move (go/printer TestFiles/alignment.input is the reproducer).
		// Keep it disabled until frame-index expressions are invalidated at
		// statepoints, or the post-statepoint rematerialization is made opaque to
		// MachineCSE.
		"-disable-machine-cse",
	}
	// LSR can turn an address rooted at a pointer-containing alloca into a
	// loop-carried derived pointer. The late statepoint pass relocates that
	// scalar address, but does not yet recover the base alloca's per-call
	// contents liveness through the recurrence. Keep LSR opt-in until that
	// provenance is represented in GoObj pointer maps. The current end-to-end
	// reproducer runs TestTokenStringAllocations before TestTokenAccessors in
	// encoding/json/jsontext; it remains a known failure even with LSR disabled,
	// so the switch also keeps the next analysis free of this transformation.
	if !enableLSR {
		args = append(args, "-disable-lsr")
	}
	return append(args, "-filetype=obj", inputPath, "-o", outputPath)
}

func resolveOpt(llc, configured string) (string, error) {
	if configured != "" {
		path, err := resolveExecutable(configured)
		if err != nil {
			return "", fmt.Errorf("invalid opt %q: %w", configured, err)
		}
		return path, nil
	}
	path, err := resolveExecutable(filepath.Join(filepath.Dir(llc), "opt"))
	if err != nil {
		return "", fmt.Errorf("missing opt next to llc %q: pass -opt or set GOALLC_OPT: %w", llc, err)
	}
	return path, nil
}

func resolvePassPlugin(llc, configured string) (string, error) {
	if configured != "" {
		path, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolving pass plugin %q: %w", configured, err)
		}
		if err := requireRegularFile(path); err != nil {
			return "", fmt.Errorf("invalid pass plugin %q: %w", path, err)
		}
		return path, nil
	}

	llcPath, err := exec.LookPath(llc)
	if err != nil {
		return "", fmt.Errorf("resolving llc %q to locate its pass plugin: %w", llc, err)
	}
	llcPath, err = filepath.Abs(llcPath)
	if err != nil {
		return "", fmt.Errorf("resolving llc %q to locate its pass plugin: %w", llc, err)
	}

	filename, err := passPluginFilename()
	if err != nil {
		return "", err
	}
	llcPaths := []string{llcPath}
	if resolved, err := filepath.EvalSymlinks(llcPath); err == nil && resolved != llcPath {
		llcPaths = append(llcPaths, resolved)
	}
	for _, path := range llcPaths {
		root := filepath.Dir(filepath.Dir(path))
		for _, libdir := range []string{"lib", "lib64"} {
			plugin := filepath.Join(root, libdir, filename)
			if requireRegularFile(plugin) == nil {
				return plugin, nil
			}
		}
	}
	return "", fmt.Errorf("GoALLC pass plugin not found next to llc %q; pass -pass-plugin or set GOALLC_PASS_PLUGIN", llcPath)
}

func resolveLLC(configured string) (string, error) {
	if configured != "" {
		path, err := resolveExecutable(configured)
		if err != nil {
			return "", fmt.Errorf("invalid llc %q: %w", configured, err)
		}
		return path, nil
	}
	var candidates []string
	if root := os.Getenv("GOALLC_LLVM_DIR"); root != "" {
		candidates = append(candidates, filepath.Join(root, "bin", "llc"))
	}
	if executable, err := os.Executable(); err == nil {
		toolDir := filepath.Dir(executable)
		root := filepath.Dir(filepath.Dir(filepath.Dir(toolDir)))
		if data, err := os.ReadFile(filepath.Join(root, "pkg", "goallc-llvm-payload")); err == nil {
			if payload := strings.TrimSpace(string(data)); payload != "" {
				candidates = append(candidates, filepath.Join(payload, "bin", "llc"))
			}
		}
		candidates = append(candidates, filepath.Join(root, "llvm", "bin", "llc"))
	}
	for _, candidate := range candidates {
		if path, err := resolveExecutable(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing llc for selected compile: pass -llc, set GOALLC_LLC or GOALLC_LLVM_DIR, or install the payload under GOROOT/llvm")
}

func passPluginFilename() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "GoALLCStatepoints.dylib", nil
	case "linux":
		return "GoALLCStatepoints.so", nil
	default:
		return "", fmt.Errorf("GoALLC pass plugins are unsupported on %s", runtime.GOOS)
	}
}

func requireRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	return nil
}

func isFullVersion(args []string) bool {
	for _, arg := range args {
		if arg == "-V=full" {
			return true
		}
	}
	return false
}

func isCompileAction(args []string) bool {
	output, ok := toolFlag(args, "-o")
	return ok && output != ""
}

func hasLLVMCompileFlags(args []string) bool {
	return boolToolFlag(args, "-enablellvm")
}

func withLLVMExternalCodegen(args []string) []string {
	if boolToolFlag(args, "-llvm-external-codegen") {
		return args
	}
	return append([]string{"-llvm-external-codegen"}, args...)
}

// printToolIdentity implements the toolexec -V=full protocol when this wrapper
// may select an LLVM compile action. The native compiler identity alone is
// insufficient because llc and the pass plugin also determine the archive
// written by this wrapper.
func printToolIdentity(tool string, args []string, llc, configuredOpt, optPasses, configuredPlugin string, enableLSR bool) {
	llc, err := resolveLLC(llc)
	if err != nil {
		fatalf("%v", err)
	}
	var opt string
	if optPasses != "" {
		opt, err = resolveOpt(llc, configuredOpt)
		if err != nil {
			fatalf("%v", err)
		}
	}
	plugin, err := resolvePassPlugin(llc, configuredPlugin)
	if err != nil {
		fatalf("%v", err)
	}

	cmd := exec.Command(tool, args...)
	// This retained wrapper owns the external llc/plugin identity below. Avoid
	// asking cmd/compile to resolve and hash its in-process backend as well;
	// the wrapper may deliberately point at a different debug plugin.
	cmd.Env = append(os.Environ(), "GOALLC_EXTERNAL_BACKEND=1")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("run %s: %v", tool, err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 3 || fields[0] != "compile" || fields[1] != "version" {
		fatalf("unexpected compile -V=full output: %q", strings.TrimSpace(string(out)))
	}
	if strings.HasPrefix(fields[len(fields)-1], "buildID=") {
		fields = fields[:len(fields)-1]
	}

	wrapper, err := os.Executable()
	if err != nil {
		fatalf("resolving llvmtoolexec executable: %v", err)
	}
	backendFiles, err := backendIdentityFiles(llc, opt, plugin)
	if err != nil {
		fatalf("resolving backend identity files: %v", err)
	}
	identityInput := append([]byte(nil), out...)
	identityInput = append(identityInput, "\x00opt-passes="...)
	identityInput = append(identityInput, optPasses...)
	identityInput = append(identityInput, "\x00native-packages="...)
	identityInput = append(identityInput, nativePackages.String()...)
	identityInput = append(identityInput, "\x00enable-lsr="...)
	identityInput = strconv.AppendBool(identityInput, enableLSR)
	identity, err := backendIdentity(identityInput, append([]string{wrapper}, backendFiles...)...)
	if err != nil {
		fatalf("computing backend identity: %v", err)
	}
	fmt.Printf("%s buildID=%s\n", strings.Join(fields, " "), identity)
}

func backendIdentityFiles(llc, opt, plugin string) ([]string, error) {
	paths := []string{llc}
	if opt != "" {
		paths = append(paths, opt)
	}
	paths = append(paths, plugin)
	root := filepath.Dir(filepath.Dir(llc))
	var candidates []string
	for _, pattern := range []string{"libLLVM*.dylib", "libLLVM*.so", "libLLVM*.so.*"} {
		matches, err := filepath.Glob(filepath.Join(root, "lib", pattern))
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, matches...)
	}
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		real := candidate
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			real = resolved
		}
		if !seen[real] {
			seen[real] = true
			paths = append(paths, real)
		}
	}
	return paths, nil
}

func backendIdentity(toolOutput []byte, paths ...string) (string, error) {
	h := sha256.New()
	io.WriteString(h, "goallc llvmtoolexec identity v1\x00")
	h.Write(toolOutput)
	for i, path := range paths {
		fmt.Fprintf(h, "\x00file%d\x00", i)
		if err := hashFile(h, path); err != nil {
			return "", fmt.Errorf("hashing %q: %w", path, err)
		}
	}
	return fmt.Sprintf("goallc-%x", h.Sum(nil)), nil
}

func resolveExecutable(path string) (string, error) {
	resolved, err := exec.LookPath(path)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if err := requireRegularFile(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func hashFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// appendArchiveMember uses the Go toolchain's archive writer. Unlike BSD ar,
// it appends an entry without inserting __.SYMDEF before the existing
// __.PKGDEF member required by cmd/link.
func appendArchiveMember(archivePath, name, memberPath string) {
	arFile, err := os.OpenFile(archivePath, os.O_RDWR, 0)
	if err != nil {
		fatalf("open %s: %v", archivePath, err)
	}
	defer arFile.Close()
	ar, err := archive.Parse(arFile, false)
	if err != nil {
		fatalf("parse %s: %v", archivePath, err)
	}
	if len(ar.Entries) == 0 || ar.Entries[0].Name != "__.PKGDEF" {
		fatalf("%s does not start with __.PKGDEF", archivePath)
	}
	for _, entry := range ar.Entries {
		if entry.Name == name {
			fatalf("%s already contains %s", archivePath, name)
		}
	}
	member, err := os.Open(memberPath)
	if err != nil {
		fatalf("open %s: %v", memberPath, err)
	}
	defer member.Close()
	info, err := member.Stat()
	if err != nil {
		fatalf("stat %s: %v", memberPath, err)
	}
	ar.AddEntry(archive.EntryGoObj, name, 0, 0, 0, 0o644, info.Size(), member)
}

func boolToolFlag(args []string, name string) bool {
	enabled := false
	for _, arg := range args {
		switch {
		case arg == name:
			enabled = true
		case strings.HasPrefix(arg, name+"="):
			value, err := strconv.ParseBool(strings.TrimPrefix(arg, name+"="))
			if err == nil {
				enabled = value
			}
		}
	}
	return enabled
}

func toolFlag(args []string, name string) (string, bool) {
	prefix := name + "="
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix), true
		}
	}
	return "", false
}

func useNativeCompiler(args []string, packages stringSetFlag) bool {
	pkg, ok := toolFlag(args, "-p")
	if !ok {
		return false
	}
	_, ok = packages[pkg]
	return ok
}

func withoutLLVMCompileFlags(args []string) []string {
	native := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "-enablellvm" || strings.HasPrefix(arg, "-enablellvm=") ||
			arg == "-llvm-external-codegen" || strings.HasPrefix(arg, "-llvm-external-codegen=") {
			continue
		}
		native = append(native, arg)
	}
	return native
}

func run(path string, args ...string) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("run %s: %v", path, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "llvm-toolexec: "+format+"\n", args...)
	os.Exit(1)
}
