// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// llvmlto is an experimental go build -toolexec driver for FullLTO followed
// by the unmodified Go linker. Package archives retain ordinary Go objects
// and a pre-codegen IR member until the final link action.
package main

import (
	"cmd/internal/archive"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var (
	backend  = flag.String("backend", "", "path to goallc-lto")
	llc      = flag.String("llc", "", "path to matching GoALLC llc")
	plugin   = flag.String("plugin", "", "path to GoALLCStatepoints plugin")
	packages = flag.String("packages", "all", "comma-separated import paths or path/... patterns to optimize")
	keep     = flag.Bool("keep", false, "keep and print the LTO work directory")
)

const irMember = "__.LLVMBC"

func main() {
	flag.Parse()
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, "llvmlto:", err)
		os.Exit(1)
	}
}

func execute() error {
	if flag.NArg() == 0 {
		return fmt.Errorf("missing Go tool")
	}
	if *backend == "" || *llc == "" || *plugin == "" {
		return fmt.Errorf("-backend, -llc and -plugin are required")
	}
	tool, args := flag.Arg(0), flag.Args()[1:]
	for _, a := range args {
		a = canonicalFlag(a)
		if a == "-V=full" {
			out, err := exec.Command(tool, args...).Output()
			if err != nil {
				return err
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			paths := []string{exe, *backend, *llc, *plugin, llvmTool("llvm-as"), llvmTool("llvm-dis")}
			libraries, err := llvmLibraries(*llc, *plugin)
			if err != nil {
				return err
			}
			id, err := cacheIdentity(append(paths, libraries...), *packages, out)
			if err != nil {
				return err
			}
			version := strings.TrimSpace(string(out))
			if i := strings.Index(version, " buildID="); i >= 0 {
				version = version[:i]
			}
			fmt.Printf("%s buildID=llvmlto-%s\n", version, id)
			return nil
		}
	}
	switch filepath.Base(tool) {
	case "compile":
		path := os.Getenv("TOOLEXEC_IMPORTPATH")
		if !selected(path) {
			return run(tool, args...)
		}
		if err := validateCompileArgs(args); err != nil {
			return err
		}
		out := argValue(args, "-o")
		if out == "" {
			return fmt.Errorf("compiler output missing")
		}
		args = append([]string{"-enablellvm", "-llvm-keep-ir"}, args...)
		if err := run(tool, args...); err != nil {
			return err
		}
		bc := out + ".lto.bc"
		if err := run(llvmTool("llvm-as"), out+".ll", "-o", bc); err != nil {
			return err
		}
		defer os.Remove(bc)
		ir, err := os.Open(bc)
		if err != nil {
			return err
		}
		defer ir.Close()
		info, err := ir.Stat()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(out, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := archive.Parse(f, false); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return err
		}
		if err := writeArchiveEntry(f, irMember, info.Size(), ir); err != nil {
			return err
		}
		return f.Close()
	case "link":
		return link(tool, args)
	default:
		return run(tool, args...)
	}
}

// llvmLibraries includes the shared LLVM implementation as well as its tools in
// the Go action identity. Rebuilding libLLVM must invalidate cached fat archives.
func llvmLibraries(llcPath, pluginPath string) ([]string, error) {
	var paths []string
	for _, dir := range []string{filepath.Join(filepath.Dir(llcPath), "..", "lib"), filepath.Dir(pluginPath)} {
		for _, pattern := range []string{"libLLVM*.dylib", "libLLVM*.so", "libLLVM*.so.*"} {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return nil, err
			}
			paths = append(paths, matches...)
		}
	}
	return paths, nil
}

