// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"fmt"

	"github.com/goallc/go-llvm"
)

// Only relocation families are translated. MC owns instruction syntax and
// converts symbolic operands into the selected object's native relocations.
func (e *emitter) instructionRelocs() (map[int64]llvm.MCReloc, error) {
	rs := map[int64]llvm.MCReloc{}
	add := func(off int64, size uint32, sym *obj.LSym, text bool, addend int64, prefix, suffix string, pcrel bool) error {
		if _, ok := rs[off]; ok {
			return fmt.Errorf("overlapping relocation at PC %d", off)
		}
		if sym == nil {
			return fmt.Errorf("missing relocation symbol at PC %d", off)
		}
		if off < 0 || off+int64(size) > e.fn.Size {
			return fmt.Errorf("relocation outside function at PC %d", off)
		}
		rs[off] = llvm.MCReloc{Offset: uint64(off), Size: size, Operand: e.operand(sym, text), Addend: addend, Prefix: prefix, Suffix: suffix, PCRelative: pcrel}
		return nil
	}
	macho := e.options.ObjectFormat == "macho"
	for _, r := range e.fn.R {
		off := int64(r.Off)
		sym := r.Sym
		var err error
		switch r.Type {
		case objabi.R_CALLIND:
			continue // Semantic relocation, preserved in Go metadata.
		case objabi.R_ADDR:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", "", false)
		case objabi.R_CALL:
			err = add(off, uint32(r.Siz), sym, true, r.Add, "", "", true)
		case objabi.R_PCREL:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", "", true)
		case objabi.R_GOTPCREL:
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", "@GOTPCREL", false)
		case objabi.R_TLS_LE:
			if macho {
				continue // Resolved as a fixed Go platform offset below.
			}
			if sym == nil {
				sym = e.ctxt.Lookup("runtime.tlsg")
			}
			err = add(off, uint32(r.Siz), sym, false, r.Add, "", "@TPOFF", false)
		case objabi.R_TLS_IE:
			if macho {
				return nil, fmt.Errorf("Go initial-exec TLS relocation is not supported by the Mach-O carrier")
			}
			if sym == nil {
				sym = e.ctxt.Lookup("runtime.tlsg")
			}
			// Unlike R_PCREL, Go's TLS_IE addend already includes the native
			// field-relative bias. Remove it before printing the memory operand.
			err = add(off, uint32(r.Siz), sym, false, r.Add+int64(r.Siz), "", "@GOTTPOFF", true)
		case objabi.R_CALLARM64:
			err = add(off, 4, sym, true, r.Add, "", "", false)
		case objabi.R_ADDRARM64, objabi.R_ARM64_PCREL, objabi.R_ARM64_PCREL_LDST8, objabi.R_ARM64_PCREL_LDST16, objabi.R_ARM64_PCREL_LDST32, objabi.R_ARM64_PCREL_LDST64:
			hi, loPrefix, loSuffix := "", ":lo12:", ""
			if macho {
				hi, loPrefix, loSuffix = "@PAGE", "", "@PAGEOFF"
			}
			err = add(off, 4, sym, false, r.Add, "", hi, false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, loPrefix, loSuffix, false)
			}
		case objabi.R_ARM64_TLS_LE:
			if macho {
				return nil, fmt.Errorf("Go local-exec TLS relocation is not supported by the Mach-O carrier")
			}
			err = add(off, 4, sym, false, r.Add, ":tprel_g0:", "", false)
		case objabi.R_ARM64_TLS_IE:
			if macho {
				return nil, fmt.Errorf("Go initial-exec TLS relocation is not supported by the Mach-O carrier")
			}
			err = add(off, 4, sym, false, r.Add, ":gottprel:", "", false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, ":gottprel_lo12:", "", false)
			}
		case objabi.R_ARM64_GOTPCREL, objabi.R_ARM64_GOT:
			hiPrefix, hiSuffix, loPrefix, loSuffix := ":got:", "", ":got_lo12:", ""
			if macho {
				hiPrefix, hiSuffix, loPrefix, loSuffix = "", "@GOTPAGE", "", "@GOTPAGEOFF"
			}
			err = add(off, 4, sym, false, r.Add, hiPrefix, hiSuffix, false)
			if err == nil {
				err = add(off+4, 4, sym, false, r.Add, loPrefix, loSuffix, false)
			}
		default:
			return nil, fmt.Errorf("unsupported encoded relocation %s at PC %d", r.Type, r.Off)
		}
		if err != nil {
			return nil, err
		}
	}
	return rs, nil
}

// Darwin's target-less x86 R_TLS_LE is fixed by the Go platform ABI. The Go
// linker resolves it even in external link mode; it has no Mach-O equivalent.
// Emit only the displacement field, retaining every other instruction byte.
func (e *emitter) fixedNativeRelocs() ([]replacement, error) {
	if e.options.ObjectFormat != "macho" {
		return nil, nil
	}
	var result []replacement
	for _, r := range e.fn.R {
		if r.Type != objabi.R_TLS_LE {
			continue
		}
		if e.options.GOARCH != "amd64" || r.Sym != nil || r.Siz != 4 || r.Off < 0 || int64(r.Off)+4 > e.fn.Size {
			return nil, fmt.Errorf("unsupported Darwin Go TLS relocation at PC %d", r.Off)
		}
		const minDisplacement, maxDisplacement = -1 << 31, 1<<31 - 1
		if r.Add < minDisplacement-objabi.DarwinAMD64TLSOffset || r.Add > maxDisplacement-objabi.DarwinAMD64TLSOffset {
			return nil, fmt.Errorf("Darwin Go TLS displacement overflows int32")
		}
		value := r.Add + objabi.DarwinAMD64TLSOffset
		result = append(result, replacement{int64(r.Off), int64(r.Off) + 4, fmt.Sprintf(".long %d", value)})
	}
	return result, nil
}
