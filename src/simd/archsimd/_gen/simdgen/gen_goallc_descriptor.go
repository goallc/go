// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"simd/archsimd/_gen/internal/goallccpu"
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

var goALLCVectorTypeRE = regexp.MustCompile(`^(Int|Uint|Float)(8|16|32|64)x([0-9]+)$`)
var goALLCScalarTypeRE = regexp.MustCompile(`^(int|uint|float)(8|16|32|64)$`)

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

func goALLCScalarLane(operand Operand) (base string, elemBits int, ok bool) {
	if operand.Go != nil {
		m := goALLCScalarTypeRE.FindStringSubmatch(*operand.Go)
		if m != nil {
			elemBits, _ = strconv.Atoi(m[2])
			return m[1], elemBits, true
		}
	}
	basePtr := operand.Base
	if operand.OverwriteBase != nil {
		basePtr = operand.OverwriteBase
	}
	if basePtr == nil {
		return "", 0, false
	}
	elemBitsPtr := operand.ElemBits
	if operand.OverwriteBits != nil {
		elemBitsPtr = operand.OverwriteBits
	} else if elemBitsPtr == nil {
		elemBitsPtr = operand.Bits
	}
	if elemBitsPtr == nil || (operand.TreatLikeAScalarOfSize == nil && operand.Lanes != nil && *operand.Lanes != 1) {
		return "", 0, false
	}
	return *basePtr, *elemBitsPtr, true
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
	"compress": 2, "expand": 2, "blend-masked": 3, "broadcast-low-masked": 2,
	"reciprocal": 1, "reciprocal-sqrt": 1,
	"scale":       2,
	"sha1-rounds": 2, "sha1-next-e": 2, "sha1-message1": 2, "sha1-message2": 2,
	"sha256-rounds": 3, "sha256-message1": 2, "sha256-message2": 2,
	"gf-mul": 2, "gf-affine": 2, "gf-affine-inverse": 2,
	"aes-encrypt": 2, "aes-encrypt-last": 2, "aes-decrypt": 2, "aes-decrypt-last": 2,
	"aes-keygen": 1, "aes-inverse-mix": 1,
	"rotate-left": 2, "rotate-right": 2,
	"funnel-left": 3, "funnel-right": 3,
	"funnel-all-left": 2, "funnel-all-right": 2,
	"dot-pairs": 2, "dot-pairs-us-sat": 2, "sum-8-abs-diff": 2,
	"pair-add":          2,
	"pair-sub":          2,
	"pair-add-128":      2,
	"pair-sub-128":      2,
	"pair-sadd-sat":     2,
	"pair-ssub-sat":     2,
	"pair-sadd-sat-128": 2,
	"pair-ssub-sat-128": 2,
	"add-odd-sub-even":  2,
	"mul-add-integer":   3,
	"fma":               3, "fma-add-even": 3, "fma-add-odd": 3,
	"shift-all-left": 2, "shift-all-right": 2, "shift-left": 2, "shift-right": 2,
	"shift-signed-count": 2, "shift-signed-saturated": 2,
	"permute": 2, "concat-permute": 3, "lookup-or-zero": 2,
	"permute-or-zero": 2, "permute-or-zero-128": 2,
	"permute-32-128": 1, "permute-low-16-128": 1, "permute-high-16-128": 1,
	"concat-select-128": 2, "concat-permute-128": 2, "concat-shift-bytes-128": 2,
	"add": 2, "sub": 2, "mul": 2, "div": 2,
	"sat-add": 2, "sat-sub": 2,
	"extract-element": 1, "insert-element": 2,
	"reduce-add": 1, "reduce-max": 1, "reduce-min": 1,
	"and": 2, "or": 2, "xor": 2, "andnot": 2, "ornot": 2,
	"not": 1, "neg": 1, "abs": 1,
	"sqrt": 1, "round-even": 1, "floor": 1, "ceil": 1, "trunc": 1,
	"round-scaled": 1, "floor-scaled": 1, "ceil-scaled": 1, "trunc-scaled": 1,
	"round-scaled-residue": 1, "floor-scaled-residue": 1, "ceil-scaled-residue": 1, "trunc-scaled-residue": 1,
	"bit-select": 3, "bit-select-not": 3, "blend-bytes": 3, "ternary": 3,
	"ones-count": 1, "leading-zeros": 1,
	"average": 2, "leading-sign-bits": 1,
	"mul-high": 2, "mul-sign": 2,
	"mul-widen-even": 2, "mul-widen-low": 2,
	"carryless-mul-widen-low": 2,
	"carryless-mul":           2,
	"extend-integer":          1, "truncate-integer": 1,
	"saturate-integer": 1, "saturate-integer-pack128": 2,
	"convert-float":       1,
	"get-low":             1,
	"get-high":            1,
	"set-low":             2,
	"set-high":            2,
	"broadcast-low":       1,
	"interleave-low":      2,
	"interleave-high":     2,
	"interleave-low-128":  2,
	"interleave-high-128": 2,
	"concat-even":         2,
	"concat-odd":          2,
	"interleave-even":     2,
	"interleave-odd":      2,
	"max":                 2, "min": 2,
	"equal": 2, "not-equal": 2, "greater": 2,
	"greater-equal": 2, "less": 2, "less-equal": 2,
}

