// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap && cgo

package ld

import (
	"cmd/internal/llvmbackend"
	"cmd/internal/objabi"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func init() {
	objabi.SetVersionFlagFullHook(linkVersionFlagFullSuffix)
}

func linkVersionFlagFullSuffix(buildID string) string {
	if !ltoVersionEnabled(os.Args[1:]) {
		return " buildID=" + buildID
	}
	identity, err := llvmbackend.Identity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "link: resolving in-process LLVM backend identity: %v\n", err)
		os.Exit(2)
	}
	return llvmVersionBuildID(buildID, identity)
}

func llvmVersionBuildID(buildID, identity string) string {
	// Like cmd/go's tool identity, use only the content ID. Action IDs encode
	// the compiler that built this compiler and change during bootstrap even
	// when the resulting tool has identical contents.
	if i := strings.LastIndexByte(buildID, '/'); i >= 0 {
		buildID = buildID[i+1:]
	}
	h := sha256.New()
	h.Write([]byte(buildID))
	h.Write([]byte{0})
	h.Write([]byte(identity))
	return " buildID=goallc-" + hex.EncodeToString(h.Sum(nil))
}

func ltoVersionEnabled(args []string) bool {
	mode := ""
	for i := 0; i < len(args); i++ {
		arg := "-" + strings.TrimLeft(args[i], "-")
		if arg == "-lto" && i+1 < len(args) {
			i++
			mode = args[i]
		} else if strings.HasPrefix(arg, "-lto=") {
			mode = strings.TrimPrefix(arg, "-lto=")
		}
	}
	return mode == "full" || mode == "thin"
}
