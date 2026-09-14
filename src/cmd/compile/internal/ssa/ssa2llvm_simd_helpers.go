// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import "github.com/goallc/go-llvm"

type llvmSIMDHelperKind uint8

const (
	llvmSIMDFromBits llvmSIMDHelperKind = iota
	llvmSIMDToBits
	llvmSIMDIsZero
	llvmSIMDIsNaN
	llvmSIMDMaskedLoad
	llvmSIMDMaskedStore
	llvmSIMDClearUpper
)

type llvmSIMDHelper struct {
	kind    llvmSIMDHelperKind
	bits    int
	profile string
}

// Hand-written generic SSA helpers have no simdgen Operation descriptor.
// Keep their lane semantics and Go-level preconditions together; profiles
// still use the single generated Go CPU feature -> LLVM mapping.
func llvmSIMDHelperInfo(v *Value) (llvmSIMDHelper, bool) {
	h := llvmSIMDHelper{profile: goCPUProfileX86AVX}
	switch v.Op {
	case OpCvt16toMask8x16, OpCvt32toMask8x32, OpCvt64toMask8x64:
		h.kind, h.bits = llvmSIMDFromBits, 8
	case OpCvt8toMask16x8, OpCvt16toMask16x16, OpCvt32toMask16x32:
		h.kind, h.bits = llvmSIMDFromBits, 16
	case OpCvt8toMask32x4, OpCvt8toMask32x8, OpCvt16toMask32x16:
		h.kind, h.bits = llvmSIMDFromBits, 32
	case OpCvt8toMask64x2, OpCvt8toMask64x4, OpCvt8toMask64x8:
		h.kind, h.bits = llvmSIMDFromBits, 64
	case OpCvtMask8x16to16, OpCvtMask8x32to32, OpCvtMask8x64to64:
		h.kind, h.bits = llvmSIMDToBits, 8
	case OpCvtMask16x8to8, OpCvtMask16x16to16, OpCvtMask16x32to32:
		h.kind, h.bits = llvmSIMDToBits, 16
	case OpCvtMask32x4to8, OpCvtMask32x8to8, OpCvtMask32x16to16:
		h.kind, h.bits = llvmSIMDToBits, 32
	case OpCvtMask64x2to8, OpCvtMask64x4to8, OpCvtMask64x8to8:
		h.kind, h.bits = llvmSIMDToBits, 64
	case OpIsZeroVec:
		h.kind, h.bits = llvmSIMDIsZero, 8
	case OpAMD64VZEROUPPER:
		h.kind, h.bits = llvmSIMDClearUpper, 8
	case OpIsNaNFloat32x4, OpIsNaNFloat32x8, OpIsNaNFloat32x16:
		h.kind, h.bits = llvmSIMDIsNaN, 32
	case OpIsNaNFloat64x2, OpIsNaNFloat64x4, OpIsNaNFloat64x8:
		h.kind, h.bits = llvmSIMDIsNaN, 64
	case OpLoadMasked8, OpStoreMasked8:
		h.kind, h.bits = llvmSIMDMaskedLoad, 8
	case OpLoadMasked16, OpStoreMasked16:
		h.kind, h.bits = llvmSIMDMaskedLoad, 16
	case OpLoadMasked32, OpStoreMasked32:
		h.kind, h.bits = llvmSIMDMaskedLoad, 32
	case OpLoadMasked64, OpStoreMasked64:
		h.kind, h.bits = llvmSIMDMaskedLoad, 64
	default:
		return h, false
	}
	if h.kind == llvmSIMDMaskedLoad && v.Type.IsMemory() {
		h.kind = llvmSIMDMaskedStore
	}
	// These are the Go API contracts in archsimd's mask type templates and
	// extra_amd64.go, not requirements inferred from LLVM's selected opcode.
	switch h.kind {
	case llvmSIMDFromBits:
		h.profile = goCPUProfileX86AVX512
	case llvmSIMDToBits:
		width := v.Args[0].Type.Size()
		if width == 64 || h.bits == 16 {
			h.profile = goCPUProfileX86AVX512
		} else if width == 32 && h.bits == 8 {
			h.profile = goCPUProfileX86AVX2
		}
	case llvmSIMDIsNaN:
		if v.Type.Size() == 64 {
			h.profile = goCPUProfileX86AVX512
		}
	case llvmSIMDMaskedLoad, llvmSIMDMaskedStore:
		width := v.Type.Size()
		if h.kind == llvmSIMDMaskedStore {
			width = v.Args[2].Type.Size()
		}
		h.profile = goCPUProfileX86AVX2
		if width == 64 || h.bits < 32 {
			h.profile = goCPUProfileX86AVX512
		}
	}
	return h, true
}