func validateGoALLCLowering(op, genericOp Operation, lowering string, genericIn inShape, genericOut outShape, genericMask maskShape, genericImm immShape) {
	wantArity, ok := goALLCLoweringArity[lowering]
	if !ok {
		panic(fmt.Errorf("simdgen: unknown LLVM lowering %q for %s", lowering, op.GenericName()))
	}
	wantOut, wantImm := OneVregOut, NoImm
	if goALLCImmediateShuffleLowering(lowering) {
		// A user immediate is a UInt8 Aux value, even when its public
		// variable form is expanded by the frontend into a jump table.
		wantImm = VarImm
	}
	switch lowering {
	case "extract-element":
		wantOut, wantImm = OneGregOut, VarImm
	case "insert-element", "funnel-all-left", "funnel-all-right", "carryless-mul", "aes-keygen", "gf-affine", "gf-affine-inverse", "sha1-rounds":
		wantImm = VarImm
	case "round-scaled", "floor-scaled", "ceil-scaled", "trunc-scaled":
		wantImm = VarImm
	case "round-scaled-residue", "floor-scaled-residue", "ceil-scaled-residue", "trunc-scaled-residue", "ternary":
		wantImm = VarImm
	}
	wantMask := NoMask
	masked := lowering == "compress" || lowering == "expand" || lowering == "blend-masked" || lowering == "broadcast-low-masked"
	validIn := genericIn == PureVregIn || (lowering == "lookup-or-zero" && genericIn == VlistIn)
	if masked {
		wantMask, validIn = OneMask, genericIn == OneKmaskIn
	}
	if !validIn || genericOut != wantOut || genericImm != wantImm || genericMask != wantMask {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has unsupported shape: %s has in=%s out=%s imm=%s mask=%s", lowering, op.GenericName(), goALLCShapeName(genericIn), goALLCShapeName(genericOut), goALLCShapeName(genericImm), goALLCShapeName(genericMask)))
	}
	if len(genericOp.In) != wantArity {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d inputs, want %d", lowering, op.GenericName(), len(genericOp.In), wantArity))
	}
	if len(genericOp.Out) != 1 {
		panic(fmt.Errorf("simdgen: LLVM lowering %q for %s has %d outputs, want 1", lowering, op.GenericName(), len(genericOp.Out)))
	}
	if masked {
		base, bits, lanes := goALLCPrimaryLane(genericOp)
		outBase, outBits, outLanes, ok := goALLCLaneFromGoType(genericOp.Out[0].Go)
		if !ok || outBase != base || outBits != bits ||
			(lowering != "broadcast-low-masked" && outLanes != lanes) ||
			(lowering == "broadcast-low-masked" && (bits*lanes != 128 || outLanes < lanes)) {
			panic(fmt.Errorf("simdgen: incompatible LLVM masked output for %s", op.GenericName()))
		}
		for _, in := range genericOp.In {
			if in.Class == "mask" {
				// Upstream broadcast masks follow the source width, including
				// widening broadcasts. Keep those semantics until upstream changes.
				if in.Go == nil || *in.Go != fmt.Sprintf("Mask%dx%d", bits, lanes) {
					panic(fmt.Errorf("simdgen: incompatible LLVM mask for %s", op.GenericName()))
				}
			} else if b, e, n, ok := goALLCLaneFromGoType(in.Go); !ok || b != base || e != bits || n != lanes {
				panic(fmt.Errorf("simdgen: incompatible LLVM masked input for %s", op.GenericName()))
			}
		}
		return
	}
	if (lowering == "extract-element" || lowering == "insert-element") &&
		(genericOp.In[0].Class != "vreg" || genericOp.In[0].TreatLikeAScalarOfSize != nil) {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires the vector as the first input for %s", lowering, op.GenericName()))
	}
	if goALLCImmediateShuffleLowering(lowering) {
		validateGoALLCImmediateShuffle(op, genericOp, lowering)
		return
	}
	if goALLCDynamicShuffleLowering(lowering) {
		validateGoALLCDynamicShuffle(op, genericOp, lowering)
		return
	}
	if goALLCShiftLowering(lowering) {
		validateGoALLCShift(op, genericOp, lowering)
		return
	}
	if goALLCStaticShuffleLowering(lowering) {
		validateGoALLCStaticShuffle(op, genericOp, lowering)
		return
	}
	if goALLCConversionLowering(lowering) {
		if lowering == "convert-float" {
			validateGoALLCFloatConversion(op, genericOp)
		} else {
			validateGoALLCIntegerConversion(op, genericOp, lowering)
		}
		return
	}
	wantBase, wantElemBits, wantLanes := goALLCPrimaryLane(genericOp)
	width := genericOp.VectorWidth()
	if width != wantElemBits*wantLanes || (wantBase != "int" && wantBase != "uint" && wantBase != "float") {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has invalid lane shape %s%d x %d for %s", lowering, wantBase, wantElemBits, wantLanes, op.GenericName()))
	}
	if (lowering == "sat-add" || lowering == "sat-sub") && wantBase != "int" && wantBase != "uint" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires integer lanes for %s", lowering, op.GenericName()))
	}
	if lowering == "reduce-add" && wantBase != "int" && wantBase != "uint" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires integer lanes for %s", lowering, op.GenericName()))
	}
	if (lowering == "sqrt" || lowering == "round-even" || lowering == "floor" || lowering == "ceil" || lowering == "trunc") && wantBase != "float" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires floating-point lanes for %s", lowering, op.GenericName()))
	}
	if (lowering == "ones-count" || lowering == "leading-zeros" || lowering == "average" || lowering == "leading-sign-bits" || lowering == "mul-high" || lowering == "mul-sign") && wantBase != "int" && wantBase != "uint" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires integer lanes for %s", lowering, op.GenericName()))
	}
	if lowering == "mul-sign" && wantBase != "int" {
		panic(fmt.Errorf("simdgen: LLVM lowering %q requires signed integer lanes for %s", lowering, op.GenericName()))
	}
	scalarInputs := 0
	for i, in := range genericOp.In {
		if in.Class == "greg" || in.TreatLikeAScalarOfSize != nil {
			scalarInputs++
			base, elemBits, ok := goALLCScalarLane(in)
			if !ok || base != wantBase || elemBits != wantElemBits {
				panic(fmt.Errorf("simdgen: LLVM lowering %q has incompatible scalar input for %s", lowering, op.GenericName()))
			}
			continue
		}
		base, elemBits, lanes, ok := goALLCLaneFromGoType(in.Go)
		if !ok && in.Base != nil && in.ElemBits != nil && in.Lanes != nil {
			base, elemBits, lanes, ok = *in.Base, *in.ElemBits, *in.Lanes, true
		}
		inputBase := wantBase
		inputElemBits, inputLanes := wantElemBits, wantLanes
		if strings.HasPrefix(lowering, "aes-") && i == 1 {
			// AES rounds accept byte states and uint32 round-key words.
			inputElemBits, inputLanes = 32, width/32
		}
		if (lowering == "gf-affine" || lowering == "gf-affine-inverse") && i == 1 {
			// Each uint64 matrix transforms eight byte lanes.
			inputElemBits, inputLanes = 64, width/64
		}
		if lowering == "dot-pairs-us-sat" && i == 1 {
			inputBase = "int"
		}
		if (lowering == "funnel-left" || lowering == "funnel-right") && i == 2 {
			inputBase = "uint"
		}
		if in.Class != "vreg" || in.Bits == nil || *in.Bits != width || !ok || base != inputBase || elemBits != inputElemBits || lanes != inputLanes {
			panic(fmt.Errorf("simdgen: LLVM lowering %q has heterogeneous input shape for %s", lowering, op.GenericName()))
		}
	}
	out := genericOp.Out[0]
	if lowering == "extract-element" {
		wantScalar := fmt.Sprintf("%s%d", wantBase, wantElemBits)
		if scalarInputs != 0 || out.Class != "greg" || op.goNormalType() != wantScalar {
			panic(fmt.Errorf("simdgen: LLVM lowering %q requires one vector input and a %s output for %s", lowering, wantScalar, op.GenericName()))
		}
	} else if lowering == "insert-element" {
		if scalarInputs != 1 || out.Class != "vreg" || out.Bits == nil || *out.Bits != width {
			panic(fmt.Errorf("simdgen: LLVM lowering %q requires one vector and one scalar input for %s", lowering, op.GenericName()))
		}
	} else if scalarInputs != 0 || out.Class != "mask" && (out.Class != "vreg" || out.Bits == nil || *out.Bits != width) {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has incompatible output shape for %s", lowering, op.GenericName()))
	}
	if lowering == "extract-element" || lowering == "insert-element" {
		immediates := 0
		for _, in := range op.In {
			if in.Class != "immediate" || in.ImmOffset == nil || in.Const != nil {
				continue
			}
			immediates++
			if *in.ImmOffset != "0" || in.ImmMax == nil || *in.ImmMax != wantLanes-1 {
				panic(fmt.Errorf("simdgen: LLVM lowering %q requires an unshifted lane index limited to %d for %s", lowering, wantLanes-1, op.GenericName()))
			}
		}
		if immediates != 1 {
			panic(fmt.Errorf("simdgen: LLVM lowering %q requires one lane index for %s", lowering, op.GenericName()))
		}
	}
}

