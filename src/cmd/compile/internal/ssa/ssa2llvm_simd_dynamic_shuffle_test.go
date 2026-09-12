// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

// Route using scalar slice indexing and remainder rather than the lowering's
// masked extraction and source-select expansion.
func dynamicShuffleReference(family string, bits int, x, y, indices []uint64) []uint64 {
	n := len(x)
	out := make([]uint64, n)
	for i, index := range indices {
		source := x
		switch family {
		case "ConcatPermute":
			source = append(append([]uint64(nil), x...), y...)
		case "LookupOrZero":
			if index >= uint64(n) {
				continue
			}
		case "PermuteOrZero", "PermuteOrZeroGrouped":
			if index >= uint64(1)<<(bits-1) {
				continue
			}
			if family == "PermuteOrZeroGrouped" {
				start := i / 16 * 16
				source = x[start : start+16]
			}
		}
		out[i] = source[index%uint64(len(source))]
	}
	return out
}

func TestLLVMGeneratedSIMDDynamicShuffles(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	pattern := regexp.MustCompile(`^(Permute|ConcatPermute|LookupOrZero|PermuteOrZero|PermuteOrZeroGrouped)(Int|Uint|Float)(8|16|32|64)x([0-9]+)$`)
	count := 0
	for op := Op(0); int(op) < len(goALLCSIMDOpcodeIndex); op++ {
		match := pattern.FindStringSubmatch(op.String())
		if match == nil {
			continue
		}
		count++
		info, ok := goALLCSIMDInfo(op)
		if !ok || info.lowering == goALLCSIMDLowerNone {
			t.Errorf("missing lowering for %s", op)
			continue
		}
		family, kind := match[1], strings.ToLower(match[2])
		bits, _ := strconv.Atoi(match[3])
		n, _ := strconv.Atoi(match[4])
		wantLane := map[string]goALLCSIMDLane{"int": goALLCSIMDLaneInt, "uint": goALLCSIMDLaneUint, "float": goALLCSIMDLaneFloat}[kind]
		if info.lane != wantLane || int(info.laneBits) != bits {
			t.Fatalf("%s descriptor does not describe its data lanes", op)
		}
		if bits == 8 && (family == "Permute" || family == "ConcatPermute") && info.amd64.cpuProfile != goCPUProfileX86AVX512VBMI {
			t.Fatalf("%s must require VBMI", op)
		}
		arch := "amd64"
		if family == "LookupOrZero" {
			arch = "arm64"
		}
		for _, carrier := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/carrier=%v", op, arch, carrier), func(t *testing.T) {
				kinds := map[string]map[int]types.Kind{
					"int":   {8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64},
					"uint":  {8: types.TUINT8, 16: types.TUINT16, 32: types.TUINT32, 64: types.TUINT64},
					"float": {32: types.TFLOAT32, 64: types.TFLOAT64},
				}
				vector := llvmTestSIMDType("dynamic-data", types.Types[kinds[kind][bits]], int64(n))
				indexVector := llvmTestSIMDType("dynamic-indices", types.Types[kinds["uint"][bits]], int64(n))
				resultType := getLLVMType(vector)
				if carrier {
					vector = map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[bits*n]
					indexVector = vector
				}
				integer := GlobalCtxt.IntType(bits)
				integerVector := llvm.VectorType(integer, n)
				module := GlobalCtxt.NewModule("dynamic-shuffle")
				CurrentModule = module
				builder := GlobalCtxt.NewBuilder()
				defer module.Dispose()
				defer builder.Dispose()
				arity := 2
				if family == "ConcatPermute" {
					arity = 3
				}
				mask := ^uint64(0) >> (64 - bits)
				// All byte values, then every individual high bit and its
				// complement. Different lanes receive different index patterns.
				controls := make([]uint64, 256)
				for i := range controls {
					controls[i] = uint64(i)
				}
				for i := 8; i < bits; i++ {
					controls = append(controls, uint64(1)<<i, mask^(uint64(1)<<i))
				}
				for control, value := range controls {
					fn := llvm.AddFunction(module, fmt.Sprintf("shuffle%d", control), llvm.FunctionType(resultType, nil, false))
					builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
					// These constant-folding unit functions run under the exact
					// descriptor capability floor; separate FMV tests verify the
					// source guard, resolver predicates, and unguarded rejection.
					ctx := &LLVMFuncContext{F: &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}}, CPUFeatureFloor: info.archInfo(arch).cpuProfile, Vs: make(map[ID]llvm.Value), b: builder}
					args := make([]*Value, arity)
					sources := make([][]uint64, arity)
					for side := range args {
						typ := vector
						if side == arity-1 {
							typ = indexVector
						}
						args[side] = &Value{ID: ID(side + 1), Op: OpArg, Type: typ}
						values := make([]llvm.Value, n)
						sources[side] = make([]uint64, n)
						for i := range values {
							v := uint64(side*n + i + 1)
							if side == arity-1 {
								v = (value + uint64(i)) & mask
							}
							sources[side][i] = v
							values[i] = llvm.ConstInt(integer, v, false)
						}
						ctx.Vs[args[side].ID] = llvm.ConstBitCast(llvm.ConstVector(values, false), getLLVMType(typ))
					}
					if family == "Permute" || family == "ConcatPermute" {
						args = append([]*Value{args[arity-1]}, args[:arity-1]...)
					}
					v := &Value{ID: 4, Op: op, Type: vector, Args: args}
					result := ctx.GenLV(v)
					builder.CreateRet(result)
					result = llvm.ConstBitCast(result, integerVector)
					want := dynamicShuffleReference(family, bits, sources[0], sources[1], sources[arity-1])
					for i, expected := range want {
						got := llvm.ConstExtractElement(result, llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(i), false))
						if got.IsAConstantInt().IsNil() || got.ZExtValue() != expected {
							t.Fatalf("control=%#x lane=%d got=%s want=%d", value, i, got.String(), expected)
						}
					}
				}
				if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
					t.Fatal(err)
				}
				for _, forbidden := range []string{" poison", " undef", "@llvm.x86.", "@llvm.aarch64."} {
					if strings.Contains(module.String(), forbidden) {
						t.Fatalf("unexpected %s in IR", forbidden)
					}
				}
			})
		}
	}
	if count != 62 {
		t.Fatalf("tested %d dynamic shuffle descriptors, want 62", count)
	}
}
