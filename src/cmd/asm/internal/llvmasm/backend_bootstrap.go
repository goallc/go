// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"fmt"
)

func Emit(_ *obj.Link, _ Options) ([]byte, error) {
	return nil, fmt.Errorf("LLVM assembly is unavailable in the bootstrap assembler")
}