// Conversion shape comes from the public Go operand types, not the native
// instruction's register arrangement or the maximum vector width.
func goALLCConversionLowering(lowering string) bool {
	switch lowering {
	case "extend-integer", "truncate-integer", "saturate-integer", "saturate-integer-pack128", "convert-float":
		return true
	}
	return false
}

func validateGoALLCFloatConversion(op, genericOp Operation) {
	in, out := genericOp.In[0], genericOp.Out[0]
	base, bits, lanes, ok := goALLCLaneFromGoType(in.Go)
	outBase, outBits, outLanes, outOK := goALLCLaneFromGoType(out.Go)
	if !ok || !outOK || (base != "float" && outBase != "float") ||
		(bits != 32 && bits != 64) || (outBits != 32 && outBits != 64) ||
		(base == outBase && bits == outBits) ||
		in.Class != "vreg" || out.Class != "vreg" || in.TreatLikeAScalarOfSize != nil ||
		in.Bits == nil || out.Bits == nil || *in.Bits != bits*lanes || *out.Bits != outBits*outLanes {
		panic(fmt.Errorf("simdgen: LLVM floating conversion has incompatible lane shape for %s", op.GenericName()))
	}
	if outLanes < lanes {
		// ARM64 FCVTL converts only the low half of a Float32x4.
		if base != "float" || outBase != "float" || bits != 32 || outBits != 64 || outLanes*2 != lanes {
			panic(fmt.Errorf("simdgen: LLVM floating conversion has invalid low-lane shape for %s", op.GenericName()))
		}
	} else if outLanes > lanes && (outBits >= bits || outBits*outLanes != 128) {
		// Narrow conversions have a minimum 128-bit result, with zero high lanes.
		panic(fmt.Errorf("simdgen: LLVM floating conversion has invalid padded shape for %s", op.GenericName()))
	}
}

