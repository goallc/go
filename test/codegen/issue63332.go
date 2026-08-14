// asmcheck

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

// LLVM-LABEL: define goabiinternal void @codegen.issue63332(ptr %c)
// LLVM: alloca i64, align 8
// LLVM: store i64 2, ptr {{%.*}}, align 4
// LLVM: call goabiinternal void @"runtime.chansend1<builtin.{{[0-9]+}}>"(ptr %c, ptr {{%.*}})

func issue63332(c chan int) {
	x := 0
	// amd64:-`MOVQ`
	x += 2
	c <- x
}
