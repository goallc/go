//go:build llvm23

package llvm

/*
#include "llvm-c/Core.h"
*/
import "C"

const (
	UncondBr Opcode = C.LLVMUncondBr
	CondBr   Opcode = C.LLVMCondBr
)
