// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/llvmbackend"
	"cmd/internal/objabi"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

func init() {
	// cmd/go probes asm without package-specific asmflags. Include LLVM in the
	// tool identity even when that particular invocation uses the native encoder.
	objabi.SetVersionFlagFullHook(func(buildID string) string {
		identity, err := llvmbackend.Identity()
		if err != nil {
			fmt.Fprintf(os.Stderr, "asm: resolving LLVM backend identity: %v\n", err)
			os.Exit(2)
		}
		if i := strings.LastIndexByte(buildID, '/'); i >= 0 {
			buildID = buildID[i+1:]
		}
		return fmt.Sprintf(" buildID=goallc-%x", sha256.Sum256([]byte(buildID+"\x00"+identity)))
	})
}
