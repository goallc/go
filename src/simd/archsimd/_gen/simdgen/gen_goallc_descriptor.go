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

// goALLCOperandShape preserves the generic-lowering-relevant portion of an
// operand. The compact form is embedded inside the versioned descriptor and is
// compared across architectures by the merge step.
func goALLCOperandShape(o Operand) string {
	return strings.Join([]string{
		o.Class,
		goALLCPointerString(o.Go),
		goALLCPointerString(o.Base),
		goALLCPointerString(o.ElemBits),
		goALLCPointerString(o.Bits),
		goALLCPointerString(o.Lanes),
		goALLCPointerString(o.Const),
		goALLCPointerString(o.ImmOffset),
		goALLCPointerString(o.ImmMax),
		goALLCPointerString(o.TreatLikeAScalarOfSize),
		goALLCPointerString(o.OverwriteClass),
		goALLCPointerString(o.OverwriteBase),
		goALLCPointerString(o.OverwriteElementBits),
		goALLCPointerString(o.OverwriteBits),
		goALLCPointerString(o.ListNumber),
		goALLCPointerString(o.FixedReg),
	}, ":")
}

func goALLCOperandShapes(ops []Operand) string {
	shapes := make([]string, len(ops))
	for i, op := range ops {
		shapes[i] = goALLCOperandShape(op)
	}
	return strings.Join(shapes, "|")
}

func goALLCGenericOperandShapes(ops []Operand) string {
	shapes := make([]string, len(ops))
	for i, op := range ops {
		class := op.Class
		if op.OverwriteClass != nil {
			class = *op.OverwriteClass
		}
		shapes[i] = class + ":" + goALLCPointerString(op.Go)
	}
	return strings.Join(shapes, "|")
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

func validateGoALLCLowering(op Operation, d sgutil.SIMDOpData) {
	wantArity, ok := goALLCLoweringArity[d.Lowering]
	if !ok {
		panic(fmt.Errorf("simdgen: unknown LLVM lowering %q for %s", d.Lowering, op.GenericName()))
	}
	if d.Input != "pure-vreg" || d.Output != "vreg" || d.Immediate != "none" || d.Mask != "none" || d.Memory != "none" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires an unmasked register-only operation: %s has in=%s out=%s imm=%s mask=%s mem=%s", d.Lowering, op.GenericName(), d.Input, d.Output, d.Immediate, d.Mask, d.Memory))
	}
	if len(op.In) != wantArity {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d inputs, want %d", d.Lowering, op.GenericName(), len(op.In), wantArity))
	}
	if len(op.Out) != 1 {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d outputs, want 1", d.Lowering, op.GenericName(), len(op.Out)))
	}
	if d.Width != d.LaneBits*d.Lanes || (d.Lane != "int" && d.Lane != "uint" && d.Lane != "float") {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has invalid lane shape %s%d x %d for %s", d.Lowering, d.Lane, d.LaneBits, d.Lanes, op.GenericName()))
	}
	for _, in := range op.In {
		base, elemBits, lanes, ok := goALLCLaneFromGoType(in.Go)
		if !ok && in.Base != nil && in.ElemBits != nil && in.Lanes != nil {
			base, elemBits, lanes, ok = *in.Base, *in.ElemBits, *in.Lanes, true
		}
		if in.Class != "vreg" || in.Bits == nil || *in.Bits != d.Width || !ok || elemBits != d.LaneBits || lanes != d.Lanes || base != d.Lane {
			panic(fmt.Errorf("simdgen: LLVM lowering %q has heterogeneous input shape for %s", d.Lowering, op.GenericName()))
		}
	}
	out := op.Out[0]
	if out.Class != "mask" && (out.Class != "vreg" || out.Bits == nil || *out.Bits != d.Width) {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has incompatible output shape for %s", d.Lowering, op.GenericName()))
	}
}

func goALLCSIMDDescriptor(op, genericOp Operation, shapeIn inShape, shapeOut outShape, maskType maskShape, immType immShape, genericIn inShape, genericOut outShape, genericMask maskShape, genericImm immShape) sgutil.SIMDOpData {
	arch := CurrentArch().Arch
	base, elemBits, lanes := goALLCPrimaryLane(genericOp)
	operandOrder := ""
	if op.OperandOrder != nil {
		operandOrder = *op.OperandOrder
	}
	memoryFeature := ""
	if op.MemFeatures != nil {
		memoryFeature = *op.MemFeatures
	}
	memoryFeatureData := ""
	if op.MemFeaturesData != nil {
		memoryFeatureData = *op.MemFeaturesData
	}
	lowering := ""
	if op.LLVMLowering != nil {
		lowering = *op.LLVMLowering
	}
	d := sgutil.SIMDOpData{
		Lowering:  lowering,
		Width:     genericOp.VectorWidth(),
		Lane:      base,
		LaneBits:  elemBits,
		Lanes:     lanes,
		Input:     goALLCShapeName(genericIn),
		Output:    goALLCShapeName(genericOut),
		Immediate: goALLCShapeName(genericImm),
		Mask:      goALLCShapeName(genericMask),
		Memory:    "none",
		Inputs:    goALLCGenericOperandShapes(genericOp.In),
		Outputs:   goALLCGenericOperandShapes(genericOp.Out),
		Arch: map[string]sgutil.SIMDArchData{
			arch: {
				CPUFeature:        op.CPUFeature,
				CPUProfile:        goALLCCPUProfile(arch, op.CPUFeature),
				OperandOrder:      operandOrder,
				Input:             goALLCShapeName(shapeIn),
				Output:            goALLCShapeName(shapeOut),
				Immediate:         goALLCShapeName(immType),
				Mask:              goALLCShapeName(maskType),
				Inputs:            goALLCOperandShapes(op.In),
				Outputs:           goALLCOperandShapes(op.Out),
				MemoryFeature:     memoryFeature,
				MemoryFeatureData: memoryFeatureData,
			},
		},
	}
	if d.Lowering != "" {
		validateGoALLCLowering(genericOp, d)
	}
	return d
}
