// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"fmt"

	"github.com/goallc/go-llvm"
)

func (lfc *LLVMFuncContext) simdAES(v *Value, lowering goALLCSIMDLowering, width int) llvm.Value {
	var op string
	switch lowering {
	case goALLCSIMDLowerAESEncrypt:
		op = "aesenc"
	case goALLCSIMDLowerAESEncryptLast:
		op = "aesenclast"
	case goALLCSIMDLowerAESDecrypt:
		op = "aesdec"
	case goALLCSIMDLowerAESDecryptLast:
		op = "aesdeclast"
	case goALLCSIMDLowerAESKeygen:
		op = "aeskeygenassist"
	case goALLCSIMDLowerAESInverseMix:
		op = "aesimc"
	}
	name := "llvm.x86.aesni." + op
	if width > 128 {
		name += fmt.Sprintf(".%d", width)
	}
	// LLVM uses i64 lanes for both the byte state and uint32 round keys.
	vec := llvm.VectorType(GlobalCtxt.Int64Type(), width/64)
	args := make([]llvm.Value, 0, len(v.Args)+1)
	for _, arg := range v.Args {
		args = append(args, lfc.simdValueAs(v, arg, vec, ".aes"))
	}
	if lowering == goALLCSIMDLowerAESKeygen {
		args = append(args, llvm.ConstInt(GlobalCtxt.Int8Type(), uint64(uint8(v.AuxInt)), false))
	}
	fn := getLLVMIntrinsicDeclaration(name)
	return lfc.simdLaneResult(v, lfc.b.CreateCall(fn.GlobalValueType(), fn, args, v.String()+".aes"))
}
