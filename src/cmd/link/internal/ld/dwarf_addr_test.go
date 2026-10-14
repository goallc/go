// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/internal/sys"
	"cmd/link/internal/loader"
	"cmd/link/internal/sym"
	"testing"
)

func TestDebugAddrWithoutFuncInfo(t *testing.T) {
	ldr := loader.NewLoader(0, &loader.ErrorReporter{})
	fn := ldr.CreateSymForUpdate("main.noDwarf", 0)
	fn.SetType(sym.STEXT)
	addr := ldr.CreateSymForUpdate(".debug_addr", 0)
	addr.SetType(sym.SDWARFADDR)
	unit := &sym.CompilationUnit{
		Lib:   &sym.Library{Pkg: "main"},
		Textp: []sym.LoaderSym{sym.LoaderSym(fn.Sym())},
	}

	// LLVM packages built with -dwarf=false still have text symbols and
	// pclntab data, but no AuxDwarfInfo. DWARF5 must not read symbol zero.
	d := &dwctxt{ldr: ldr, arch: sys.ArchAMD64}
	d.writedebugaddr(unit, addr.Sym())
	if len(addr.Data()) != 0 || len(unit.Addrs) != 0 {
		t.Fatal("function without DWARF references created debug_addr entries")
	}
}
