// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package llvmasm carries Go-encoded assembly in naked LLVM functions.
package llvmasm

// Options selects the LLVM artifact and target object format.
type Options struct {
	GOOS, GOARCH string
	Output       string // obj, bc, or ir
	ObjectFormat string // goobj (default), elf, or macho
}
