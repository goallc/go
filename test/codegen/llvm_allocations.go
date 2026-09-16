// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import "unsafe"

//go:linkname llvmAllocate runtime.mallocgc
func llvmAllocate(size uintptr, typ unsafe.Pointer, zero bool) unsafe.Pointer

func llvmAllocZeroed() unsafe.Pointer                  { return llvmAllocate(8, nil, true) }
func llvmAllocUnknownContents() unsafe.Pointer         { return llvmAllocate(8, nil, false) }
func llvmAllocDynamicZero(zero bool) unsafe.Pointer    { return llvmAllocate(8, nil, zero) }
func llvmAllocZeroSize() unsafe.Pointer                { return llvmAllocate(0, nil, true) }
func llvmAllocDynamicSize(size uintptr) unsafe.Pointer { return llvmAllocate(size, nil, true) }

func llvmAllocUnused()          { _ = llvmAllocate(8, nil, true) }
func llvmAllocReadZero() uint64 { return *(*uint64)(llvmAllocate(8, nil, true)) }

// LLVM-DAG: call goabiinternal noalias ptr @"runtime.mallocgc<linkname>"(i64 8, ptr null, i1 true) #[[ZERO:[0-9]+]]
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.mallocgc<linkname>"(i64 8, ptr null, i1 false) #[[ALLOC:[0-9]+]]
// LLVM-DAG: call goabiinternal noalias ptr @"runtime.mallocgc<linkname>"(i64 8, ptr null, i1 %{{[^)]+}}) #[[ALLOC]]
// LLVM-DAG: call goabiinternal ptr @"runtime.mallocgc<linkname>"(i64 0, ptr null, i1 true), !dbg
// LLVM-DAG: call goabiinternal ptr @"runtime.mallocgc<linkname>"(i64 %size, ptr null, i1 true), !dbg
// LLVM-DAG: call goabiinternal noalias ptr @runtime.mallocgcTinySC2(i64 1, ptr {{.*}}, i1 true) #[[ZERO]]
// LLVM-DAG: call goabiinternal noalias ptr @runtime.mallocgcSmallNoScanSC7(i64 80, ptr {{.*}}, i1 true) #[[ZERO]]
// LLVM-DAG: call goabiinternal noalias ptr @runtime.mallocgcSmallScanNoHeaderSC7(i64 80, ptr {{.*}}, i1 true) #[[ZERO]]
// LLVM-DAG: call goabiinternal noalias dereferenceable(128) ptr @"runtime.newobject<linkname>"(ptr @"type:[128]uint8") #[[ZERO]]
// LLVM-DAG: call goabiinternal ptr @"runtime.newobject<linkname>"(ptr %typ), !dbg
// LLVM-DAG: declare goabiinternal nonnull ptr @runtime.mallocgcTinySC2(i64, ptr, i1) #[[SIZE:[0-9]+]]
// LLVM-DAG: declare goabiinternal nonnull ptr @runtime.mallocgcSmallNoScanSC7(i64, ptr, i1) #[[SIZE]]
// LLVM-DAG: declare goabiinternal nonnull ptr @runtime.mallocgcSmallScanNoHeaderSC7(i64, ptr, i1) #[[SIZE]]
// LLVM-DAG: attributes #[[SIZE]] = { allocsize(0) }
// LLVM-DAG: attributes #[[ZERO]] = { allockind("alloc,zeroed") "alloc-family"="runtime.mallocgc" }
// LLVM-DAG: attributes #[[ALLOC]] = { allockind("alloc") "alloc-family"="runtime.mallocgc" }

// LLVM-OPT-LABEL: define goabiinternal {{.*}}i64 @codegen.llvmAllocReadZero()
// LLVM-OPT-NEXT: b1:
// LLVM-OPT-NEXT: ret i64 0,
// LLVM-OPT-NEXT: }

// These source-level new expressions cover the specialized frontend entries
// and the non-specialized newobject fallback without changing runtime APIs.
func llvmNewTiny() *byte       { return new(byte) }
func llvmNewNoScan() *[80]byte { return new([80]byte) }
func llvmNewScan() *[10]*int   { return new([10]*int) }
func llvmNewLarge() *[128]byte { return new([128]byte) }

//go:linkname llvmNewObject runtime.newobject
func llvmNewObject(typ unsafe.Pointer) unsafe.Pointer
func llvmNewDynamic(typ unsafe.Pointer) unsafe.Pointer { return llvmNewObject(typ) }
