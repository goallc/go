//go:build !staticllvm

package llvm

/*
#include "llvm-c/Core.h"
#include "llvm-c/Error.h"
#include "llvm-c/TargetMachine.h"
#include "SupportBindings.h"
#include <stdlib.h>
*/
import "C"

import "unsafe"

// UsesLinkedPassPlugin reports whether the GoALLC pass implementation is
// linked into the current process instead of loaded from a shared library.
func UsesLinkedPassPlugin() bool { return false }

// RunPassPluginEarlyIR loads the GoALLC pass plugin and runs its early IR
// pipeline before standard LLVM optimization.
func (m Module) RunPassPluginEarlyIR(plugin string) error {
	cplugin := C.CString(plugin)
	defer C.free(unsafe.Pointer(cplugin))
	return passPluginError(C.LLVMRunPassPluginEarlyIR(m.C, cplugin))
}

// RunPassPluginPreCodeGen loads a pass plugin into the current process and
// invokes its pre-codegen callback.
func (tm TargetMachine) RunPassPluginPreCodeGen(m Module, ft CodeGenFileType, plugin string) error {
	cplugin := C.CString(plugin)
	defer C.free(unsafe.Pointer(cplugin))
	return passPluginError(C.LLVMRunPassPluginPreCodeGen(m.C, tm.C, C.LLVMCodeGenFileType(ft), cplugin))
}

func runPassPluginThinLTO(inputs *C.LLVMMemoryBufferRef, count C.size_t, diagnostics *C.char, pic C.LLVMBool, jobs C.unsigned, outputs *C.LLVMMemoryBufferRef, plugin string) error {
	cplugin := C.CString(plugin)
	defer C.free(unsafe.Pointer(cplugin))
	return passPluginError(C.LLVMRunPassPluginThinLTO(inputs, count, diagnostics, pic, jobs, outputs, cplugin))
}

func writePassPluginThinBitcode(module C.LLVMModuleRef, output *C.LLVMMemoryBufferRef, plugin string) error {
	cplugin := C.CString(plugin)
	defer C.free(unsafe.Pointer(cplugin))
	return passPluginError(C.LLVMWritePassPluginThinBitcode(module, output, cplugin))
}
