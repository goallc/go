// buildrun

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import _ "embed"

//go:embed llvm_file_backed_embed.go
var llvmFileBackedSource string

//go:embed llvm_file_backed_embed.go
var llvmFileBackedBytes []byte

const llvmFileBackedSentinel = "GOALLC_FILE_BACKED_EMBED_SENTINEL"

func containsFileBackedSentinel(data, sentinel string) bool {
	for i := 0; i+len(sentinel) <= len(data); i++ {
		if data[i:i+len(sentinel)] == sentinel {
			return true
		}
	}
	return false
}

func main() {
	if len(llvmFileBackedSource) <= 1024 {
		panic("fixture did not use a file-backed linker symbol")
	}
	if !containsFileBackedSentinel(llvmFileBackedSource, llvmFileBackedSentinel) {
		panic("file-backed linker symbol lost its contents")
	}
	if len(llvmFileBackedBytes) != len(llvmFileBackedSource) || !containsFileBackedSentinel(string(llvmFileBackedBytes), llvmFileBackedSentinel) {
		panic("writable file-backed linker symbol lost its contents")
	}
	original := llvmFileBackedSource[0]
	llvmFileBackedBytes[0] ^= 0xff
	if llvmFileBackedBytes[0] == original || llvmFileBackedSource[0] != original {
		panic("writable file-backed linker symbol did not retain separate storage")
	}
}

// Keep this checked-in fixture larger than staticdata.fileStringSym's 1 KiB
// in-memory threshold. The embedded file is this source itself, so testdir's
// buildrun recipe supplies a real embed configuration through the Go command.
// The padding is deliberately readable and stable: it is part of the compiler
// input and lets the runtime assertion
// distinguish a correctly materialized LLVM constant from a zero initializer.
//
// LLVM file-backed data padding 01: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 02: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 03: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 04: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 05: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 06: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 07: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 08: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 09: abcdefghijklmnopqrstuvwxyz0123456789
// LLVM file-backed data padding 10: abcdefghijklmnopqrstuvwxyz0123456789
