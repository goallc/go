// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type goallcPluginStamp struct {
	Input  string `json:"input"`
	Plugin string `json:"plugin"`
}

func goallcPassPluginFilename(goos string) string {
	if goos == "darwin" {
		return "GoALLCStatepoints.dylib"
	}
	return "GoALLCStatepoints.so"
}

func writeGoallcLLVMPayloadConfig() {
	path := pathf("%s/pkg/goallc-llvm-payload", goroot)
	xmkdirall(filepath.Dir(path))
	temporary, err := os.CreateTemp(filepath.Dir(path), ".goallc-llvm-payload-")
	if err != nil {
		fatalf("creating LLVM payload configuration: %v", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := fmt.Fprintln(temporary, goallcLLVMDir); err != nil {
		temporary.Close()
		fatalf("writing LLVM payload configuration: %v", err)
	}
	if err := temporary.Close(); err != nil {
		fatalf("writing LLVM payload configuration: %v", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		fatalf("installing LLVM payload configuration: %v", err)
	}
}

// ensureGoallcPassPlugin keeps the Go-owned pass plugin in the selected LLVM
// payload synchronized with its sources and the exact llc that loads it.
func ensureGoallcPassPlugin() {
	sourceDir := pathf("%s/src/cmd/llvmplugin", goroot)
	buildDir := pathf("%s/pkg/goallc-llvmplugin", goroot)
	llc := pathf("%s/bin/llc", goallcLLVMDir)
	llvmConfig := pathf("%s/bin/llvm-config", goallcLLVMDir)
	destination := pathf("%s/lib/%s", goallcLLVMDir, goallcPassPluginFilename(gohostos))
	stampPath := pathf("%s/goallc-plugin.stamp", buildDir)

	runtimeLibraries, err := goallcLLVMRuntimeLibraries(llvmConfig)
	if err != nil {
		fatalf("resolving LLVM runtime libraries: %v", err)
	}
	input, err := goallcPluginInputID(sourceDir, llc, llvmConfig, runtimeLibraries...)
	if err != nil {
		fatalf("computing GoALLC pass plugin identity: %v", err)
	}
	if stamp, err := readGoallcPluginStamp(stampPath); err == nil && stamp.Input == input {
		if plugin, err := goallcFileSHA256(destination); err == nil && plugin == stamp.Plugin {
			return
		}
	}

	xprintf("Building GoALLC pass plugin for LLVM payload.\n")
	xmkdirall(buildDir)
	buildModeOutput, err := exec.Command(llvmConfig, "--build-mode").Output()
	if err != nil {
		fatalf("reading LLVM build mode: %v", err)
	}
	buildMode := strings.TrimSpace(string(buildModeOutput))
	if buildMode == "" {
		buildMode = "Release"
	}
	run("", ShowOutput|CheckExit, "cmake",
		"-S", sourceDir,
		"-B", buildDir,
		"-G", "Ninja",
		"-DLLVM_DIR="+pathf("%s/lib/cmake/llvm", goallcLLVMDir),
		"-DCMAKE_BUILD_TYPE="+buildMode,
	)
	run("", ShowOutput|CheckExit, "cmake", "--build", buildDir, "--target", "GoALLCStatepoints")

	built := pathf("%s/%s", buildDir, goallcPassPluginFilename(gohostos))
	if err := installGoallcPluginAtomically(built, destination); err != nil {
		fatalf("installing GoALLC pass plugin: %v", err)
	}
	plugin, err := goallcFileSHA256(destination)
	if err != nil {
		fatalf("hashing installed GoALLC pass plugin: %v", err)
	}
	if err := writeGoallcPluginStamp(stampPath, goallcPluginStamp{Input: input, Plugin: plugin}); err != nil {
		fatalf("writing GoALLC pass plugin stamp: %v", err)
	}
}

func goallcPluginInputID(sourceDir, llc, llvmConfig string, runtimeLibraries ...string) (string, error) {
	h := sha256.New()
	io.WriteString(h, "goallc pass plugin input v1\x00")
	var paths []string
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != sourceDir && (strings.HasPrefix(entry.Name(), "cmake-build") || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if entry.Name() == "CMakeLists.txt" || ext == ".cmake" || ext == ".c" || ext == ".cc" || ext == ".cpp" || ext == ".h" || ext == ".hpp" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	paths = append(paths, llc, llvmConfig, filepath.Join(filepath.Dir(llvmConfig), "..", "lib", "cmake", "llvm", "LLVMConfig.cmake"))
	paths = append(paths, runtimeLibraries...)
	sort.Strings(paths)
	for _, path := range paths {
		rel := path
		if candidate, err := filepath.Rel(sourceDir, path); err == nil && !strings.HasPrefix(candidate, "..") {
			rel = candidate
		}
		io.WriteString(h, rel+"\x00")
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func goallcLLVMRuntimeLibraries(llvmConfig string) ([]string, error) {
	root := filepath.Dir(filepath.Dir(llvmConfig))
	var candidates []string
	for _, pattern := range []string{"libLLVM*.dylib", "libLLVM*.so", "libLLVM*.so.*"} {
		matches, err := filepath.Glob(filepath.Join(root, "lib", pattern))
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, matches...)
	}
	seen := make(map[string]bool)
	var libraries []string
	for _, candidate := range candidates {
		real := candidate
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			real = resolved
		}
		if !seen[real] {
			seen[real] = true
			libraries = append(libraries, real)
		}
	}
	return libraries, nil
}

func goallcFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readGoallcPluginStamp(path string) (goallcPluginStamp, error) {
	var stamp goallcPluginStamp
	data, err := os.ReadFile(path)
	if err != nil {
		return stamp, err
	}
	if err := json.Unmarshal(data, &stamp); err != nil {
		return stamp, err
	}
	if stamp.Input == "" || stamp.Plugin == "" {
		return stamp, fmt.Errorf("incomplete stamp")
	}
	return stamp, nil
}

func writeGoallcPluginStamp(path string, stamp goallcPluginStamp) error {
	data, err := json.Marshal(stamp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".goallc-plugin-stamp-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func installGoallcPluginAtomically(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".goallc-plugin-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := io.Copy(temporary, in); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, destination)
}