func (lfc *LLVMFuncContext) lowerSIMDHelper(v *Value, h llvmSIMDHelper) llvm.Value {
	laneType := GlobalCtxt.IntType(h.bits)
	switch h.kind {
	case llvmSIMDClearUpper:
		fn := getLLVMIntrinsicDeclaration("llvm.x86.avx.vzeroupper")
		return lfc.b.CreateCall(fn.GlobalValueType(), fn, nil, "")
	case llvmSIMDFromBits:
		lanes := int(v.Type.Size()) * 8 / h.bits
		bitmap := lfc.GenLV(v.Args[0])
		if bitmap.Type().IntTypeWidth() != lanes {
			bitmap = lfc.b.CreateTrunc(bitmap, GlobalCtxt.IntType(lanes), v.String()+".low-bits")
		}
		// On amd64, lane zero occupies the least significant bit.
		mask := lfc.b.CreateBitCast(bitmap, llvm.VectorType(GlobalCtxt.Int1Type(), lanes), v.String()+".mask")
		return lfc.simdLaneResult(v, lfc.b.CreateSExt(mask, llvm.VectorType(laneType, lanes), v.String()+".lanes"))
	case llvmSIMDToBits, llvmSIMDIsZero, llvmSIMDIsNaN:
		lanes := int(v.Args[0].Type.Size()) * 8 / h.bits
		x := lfc.simdValueAs(v, v.Args[0], llvm.VectorType(laneType, lanes), ".x")
		if h.kind == llvmSIMDIsZero {
			fn := getLLVMIntrinsicDeclaration("llvm.vector.reduce.or", x.Type())
			bits := lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{x}, v.String()+".bits")
			return lfc.b.CreateICmp(llvm.IntEQ, bits, llvm.ConstNull(laneType), v.String()+".zero")
		}
		if h.kind == llvmSIMDIsNaN {
			fp := GlobalCtxt.FloatType()
			if h.bits == 64 {
				fp = GlobalCtxt.DoubleType()
			}
			x = lfc.b.CreateBitCast(x, llvm.VectorType(fp, lanes), v.String()+".float")
			mask := lfc.b.CreateFCmp(llvm.FloatUNO, x, x, v.String()+".nan")
			return lfc.simdLaneResult(v, lfc.b.CreateSExt(mask, llvm.VectorType(laneType, lanes), v.String()+".lanes"))
		}
		mask := lfc.b.CreateICmp(llvm.IntSLT, x, llvm.ConstNull(x.Type()), v.String()+".mask")
		bitmap := lfc.b.CreateBitCast(mask, GlobalCtxt.IntType(lanes), v.String()+".bits")
		if bitmap.Type() != getLLVMType(v.Type) {
			bitmap = lfc.b.CreateZExt(bitmap, getLLVMType(v.Type), v.String())
		}
		return bitmap
	case llvmSIMDMaskedLoad, llvmSIMDMaskedStore:
		t := v.Type
		if h.kind == llvmSIMDMaskedStore {
			t = v.Args[2].Type
		}
		vec := llvm.VectorType(laneType, int(t.Size())*8/h.bits)
		mask := lfc.simdValueAs(v, v.Args[1], vec, ".mask")
		condition := lfc.b.CreateICmp(llvm.IntSLT, mask, llvm.ConstNull(vec), v.String()+".condition")
		ptr := lfc.llvmAddressPointer(v, lfc.GenLV(v.Args[0]), v.Args[0].Type, v.String()+".address")
		// LLVM 23 masked memory intrinsics have no alignment argument.
		// Do not widen this to an unconditional memory access plus a select:
		// inactive lanes must not read or write even an inaccessible page.
		if h.kind == llvmSIMDMaskedStore {
			x := lfc.simdValueAs(v, v.Args[2], vec, ".x")
			fn := getLLVMIntrinsicDeclaration("llvm.masked.store", vec, ptr.Type())
			return lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{x, ptr, condition}, "")
		}
		fn := getLLVMIntrinsicDeclaration("llvm.masked.load", vec, ptr.Type())
		result := lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{ptr, condition, llvm.ConstNull(vec)}, v.String()+".loaded")
		return lfc.simdLaneResult(v, result)
	}
	panic("unreachable SIMD helper")
}
