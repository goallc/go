// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

type llvmChainFinal func(int) int

type llvmChainInterceptor func(int, llvmChainFinal) int

// Recursively constructing the next closure makes the returned closure and
// the closure-conversion helper materialize each other's function pointer.
// Both are content-addressable, so the LLVM GoObj writer must hash the cycle
// as a component.
func llvmChain(interceptors []llvmChainInterceptor, current int, final llvmChainFinal) llvmChainFinal {
	if current == len(interceptors)-1 {
		return final
	}
	return func(value int) int {
		return interceptors[current+1](value, llvmChain(interceptors, current+1, final))
	}
}

// The amd64 address materialization is absolute on ELF and PC-relative on PE.
// Both edges must still resolve to the hashed symbols in the cycle.
// LLVM-OBJSUMMARY-DAG: LLVM symbol name="codegen.llvmChain.func1" kind=STEXT flags={{.*}} class=hashed hash={{[0-9a-f]+}}
// LLVM-OBJSUMMARY-DAG: LLVM symbol name="codegen.llvmChain.1" kind=STEXT flags={{.*}} class=hashed hash={{[0-9a-f]+}}
// LLVM-OBJSUMMARY-AMD64-DAG: LLVM relocation owner="codegen.llvmChain.func1" type=R_{{(ADDR|PCREL)}} size=4 target_kind=hashed target_package="" target_name="codegen.llvmChain.1" target_index={{[0-9]+}}
// LLVM-OBJSUMMARY-AMD64-DAG: LLVM relocation owner="codegen.llvmChain.1" type=R_{{(ADDR|PCREL)}} size=4 target_kind=hashed target_package="" target_name="codegen.llvmChain.func1" target_index={{[0-9]+}}
// LLVM-OBJSUMMARY-ARM64-DAG: LLVM relocation owner="codegen.llvmChain.func1" type=R_ADDRARM64 size=8 target_kind=hashed target_package="" target_name="codegen.llvmChain.1" target_index={{[0-9]+}}
// LLVM-OBJSUMMARY-ARM64-DAG: LLVM relocation owner="codegen.llvmChain.1" type=R_ADDRARM64 size=8 target_kind=hashed target_package="" target_name="codegen.llvmChain.func1" target_index={{[0-9]+}}