func validateGoALLCIntegerConversion(op, genericOp Operation, lowering string) {
	in, out := genericOp.In[0], genericOp.Out[0]
	base, bits, lanes, ok := goALLCLaneFromGoType(in.Go)
	outBase, outBits, outLanes, outOK := goALLCLaneFromGoType(out.Go)
	saturating := lowering == "saturate-integer" || lowering == "saturate-integer-pack128"
	if !ok || !outOK || (base != "int" && base != "uint") || (outBase != "int" && outBase != "uint") ||
		(!saturating && base != outBase) ||
		in.Class != "vreg" || out.Class != "vreg" || in.TreatLikeAScalarOfSize != nil ||
		in.Bits == nil || out.Bits == nil || *in.Bits != bits*lanes || *out.Bits != outBits*outLanes {
		panic(fmt.Errorf("simdgen: LLVM lowering %q has incompatible integer conversion shape for %s", lowering, op.GenericName()))
	}
	for _, other := range genericOp.In[1:] {
		otherBase, otherBits, otherLanes, ok := goALLCLaneFromGoType(other.Go)
		if !ok || other.Class != "vreg" || other.TreatLikeAScalarOfSize != nil || other.Bits == nil ||
			*other.Bits != bits*lanes || otherBase != base || otherBits != bits || otherLanes != lanes {
			panic(fmt.Errorf("simdgen: LLVM lowering %q has heterogeneous conversion inputs for %s", lowering, op.GenericName()))
		}
	}
	if lowering == "extend-integer" {
		if outBits <= bits || outLanes > lanes {
			panic(fmt.Errorf("simdgen: LLVM integer extension has invalid lane shape for %s", op.GenericName()))
		}
	} else if lowering == "saturate-integer-pack128" {
		if bits != 2*outBits || outLanes != 2*lanes || bits*lanes < 128 || bits*lanes%128 != 0 {
			panic(fmt.Errorf("simdgen: LLVM saturating pack has invalid 128-bit group shape for %s", op.GenericName()))
		}
	} else if outBits >= bits || outLanes < lanes {
		panic(fmt.Errorf("simdgen: LLVM integer narrowing has invalid lane shape for %s", op.GenericName()))
	}
}

