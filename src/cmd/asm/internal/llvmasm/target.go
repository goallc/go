// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmasm

import (
	"fmt"
	"strings"
)

func (o Options) isGoObj() bool { return o.ObjectFormat == "" || o.ObjectFormat == "goobj" }

func (o Options) triple() (string, error) {
	arch := ""
	switch o.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		return "", fmt.Errorf("LLVM assembly is not supported for %s", o.GOARCH)
	}
	if o.GOOS != "linux" && o.GOOS != "darwin" {
		return "", fmt.Errorf("LLVM assembly is not supported for %s", o.GOOS)
	}
	switch o.ObjectFormat {
	case "", "goobj":
		return arch + "-unknown-" + o.GOOS + "-goobj", nil
	case "elf":
		if o.GOOS == "linux" {
			return arch + "-unknown-linux-gnu", nil
		}
	case "macho":
		if o.GOOS == "darwin" {
			return arch + "-apple-macosx11.0.0", nil
		}
	default:
		return "", fmt.Errorf("unknown LLVM object format %q (want goobj, elf, or macho)", o.ObjectFormat)
	}
	return "", fmt.Errorf("LLVM object format %s does not match GOOS=%s", o.ObjectFormat, o.GOOS)
}

func (o Options) dataSection(name string) string {
	if o.isGoObj() {
		return name
	}
	// LLVM selects ordinary native sections, including BSS, TLS and constants
	// that need dynamic relocations. Go's finer data kinds live in metadata.
	// Only the explicitly segregated FIPS data keeps a custom native section.
	if !strings.HasSuffix(name, ".fips") {
		return ""
	}
	if o.ObjectFormat == "elf" {
		return name
	}
	switch name {
	case ".rodata.fips":
		return "__TEXT,__rodata_fips"
	case ".noptrdata.fips":
		return "__DATA,__noptr_fips"
	case ".data.fips":
		return "__DATA,__data_fips"
	default:
		return ""
	}
}
