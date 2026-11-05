//===- mc.go - Go assembler bridge to LLVM MC -----------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//

package llvm

/*
#cgo CXXFLAGS: -fno-rtti
#include "MCBindings.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// MCDecoder decodes bytes into MCInsts and prints them using LLVM's assembler
// syntax. Symbolic operands come from relocation records, never from text edits
// to decoded numeric immediates.
type MCDecoder struct{ ref unsafe.Pointer }
type MCReloc struct {
	Offset     uint64
	Size       uint32
	PCRelative bool
	Addend     int64
	Operand    uint32 // LLVM inline-asm operand index
	Prefix     string // target relocation specifier, e.g. :lo12:
	Suffix     string // target relocation specifier, e.g. @PAGEOFF
}

func NewMCDecoder(triple, features string) (MCDecoder, error) {
	t := C.CString(triple)
	defer C.free(unsafe.Pointer(t))
	f := C.CString(features)
	defer C.free(unsafe.Pointer(f))
	d := MCDecoder{C.LLVMGoCreateMCDecoder(t, f)}
	if d.ref == nil {
		return d, fmt.Errorf("cannot create LLVM MC decoder for %s", triple)
	}
	return d, nil
}
func (d MCDecoder) Dispose() { C.LLVMGoDisposeMCDecoder(d.ref) }
func (d MCDecoder) Decode(code []byte, pc uint64, relocs []MCReloc) (text string, size uint64, err error) {
	if len(code) == 0 {
		return "", 0, fmt.Errorf("empty instruction")
	}
	var ptr *C.LLVMGoMCReloc
	if len(relocs) != 0 {
		ptr = (*C.LLVMGoMCReloc)(C.calloc(C.size_t(len(relocs)), C.size_t(C.sizeof_LLVMGoMCReloc)))
		defer C.free(unsafe.Pointer(ptr))
		rs := unsafe.Slice(ptr, len(relocs))
		for i, r := range relocs {
			rs[i].Offset = C.uint64_t(r.Offset)
			rs[i].Size = C.uint(r.Size)
			rs[i].Addend = C.int64_t(r.Addend)
			if r.PCRelative {
				rs[i].PCRelative = 1
			}
			rs[i].Operand = C.uint(r.Operand)
			rs[i].Prefix = C.CString(r.Prefix)
			rs[i].Suffix = C.CString(r.Suffix)
			defer C.free(unsafe.Pointer(rs[i].Prefix))
			defer C.free(unsafe.Pointer(rs[i].Suffix))
		}
	}
	var n C.uint64_t
	var msg *C.char
	out := C.LLVMGoDecodeMCInstruction(d.ref, (*C.uint8_t)(unsafe.Pointer(&code[0])), C.size_t(len(code)), C.uint64_t(pc), ptr, C.size_t(len(relocs)), &n, &msg)
	if out == nil {
		defer C.free(unsafe.Pointer(msg))
		return "", 0, fmt.Errorf("decode at PC %d: %s", pc, C.GoString(msg))
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out), uint64(n), nil
}

func (d MCDecoder) InstructionSize(code []byte, pc uint64) uint64 {
	if len(code) == 0 {
		return 0
	}
	return uint64(C.LLVMGoMCInstructionSize(d.ref, (*C.uint8_t)(unsafe.Pointer(&code[0])), C.size_t(len(code)), C.uint64_t(pc)))
}
