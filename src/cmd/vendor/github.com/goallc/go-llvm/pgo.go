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

// InstrProfLowering is a native LLVMErrorRef (*)(LLVMModuleRef) callback.
// The callback lowers instrumentation intrinsics for the caller's runtime.
type InstrProfLowering unsafe.Pointer

// RunPassesWithPGO runs a single PassBuilder pipeline with IR instrumentation.
// An empty profile selects generation. A non-empty profile names an indexed
// profile to use. A nil lowering uses LLVM's standard compiler-rt lowering.
// An explicit pass list without a profiling site profiles the input module.
func (mod Module) RunPassesWithPGO(passes string, tm TargetMachine, profile string, lower InstrProfLowering) error {
	if profile != "" && lower != nil {
		return errors.New("custom counter lowering requires profile generation")
	}
	pipeline := C.CString(passes)
	defer C.free(unsafe.Pointer(pipeline))
	path := C.CString(profile)
	defer C.free(unsafe.Pointer(path))
	err := C.LLVMGoRunPassesWithPGO(mod.C, tm.C, pipeline, path, C.LLVMGoInstrProfLowering(lower))
	if err == nil {
		return nil
	}
	msg := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(msg)
	return errors.New(C.GoString(msg))
}