func cacheIdentity(paths []string, selection string, version []byte) (string, error) {
	h := sha256.New()
	seen := make(map[string]bool)
	for _, path := range paths {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		if seen[real] {
			continue
		}
		seen[real] = true
		f, err := os.Open(real)
		if err != nil {
			return "", err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return "", err
		}
		// Length framing distinguishes ["ab", "c"] from ["a", "bc"].
		fmt.Fprintf(h, "file %d\n", info.Size())
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}
	fmt.Fprintf(h, "packages %d\n%s", len(selection), selection)
	fmt.Fprintf(h, "tool-version %d\n", len(version))
	h.Write(version)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// The driver reruns a fixed O2 pipeline and default relocation code generation.
// Reject options whose meaning cannot be preserved by that second compilation.
func validateCompileArgs(args []string) error {
	for _, name := range []string{"-shared", "-dynlink", "-linkshared", "-race", "-msan", "-asan"} {
		if value, ok := boolArg(args, name); ok && value {
			return fmt.Errorf("FullLTO does not support compiler option %s", name)
		}
	}
	for _, name := range []string{"-enablellvm", "-llvm-keep-ir"} {
		if value, ok := boolArg(args, name); ok && !value {
			return fmt.Errorf("FullLTO requires %s=true", name)
		}
	}
	noopt := 0
	customPipeline := false
	for _, a := range args {
		a = canonicalFlag(a)
		if a == "-N" {
			noopt++
		} else if strings.HasPrefix(a, "-N=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "-N=")); err == nil {
				noopt = n
			}
		}
		if a == "-llvm-opt-passes" || strings.HasPrefix(a, "-llvm-opt-passes=") {
			customPipeline = true
		}
	}
	if noopt != 0 {
		return fmt.Errorf("FullLTO does not support -N (the LTO pipeline enables optimization)")
	}
	if pipeline := argValue(args, "-llvm-opt-passes"); customPipeline && pipeline != "auto" {
		return fmt.Errorf("FullLTO does not support -llvm-opt-passes=%q (the LTO pipeline is fixed)", pipeline)
	}
	if argValue(args, "-linkobj") != "" {
		return fmt.Errorf("FullLTO does not support separate -linkobj output")
	}
	return nil
}

// Go boolean flags accept -name or -name=value, with the last value winning.
// Invalid values are left for the real tool to diagnose.
func boolArg(args []string, name string) (value, present bool) {
	for _, a := range args {
		a = canonicalFlag(a)
		if a == name {
			value, present = true, true
		} else if strings.HasPrefix(a, name+"=") {
			if b, err := strconv.ParseBool(strings.TrimPrefix(a, name+"=")); err == nil {
				value, present = b, true
			}
		}
	}
	return
}

func llvmTool(name string) string { return filepath.Join(filepath.Dir(*llc), name) }

func selected(path string) bool {
	for _, p := range strings.Split(*packages, ",") {
		if p == "all" || path == p {
			return true
		}
		if strings.HasSuffix(p, "/...") {
			root := strings.TrimSuffix(p, "/...")
			if path == root || strings.HasPrefix(path, root+"/") {
				return true
			}
		}
	}
	return false
}

// The flag package accepts both -name and --name.
func canonicalFlag(arg string) string {
	if strings.HasPrefix(arg, "--") {
		return arg[1:]
	}
	return arg
}

func argValue(args []string, name string) string {
	value := ""
	for i := 0; i < len(args); i++ {
		a := canonicalFlag(args[i])
		if a == name && i+1 < len(args) {
			i++
			value = args[i]
		} else if strings.HasPrefix(a, name+"=") {
			value = strings.TrimPrefix(a, name+"=")
		}
	}
	return value
}

func replaceArg(args []string, name, value string) {
	for i := 0; i < len(args); i++ {
		a := canonicalFlag(args[i])
		if a == name && i+1 < len(args) {
			i++
			args[i] = value
		} else if strings.HasPrefix(a, name+"=") {
			args[i] = name + "=" + value
		}
	}
}

func run(tool string, args ...string) error {
	c := exec.Command(tool, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", tool, err)
	}
	return nil
}

type input struct{ path, ir, output string }

func readIR(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	a, err := archive.Parse(f, false)
	if err != nil {
		return nil, err
	}
	var ir []byte
	for _, e := range a.Entries {
		if e.Name != irMember {
			continue
		}
		if ir != nil {
			return nil, fmt.Errorf("%s: duplicate %s member", path, irMember)
		}
		if e.Size == 0 {
			return nil, fmt.Errorf("%s: empty %s member", path, irMember)
		}
		ir, err = io.ReadAll(io.NewSectionReader(f, e.Offset, e.Size))
		if err != nil {
			return nil, err
		}
		if int64(len(ir)) != e.Size {
			return nil, io.ErrUnexpectedEOF
		}
	}
	return ir, nil
}

func validateLinkArgs(args []string) error {
	if mode := argValue(args, "-buildmode"); mode != "" && mode != "exe" {
		return fmt.Errorf("FullLTO currently supports buildmode=exe, not %s", mode)
	}
	for _, name := range []string{"-linkshared", "-race", "-msan", "-asan"} {
		if value, ok := boolArg(args, name); ok && value {
			return fmt.Errorf("FullLTO does not support linker option %s", name)
		}
	}
	if len(args) == 0 {
		return fmt.Errorf("link input missing")
	}
	return nil
}

