//go:build !compiler_bootstrap && cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

import (
	"cmd/internal/llvmbackend"
	"sync"

	"github.com/goallc/go-llvm"
)

var initialize sync.Once

// Output slices borrow plugin buffers until archive rewriting calls release.
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
	return llvmbackend.RunThinLTO(names, inputs, plugin, diagnostics, pic, jobs)
}
