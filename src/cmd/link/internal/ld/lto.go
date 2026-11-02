// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import (
	"cmd/internal/goobj"
	"cmd/internal/objabi"
	"encoding/hex"
	"strconv"
)

// The temporary dependency archives contain assembly/native objects only.
// Their Go package identities and import DAG come from the FullLTO manifest.
func (ctxt *Link) addLTOLibraries() {
	fingerprint := func(value string) (fp goobj.FingerprintType) {
		b, err := hex.DecodeString(value)
		if err != nil || len(b) != len(fp) {
			Exitf("invalid LTO fingerprint %q", value)
		}
		copy(fp[:], b)
		return fp
	}
	for _, p := range ctxt.ltoManifest.Packages {
		file := ctxt.PackageFile[p.Path]
		if p.Path == "main" {
			file = ctxt.ltoManifest.Archive
		}
		if file == "" {
			Exitf("LTO package %q has no archive", p.Path)
		}
		lib := addlibpath(ctxt, "FullLTO", "FullLTO", file, p.Path, "", fingerprint(p.Fingerprint))
		lib.Fingerprint = fingerprint(p.Fingerprint)
	}
	for _, p := range ctxt.ltoManifest.Packages {
		lib := ctxt.LibraryByPkg[p.Path]
		for _, quoted := range p.Cgo {
			directives, err := strconv.Unquote(quoted)
			if err != nil {
				Exitf("invalid FullLTO cgo directives in %s: %v", p.Path, err)
			}
			loadcgo(ctxt, lib.File, p.Prefix, directives)
		}
		for _, imp := range p.Imports {
			if objabi.PathToPrefix(imp.Path) != imp.Prefix {
				Exitf("invalid LTO import identity %q", imp.Path)
			}
			lib.Autolib = append(lib.Autolib, goobj.ImportedPkg{Pkg: imp.Path, Fingerprint: fingerprint(imp.Fingerprint)})
		}
		addImports(ctxt, lib, "FullLTO")
	}
}
