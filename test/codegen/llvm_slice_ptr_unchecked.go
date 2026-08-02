// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

// Converting a slice to a pointer to a zero-length array does not require a
// bounds check. Its data pointer can therefore be nil.
//
// LLVM-LABEL: define goabiinternal ptr @codegen.llvmZeroArrayPtrUnchecked(
// LLVM-NOT: nonnull
// LLVM: extractvalue { ptr, i64, i64 } {{%.*}}, 0
// LLVM: ret ptr
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmZeroArrayPtrUnchecked(
// LLVM-OPT-NOT: nonnull
// LLVM-OPT: extractvalue { ptr, i64, i64 } {{%.*}}, 0
// LLVM-OPT: ret ptr
//
// LLVM-LABEL: define goabiinternal ptr @codegen.llvmSliceDataUnchecked(
// LLVM-NOT: nonnull
// LLVM: extractvalue { ptr, i64, i64 } {{%.*}}, 0
// LLVM: ret ptr
// LLVM-OPT-LABEL: define goabiinternal ptr @codegen.llvmSliceDataUnchecked(
// LLVM-OPT-NOT: nonnull
// LLVM-OPT: extractvalue { ptr, i64, i64 } {{%.*}}, 0
// LLVM-OPT: ret ptr
func llvmSliceDataUnchecked(s []byte) *byte {
	return unsafe.SliceData(s)
}

func llvmZeroArrayPtrUnchecked(s []byte) *[0]byte {
	return (*[0]byte)(s)
}
