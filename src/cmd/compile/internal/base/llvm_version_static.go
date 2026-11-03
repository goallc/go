// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap && staticllvm

package base

func llvmVersionBackendIdentity() (string, error) {
	// LLVM and the GoALLC passes are part of this executable. Its content ID
	// already identifies them, including changes to the linked archives.
	// In particular, a static SDK must not need a plugin or build payload to
	// answer -V=full, which cmd/go uses before every build action.
	return "goallc-static-v1", nil
}
