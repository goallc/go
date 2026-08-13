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

func MarkGoObjDataReferencedOutsideLLVM(syms ...*obj.LSym) {
}

func FinalizeGoObjSymbolMetadata() {
}

func Output(fileName string) error {
	return nil
}
