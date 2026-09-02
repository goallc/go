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
	Input        string `json:"input"`
	Plugin       string `json:"plugin"`
	StaticPlugin string `json:"static_plugin"`
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

// ensureGoallcPassPlugin keeps the shared and static forms of the Go-owned pass
// plugin synchronized with the LLVM payload used to build the Go toolchain.
func ensureGoallcPassPlugin() {
	sourceDir := pathf("%s/src/cmd/llvmplugin", goroot)
	buildDir := pathf("%s/pkg/goallc-llvmplugin", goroot)
	installDir := pathf("%s/lib", buildDir)
	llc := pathf("%s/bin/llc", goallcLLVMDir)
	llvmConfig := pathf("%s/bin/llvm-config", goallcLLVMDir)
	destination := pathf("%s/%s", installDir, goallcPassPluginFilename(gohostos))
	staticDestination := pathf("%s/libGoALLCStatepointsStatic.a", installDir)
	stampPath := pathf("%s/goallc-plugin.stamp", buildDir)

	buildModeOutput, err := exec.Command(llvmConfig, "--build-mode").Output()
	if err != nil {
		fatalf("reading LLVM build mode: %v", err)
	}
	buildMode := strings.TrimSpace(string(buildModeOutput))
	if buildMode == "" {
		buildMode = "Release"
	}
	configuration := []string{
		"-DLLVM_DIR=" + pathf("%s/lib/cmake/llvm", goallcLLVMDir),
		"-DCMAKE_INSTALL_PREFIX=" + buildDir,
		"-DCMAKE_BUILD_TYPE=" + buildMode,
		"-DBUILD_TESTING=OFF",
	}
	if gohostos == "darwin" {
		deploymentTarget, err := goallcDarwinDeploymentTarget(goallcLLVMDir)
		if err != nil {
			fatalf("resolving GoALLC macOS deployment target: %v", err)
		}
		if deploymentTarget != "" {
			configuration = append(configuration, "-DCMAKE_OSX_DEPLOYMENT_TARGET="+deploymentTarget)
		}
	}
	runtimeLibraries, err := goallcLLVMRuntimeLibraries(llvmConfig)
	if err != nil {
		fatalf("resolving LLVM runtime libraries: %v", err)
	}
	input, err := goallcPluginInputID(sourceDir, llc, llvmConfig, configuration, runtimeLibraries...)
	if err != nil {
		fatalf("computing GoALLC pass plugin identity: %v", err)
	}
	if stamp, err := readGoallcPluginStamp(stampPath); err == nil && stamp.Input == input {
		if plugin, err := goallcFileSHA256(destination); err == nil && plugin == stamp.Plugin {
			if staticPlugin, err := goallcFileSHA256(staticDestination); err == nil && staticPlugin == stamp.StaticPlugin {
				return
			}
		}
	}

	xprintf("Building GoALLC pass plugin for Go toolchain.\n")
	// The input identity includes the installed LLVM headers and CMake package,
	// but Ninja normally decides whether to rebuild from timestamps. An LLVM
	// payload restored from an archive can therefore have different contents
	// with older timestamps and leave stale plugin objects behind. Keep ccache,
	// but discard the CMake/Ninja state whenever the content identity changes.
	xremoveall(buildDir)
	xmkdirall(buildDir)
	xmkdirall(installDir)
	cmakeArgs := []string{
		"cmake",
		"-S", sourceDir,
		"-B", buildDir,
		"-G", "Ninja",
	}
	cmakeArgs = append(cmakeArgs, configuration...)
	cmakeArgs = append(cmakeArgs, goallcCMakeLauncherArgs()...)
	run("", ShowOutput|CheckExit, cmakeArgs...)
	run("", ShowOutput|CheckExit, "cmake", "--build", buildDir, "--target", "GoALLCStatepoints", "GoALLCStatepointsStatic")

	built := pathf("%s/%s", buildDir, goallcPassPluginFilename(gohostos))
	if err := installGoallcPluginAtomically(built, destination); err != nil {
		fatalf("installing GoALLC pass plugin: %v", err)
	}
	staticBuilt := pathf("%s/libGoALLCStatepointsStatic.a", buildDir)
	if err := installGoallcPluginAtomically(staticBuilt, staticDestination); err != nil {
		fatalf("installing static GoALLC pass plugin: %v", err)
	}
	plugin, err := goallcFileSHA256(destination)
	if err != nil {
		fatalf("hashing installed GoALLC pass plugin: %v", err)
	}
	staticPlugin, err := goallcFileSHA256(staticDestination)
	if err != nil {
		fatalf("hashing installed static GoALLC pass plugin: %v", err)
	}
	if err := writeGoallcPluginStamp(stampPath, goallcPluginStamp{Input: input, Plugin: plugin, StaticPlugin: staticPlugin}); err != nil {
		fatalf("writing GoALLC pass plugin stamp: %v", err)
	}
}

