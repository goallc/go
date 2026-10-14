// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goobj

// LTOManifest preserves logical package identities when FullLTO emits one
// physical Go object. It is an ephemeral cmd/link input, not a new GoObj format.
// Ordinary links never consume this manifest.
type LTOManifest struct {
	Version   int            `json:"version"`
	Main      int            `json:"main"`
	Packages  []LTOPackage   `json:"packages"`
	Symbols   []LTOSymbol    `json:"symbols"`
	Linknames []LTOReference `json:"linknames"`
	// Archive is filled by the LTO driver, after writing the temporary main.a.
	Archive string `json:"-"`
}

type LTOPackage struct {
	// Path is supplied by the driver's importcfg; Prefix is the escaped object
	// spelling, emitted by the compiler. They must agree.
	Path        string      `json:"-"`
	Prefix      string      `json:"prefix"`
	Name        string      `json:"name"`
	Std         bool        `json:"std"`
	Fingerprint string      `json:"fingerprint"`
	Imports     []LTOImport `json:"imports"`
	Cgo         []string    `json:"cgo"`
}

type LTOImport struct {
	Path        string `json:"path"`
	Prefix      string `json:"prefix"`
	Fingerprint string `json:"fingerprint"`
}

type LTOSymbol struct {
	Package       int     `json:"package"`
	Name          string  `json:"name"`
	OriginalName  string  `json:"original_name,omitempty"`
	ABI           uint16  `json:"abi"`
	OriginalIndex *uint32 `json:"original_index,omitempty"`
	Index         *uint32 `json:"index,omitempty"`
}

type LTOReference struct {
	Package int    `json:"package"`
	Name    string `json:"name"`
	ABI     uint16 `json:"abi"`
}
