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

func TestDebugAddrWithoutDwarfInfo(t *testing.T) {
	er := loader.ErrorReporter{}
	ldr := loader.NewLoader(0, &er)
	fn := ldr.CreateSymForUpdate("synthetic", 0)
	fn.SetType(sym.STEXT)
	addr := ldr.CreateSymForUpdate(".debug_addr", 0)
	addr.SetType(sym.SDWARFSECT)
	unit := &sym.CompilationUnit{Textp: []loader.Sym{fn.Sym()}}
	d := dwctxt{ldr: ldr, arch: sys.ArchAMD64}
	d.writedebugaddr(unit, addr.Sym())
	if got := len(addr.Data()); got != 0 {
		t.Fatalf("function without DWARF references added %d address bytes", got)
	}
}
