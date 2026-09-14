package llvm

/*
#include "llvm-c/Core.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

func LLVMPrintModuleToFile(module Module, filename string) error {
	filenamestr := C.CString(filename)
	defer C.free(unsafe.Pointer(filenamestr))
	var errmsg *C.char
	if C.LLVMPrintModuleToFile(module.C, filenamestr, &errmsg) != 0 {
		err := errors.New(C.GoString(errmsg))
		C.LLVMDisposeMessage(errmsg)
		return err
	}
	return nil
}

func LLVMSetValueName2(value Value, name string) {
	namestr := C.CString(name)
	defer C.free(unsafe.Pointer(namestr))
	C.LLVMSetValueName2(value.C, namestr, C.size_t(len(name)))
}

// SetDiscardValueNames controls names of local IR values. Global symbols and
// debug-info metadata retain their names.
func (c Context) SetDiscardValueNames(discard bool) {
	C.LLVMContextSetDiscardValueNames(c.C, boolToLLVMBool(discard))
}

func (c Context) PointerType(addressSpace uint32) (t Type) {
	t.C = C.LLVMPointerTypeInContext(c.C, C.unsigned(addressSpace))
	return
}

func (c Context) MetadataAsValue(md Metadata) (v Value) {
	v.C = C.LLVMMetadataAsValue(c.C, md.C)
	return
}

func (b Builder) CreateFence(ordering AtomicOrdering, singleThread bool, name string) (v Value) {
	namestr := C.CString(name)
	defer C.free(unsafe.Pointer(namestr))
	v.C = C.LLVMBuildFence(b.C, C.LLVMAtomicOrdering(ordering), boolToLLVMBool(singleThread), namestr)
	return
}