func goALLCSIMDDescriptor(op, genericOp Operation, genericIn inShape, genericOut outShape, genericMask maskShape, genericImm immShape) sgutil.SIMDOpData {
	if op.LLVMLowering == nil {
		return sgutil.SIMDOpData{}
	}
	arch := CurrentArch().Arch
	base, elemBits, _ := goALLCPrimaryLane(genericOp)
	if *op.LLVMLowering == "permute" || *op.LLVMLowering == "concat-permute" {
		base, elemBits, _, _ = goALLCLaneFromGoType(genericOp.In[1].Go)
	}
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
				CPUProfile:   goallccpu.ProfileForSIMD(arch, op.CPUFeature),
				OperandOrder: operandOrder,
			},
		},
	}
	if d.Lowering == "broadcast-low" && genericMask != NoMask {
		d.Lowering = "broadcast-low-masked"
	}
	validateGoALLCLowering(op, genericOp, d.Lowering, genericIn, genericOut, genericMask, genericImm)
	if goALLCConversionLowering(d.Lowering) {
		var resultLane string
		resultLane, d.ResultLaneBits, _, _ = goALLCLaneFromGoType(genericOp.Out[0].Go)
		if resultLane != d.Lane {
			d.ResultLane = resultLane
		}
	}
	return d
}
