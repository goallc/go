//go:build compiler_bootstrap || !cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

import "fmt"

func optimize([]string, string, bool) error {
	return fmt.Errorf("LTO requires a linker built with LLVM support")
}
func emit(string, string, bool) error {
	return fmt.Errorf("LTO requires a linker built with LLVM support")
}
