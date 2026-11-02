//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

import (
	"cmd/internal/llvmbackend"
	"fmt"
	"sync"

	"github.com/goallc/go-llvm"
)

var initialize sync.Once

// Output slices borrow LLVM buffers until release. Input buffers remain alive
// until the synchronous plugin call has finished all importing and codegen.
func optimize(names []string, inputs [][]byte, diagnostics string, pic bool, jobs int, arch string) ([][]byte, func(), error) {
	initialize.Do(func() {
		llvm.ParseCommandLineOptions(append([]string{"cmd/link LTO"}, llvmbackend.CodeGenOptions(arch)...), "GoALLC LTO")
	})
	plugin := ""
	if !llvm.UsesLinkedPassPlugin() {
		var err error
		plugin, err = llvmbackend.PassPlugin()
		if err != nil {
			return nil, nil, err
		}
	}
	buffers := make([]llvm.MemoryBuffer, len(inputs))
	for i, input := range inputs {
		buffers[i] = llvm.NewMemoryBufferFromBytes(input, names[i])
		defer buffers[i].Dispose()
	}
	outputs, err := llvm.RunPassPluginThinLTO(buffers, plugin, diagnostics, pic, jobs)
	if err != nil {
		return nil, nil, fmt.Errorf("LTO: %w", err)
	}
	release := func() {
		for _, output := range outputs {
			output.Dispose()
		}
	}
	objects := make([][]byte, len(outputs))
	for i, output := range outputs {
		objects[i] = output.BytesView()
	}
	return objects, release, nil
}
