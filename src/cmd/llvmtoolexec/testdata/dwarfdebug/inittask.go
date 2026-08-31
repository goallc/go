// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import _ "unsafe"

type initTaskAlias struct {
	state uint32
	nfns  uint32
}

// mainInitTask aliases compiler-generated storage and therefore has no
// independent GoObj gotype mapping or global DWARF entry.
//
//go:linkname mainInitTask main..inittask
var mainInitTask initTaskAlias
