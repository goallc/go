// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loader

import (
	"cmd/internal/goobj"
	"cmd/link/internal/sym"
	"log"
)

// ltoObject has one physical reader and one logical compilation unit per
// original package. Its symbol and file indices always refer to the single
// physical object; package identity is independent of that storage layout.
type ltoObject struct {
	manifest *goobj.LTOManifest
	units    []*sym.CompilationUnit
	owners   map[uint32]int
}

func (l *Loader) SetLTO(manifest *goobj.LTOManifest, libraries map[string]*sym.Library) {
	l.ltoManifest = manifest
	l.ltoLibraries = libraries
}

func (r *oReader) unitFor(i uint32) *sym.CompilationUnit {
	if r.lto != nil {
		if owner, ok := r.lto.owners[i]; ok {
			return r.lto.units[owner]
		}
	}
	return r.unit
}

func (r *oReader) Std() bool {
	if r.ltoStd != nil {
		return *r.ltoStd
	}
	return r.Reader.Std()
}

func (r *oReader) stdFor(i uint32) bool {
	if r.lto != nil {
		if owner, ok := r.lto.owners[i]; ok {
			return r.lto.manifest.Packages[owner].Std
		}
	}
	return r.Std()
}

func (r *oReader) referenceReader(i uint32) *oReader {
	if r.lto == nil {
		return r
	}
	clone := *r
	clone.unit = r.unitFor(i)
	std := r.stdFor(i)
	clone.ltoStd = &std
	return &clone
}

func (l *Loader) attachLTO(r *oReader) {
	m := l.ltoManifest
	if l.ltoReader != nil {
		log.Fatal("multiple compiler objects in single-object FullLTO")
	}
	l.ltoReader = r
	a := &ltoObject{manifest: m, owners: make(map[uint32]int)}
	r.lto = a
	for i, p := range m.Packages {
		lib := l.ltoLibraries[p.Path]
		if lib == nil {
			log.Fatalf("missing FullLTO library %s", p.Path)
		}
		unit := r.unit
		if i != m.Main {
			unit = &sym.CompilationUnit{Lib: lib, FileTable: r.unit.FileTable}
			lib.Units = append(lib.Units, unit)
		}
		a.units = append(a.units, unit)
	}
	type identity struct {
		name string
		abi  uint16
	}
	named := make(map[identity][]uint32)
	for i := uint32(0); i < uint32(r.NAlldef()); i++ {
		s := r.Sym(i)
		if name := s.Name(r.Reader); name != "" {
			key := identity{name, s.ABI()}
			named[key] = append(named[key], i)
		}
	}
	for _, s := range m.Symbols {
		if s.Package < 0 || s.Package >= len(m.Packages) {
			log.Fatal("invalid FullLTO symbol owner")
		}
		if s.Index != nil {
			i := *s.Index
			if i >= uint32(r.ndef) {
				log.Fatalf("invalid FullLTO symbol index for %s", s.Name)
			}
			os := r.Sym(i)
			// LLVM may assign a storage name to anonymous/file-local data.
			// Its explicit index and static ABI are the stable identity.
			if (s.ABI != goobj.SymABIstatic && os.Name(r.Reader) != s.Name) || os.ABI() != s.ABI {
				log.Fatalf("FullLTO symbol identity mismatch: %s[%d], object has %s ABI %d", s.Name, i, os.Name(r.Reader), os.ABI())
			}
			if _, exists := a.owners[i]; exists {
				log.Fatalf("duplicate FullLTO symbol index %d", i)
			}
			a.owners[i] = s.Package
		} else {
			for _, i := range named[identity{s.Name, s.ABI}] {
				a.owners[i] = s.Package
			}
		}
	}
	// Backend-generated function metadata belongs to its function's CU. Shared
	// type descriptors retain their own identity, rather than the first user's.
	work := make([]uint32, 0, len(a.owners))
	for i := range a.owners {
		work = append(work, i)
	}
	for len(work) > 0 {
		i := work[len(work)-1]
		work = work[:len(work)-1]
		for _, aux := range r.Auxs(i) {
			if aux.Type() == goobj.AuxGotype {
				continue
			}
			ref := aux.Sym()
			var target uint32
			switch ref.PkgIdx {
			case goobj.PkgIdxSelf:
				target = ref.SymIdx
			case goobj.PkgIdxHashed64:
				target = uint32(r.ndef) + ref.SymIdx
			case goobj.PkgIdxHashed:
				target = uint32(r.ndef+r.nhashed64def) + ref.SymIdx
			case goobj.PkgIdxNone:
				target = uint32(r.ndef+r.nhashed64def+r.nhasheddef) + ref.SymIdx
			default:
				continue
			}
			if target >= uint32(r.NAlldef()) {
				continue
			}
			if _, exists := a.owners[target]; !exists {
				a.owners[target] = a.owners[i]
				work = append(work, target)
			}
		}
	}
}

// Called after all definitions have been assigned linker symbols, before any
// original package-indexed references or linkname checks are resolved.
func (l *Loader) prepareLTOImports() {
	if l.ltoManifest == nil {
		return
	}
	r := l.ltoReader
	if r == nil {
		log.Fatal("single-object FullLTO compiler object was not loaded")
	}
	l.ltoImports = make(map[string]map[uint32]Sym)
	l.ltoUnits = make(map[Sym]*sym.CompilationUnit)
	for _, p := range l.ltoManifest.Packages {
		l.ltoImports[p.Prefix] = make(map[uint32]Sym)
	}
	for _, s := range l.ltoManifest.Symbols {
		if s.OriginalIndex == nil {
			continue
		}
		if s.Index == nil {
			log.Fatal("FullLTO symbol missing output index")
		}
		refs := l.ltoImports[l.ltoManifest.Packages[s.Package].Prefix]
		if refs[*s.OriginalIndex] != 0 {
			log.Fatal("duplicate FullLTO original symbol index")
		}
		refs[*s.OriginalIndex] = l.toGlobal(r, *s.Index)
	}
	for i, owner := range r.lto.owners {
		global := l.toGlobal(r, i)
		if actual, local := l.toLocal(global); actual == r && local == i {
			l.ltoUnits[global] = r.lto.units[owner]
		}
	}
}

func (l *Loader) checkLTOLinknames() {
	if l.ltoManifest == nil {
		return
	}
	for _, ref := range l.ltoManifest.Linknames {
		if ref.Package < 0 || ref.Package >= len(l.ltoManifest.Packages) {
			log.Fatal("invalid FullLTO reference owner")
		}
		s := l.Lookup(ref.Name, abiToVer(ref.ABI, l.ltoReader.version))
		if s == 0 {
			continue
		} // An optimized-away reference has no output symbol.
		p := &l.ltoManifest.Packages[ref.Package]
		r := *l.ltoReader
		r.unit = l.ltoReader.lto.units[ref.Package]
		r.ltoStd = &p.Std
		l.checkLinkname(&r, ref.Name, s)
	}
}
