// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

func llvmMakeChannel(n int) chan int                       { return make(chan int, n) }
func llvmMakeMap(n int) map[string]int                     { return make(map[string]int, n) }
func llvmReadMap(m map[string]int, key string) int         { return m[key] }
func llvmWriteMap(m map[string]int, key string, value int) { m[key] = value }
func llvmReceive(c chan int) bool                          { _, ok := <-c; return ok }
func llvmTrySend(c chan int, v int) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}
func llvmInterfaceEqual(x, y any) bool { return x == y }

// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.makechan<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.makemap<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.mapaccess1_faststr<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal nonnull ptr @"runtime.mapassign_faststr<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal i1 @"runtime.chanrecv2<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal i1 @"runtime.selectnbsend<builtin.{{[0-9]+}}>"
// LLVM-DAG: declare goabiinternal i1 @"runtime.efaceeq<builtin.{{[0-9]+}}>"
