//go:build staticllvm && goallcplugin

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../../../../pkg/goallc-llvmplugin/lib -lGoALLCStatepointsStatic
#cgo linux LDFLAGS: -L${SRCDIR}/../../../../../../pkg/goallc-llvmplugin/lib -Wl,--whole-archive -lGoALLCStatepointsStatic -Wl,--no-whole-archive

#cgo windows LDFLAGS: -L${SRCDIR}/../../../../../../pkg/goallc-llvmplugin/lib -lGoALLCStatepointsStatic -lLLVMGoALLC

#include "SupportBindings.h"
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
	return passPluginError(C.LLVMGoALLCRunEarlyIR(m.C))
}

// RunPassPluginPreCodeGen invokes the statically linked GoALLC pre-codegen
// callback. A static toolchain cannot replace this callback with a DSO without
// loading a second LLVM runtime into the compiler process.
func (tm TargetMachine) RunPassPluginPreCodeGen(m Module, ft CodeGenFileType, plugin string) error {
	if plugin != "" {
		return errors.New("linked GoALLC pre-codegen callback does not accept a plugin path")
	}
	return passPluginError(C.LLVMGoALLCRunPreCodeGen(m.C, tm.C, C.LLVMCodeGenFileType(ft)))
}

func runPassPluginThinLTO(inputs *C.LLVMMemoryBufferRef, count C.size_t, diagnostics *C.char, pic C.LLVMBool, jobs C.unsigned, outputs *C.LLVMMemoryBufferRef, plugin string) error {
	if plugin != "" {
		return errors.New("linked GoALLC ThinLTO callback does not accept a plugin path")
	}
	return passPluginError(C.LLVMGoALLCThinLTO(inputs, count, diagnostics, pic, jobs, outputs))
}

func writePassPluginThinBitcode(module C.LLVMModuleRef, output *C.LLVMMemoryBufferRef, plugin string) error {
	if plugin != "" {
		return errors.New("linked GoALLC ThinLTO bitcode callback does not accept a plugin path")
	}
	return passPluginError(C.LLVMGoALLCWriteThinBitcode(module, output))
}
