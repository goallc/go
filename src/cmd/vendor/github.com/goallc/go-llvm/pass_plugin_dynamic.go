//go:build dynamicllvm

package llvm

/*
#include "llvm-c/Core.h"
#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"
#include "SupportBindings.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

// UsesLinkedPassPlugin reports whether the GoALLC pass implementation is
// linked into the current process instead of loaded from a shared library.
func UsesLinkedPassPlugin() bool { return false }

// RunPassPluginPreCodeGen loads a pass plugin into the current process and
// invokes its pre-codegen callback.
func (tm TargetMachine) RunPassPluginPreCodeGen(m Module, ft CodeGenFileType, plugin string) error {
	cplugin := C.CString(plugin)
	defer C.free(unsafe.Pointer(cplugin))
	err := C.LLVMRunPassPluginPreCodeGen(m.C, tm.C, C.LLVMCodeGenFileType(ft), cplugin)
	if err == nil {
		return nil
	}
	cstr := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(cstr)
	return errors.New(C.GoString(cstr))
}
