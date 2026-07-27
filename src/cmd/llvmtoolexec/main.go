// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// llvm-toolexec is a go build -toolexec wrapper that replaces a selected Go
// package's native linker object with one produced by llc from compiler LLVM
// IR. The compiler still writes __.PKGDEF, so dependents retain normal Go
// export-data handling; it intentionally writes no native machine code.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	llcPath     = flag.String("llc", os.Getenv("GOALLC_LLC"), "path to llc")
	target      = flag.String("target", os.Getenv("GOALLC_LLVM_TARGET"), "LLVM GoObj target triple")
	keepIR      = flag.Bool("keep-ir", false, "keep the compiler-generated .ll sidecar")
	packageOnly = flag.String("package", os.Getenv("GOALLC_LLVM_PACKAGE"), "only replace this Go import path")
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

	output, ok := toolFlag(args, "-o")
	if !ok || output == "" {
		fatalf("compile invocation has no -o output")
	}
	pkg, ok := toolFlag(args, "-p")
	if !ok || pkg == "" {
		fatalf("compile invocation has no -p package path")
	}
	version, ok := toolFlag(args, "-goversion")
	if !ok || version == "" {
		fatalf("compile invocation has no -goversion")
	}

	compileArgs := make([]string, 0, len(args)+2)
	compileArgs = append(compileArgs, "-enablellvm", "-llvmironly")
	compileArgs = append(compileArgs, args...)
	run(tool, compileArgs...)

	irPath := output + ".ll"
	if _, err := os.Stat(irPath); err != nil {
		fatalf("compiler did not produce %s: %v", irPath, err)
	}
	objectHeader, err := archiveGoObjectHeader(output)
	if err != nil {
		fatalf("read Go object header from %s: %v", output, err)
	}
	experiments := goObjExperiments(objectHeader)
	triple := *target
	if triple == "" {
		var err error
		triple, err = defaultTriple(os.Getenv("GOOS"), os.Getenv("GOARCH"))
		if err != nil {
			fatalf("%v", err)
		}
	}

	objPath := filepath.Join(filepath.Dir(output), "llvm-goobj.o")
	llcArgs := []string{
		"-mtriple=" + triple,
		"-goobj-package-path=" + pkg,
		"-goobj-version=" + version,
		"-filetype=obj",
		irPath,
		"-o", objPath,
	}
	if experiments != "" {
		llcArgs = append(llcArgs, "-goobj-experiments="+experiments)
	}
	if hasFlag(args, "-shared") {
		llcArgs = append(llcArgs, "-goobj-shared")
	}
	run(*llcPath, llcArgs...)
	decorateGoObj(objPath, pkg == "main")
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

// archiveGoObjectHeader returns the text header stored in the first
// __.PKGDEF archive member, through its terminating blank line.
func archiveGoObjectHeader(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	const archiveMagic = "!<arch>\n"
	const archiveHeaderSize = 60
	if len(data) < len(archiveMagic)+archiveHeaderSize || string(data[:len(archiveMagic)]) != archiveMagic {
		return "", fmt.Errorf("not a Go archive")
	}
	header := data[len(archiveMagic) : len(archiveMagic)+archiveHeaderSize]
	if strings.TrimSpace(string(header[:16])) != "__.PKGDEF" {
		return "", fmt.Errorf("first archive member is %q, want __.PKGDEF", strings.TrimSpace(string(header[:16])))
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(header[48:58])), 10, 64)
	if err != nil || size < 0 {
		return "", fmt.Errorf("invalid __.PKGDEF size")
	}
	start := len(archiveMagic) + archiveHeaderSize
	end := start + int(size)
	if end > len(data) {
		return "", fmt.Errorf("truncated __.PKGDEF member")
	}
	objectHeaderEnd := strings.Index(string(data[start:end]), "\n\n")
	if objectHeaderEnd < 0 {
		return "", fmt.Errorf("missing Go object header")
	}
	return string(data[start : start+objectHeaderEnd+2]), nil
}

func goObjExperiments(objectHeader string) string {
	for _, field := range strings.Fields(objectHeader) {
		if strings.HasPrefix(field, "X:") {
			return strings.TrimPrefix(field, "X:")
		}
	}
	return ""
}

// decorateGoObj marks a package-main GoObj as main. llc already emits the
// standard Go object text header and raw go120ld payload; cmd/link additionally
// requires the "main" header line before it accepts the executable package.
func decorateGoObj(path string, isMain bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if !isMain {
		return
	}
	marker := strings.Index(string(raw), "\n\n!\n")
	if marker < 0 || !strings.HasPrefix(string(raw), "go object ") {
		fatalf("llc produced an invalid Go object %s", path)
	}
	decorated := make([]byte, 0, len(raw)+len("main\n"))
	decorated = append(decorated, raw[:marker+1]...)
	decorated = append(decorated, "main\n"...)
	decorated = append(decorated, raw[marker+1:]...)
	if err := os.WriteFile(path, decorated, 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

// appendArchiveMember writes a plain ar member without a symbol table. The Go
// linker expects __.PKGDEF to be the first archive member; BSD ar prepends an
// __.SYMDEF member, so invoking it here would make an otherwise valid package
// archive unreadable to cmd/link.
func appendArchiveMember(archivePath, name, memberPath string) {
	data, err := os.ReadFile(memberPath)
	if err != nil {
		fatalf("read %s: %v", memberPath, err)
	}
	if len(name) > 16 {
		fatalf("archive member name %q is too long", name)
	}
	header := formatArchiveHeader(name, len(data))
	if len(header) != 60 {
		fatalf("internal error: archive header for %q is %d bytes", name, len(header))
	}

	f, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		fatalf("open %s: %v", archivePath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fatalf("close %s: %v", archivePath, err)
		}
	}()
	if _, err := f.WriteString(header); err != nil {
		fatalf("write %s: %v", archivePath, err)
	}
	if _, err := f.Write(data); err != nil {
		fatalf("write %s: %v", archivePath, err)
	}
	if len(data)%2 != 0 {
		if _, err := f.Write([]byte{0}); err != nil {
			fatalf("write %s padding: %v", archivePath, err)
		}
	}
}

func formatArchiveHeader(name string, size int) string {
	return fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n", name, 0, 0, 0, 0o644, size)
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

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func defaultTriple(goos, goarch string) (string, error) {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "aarch64-apple-darwin-goobj", nil
	case "linux/amd64":
		return "x86_64-unknown-linux-goobj", nil
	default:
		return "", fmt.Errorf("no default GoObj triple for %s/%s; pass -target", goos, goarch)
	}
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
