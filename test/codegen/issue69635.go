// asmcheck

// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal i64 @codegen.calc(i64 %a)
// LLVM: lshr i64 %a, 17
// LLVM: and i64 {{%.*}}, 1016

func calc(a uint64) uint64 {
	v := a >> 20 & 0x7f
	// amd64: `SHRQ \$17, AX$` `ANDL \$1016, AX$`
	return v << 3
}