func link(tool string, args []string) error {
	if err := validateLinkArgs(args); err != nil {
		return err
	}
	cfg := argValue(args, "-importcfg")
	if cfg == "" {
		return fmt.Errorf("link importcfg missing")
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	paths := map[string]bool{args[len(args)-1]: true}
	for _, line := range lines {
		if strings.HasPrefix(line, "packagefile ") {
			_, path, ok := strings.Cut(line, "=")
			if !ok {
				return fmt.Errorf("invalid packagefile")
			}
			paths[path] = true
		}
	}
	var names []string
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	work, err := os.MkdirTemp("", "go-full-lto-")
	if err != nil {
		return err
	}
	if *keep {
		fmt.Fprintln(os.Stderr, "LTO_WORK="+work)
	} else {
		defer os.RemoveAll(work)
	}
	var inputs []input
	for _, path := range names {
		ir, err := readIR(path)
		if err != nil {
			return err
		}
		if ir == nil {
			continue
		}
		base := filepath.Join(work, fmt.Sprint(len(inputs)))
		if err := os.WriteFile(base+".input.bc", ir, 0600); err != nil {
			return err
		}
		if *keep {
			if err := run(llvmTool("llvm-dis"), base+".input.bc", "-o", base+".input.ll"); err != nil {
				return err
			}
		}
		inputs = append(inputs, input{path, base + ".input.bc", base + ".a"})
	}
	if len(inputs) == 0 {
		return run(tool, args...)
	}
	backendArgs := []string{"--plugin=" + *plugin, "--output-dir=" + filepath.Join(work, "optimized")}
	for _, in := range inputs {
		backendArgs = append(backendArgs, in.ir)
	}
	if err := run(*backend, backendArgs...); err != nil {
		return err
	}
	replacements := make(map[string]string)
	for i, in := range inputs {
		ir := filepath.Join(work, "optimized", fmt.Sprint(i)+".ll")
		obj := filepath.Join(work, fmt.Sprint(i)+".o")
		if err := run(*llc, "-load-pass-plugin="+*plugin, "-trap-unreachable", "-disable-machine-cse", "-force-loop-cold-block", "-disable-lsr", "-aarch64-goobj-composite-relocations", "-filetype=obj", ir, "-o", obj); err != nil {
			return err
		}
		if err := rewriteArchive(in.path, in.output, obj); err != nil {
			return err
		}
		replacements[in.path] = in.output
	}
	for i, line := range lines {
		if strings.HasPrefix(line, "packagefile ") {
			key, path, _ := strings.Cut(line, "=")
			if replacement := replacements[path]; replacement != "" {
				lines[i] = key + "=" + replacement
			}
		}
	}
	newCfg := filepath.Join(work, "importcfg")
	if err := os.WriteFile(newCfg, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return err
	}
	args = append([]string{}, args...)
	replaceArg(args, "-importcfg", newCfg)
	if r := replacements[args[len(args)-1]]; r != "" {
		args[len(args)-1] = r
	}
	return run(tool, args...)
}

// writeArchiveEntry uses the same header as cmd/pack, but propagates write
// errors instead of archive.AddEntry's process-wide log.Fatal path.
func writeArchiveEntry(out io.Writer, name string, size int64, content io.Reader) error {
	var header [archive.HeaderSize]byte
	archive.FormatHeader(header[:], name, size)
	if n, err := out.Write(header[:]); err != nil {
		return err
	} else if n != len(header) {
		return io.ErrShortWrite
	}
	if _, err := io.CopyN(out, content, size); err != nil {
		return err
	}
	if size&1 != 0 {
		if n, err := out.Write([]byte{0}); err != nil {
			return err
		} else if n != 1 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func rewriteArchive(src, dst, object string) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	a, err := archive.Parse(f, false)
	if err != nil {
		return err
	}
	count := 0
	for _, e := range a.Entries {
		if e.Name == "_go_.o" {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("%s: expected one compiler GoObj member, found %d", src, count)
	}
	obj, err := os.Open(object)
	if err != nil {
		return err
	}
	defer obj.Close()
	info, err := obj.Stat()
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(dst)
		}
	}()
	if _, err := archive.New(out); err != nil {
		return err
	}
	for _, e := range a.Entries {
		if e.Name == irMember {
			continue
		}
		if e.Name == "_go_.o" {
			if err := writeArchiveEntry(out, e.Name, info.Size(), obj); err != nil {
				return err
			}
		} else {
			// Preserve the complete member, including its header and padding.
			size := archive.HeaderSize + e.Size + (e.Size & 1)
			if _, err := io.CopyN(out, io.NewSectionReader(f, e.Offset-archive.HeaderSize, size), size); err != nil {
				return err
			}
		}
	}
	return nil
}
