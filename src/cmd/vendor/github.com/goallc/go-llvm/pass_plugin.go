//go:build !staticllvm || goallcplugin

package llvm

/*
#include "SupportBindings.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

func passPluginError(err C.LLVMErrorRef) error {
	if err == nil {
		return nil
	}
	message := C.LLVMGetErrorMessage(err)
	defer C.LLVMDisposeErrorMessage(message)
	return errors.New(C.GoString(message))
}

// RunPassPluginThinLTO optimizes named bitcode buffers and emits one Go object
// per input through the GoALLC plugin. Inputs are borrowed until it returns;
// the caller owns the returned buffers and must dispose them. On error, all
// partial outputs are disposed here. Empty diagnostics disables saved files.
// plugin must be empty when the plugin is statically linked.
func RunPassPluginThinLTO(inputs []MemoryBuffer, plugin, diagnostics string, pic bool, jobs int) ([]MemoryBuffer, error) {
	if len(inputs) == 0 || jobs <= 0 || int(C.unsigned(jobs)) != jobs {
		return nil, errors.New("ThinLTO requires inputs and a positive unsigned job count")
	}
	refs := make([]C.LLVMMemoryBufferRef, len(inputs))
	for i, input := range inputs {
		if input.IsNil() {
			return nil, errors.New("ThinLTO input buffer is nil")
		}
		refs[i] = input.C
	}
	cdir := C.CString(diagnostics)
	defer C.free(unsafe.Pointer(cdir))
	var reloc C.LLVMBool
	if pic {
		reloc = 1
	}
	outputs := make([]C.LLVMMemoryBufferRef, len(inputs))
	err := runPassPluginThinLTO(&refs[0], C.size_t(len(refs)), cdir, reloc, C.unsigned(jobs), &outputs[0], plugin)
	if err != nil {
		for _, output := range outputs {
			if output != nil {
				C.LLVMDisposeMemoryBuffer(output)
			}
		}
		return nil, err
	}
	objects := make([]MemoryBuffer, len(outputs))
	for i, output := range outputs {
		objects[i] = MemoryBuffer{output}
	}
	return objects, nil
}

// WritePassPluginThinBitcode writes a ThinLTO summary with the Go export
// identities prepared by the GoALLC plugin. The caller owns the returned buffer.
func (m Module) WritePassPluginThinBitcode(plugin string) (MemoryBuffer, error) {
	var output C.LLVMMemoryBufferRef
	if err := writePassPluginThinBitcode(m.C, &output, plugin); err != nil {
		if output != nil {
			C.LLVMDisposeMemoryBuffer(output)
		}
		return MemoryBuffer{}, err
	}
	return MemoryBuffer{output}, nil
}
