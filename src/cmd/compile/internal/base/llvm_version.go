// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package base

import (
	"cmd/internal/llvmbackend"
	"cmd/internal/objabi"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func init() {
	objabi.SetVersionFlagFullHook(compileVersionFlagFullSuffix)
}

func compileVersionFlagFullSuffix(buildID string) string {
	if !llvmVersionEnabled(os.Args[1:]) || os.Getenv("GOALLC_EXTERNAL_BACKEND") == "1" {
		return " buildID=" + buildID
	}
	identity, err := llvmbackend.Identity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "compile: resolving in-process LLVM backend identity: %v\n", err)
		os.Exit(2)
	}
	h := sha256.New()
	h.Write([]byte(buildID))
	h.Write([]byte{0})
	h.Write([]byte(identity))
	return " buildID=goallc-" + hex.EncodeToString(h.Sum(nil))
}

func llvmVersionEnabled(args []string) (enabled bool) {
	for _, arg := range args {
		switch {
		case arg == "-enablellvm":
			enabled = true
		case strings.HasPrefix(arg, "-enablellvm="):
			if value, err := strconv.ParseBool(strings.TrimPrefix(arg, "-enablellvm=")); err == nil {
				enabled = value
			}
		}
	}
	return enabled
}
