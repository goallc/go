// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"simd/archsimd/_gen/sgutil"
	"strconv"
	"strings"
)

func goALLCShapeName(v any) string {
	switch v := v.(type) {
	case inShape:
		return [...]string{"invalid", "pure-vreg", "vreg-mask", "vreg-immediate", "vreg-mask-immediate", "pure-mask", "vreg-list"}[v]
	case outShape:
		return [...]string{"invalid", "none", "vreg", "greg", "mask", "vreg-at-input", "vreg-scalar"}[v]
	case maskShape:
		return [...]string{"invalid", "none", "one", "all"}[v]
	case immShape:
		return [...]string{"invalid", "none", "const", "variable", "const-variable", "variable-limited"}[v]
	default:
		panic(fmt.Sprintf("unknown SIMD shape %T", v))
	}
}

func goALLCPointerString[T ~int | ~string](p *T) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprint(*p)
}

func goALLCCPUProfile(arch, feature string) string {
	if arch != "amd64" {
		return ""
	}
	switch {
	case strings.HasPrefix(feature, "AVX512"):
		return "x86.avx512"
	case feature == "AVX2" || feature == "AVXVNNI":
		return "x86.avx2"
	case feature == "FMA":
		return "x86.fma"
	case feature == "AVX" || feature == "AVXAES" || feature == "VAES":
		return "x86.avx"
	default:
		return ""
	}
}

var goALLCVectorTypeRE = regexp.MustCompile(`^(Int|Uint|Float)(8|16|32|64)x([0-9]+)$`)

func goALLCLaneFromGoType(goType *string) (base string, elemBits, lanes int, ok bool) {
	if goType == nil {
		return "", 0, 0, false
	}
	m := goALLCVectorTypeRE.FindStringSubmatch(*goType)
	if m == nil {
		return "", 0, 0, false
	}
	base = strings.ToLower(m[1])
	elemBits, _ = strconv.Atoi(m[2])
	lanes, _ = strconv.Atoi(m[3])
	return base, elemBits, lanes, true
}

func goALLCGenericOutputShape(op Operation, fallback outShape) outShape {
	if len(op.Out) != 1 || op.Out[0].Go == nil {
		return fallback
	}
	goType := *op.Out[0].Go
	if goALLCVectorTypeRE.MatchString(goType) || strings.HasPrefix(goType, "Mask") {
		return OneVregOut
	}
	return OneGregOut
}

func goALLCPrimaryLane(op Operation) (base string, elemBits, lanes int) {
	for _, in := range op.In {
		if in.Class == "vreg" {
			if base, elemBits, lanes, ok := goALLCLaneFromGoType(in.Go); ok {
				return base, elemBits, lanes
			}
			return goALLCPointerString(in.Base), *in.ElemBits, *in.Lanes
		}
	}
	for _, out := range op.Out {
		if out.Class == "vreg" {
			if base, elemBits, lanes, ok := goALLCLaneFromGoType(out.Go); ok {
				return base, elemBits, lanes
			}
			return goALLCPointerString(out.Base), *out.ElemBits, *out.Lanes
		}
	}
	return "none", 0, 0
}

var goALLCLoweringArity = map[string]int{
	"add": 2, "sub": 2, "mul": 2, "div": 2,
	"and": 2, "or": 2, "xor": 2, "andnot": 2, "ornot": 2,
	"not": 1, "neg": 1, "abs": 1,
	"equal": 2, "not-equal": 2, "greater": 2,
	"greater-equal": 2, "less": 2, "less-equal": 2,
}

func validateGoALLCLowering(op, genericOp Operation, lowering string, genericIn inShape, genericOut outShape, genericMask maskShape, genericImm immShape) {
	wantArity, ok := goALLCLoweringArity[lowering]
	if !ok {
		panic(fmt.Errorf("simdgen: unknown LLVM lowering %q for %s", lowering, op.GenericName()))
	}
	if genericIn != PureVregIn || genericOut != OneVregOut || genericImm != NoImm || genericMask != NoMask {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires an unmasked register-only operation: %s has in=%s out=%s imm=%s mask=%s", lowering, op.GenericName(), goALLCShapeName(genericIn), goALLCShapeName(genericOut), goALLCShapeName(genericImm), goALLCShapeName(genericMask)))
	}
	if len(genericOp.In) != wantArity {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d inputs, want %d", lowering, op.GenericName(), len(genericOp.In), wantArity))
	}
	if len(genericOp.Out) != 1 {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d outputs, want 1", lowering, op.GenericName(), len(genericOp.Out)))
	}
	wantBase, wantElemBits, wantLanes := goALLCPrimaryLane(genericOp)
	width := genericOp.VectorWidth()
	if width != wantElemBits*wantLanes || (wantBase != "int" && wantBase != "uint" && wantBase != "float") {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has invalid lane shape %s%d x %d for %s", lowering, wantBase, wantElemBits, wantLanes, op.GenericName()))
	}
	for _, in := range genericOp.In {
		base, elemBits, lanes, ok := goALLCLaneFromGoType(in.Go)
		if !ok && in.Base != nil && in.ElemBits != nil && in.Lanes != nil {
			base, elemBits, lanes, ok = *in.Base, *in.ElemBits, *in.Lanes, true
		}
		if in.Class != "vreg" || in.Bits == nil || *in.Bits != width || !ok || base != wantBase || elemBits != wantElemBits || lanes != wantLanes {
			panic(fmt.Errorf("simdgen: LLVM lowering %q has heterogeneous input shape for %s", lowering, op.GenericName()))
		}
	}
	out := genericOp.Out[0]
	if out.Class != "mask" && (out.Class != "vreg" || out.Bits == nil || *out.Bits != width) {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has incompatible output shape for %s", lowering, op.GenericName()))
	}
}

func goALLCSIMDDescriptor(op, genericOp Operation, genericIn inShape, genericOut outShape, genericMask maskShape, genericImm immShape) sgutil.SIMDOpData {
	if op.LLVMLowering == nil {
		return sgutil.SIMDOpData{}
	}
	arch := CurrentArch().Arch
	base, elemBits, _ := goALLCPrimaryLane(genericOp)
	operandOrder := ""
	if op.OperandOrder != nil {
		operandOrder = *op.OperandOrder
	}
	d := sgutil.SIMDOpData{
		Lowering: *op.LLVMLowering,
		Lane:     base,
		LaneBits: elemBits,
		Arch: map[string]sgutil.SIMDArchData{
			arch: {
				CPUProfile:   goALLCCPUProfile(arch, op.CPUFeature),
				OperandOrder: operandOrder,
			},
		},
	}
	validateGoALLCLowering(op, genericOp, d.Lowering, genericIn, genericOut, genericMask, genericImm)
	return d
}
