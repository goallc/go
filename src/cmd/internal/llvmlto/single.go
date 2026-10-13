// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package llvmlto

import (
	"bytes"
	"cmd/internal/goobj"
	"encoding/json"
	"fmt"
	"os"
)

// LLVM needs distinct storage names for content-addressed function copies in
// one module. The final GoObj has distinct symbol indices and can restore their
// original names, including for runtime.FuncForPC, pclntab, and profiling.
func finalizeSingleObject(path, manifestPath string, manifest *goobj.LTOManifest) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	start := bytes.Index(data, []byte(goobj.Magic))
	if start < 0 {
		return fmt.Errorf("FullLTO output is not a Go object")
	}
	r := goobj.NewReaderFromBytes(data[start:], false)
	if r == nil {
		return fmt.Errorf("invalid FullLTO Go object")
	}
	for i := range manifest.Symbols {
		s := &manifest.Symbols[i]
		if s.OriginalName == "" {
			continue
		}
		if s.Index == nil || *s.Index >= uint32(r.NSym()) {
			return fmt.Errorf("FullLTO renamed symbol %s has no output index", s.Name)
		}
		objectSym := r.Sym(*s.Index)
		if objectSym.Name(r) != s.Name || objectSym.ABI() != s.ABI {
			return fmt.Errorf("FullLTO renamed symbol %s does not match its output index", s.Name)
		}
		if err := objectSym.SetNamePrefix(s.OriginalName, r); err != nil {
			return err
		}
		s.Name = s.OriginalName
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, encoded, 0600)
}
