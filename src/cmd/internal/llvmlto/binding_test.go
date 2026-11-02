//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmlto

import (
	"bytes"
	"cmd/internal/llvmbackend"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

// Exercise the same public binding with either a shared or linked plugin.
// The compiler and linker integrations additionally test real package archives.
func TestThinLTOPluginBinding(t *testing.T) {
	thinLTOToolchain(t)
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			t.Fatal(err)
		}
	}
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllAsmParsers()
	ctx := llvm.NewContext()
	defer ctx.Dispose()
	var inputs []llvm.MemoryBuffer
	for _, name := range []string{"debug-caller", "debug-callee"} {
		input, err := llvm.NewMemoryBufferFromFile(filepath.Join(runtime.GOROOT(), "src/cmd/llvmplugin/testdata/thinlto", name+".ll"))
		if err != nil {
			t.Fatal(err)
		}
		// ParseIR takes ownership of its input buffer.
		module, err := ctx.ParseIR(input)
		if err != nil {
			t.Fatal(err)
		}
		defer module.Dispose()
		bitcode, err := module.WritePassPluginThinBitcode(plugin)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, llvm.NewMemoryBufferFromBytes(bitcode.BytesView(), "memory:"+name))
		bitcode.Dispose()
		defer inputs[len(inputs)-1].Dispose()
		if name == "debug-caller" {
			bad, err := module.WritePassPluginThinBitcode(filepath.Join(t.TempDir(), "missing-plugin"))
			if err == nil || !bad.IsNil() {
				t.Fatalf("invalid plugin: buffer=%v, error=%v", bad, err)
			}
		}
	}
	for _, tc := range []struct {
		name   string
		inputs []llvm.MemoryBuffer
		plugin string
	}{
		{"objects", inputs, plugin},
		{"duplicate", []llvm.MemoryBuffer{inputs[0], inputs[0]}, plugin},
		{"invalid-plugin", inputs, filepath.Join(t.TempDir(), "missing-plugin")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objects, err := llvm.RunPassPluginThinLTO(tc.inputs, tc.plugin, "", false, 2)
			if tc.name != "objects" {
				if err == nil || objects != nil {
					t.Fatalf("got %d objects, error=%v", len(objects), err)
				}
				if tc.name == "duplicate" && !strings.Contains(err.Error(), "unique nonempty module identifiers") {
					t.Fatalf("unexpected duplicate input error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(objects) != len(tc.inputs) {
				t.Fatalf("got %d objects for %d inputs", len(objects), len(tc.inputs))
			}
			for _, object := range objects {
				defer object.Dispose()
				if !bytes.HasPrefix(object.BytesView(), []byte("go object ")) {
					t.Fatal("plugin did not return a Go object buffer")
				}
			}
		})
	}
}
