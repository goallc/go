// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/link/internal/loader"
	"cmd/link/internal/sym"
	"testing"
)

func TestDebugAddrWithoutFunctionDIE(t *testing.T) {
	// LLVM assembly supplies Go runtime tables without a DWARF function DIE.
	// The DWARF5 address-table scan must also accept such text symbols.
	ldr := loader.NewLoader(0, &loader.ErrorReporter{})
	fn := ldr.CreateSymForUpdate("assembly", 0)
	fn.SetType(sym.STEXT)
	addr := ldr.CreateSymForUpdate(".debug_addr", 0)
	addr.SetType(sym.SDWARFSECT)
	d := dwctxt{ldr: ldr}
	d.writedebugaddr(&sym.CompilationUnit{Textp: []loader.Sym{fn.Sym()}}, addr.Sym())
	relocs := addr.Relocs()
	if addr.Size() != 0 || relocs.Count() != 0 {
		t.Fatal("function without DWARF acquired debug addresses")
	}
}
