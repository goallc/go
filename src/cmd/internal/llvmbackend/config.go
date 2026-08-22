// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package llvmbackend resolves the project-owned LLVM runtime components used
// by cmd/compile's in-process backend.
package llvmbackend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// PassPlugin resolves the GoALLC pre-codegen plugin from the payload recorded
// by cmd/dist, with GOALLC_LLVM_DIR and GOROOT/llvm retained as development
// payload-root fallbacks.
func PassPlugin() (string, error) {
	name, err := passPluginFilename()
	if err != nil {
		return "", err
	}
	for _, root := range payloadRoots() {
		for _, libdir := range []string{"lib", "lib64"} {
			candidate := filepath.Join(root, libdir, name)
			if requireRegularFile(candidate) == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("GoALLC pass plugin not found; rebuild the toolchain with -llvm-dir or select its payload with GOALLC_LLVM_DIR")
}

// Identity hashes the runtime LLVM components which can change without
// changing cmd/compile itself. The caller combines this with the compiler build
// ID and the ordinary gcflags action input.
func Identity() (string, error) {
	plugin, err := PassPlugin()
	if err != nil {
		return "", err
	}
	paths := []string{plugin}

	var candidates []string
	for _, root := range payloadRoots() {
		for _, pattern := range []string{"libLLVM*.dylib", "libLLVM*.so", "libLLVM*.so.*"} {
			matches, err := filepath.Glob(filepath.Join(root, "lib", pattern))
			if err != nil {
				return "", err
			}
			candidates = append(candidates, matches...)
		}
		if len(candidates) != 0 {
			break
		}
	}
	sort.Strings(candidates)
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

	h := sha256.New()
	io.WriteString(h, "goallc in-process llvm backend identity v1\x00")
	for i, path := range paths {
		fmt.Fprintf(h, "file%d\x00", i)
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("hashing %q: %w", path, err)
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hashing %q: %w", path, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("hashing %q: %w", path, closeErr)
		}
	}
	return "goallc-" + hex.EncodeToString(h.Sum(nil)), nil
}

func payloadRoots() []string {
	var roots []string
	if root := strings.TrimSpace(os.Getenv("GOALLC_LLVM_DIR")); root != "" {
		roots = append(roots, root)
	}
	if executable, err := os.Executable(); err == nil {
		toolDir := filepath.Dir(executable)
		goroot := filepath.Dir(filepath.Dir(filepath.Dir(toolDir)))
		if data, err := os.ReadFile(filepath.Join(goroot, "pkg", "goallc-llvm-payload")); err == nil {
			if payload := strings.TrimSpace(string(data)); payload != "" {
				roots = append(roots, payload)
			}
		}
		roots = append(roots, filepath.Join(goroot, "llvm"))
	}
	seen := make(map[string]bool)
	unique := roots[:0]
	for _, root := range roots {
		path, err := filepath.Abs(root)
		if err != nil || seen[path] {
			continue
		}
		seen[path] = true
		unique = append(unique, path)
	}
	return unique
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
