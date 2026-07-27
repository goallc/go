//go:build llvm23

package llvm

/*
#include "llvm-c/Core.h"
*/
import "C"

func (v Value) IsABranchInst() (rv Value) {
	rv.C = C.LLVMIsAUncondBrInst(v.C)
	if rv.C == nil {
		rv.C = C.LLVMIsACondBrInst(v.C)
	}
	return
}

func (v Value) IsAUncondBrInst() (rv Value) {
	rv.C = C.LLVMIsAUncondBrInst(v.C)
	return
}

func (v Value) IsACondBrInst() (rv Value) {
	rv.C = C.LLVMIsACondBrInst(v.C)
	return
}
