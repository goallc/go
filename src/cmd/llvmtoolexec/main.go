// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// llvm-toolexec is a go build -toolexec wrapper that replaces a Go compiler
// action carrying -enablellvm with an object produced by llc from compiler
// LLVM IR. The compiler still writes __.PKGDEF, so dependents retain normal Go
// export-data handling; it intentionally writes no native machine code.
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
	"strconv"
	"strings"
)

var (
	llcPath        = flag.String("llc", os.Getenv("GOALLC_LLC"), "path to llc")
	passPluginPath = flag.String("pass-plugin", os.Getenv("GOALLC_PASS_PLUGIN"), "path to the GoALLC LLVM pass plugin (default next to llc)")
	keepIR         = flag.Bool("keep-ir", false, "keep the compiler-generated .ll sidecar")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fatalf("missing Go tool path")
	}

	tool, args := flag.Arg(0), flag.Args()[1:]
	if filepath.Base(tool) != "compile" || !boolToolFlag(args, "-enablellvm") {
		run(tool, args...)
		return
	}
	if isFullVersion(args) {
		printToolIdentity(tool, args, *llcPath, *passPluginPath)
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
	compileArgs := make([]string, 0, len(args)+1)
	compileArgs = append(compileArgs, "-llvmironly")
	compileArgs = append(compileArgs, args...)
	run(tool, compileArgs...)

	irPath := output + ".ll"
	if _, err := os.Stat(irPath); err != nil {
		fatalf("compiler did not produce %s: %v", irPath, err)
	}
	objPath := filepath.Join(filepath.Dir(output), "llvm-goobj.o")
	llcArgs := []string{
		"-load-pass-plugin=" + pluginPath,
		"-filetype=obj",
		irPath,
		"-o", objPath,
	}
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
	}
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

// printToolIdentity implements the toolexec -V=full protocol for an
// -enablellvm compile action. The native compiler identity alone is
// insufficient because llc and the pass plugin also determine the archive
// written by this wrapper.
func printToolIdentity(tool string, args []string, llc, configuredPlugin string) {
	llc, err := resolveLLC(llc)
	if err != nil {
		fatalf("%v", err)
	}
	plugin, err := resolvePassPlugin(llc, configuredPlugin)
	if err != nil {
		fatalf("%v", err)
	}

	cmd := exec.Command(tool, args...)
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
	backendFiles, err := backendIdentityFiles(llc, plugin)
	if err != nil {
		fatalf("resolving backend identity files: %v", err)
	}
	identity, err := backendIdentity(out, append([]string{wrapper}, backendFiles...)...)
	if err != nil {
		fatalf("computing backend identity: %v", err)
	}
	fmt.Printf("%s buildID=%s\n", strings.Join(fields, " "), identity)
}

func backendIdentityFiles(llc, plugin string) ([]string, error) {
	paths := []string{llc, plugin}
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
