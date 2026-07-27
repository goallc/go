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

func (c Context) PointerType(addressSpace uint32) (t Type) {
	t.C = C.LLVMPointerTypeInContext(c.C, C.unsigned(addressSpace))
	return
}
