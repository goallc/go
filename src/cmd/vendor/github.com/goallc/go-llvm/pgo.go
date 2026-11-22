package llvm

/*
#include "PGOBindings.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

// RunPassesWithPGO runs a single PassBuilder pipeline with IR instrumentation.
// An empty profile selects generation. A non-empty profile names an indexed
// profile to use. Training uses LLVM's native atomic counter lowering.
// An explicit pass list without a profiling site profiles the input module.
func (mod Module) RunPassesWithPGO(passes string, tm TargetMachine, profile string) error {
	pipeline := C.CString(passes)
	defer C.free(unsafe.Pointer(pipeline))
	path := C.CString(profile)
	defer C.free(unsafe.Pointer(path))
	err := C.LLVMGoRunPassesWithPGO(mod.C, tm.C, pipeline, path)
	if err == nil {
		return nil
	}
	msg := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(msg)
	return errors.New(C.GoString(msg))
}
