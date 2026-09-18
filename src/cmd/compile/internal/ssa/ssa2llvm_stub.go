// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

func LLVMCompile(f *Func) {}

func InitModule(pkg *types.Pkg) {
}

func LowerGoObjData() {
}

func SetLLVMWeakCallTargets(source *obj.LSym, targets []*obj.LSym) {
}

func MarkGoObjDataReferencedOutsideLLVM(syms ...*obj.LSym) {
}

func FinalizeGoObjSymbolMetadata() {
}

func Output(fileName string) error {
	return nil
}

func EmitLLVMGoObj(outputFile string) ([]byte, error) {
	return nil, nil
}
