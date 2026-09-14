// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

func (lfc *LLVMFuncContext) simdSHA(v *Value, lowering goALLCSIMDLowering) llvm.Value {
	var op string
	switch lowering {
	case goALLCSIMDLowerSHA1Rounds:
		op = "sha1rnds4"
	case goALLCSIMDLowerSHA1NextE:
		op = "sha1nexte"
	case goALLCSIMDLowerSHA1Message1:
		op = "sha1msg1"
	case goALLCSIMDLowerSHA1Message2:
		op = "sha1msg2"
	case goALLCSIMDLowerSHA256Rounds:
		op = "sha256rnds2"
	case goALLCSIMDLowerSHA256Message1:
		op = "sha256msg1"
	case goALLCSIMDLowerSHA256Message2:
		op = "sha256msg2"
	}
	vec := llvm.VectorType(GlobalCtxt.Int32Type(), 4)
	args := make([]llvm.Value, 0, len(v.Args)+1)
	for _, arg := range v.Args {
		args = append(args, lfc.simdValueAs(v, arg, vec, ".sha"))
	}
	if lowering == goALLCSIMDLowerSHA1Rounds {
		args = append(args, llvm.ConstInt(GlobalCtxt.Int8Type(), uint64(uint8(v.AuxInt)), false))
	}
	fn := getLLVMIntrinsicDeclaration("llvm.x86." + op)
	return lfc.simdLaneResult(v, lfc.b.CreateCall(fn.GlobalValueType(), fn, args, v.String()+".sha"))
}
