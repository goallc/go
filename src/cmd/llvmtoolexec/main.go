// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// llvm-toolexec is a go build -toolexec wrapper that replaces a selected Go
// package's native linker object with one produced by llc from compiler LLVM
// IR. The compiler still writes __.PKGDEF, so dependents retain normal Go
// export-data handling; it intentionally writes no native machine code.
package main

import (
	"cmd/internal/archive"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	llcPath        = flag.String("llc", os.Getenv("GOALLC_LLC"), "path to llc")
	passPluginPath = flag.String("pass-plugin", os.Getenv("GOALLC_PASS_PLUGIN"), "path to the GoALLC LLVM pass plugin (default next to llc)")
	keepIR         = flag.Bool("keep-ir", false, "keep the compiler-generated .ll sidecar")
	packageOnly    = flag.String("package", os.Getenv("GOALLC_LLVM_PACKAGE"), "only replace this Go import path")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fatalf("missing Go tool path")
	}

	tool, args := flag.Arg(0), flag.Args()[1:]
	if filepath.Base(tool) != "compile" || !selectedPackage(args, *packageOnly) {
		run(tool, args...)
		return
	}
	if *llcPath == "" {
		fatalf("missing llc path: pass -llc or set GOALLC_LLC")
	}
	pluginPath, err := resolvePassPlugin(*llcPath, *passPluginPath)
	if err != nil {
		fatalf("%v", err)
	}

	output, ok := toolFlag(args, "-o")
	if !ok || output == "" {
		fatalf("compile invocation has no -o output")
	}
	compileArgs := make([]string, 0, len(args)+2)
	compileArgs = append(compileArgs, "-enablellvm", "-llvmironly")
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
	run(*llcPath, llcArgs...)
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

func selectedPackage(args []string, want string) bool {
	if want == "" {
		return true
	}
	pkg, ok := toolFlag(args, "-p")
	return ok && pkg == want
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
