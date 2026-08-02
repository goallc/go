// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// A package with neither imports nor init functions must not synthesize an
// initialization task merely because LLVM data lowering scans for one.

// LLVM-NOT: @codegen..inittask
// LLVM-NOT: !goobj.marker_relocs

var llvmNoInitTaskValue int
