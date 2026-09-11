// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/token"
	"go/types"
	_ "unsafe"
)

// An imported method's package index must survive a linkname to the same
// symbol. Treating it as a name-based pull incorrectly fails the link check.
//
//go:linkname validType go/types.(*Checker).validType
func validType(*types.Checker, *types.Named)

func main() {
	pkg := types.NewPackage("example.com/p", "p")
	check := types.NewChecker(&types.Config{}, token.NewFileSet(), pkg, nil)
	typ := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "T", nil), types.Typ[types.Int], nil)
	validType(check, typ)
	if typ.Underlying() != types.Typ[types.Int] {
		panic("unexpected underlying type")
	}
}