func goallcPluginInputID(sourceDir, llc, llvmConfig string, configuration []string, runtimeLibraries ...string) (string, error) {
	h := sha256.New()
	io.WriteString(h, "goallc pass plugin input v5\x00")
	for _, setting := range configuration {
		io.WriteString(h, "configuration\x00"+setting+"\x00")
	}
	var paths []string
	collect := func(root string, include func(string, fs.DirEntry) bool) error {
		return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != root && (strings.HasPrefix(entry.Name(), "cmake-build") || entry.Name() == ".git") {
					return filepath.SkipDir
				}
				return nil
			}
			if include(path, entry) {
				paths = append(paths, path)
			}
			return nil
		})
	}
	err := collect(sourceDir, func(path string, entry fs.DirEntry) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return entry.Name() == "CMakeLists.txt" || ext == ".cmake" || ext == ".c" || ext == ".cc" || ext == ".cpp" || ext == ".h" || ext == ".hpp"
	})
	if err != nil {
		return "", err
	}
	payloadRoot := filepath.Dir(filepath.Dir(llvmConfig))
	for _, root := range []string{
		filepath.Join(payloadRoot, "include", "llvm"),
		filepath.Join(payloadRoot, "include", "llvm-c"),
		filepath.Join(payloadRoot, "lib", "cmake", "llvm"),
	} {
		err := collect(root, func(path string, entry fs.DirEntry) bool {
			ext := strings.ToLower(filepath.Ext(path))
			return ext == ".cmake" || ext == ".def" || ext == ".h" || ext == ".inc" || ext == ".td"
		})
		if err != nil {
			return "", err
		}
	}
	paths = append(paths, llc, llvmConfig)
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

func goallcDarwinDeploymentTarget(payloadRoot string) (string, error) {
	target := os.Getenv("MACOSX_DEPLOYMENT_TARGET")
	if target == "" {
		manifest := filepath.Join(payloadRoot, "share", "goallc", "build-manifest")
		data, err := os.ReadFile(manifest)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if value, ok := strings.CutPrefix(line, "macos_deployment_target="); ok {
					target = value
					break
				}
			}
		}
	}
	if target == "" {
		return "", nil
	}
	for _, component := range strings.Split(target, ".") {
		if component == "" || strings.Trim(component, "0123456789") != "" {
			return "", fmt.Errorf("invalid value %q", target)
		}
	}
	return target, nil
}

func goallcCMakeLauncherArgs() []string {
	ccache := os.Getenv("GOALLC_CCACHE")
	if ccache == "" {
		ccache, _ = exec.LookPath("ccache")
	}
	if ccache == "" {
		return nil
	}
	abs, err := filepath.Abs(ccache)
	if err != nil {
		fatalf("resolving GoALLC ccache %q: %v", ccache, err)
	}
	if err := requireExecutableFile(abs); err != nil {
		fatalf("invalid GoALLC ccache %q: %v", abs, err)
	}
	xprintf("Using ccache for the GoALLC pass plugin: %s\n", abs)
	return []string{
		"-DCMAKE_C_COMPILER_LAUNCHER=" + abs,
		"-DCMAKE_CXX_COMPILER_LAUNCHER=" + abs,
	}
}

func requireExecutableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("not executable")
	}
	return nil
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
	if stamp.Input == "" || stamp.Plugin == "" || stamp.StaticPlugin == "" {
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
