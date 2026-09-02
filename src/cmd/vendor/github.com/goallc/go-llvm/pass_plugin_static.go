//go:build staticllvm && goallcplugin

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../../../../pkg/goallc-llvmplugin/lib -lGoALLCStatepointsStatic
#cgo linux LDFLAGS: -L${SRCDIR}/../../../../../../pkg/goallc-llvmplugin/lib -Wl,--whole-archive -lGoALLCStatepointsStatic -Wl,--no-whole-archive

#include "llvm-c/Core.h"
#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"

LLVMErrorRef LLVMGoALLCRunPreCodeGen(LLVMModuleRef Module,
                                     LLVMTargetMachineRef TargetMachine,
                                     LLVMCodeGenFileType FileType);
LLVMErrorRef LLVMGoALLCRunEarlyIR(LLVMModuleRef Module);
*/
import "C"

import (
	"errors"
)

// UsesLinkedPassPlugin reports whether the GoALLC pass implementation is
// linked into the current process instead of loaded from a shared library.
func UsesLinkedPassPlugin() bool { return true }

// RunPassPluginEarlyIR invokes the statically linked GoALLC early IR pipeline.
func (m Module) RunPassPluginEarlyIR(plugin string) error {
	if plugin != "" {
		return errors.New("linked GoALLC early IR callback does not accept a plugin path")
	}
	err := C.LLVMGoALLCRunEarlyIR(m.C)
	if err == nil {
		return nil
	}
	cstr := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(cstr)
	return errors.New(C.GoString(cstr))
}

// RunPassPluginPreCodeGen invokes the statically linked GoALLC pre-codegen
// callback. A static toolchain cannot replace this callback with a DSO without
// loading a second LLVM runtime into the compiler process.
func (tm TargetMachine) RunPassPluginPreCodeGen(m Module, ft CodeGenFileType, plugin string) error {
	if plugin != "" {
		return errors.New("linked GoALLC pre-codegen callback does not accept a plugin path")
	}
	err := C.LLVMGoALLCRunPreCodeGen(m.C, tm.C, C.LLVMCodeGenFileType(ft))
	if err == nil {
		return nil
	}
	cstr := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(cstr)
	return errors.New(C.GoString(cstr))
}
