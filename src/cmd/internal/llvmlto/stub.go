//go:build compiler_bootstrap || !cgo

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
package llvmlto

import "fmt"

func optimize([]string, [][]byte, string, bool, int, string) ([][]byte, func(), error) {
	return nil, nil, fmt.Errorf("LTO requires a linker built with LLVM support")
}
