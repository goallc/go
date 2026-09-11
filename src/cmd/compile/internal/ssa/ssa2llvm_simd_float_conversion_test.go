// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMGeneratedSIMDFloatConversions(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	tests := []struct {
		op                Op
		src               string
		srcBits, srcLanes int
		dst               string
		dstBits, dstLanes int
		amd64, arm64      string
	}{
		{OpConvertLo2ToFloat64Float32x4, "float", 32, 4, "float", 64, 2, "", "fpext"},
		{OpConvertToFloat32Float64x2, "float", 64, 2, "float", 32, 4, "fptrunc", "fptrunc"},
		{OpConvertToFloat32Float64x4, "float", 64, 4, "float", 32, 4, "fptrunc", ""},
		{OpConvertToFloat32Float64x8, "float", 64, 8, "float", 32, 8, "fptrunc", ""},
		{OpConvertToFloat32Int32x16, "int", 32, 16, "float", 32, 16, "sitofp", ""},
		{OpConvertToFloat32Int32x4, "int", 32, 4, "float", 32, 4, "sitofp", "sitofp"},
		{OpConvertToFloat32Int32x8, "int", 32, 8, "float", 32, 8, "sitofp", ""},
		{OpConvertToFloat32Int64x2, "int", 64, 2, "float", 32, 4, "sitofp", ""},
		{OpConvertToFloat32Int64x4, "int", 64, 4, "float", 32, 4, "sitofp", ""},
		{OpConvertToFloat32Int64x8, "int", 64, 8, "float", 32, 8, "sitofp", ""},
		{OpConvertToFloat32Uint32x16, "uint", 32, 16, "float", 32, 16, "uitofp", ""},
		{OpConvertToFloat32Uint32x4, "uint", 32, 4, "float", 32, 4, "uitofp", "uitofp"},
		{OpConvertToFloat32Uint32x8, "uint", 32, 8, "float", 32, 8, "uitofp", ""},
		{OpConvertToFloat32Uint64x2, "uint", 64, 2, "float", 32, 4, "uitofp", ""},
		{OpConvertToFloat32Uint64x4, "uint", 64, 4, "float", 32, 4, "uitofp", ""},
		{OpConvertToFloat32Uint64x8, "uint", 64, 8, "float", 32, 8, "uitofp", ""},
		{OpConvertToFloat64Float32x4, "float", 32, 4, "float", 64, 4, "fpext", ""},
		{OpConvertToFloat64Float32x8, "float", 32, 8, "float", 64, 8, "fpext", ""},
		{OpConvertToFloat64Int32x4, "int", 32, 4, "float", 64, 4, "sitofp", ""},
		{OpConvertToFloat64Int32x8, "int", 32, 8, "float", 64, 8, "sitofp", ""},
		{OpConvertToFloat64Int64x2, "int", 64, 2, "float", 64, 2, "sitofp", "sitofp"},
		{OpConvertToFloat64Int64x4, "int", 64, 4, "float", 64, 4, "sitofp", ""},
		{OpConvertToFloat64Int64x8, "int", 64, 8, "float", 64, 8, "sitofp", ""},
		{OpConvertToFloat64Uint32x4, "uint", 32, 4, "float", 64, 4, "uitofp", ""},
		{OpConvertToFloat64Uint32x8, "uint", 32, 8, "float", 64, 8, "uitofp", ""},
		{OpConvertToFloat64Uint64x2, "uint", 64, 2, "float", 64, 2, "uitofp", "uitofp"},
		{OpConvertToFloat64Uint64x4, "uint", 64, 4, "float", 64, 4, "uitofp", ""},
		{OpConvertToFloat64Uint64x8, "uint", 64, 8, "float", 64, 8, "uitofp", ""},
		{OpConvertToInt32Float32x16, "float", 32, 16, "int", 32, 16, "llvm.x86.avx512.mask.cvttps2dq.512", ""},
		{OpConvertToInt32Float32x4, "float", 32, 4, "int", 32, 4, "llvm.x86.sse2.cvttps2dq", "llvm.fptosi.sat.v4i32.v4f32"},
		{OpConvertToInt32Float32x8, "float", 32, 8, "int", 32, 8, "llvm.x86.avx.cvtt.ps2dq.256", ""},
		{OpConvertToInt32Float64x2, "float", 64, 2, "int", 32, 4, "llvm.x86.sse2.cvttpd2dq", ""},
		{OpConvertToInt32Float64x4, "float", 64, 4, "int", 32, 4, "llvm.x86.avx.cvtt.pd2dq.256", ""},
		{OpConvertToInt32Float64x8, "float", 64, 8, "int", 32, 8, "llvm.x86.avx512.mask.cvttpd2dq.512", ""},
		{OpConvertToInt64Float32x4, "float", 32, 4, "int", 64, 4, "llvm.x86.avx512.mask.cvttps2qq.256", ""},
		{OpConvertToInt64Float32x8, "float", 32, 8, "int", 64, 8, "llvm.x86.avx512.mask.cvttps2qq.512", ""},
		{OpConvertToInt64Float64x2, "float", 64, 2, "int", 64, 2, "llvm.x86.avx512.mask.cvttpd2qq.128", "llvm.fptosi.sat.v2i64.v2f64"},
		{OpConvertToInt64Float64x4, "float", 64, 4, "int", 64, 4, "llvm.x86.avx512.mask.cvttpd2qq.256", ""},
		{OpConvertToInt64Float64x8, "float", 64, 8, "int", 64, 8, "llvm.x86.avx512.mask.cvttpd2qq.512", ""},
		{OpConvertToUint32Float32x16, "float", 32, 16, "uint", 32, 16, "llvm.x86.avx512.mask.cvttps2udq.512", ""},
		{OpConvertToUint32Float32x4, "float", 32, 4, "uint", 32, 4, "llvm.x86.avx512.mask.cvttps2udq.128", "llvm.fptoui.sat.v4i32.v4f32"},
		{OpConvertToUint32Float32x8, "float", 32, 8, "uint", 32, 8, "llvm.x86.avx512.mask.cvttps2udq.256", ""},
		{OpConvertToUint32Float64x2, "float", 64, 2, "uint", 32, 4, "llvm.x86.avx512.mask.cvttpd2udq.128", ""},
		{OpConvertToUint32Float64x4, "float", 64, 4, "uint", 32, 4, "llvm.x86.avx512.mask.cvttpd2udq.256", ""},
		{OpConvertToUint32Float64x8, "float", 64, 8, "uint", 32, 8, "llvm.x86.avx512.mask.cvttpd2udq.512", ""},
		{OpConvertToUint64Float32x4, "float", 32, 4, "uint", 64, 4, "llvm.x86.avx512.mask.cvttps2uqq.256", ""},
		{OpConvertToUint64Float32x8, "float", 32, 8, "uint", 64, 8, "llvm.x86.avx512.mask.cvttps2uqq.512", ""},
		{OpConvertToUint64Float64x2, "float", 64, 2, "uint", 64, 2, "llvm.x86.avx512.mask.cvttpd2uqq.128", "llvm.fptoui.sat.v2i64.v2f64"},
		{OpConvertToUint64Float64x4, "float", 64, 4, "uint", 64, 4, "llvm.x86.avx512.mask.cvttpd2uqq.256", ""},
		{OpConvertToUint64Float64x8, "float", 64, 8, "uint", 64, 8, "llvm.x86.avx512.mask.cvttpd2uqq.512", ""},
	}
	seen := make(map[Op]bool)
	for _, test := range tests {
		seen[test.op] = true
		for _, arch := range []string{"amd64", "arm64"} {
			expected := test.amd64
			if arch == "arm64" {
				expected = test.arm64
			}
			if expected == "" {
				continue
			}
			for _, carrier := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/carrier=%v", test.op, arch, carrier), func(t *testing.T) {
					elem := func(kind string, bits int) *types.Type {
						kinds := map[string]map[int]types.Kind{
							"int":   {32: types.TINT32, 64: types.TINT64},
							"uint":  {32: types.TUINT32, 64: types.TUINT64},
							"float": {32: types.TFLOAT32, 64: types.TFLOAT64},
						}
						return types.Types[kinds[kind][bits]]
					}
					input := llvmTestSIMDType("float-conversion-input", elem(test.src, test.srcBits), int64(test.srcLanes))
					output := llvmTestSIMDType("float-conversion-output", elem(test.dst, test.dstBits), int64(test.dstLanes))
					resultType := getLLVMType(output)
					if carrier {
						vec := func(bits int) *types.Type {
							return map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[bits]
						}
						input, output = vec(test.srcBits*test.srcLanes), vec(test.dstBits*test.dstLanes)
					}
					module := GlobalCtxt.NewModule("float-conversion")
					CurrentModule = module
					builder := GlobalCtxt.NewBuilder()
					defer module.Dispose()
					defer builder.Dispose()
					function := llvm.AddFunction(module, "convert", llvm.FunctionType(resultType, []llvm.Type{getLLVMType(input)}, false))
					builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
					context := &LLVMFuncContext{
						F:  &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
						Vs: make(map[ID]llvm.Value), b: builder,
					}
					arg := &Value{ID: 1, Op: OpArg, Type: input}
					context.Vs[arg.ID] = function.Param(0)
					v := &Value{ID: 2, Op: test.op, Type: output, Args: []*Value{arg}}
					builder.CreateRet(context.GenLV(v))
					if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
						t.Fatalf("invalid floating conversion IR: %v\n%s", err, module.String())
					}
					ir := module.String()
					if !strings.Contains(ir, expected) {
						t.Errorf("missing %q\n%s", expected, ir)
					}
					if strings.HasPrefix(expected, "llvm.") {
						if strings.Contains(ir, " fptosi ") || strings.Contains(ir, " fptoui ") {
							t.Errorf("float-to-integer conversion must not expose a poison-producing cast\n%s", ir)
						}
					} else {
						lane := "i"
						if test.src == "float" {
							lane = "float"
							if test.srcBits == 64 {
								lane = "double"
							}
						} else {
							lane += fmt.Sprint(test.srcBits)
						}
						want := fmt.Sprintf("%s <%d x %s>", expected, min(test.srcLanes, test.dstLanes), lane)
						if !strings.Contains(ir, want) {
							t.Errorf("missing natural conversion shape %q\n%s", want, ir)
						}
					}
					if test.dstLanes < test.srcLanes && !strings.Contains(ir, "<i32 0, i32 1>") {
						t.Errorf("missing low-half selection\n%s", ir)
					}
					if test.dstLanes > test.srcLanes && !strings.HasPrefix(expected, "llvm.x86.") &&
						(!strings.Contains(ir, "shufflevector") || !strings.Contains(ir, "zeroinitializer")) {
						t.Errorf("missing zero high lanes\n%s", ir)
					}
					if strings.Contains(ir, " fast ") || strings.Contains(ir, " nnan ") || strings.Contains(ir, " nsz ") {
						t.Errorf("floating conversions must preserve strict value semantics\n%s", ir)
					}
				})
			}
		}
	}
	for op := Op(0); int(op) < len(goALLCSIMDOpcodeIndex); op++ {
		info, ok := goALLCSIMDInfo(op)
		if ok && info.lowering == goALLCSIMDLowerConvertFloat && !seen[op] {
			t.Errorf("missing generated floating conversion test for %s", op)
		}
	}
}
