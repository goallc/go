// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goobj

import (
	"internal/buildcfg"
	"strconv"
)

const (
	BuiltinSymbolSuffixPrefix = "<builtin."
	LinknameSymbolSuffix      = "<linkname>"
)

// Builtin (compiler-generated) function references appear
// frequently. We assign special indices for them, so they
// don't need to be referenced by name.

// NBuiltin returns the number of listed builtin
// symbols.
func NBuiltin() int {
	return len(builtins)
}

// BuiltinName returns the name and ABI of the i-th
// builtin symbol.
func BuiltinName(i int) (string, int) {
	return builtins[i].name, builtins[i].abi
}

// BuiltinIdx returns the index of the builtin with the
// given name and abi, or -1 if it is not a builtin.
func BuiltinIdx(name string, abi int) int {
	i, ok := builtinMap[name]
	if !ok {
		return -1
	}
	if buildcfg.Experiment.RegabiWrappers && builtins[i].abi != abi {
		return -1
	}
	return i
}

// BuiltinSymbolName returns the LLVM declaration name for a predefined Go
// symbol. The encoded index lets the GoObj writer recover PkgIdxBuiltin from
// the final relocation without duplicating this table in LLVM.
func BuiltinSymbolName(name string, abi int) (string, bool) {
	i := BuiltinIdx(name, abi)
	if i < 0 {
		return name, false
	}
	return name + BuiltinSymbolSuffixPrefix + strconv.Itoa(i) + ">", true
}

// BuiltinIsLate reports whether LLVM's machine passes may introduce a
// reference after ordinary IR call lowering. The classification is generated
// beside builtins so the serialized index and the declaration stay in sync.
func BuiltinIsLate(i int) bool {
	return i >= 0 && i < len(lateBuiltins) && lateBuiltins[i]
}

//go:generate go run mkbuiltin.go

var builtinMap map[string]int

func init() {
	builtinMap = make(map[string]int, len(builtins))
	for i, b := range builtins {
		builtinMap[b.name] = i
	}
}
