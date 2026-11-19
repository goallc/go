// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/link/internal/loader"
	"cmd/link/internal/sym"
	"strings"
)

// Populate runtime/llvmprofile.packages before dead-code elimination so a
// function that never ran still has a zero-count record in the profile.
func (ctxt *Link) llvmProfileTables() {
	ldr := ctxt.loader
	header := ldr.Lookup("runtime/llvmprofile.packages", 0)
	if header == 0 {
		return
	}
	tables := ldr.MakeSymbolBuilder("go:llvmpgotables")
	tables.SetType(sym.SRODATA)
	for s, end := loader.Sym(1), loader.Sym(ldr.NSym()); s < end; s++ {
		if strings.HasPrefix(ldr.SymName(s), "go:llvmpgopkg.") && ldr.SymType(s) == sym.SRODATA {
			tables.AddAddr(ctxt.Arch, s)
		}
	}
	n := uint64(tables.Size() / int64(ctxt.Arch.PtrSize))
	h := ldr.MakeSymbolUpdater(header)
	h.SetType(sym.SNOPTRDATA)
	h.SetSize(0)
	h.AddAddr(ctxt.Arch, tables.Sym())
	h.AddUint(ctxt.Arch, n)
	h.AddUint(ctxt.Arch, n)
}
