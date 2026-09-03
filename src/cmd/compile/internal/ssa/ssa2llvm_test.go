// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"bytes"
	"cmd/compile/internal/abi"
	"fmt"
	"internal/buildcfg"
	"os"
	"strings"
	"testing"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"

	"github.com/goallc/go-llvm"
)

type llvmTestTypeName struct {
	sym *types.Sym
}

func (n *llvmTestTypeName) Sym() *types.Sym { return n.sym }
func (*llvmTestTypeName) Pos() src.XPos     { return src.NoXPos }
func (*llvmTestTypeName) Type() *types.Type { return nil }

func llvmTestSIMDType(name string, elem *types.Type, lanes int64) *types.Type {
	pkg := types.NewPkg("simd/archsimd", "archsimd")
	width := elem.Size() * lanes * 8
	tag := types.NewNamed(&llvmTestTypeName{sym: pkg.Lookup(fmt.Sprintf("v%d", width))})
	tag.SetUnderlying(types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, nil, types.NewArray(types.NewSignature(nil, nil, nil), 0)),
	}))
	types.CalcStructSize(tag)

	typ := types.NewNamed(&llvmTestTypeName{sym: pkg.Lookup(name)})
	typ.SetUnderlying(types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, pkg.Lookup("simd"), tag),
		types.NewField(src.NoXPos, pkg.Lookup("vals"), types.NewArray(elem, lanes)),
	}))
	types.CalcStructSize(typ)
	if !typ.IsSIMD() {
		panic(fmt.Sprintf("test type %v was not recognized as SIMD", typ))
	}
	return typ
}

func llvmTestIRVectorType(typ llvm.Type) string {
	if typ.TypeKind() != llvm.VectorTypeKind {
		panic(fmt.Sprintf("test type %v is not an LLVM vector", typ))
	}
	elem := typ.ElementType()
	var lane string
	switch elem.TypeKind() {
	case llvm.IntegerTypeKind:
		lane = fmt.Sprintf("i%d", elem.IntTypeWidth())
	case llvm.FloatTypeKind:
		lane = "float"
	case llvm.DoubleTypeKind:
		lane = "double"
	default:
		panic(fmt.Sprintf("unsupported LLVM test vector element %v", elem))
	}
	return fmt.Sprintf("<%d x %s>", typ.VectorSize(), lane)
}

func TestLLVMABICarrierPreservesNamedAggregateIdentity(t *testing.T) {
	pkg := types.NewPkg("runtime", "runtime")
	namedSlice := types.NewNamed(&llvmTestTypeName{sym: pkg.Lookup("slice")})
	namedSlice.SetUnderlying(types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, pkg.Lookup("array"), types.Types[types.TUNSAFEPTR]),
		types.NewField(src.NoXPos, pkg.Lookup("len"), types.Types[types.TINT]),
		types.NewField(src.NoXPos, pkg.Lookup("cap"), types.Types[types.TINT]),
	}))
	types.CalcSize(namedSlice)
	builtinSlice := types.NewSlice(types.Types[types.TUINT8])
	types.CalcSize(builtinSlice)

	if getLLVMType(namedSlice) == getLLVMType(builtinSlice) {
		t.Fatal("semantic LLVM types unexpectedly lost named aggregate identity")
	}
	if got, want := getLLVMABIType(namedSlice), getLLVMType(namedSlice); got != want {
		t.Fatalf("named ABI carrier = %v, want semantic type %v", got, want)
	}
	if got, other := getLLVMABIType(namedSlice), getLLVMABIType(builtinSlice); got == other {
		t.Fatalf("named ABI carrier unexpectedly collapsed to builtin carrier %v", got)
	}
}

func TestLLVMNaturalSIMDABIType(t *testing.T) {
	if !buildcfg.Experiment.SIMD {
		t.Skip("requires GOEXPERIMENT=simd")
	}

	oldTypes := type2lTypes
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
	}()

	for _, test := range []struct {
		name  string
		elem  *types.Type
		lanes int64
		kind  llvm.TypeKind
		bits  int
	}{
		{name: "int32x4", elem: types.Types[types.TINT32], lanes: 4, kind: llvm.IntegerTypeKind, bits: 32},
		{name: "float32x8", elem: types.Types[types.TFLOAT32], lanes: 8, kind: llvm.FloatTypeKind},
		{name: "float64x8", elem: types.Types[types.TFLOAT64], lanes: 8, kind: llvm.DoubleTypeKind},
	} {
		t.Run(test.name, func(t *testing.T) {
			typ := llvmTestSIMDType(test.name, test.elem, test.lanes)
			got := getLLVMType(typ)
			if got.TypeKind() != llvm.VectorTypeKind || got.VectorSize() != int(test.lanes) || got.ElementType().TypeKind() != test.kind {
				t.Fatalf("SIMD type = %v, want %d lanes of kind %v", got, test.lanes, test.kind)
			}
			if test.bits != 0 && got.ElementType().IntTypeWidth() != test.bits {
				t.Fatalf("SIMD element width = %d, want %d", got.ElementType().IntTypeWidth(), test.bits)
			}
			if abi := getLLVMABIType(typ); abi != got {
				t.Fatalf("SIMD ABI type = %v, want storage type %v", abi, got)
			}
		})
	}
}

func TestLLVMWidthOnlySIMDValuesKeepNaturalOperationTypes(t *testing.T) {
	if !buildcfg.Experiment.SIMD {
		t.Skip("requires GOEXPERIMENT=simd")
	}

	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	floatType := llvmTestSIMDType("float32x4", types.Types[types.TFLOAT32], 4)
	vectorType := getLLVMType(floatType)
	module := GlobalCtxt.NewModule("width_only_simd_values")
	CurrentModule = module
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	function := llvm.AddFunction(module, "chain", llvm.FunctionType(vectorType, []llvm.Type{vectorType, vectorType}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
	context := &LLVMFuncContext{
		F:  &Func{Config: &Config{arch: "arm64"}},
		Vs: make(map[ID]llvm.Value),
		b:  builder,
	}
	x := &Value{ID: 1, Op: OpArg, Type: floatType}
	y := &Value{ID: 2, Op: OpArg, Type: floatType}
	context.Vs[x.ID] = function.Param(0)
	context.Vs[y.ID] = function.Param(1)
	add := &Value{ID: 3, Op: OpAddFloat32x4, Type: types.TypeVec128, Args: []*Value{x, y}}
	mul := &Value{ID: 4, Op: OpMulFloat32x4, Type: types.TypeVec128, Args: []*Value{add, y}}
	builder.CreateRet(context.GenLV(mul))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected width-only SIMD values: %v\n%s", err, module.String())
	}
	ir := module.String()
	for _, want := range []string{"fadd <4 x float>", "fmul <4 x float>", "ret <4 x float>"} {
		if !strings.Contains(ir, want) {
			t.Errorf("LLVM IR does not contain %q\n%s", want, ir)
		}
	}
	if strings.Contains(ir, "bitcast ") {
		t.Fatalf("same-shape SIMD chain introduced a carrier bitcast\n%s", ir)
	}
}

func TestLLVMSIMDLaneViewsAreOnDemand(t *testing.T) {
	if !buildcfg.Experiment.SIMD {
		t.Skip("requires GOEXPERIMENT=simd")
	}

	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	floatType := llvmTestSIMDType("float32x4", types.Types[types.TFLOAT32], 4)
	intType := llvmTestSIMDType("int32x4", types.Types[types.TINT32], 4)
	floatVectorType := getLLVMType(floatType)
	intVectorType := getLLVMType(intType)
	module := GlobalCtxt.NewModule("on_demand_simd_lane_views")
	CurrentModule = module
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	function := llvm.AddFunction(module, "chain", llvm.FunctionType(intVectorType, []llvm.Type{floatVectorType, floatVectorType}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
	context := &LLVMFuncContext{
		F:  &Func{Config: &Config{arch: "arm64"}},
		Vs: make(map[ID]llvm.Value),
		b:  builder,
	}
	x := &Value{ID: 1, Op: OpArg, Type: floatType}
	y := &Value{ID: 2, Op: OpArg, Type: floatType}
	context.Vs[x.ID] = function.Param(0)
	context.Vs[y.ID] = function.Param(1)
	add := &Value{ID: 3, Op: OpAddInt32x4, Type: types.TypeVec128, Args: []*Value{x, y}}
	xor := &Value{ID: 4, Op: OpXorInt32x4, Type: types.TypeVec128, Args: []*Value{add, add}}
	builder.CreateRet(context.GenLV(xor))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected on-demand SIMD lane views: %v\n%s", err, module.String())
	}
	ir := module.String()
	for _, want := range []string{"add <4 x i32>", "xor <4 x i32>", "ret <4 x i32>"} {
		if !strings.Contains(ir, want) {
			t.Errorf("LLVM IR does not contain %q\n%s", want, ir)
		}
	}
	if got := strings.Count(ir, "bitcast <4 x float>"); got != 2 {
		t.Fatalf("lane reinterpretation emitted %d float-to-int views, want 2\n%s", got, ir)
	}
	if got := strings.Count(ir, "bitcast "); got != 2 {
		t.Fatalf("integer SIMD chain emitted %d total bitcasts after its two input views, want 2\n%s", got, ir)
	}
}

func TestLLVMABICarrierBridgesPromotedReceiverAtCaller(t *testing.T) {
	module := GlobalCtxt.NewModule("promoted_receiver_carrier")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	pointer := GlobalCtxt.PointerType(0)
	receiver := GlobalCtxt.StructCreateNamed("goallc.test.promoted.receiver")
	receiver.StructSetBody([]llvm.Type{pointer}, false)

	wrap := llvm.AddFunction(module, "wrap", llvm.FunctionType(receiver, []llvm.Type{pointer}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(wrap, "entry"))
	context := &LLVMFuncContext{b: builder}
	builder.CreateRet(context.reshapeLLVMValueToType(wrap.Param(0), receiver, "receiver"))

	unwrap := llvm.AddFunction(module, "unwrap", llvm.FunctionType(pointer, []llvm.Type{receiver}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(unwrap, "entry"))
	builder.CreateRet(context.reshapeLLVMValueToType(unwrap.Param(0), pointer, "receiver"))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected promoted receiver carrier bridge: %v\n%s", err, module.String())
	}
	ir := module.String()
	for _, want := range []string{
		"insertvalue %goallc.test.promoted.receiver undef, ptr %0, 0",
		"extractvalue %goallc.test.promoted.receiver %0, 0",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("promoted receiver bridge does not contain %q\n%s", want, ir)
		}
	}
}

func TestLLVMBuiltinDeclarationKeepsCallSiteSignatures(t *testing.T) {
	oldModule := CurrentModule
	module := GlobalCtxt.NewModule("builtin_call_signatures")
	CurrentModule = module
	t.Cleanup(func() {
		CurrentModule = oldModule
		module.Dispose()
	})

	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(builder.Dispose)
	caller := llvm.AddFunction(module, "caller", llvm.FunctionType(GlobalCtxt.VoidType(), nil, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(caller, "entry"))

	fields := []llvm.Type{GlobalCtxt.PointerType(0), GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type()}
	builtinSlice := llvm.StructType(fields, false)
	runtimeSlice := GlobalCtxt.StructCreateNamed("runtime.slice.call.signature")
	runtimeSlice.StructSetBody(fields, false)
	name, ok := goobj.BuiltinSymbolName("runtime.growslice", int(obj.ABIInternal))
	if !ok {
		t.Fatal("runtime.growslice is absent from the GoObj builtin table")
	}

	newSignature := func(result llvm.Type) llvmFuncSignature {
		return llvmFuncSignature{
			Type:                llvm.FunctionType(result, nil, false),
			ReturnType:          result,
			ResultCount:         1,
			ReturnCount:         1,
			ClosureContextIndex: -1,
		}
	}
	builtinSig := newSignature(builtinSlice)
	fn := getOrInsertLLVMFunction(name, builtinSig, goABIInternalCallConv)
	builtinCall := builder.CreateCall(builtinSig.Type, fn, nil, "builtin.call")
	builtinCall.SetInstructionCallConv(goABIInternalCallConv)

	runtimeSig := newSignature(runtimeSlice)
	fn = getOrInsertLLVMFunction(name, runtimeSig, goABIInternalCallConv)
	runtimeCall := builder.CreateCall(runtimeSig.Type, fn, nil, "runtime.call")
	runtimeCall.SetInstructionCallConv(goABIInternalCallConv)
	builder.CreateRetVoid()

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected builtin calls with distinct signatures: %v\n%s", err, module.String())
	}
}

func TestLLVMGoObjCompilerUsedOnlyKeepsExternalDataRoots(t *testing.T) {
	oldModule := CurrentModule
	oldLowerer := currentLLVMDataLowerer
	oldCompilerUsed := goObjCompilerUsed
	oldCompilerUsedNames := goObjCompilerUsedNames
	oldData := base.Ctxt.Data
	module := GlobalCtxt.NewModule("goobj_external_data_roots")
	CurrentModule = module
	currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
	t.Cleanup(func() {
		base.Ctxt.Data = oldData
		goObjCompilerUsedNames = oldCompilerUsedNames
		goObjCompilerUsed = oldCompilerUsed
		currentLLVMDataLowerer = oldLowerer
		CurrentModule = oldModule
		module.Dispose()
	})

	newLocalData := func(name string, value byte) *obj.LSym {
		s := &obj.LSym{Name: name, Type: objabi.SRODATA, Size: 1, P: []byte{value}}
		s.Set(obj.AttrLocal, true)
		return s
	}
	externalRoot := newLocalData("test.external.root", 1)
	ordinaryLocal := newLocalData("test.ordinary.local", 2)
	base.Ctxt.Data = []*obj.LSym{externalRoot, ordinaryLocal}
	MarkGoObjDataReferencedOutsideLLVM(externalRoot)
	LowerGoObjData()

	var used string
	for _, line := range strings.Split(module.String(), "\n") {
		if strings.HasPrefix(line, "@llvm.compiler.used =") {
			used = line
			break
		}
	}
	if used == "" {
		t.Fatalf("module has no llvm.compiler.used:\n%s", module.String())
	}
	if !strings.Contains(used, "@test.external.root") {
		t.Fatalf("external GoObj root is not compiler-used: %s", used)
	}
	if strings.Contains(used, "@test.ordinary.local") {
		t.Fatalf("ordinary local GoObj data is unnecessarily compiler-used: %s", used)
	}
}

func TestLLVMFileBackedDataLowering(t *testing.T) {
	payload := append(bytes.Repeat([]byte("file-backed-data-"), 80), []byte("GOALLC_FILE_END")...)
	path := t.TempDir() + "/payload"
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	s := &obj.LSym{Name: "test.file.backed", Type: objabi.SRODATA, Size: int64(len(payload))}
	file := s.NewFileInfo()
	file.Name = path
	file.Size = int64(len(payload))
	lowerer := newLLVMDataLowerer(map[*obj.LSym]bool{s: true})

	module := GlobalCtxt.NewModule("file_backed_data")
	t.Cleanup(module.Dispose)
	g := llvm.AddGlobal(module, lowerer.dataType(s), s.Name)
	g.SetInitializer(lowerer.dataInitializer(s, map[*obj.LSym]llvm.Value{s: g}))
	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected file-backed data: %v\n%s", err, module.String())
	}

	ir := module.String()
	if strings.Contains(ir, "zeroinitializer") {
		t.Fatalf("file-backed LLVM data was lowered as zeros:\n%s", ir)
	}
	if !strings.Contains(ir, "GOALLC_FILE_END") {
		t.Fatalf("file-backed LLVM data lost its payload:\n%s", ir)
	}
}

func TestReadLLVMFileDataRejectsLengthMismatches(t *testing.T) {
	path := t.TempDir() + "/payload"
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}

	newSymbol := func(symbolSize, fileSize int64) *obj.LSym {
		s := &obj.LSym{Name: "test.file.backed", Type: objabi.SRODATA, Size: symbolSize}
		file := s.NewFileInfo()
		file.Name = path
		file.Size = fileSize
		return s
	}
	for _, tc := range []struct {
		name       string
		symbolSize int64
		fileSize   int64
		want       string
	}{
		{name: "metadata", symbolSize: 5, fileSize: 4, want: "metadata length 4 does not match symbol size 5"},
		{name: "short", symbolSize: 6, fileSize: 6, want: "expected 6 bytes"},
		{name: "long", symbolSize: 4, fileSize: 4, want: "longer than expected 4 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readLLVMFileData(newSymbol(tc.symbolSize, tc.fileSize))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readLLVMFileData error = %v, want substring %q", err, tc.want)
			}
		})
	}

	s := newSymbol(5, 5)
	s.P = []byte("12345")
	if _, err := readLLVMFileData(s); err == nil || !strings.Contains(err.Error(), "also has 5 inline bytes") {
		t.Fatalf("readLLVMFileData inline/file conflict error = %v", err)
	}
}

func TestLLVMUntypedABI0FunctionAddressCreatesFunctionDeclaration(t *testing.T) {
	oldModule := CurrentModule
	oldLowerer := currentLLVMDataLowerer
	oldTarget := typecheck.Target
	module := GlobalCtxt.NewModule("abi0_function_address")
	CurrentModule = module
	currentLLVMDataLowerer = nil
	typecheck.Target = new(ir.Package)
	t.Cleanup(func() {
		typecheck.Target = oldTarget
		currentLLVMDataLowerer = oldLowerer
		CurrentModule = oldModule
		module.Dispose()
	})

	typ := llvm.FunctionType(GlobalCtxt.VoidType(), nil, false)
	internal := llvm.AddFunction(module, "runtime.asyncPreempt", typ)
	internal.SetFunctionCallConv(goABIInternalCallConv)

	pkg := types.NewPkg("runtime", "runtime")
	fn := ir.NewFunc(src.NoXPos, src.NoXPos, pkg.Lookup("asyncPreempt"), nil)
	fn.ABI = obj.ABI0
	typecheck.Target.Funcs = append(typecheck.Target.Funcs, fn)
	sym := fn.LinksymABI(fn.ABI)
	if sym.Type != objabi.Sxxx {
		t.Fatalf("test requires an unresolved bodyless LSym, got %v", sym.Type)
	}
	got := llvmGoDataRef(sym)
	if got.IsAFunction().IsNil() || got.Name() != "runtime.asyncPreempt<ABI0>" {
		t.Fatalf("ABI0 function address resolved to %q, want ABI0 function declaration", got.Name())
	}
}

func TestLLVMNamedAggregateConversionReshapesValue(t *testing.T) {
	module := GlobalCtxt.NewModule("named_aggregate_conversion")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	fields := []llvm.Type{GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type()}
	pageCache := GlobalCtxt.StructCreateNamed("runtime.pageCache")
	pageCache.StructSetBody(fields, false)
	exportedPageCache := GlobalCtxt.StructCreateNamed("runtime.PageCache")
	exportedPageCache.StructSetBody(fields, false)

	fn := llvm.AddFunction(module, "convert", llvm.FunctionType(exportedPageCache, []llvm.Type{pageCache}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
	context := &LLVMFuncContext{b: builder}
	builder.CreateRet(context.reshapeLLVMValueToType(fn.Param(0), exportedPageCache, "pagecache"))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected named aggregate reshape: %v\n%s", err, module.String())
	}
}

func TestLLVMJumpTableDefaultIsUnreachable(t *testing.T) {
	module := GlobalCtxt.NewModule("jump_table_default")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	i64 := GlobalCtxt.Int64Type()
	function := llvm.AddFunction(module, "jump_table_default", llvm.FunctionType(i64, []llvm.Type{i64}, false))
	jumpLLVM := llvm.AddBasicBlock(function, "jump")
	mergeLLVM := llvm.AddBasicBlock(function, "merge")
	otherLLVM := llvm.AddBasicBlock(function, "other")

	jump := &Block{ID: 1, Kind: BlockJumpTable}
	merge := &Block{ID: 2}
	other := &Block{ID: 3}
	control := &Value{ID: 1, Type: types.Types[types.TINT]}
	jump.Controls[0] = control
	jump.Succs = []Edge{{b: merge}, {b: merge}, {b: other}}
	context := &LLVMFuncContext{
		BBs: map[ID]llvm.BasicBlock{
			jump.ID:  jumpLLVM,
			merge.ID: mergeLLVM,
			other.ID: otherLLVM,
		},
		Vs: map[ID]llvm.Value{control.ID: function.Param(0)},
		LF: function,
		b:  builder,
	}
	context.CompileBlock(jump, nil)

	builder.SetInsertPointAtEnd(mergeLLVM)
	phi := builder.CreatePHI(i64, "carried")
	seven := llvm.ConstInt(i64, 7, false)
	phi.AddIncoming([]llvm.Value{seven, seven}, []llvm.BasicBlock{jumpLLVM, jumpLLVM})
	builder.CreateRet(phi)
	builder.SetInsertPointAtEnd(otherLLVM)
	builder.CreateRet(llvm.ConstInt(i64, 9, false))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("jump table added a non-SSA default edge: %v\n%s", err, module.String())
	}
	if ir := module.String(); !strings.Contains(ir, "b1.jump.default") || !strings.Contains(ir, "unreachable") {
		t.Fatalf("jump table has no unreachable default block\n%s", ir)
	}
}

func TestLLVMCurrentGRegister(t *testing.T) {
	for _, test := range []struct {
		name     string
		arch     string
		abi      obj.ABI
		register string
		ok       bool
	}{
		{"amd64 ABIInternal", "amd64", obj.ABIInternal, "r14", true},
		{"amd64 ABI0", "amd64", obj.ABI0, "", false},
		{"arm64 ABIInternal", "arm64", obj.ABIInternal, "x28", true},
		{"arm64 ABI0", "arm64", obj.ABI0, "x28", true},
		{"unsupported target", "riscv64", obj.ABIInternal, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			register, ok := llvmCurrentGRegister(test.arch, test.abi)
			if register != test.register || ok != test.ok {
				t.Fatalf("llvmCurrentGRegister(%q, %v) = (%q, %v), want (%q, %v)", test.arch, test.abi, register, ok, test.register, test.ok)
			}
		})
	}
}

func TestLLVMInterfaceTypeWordIsInteger(t *testing.T) {
	interfaceType := getLLVMType(types.Types[types.TINTER])
	if interfaceType.TypeKind() != llvm.StructTypeKind {
		t.Fatalf("LLVM interface type kind = %s, want struct", interfaceType.TypeKind())
	}
	fields := interfaceType.StructElementTypes()
	if len(fields) != 2 {
		t.Fatalf("LLVM interface field count = %d, want 2", len(fields))
	}
	if fields[0].TypeKind() != llvm.IntegerTypeKind || fields[0].IntTypeWidth() != types.PtrSize*8 {
		t.Fatalf("LLVM interface type word = %s, want pointer-sized integer", fields[0])
	}
	if fields[1].TypeKind() != llvm.PointerTypeKind {
		t.Fatalf("LLVM interface data word = %s, want pointer", fields[1])
	}
}

func TestLLVMNotInHeapAddressCarriers(t *testing.T) {
	nih := types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, nil, types.Types[types.TUINTPTR]),
	})
	nih.SetNotInHeap(true)
	types.CalcSize(nih)

	nihPointer := types.NewPtr(nih)
	if got := getLLVMType(nihPointer); got.TypeKind() != llvm.IntegerTypeKind || got.IntTypeWidth() != types.PtrSize*8 {
		t.Fatalf("LLVM *NotInHeap carrier = %s, want pointer-sized integer", got)
	}

	ordinaryPointer := types.NewPtr(types.Types[types.TUINTPTR])
	if got := getLLVMType(ordinaryPointer); got.TypeKind() != llvm.PointerTypeKind {
		t.Fatalf("LLVM ordinary pointer carrier = %s, want pointer", got)
	}
	if got := getLLVMType(types.NewPtr(nihPointer)); got.TypeKind() != llvm.PointerTypeKind {
		t.Fatalf("LLVM **NotInHeap carrier = %s, want pointer", got)
	}

	nihSlice := getLLVMType(types.NewSlice(nih))
	if got := nihSlice.StructElementTypes()[0]; got.TypeKind() != llvm.IntegerTypeKind || got.IntTypeWidth() != types.PtrSize*8 {
		t.Fatalf("LLVM []NotInHeap data carrier = %s, want pointer-sized integer", got)
	}
	ordinarySlice := getLLVMType(types.NewSlice(types.Types[types.TUINTPTR]))
	if got := ordinarySlice.StructElementTypes()[0]; got.TypeKind() != llvm.PointerTypeKind {
		t.Fatalf("LLVM ordinary slice data carrier = %s, want pointer", got)
	}

	oldModule := CurrentModule
	module := GlobalCtxt.NewModule("not_in_heap_address_materialization")
	CurrentModule = module
	t.Cleanup(func() {
		CurrentModule = oldModule
		module.Dispose()
	})
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(builder.Dispose)
	uintptrType := getLLVMType(types.Types[types.TUINTPTR])
	pointerType := GlobalCtxt.PointerType(0)
	fn := llvm.AddFunction(module, "materialize", llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{uintptrType, uintptrType, pointerType}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
	context := &LLVMFuncContext{b: builder}
	context.materializeAddressPointer(fn.Param(0), nihPointer, pointerType, "nih")
	context.materializeAddressPointer(fn.Param(1), types.Types[types.TUINTPTR], pointerType, "ordinary")
	context.reshapeLLVMValue(&Value{ID: 1}, fn.Param(0), nihPointer, ordinaryPointer, "nih.reshape")
	context.reshapeLLVMValue(&Value{ID: 2}, fn.Param(2), ordinaryPointer, nihPointer, "nih.observe")
	nilPointer := context.materializeAddressPointer(llvm.ConstNull(uintptrType), nihPointer, pointerType, "nil")
	if nilPointer.IsAConstantPointerNull().IsNil() {
		t.Fatalf("LLVM nil *NotInHeap materialization = %s, want pointer constant", nilPointer)
	}
	builder.CreateRetVoid()

	nilcheck := llvm.AddFunction(module, "nilcheck", llvm.FunctionType(pointerType, []llvm.Type{uintptrType}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(nilcheck, "entry"))
	arg := &Value{ID: 3, Op: OpArg, Type: nihPointer}
	memory := &Value{ID: 4, Op: OpInitMem, Type: types.TypeMem}
	check := &Value{ID: 5, Op: OpNilCheck, Type: ordinaryPointer, Args: []*Value{arg, memory}}
	context = &LLVMFuncContext{
		b:  builder,
		Vs: map[ID]llvm.Value{arg.ID: nilcheck.Param(0)},
	}
	builder.CreateRet(context.emitNilCheckIntrinsic(check))

	ir := module.String()
	if !strings.Contains(ir, "%nih = inttoptr") || !strings.Contains(ir, "!goallc.notinheap") {
		t.Fatalf("LLVM *NotInHeap materialization does not use marked direct inttoptr\n%s", ir)
	}
	if !strings.Contains(ir, "%ordinary = call ptr @llvm.go.pointer.from.address") {
		t.Fatalf("LLVM ordinary address materialization does not retain pointer.from.address marker\n%s", ir)
	}
	if !strings.Contains(ir, "%nih.reshape = inttoptr") || !strings.Contains(ir, "%nih.observe = call i64 @llvm.go.pointer.address") {
		t.Fatalf("LLVM *NotInHeap semantic reshaping lost its address provenance\n%s", ir)
	}
	if !strings.Contains(ir, "%v5.ptr = inttoptr") || !strings.Contains(ir, "call void @llvm.goallc.nilcheck(ptr %v5.ptr)") || !strings.Contains(ir, "ret ptr %v5.ptr") {
		t.Fatalf("LLVM nil check did not adapt its *NotInHeap result carrier\n%s", ir)
	}
}

func TestLLVMCanEmitMustTail(t *testing.T) {
	internalConfig := abi.NewABIConfig(16, 16, 0, uint8(obj.ABIInternal))
	abi0Config := abi.NewABIConfig(0, 0, 0, uint8(obj.ABI0))
	aux := func(config *abi.ABIConfig, fn *obj.LSym, params, results []*types.Type) *AuxCall {
		return StaticAuxCall(fn, config.ABIAnalyzeTypes(params, results))
	}
	internalZero := aux(internalConfig, &obj.LSym{}, nil, nil)
	abi0Zero := aux(abi0Config, &obj.LSym{}, nil, nil)
	oneArg := aux(internalConfig, &obj.LSym{}, []*types.Type{types.Types[types.TINT]}, nil)
	oneResult := aux(internalConfig, &obj.LSym{}, nil, []*types.Type{types.Types[types.TINT]})

	context := &LLVMFuncContext{F: &Func{OwnAux: internalZero}}
	for _, test := range []struct {
		name string
		call *Value
		aux  *AuxCall
		own  *AuxCall
		want bool
	}{
		{"same ABI", &Value{Op: OpTailLECall}, internalZero, internalZero, true},
		{"ABI0 to internal", &Value{Op: OpTailLECall}, internalZero, abi0Zero, true},
		{"internal to ABI0", &Value{Op: OpTailLECall}, abi0Zero, internalZero, false},
		{"non-tail call", &Value{Op: OpStaticLECall}, internalZero, internalZero, false},
		{"indirect target", &Value{Op: OpTailLECall}, aux(internalConfig, nil, nil, nil), internalZero, false},
		{"callee argument", &Value{Op: OpTailLECall}, oneArg, internalZero, false},
		{"callee result", &Value{Op: OpTailLECall}, oneResult, internalZero, false},
		{"caller argument", &Value{Op: OpTailLECall}, internalZero, oneArg, false},
		{"caller result", &Value{Op: OpTailLECall}, internalZero, oneResult, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			context.F.OwnAux = test.own
			if got := context.llvmCanEmitMustTail(test.call, test.aux); got != test.want {
				t.Fatalf("llvmCanEmitMustTail() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLLVMFunctionStorageName(t *testing.T) {
	morestack, ok := goobj.BuiltinSymbolName("runtime.morestack", int(obj.ABI0))
	if !ok {
		t.Fatal("runtime.morestack ABI0 is absent from GoObj builtin table")
	}
	for _, test := range []struct {
		name string
		cc   llvm.CallConv
		want string
	}{
		{"runtime.morestack", goABI0CallConv, "runtime.morestack<ABI0>"},
		{"runtime.morestack", goABIInternalCallConv, "runtime.morestack"},
		{morestack, goABI0CallConv, morestack + "<ABI0>"},
	} {
		if got := llvmFunctionStorageName(test.name, test.cc); got != test.want {
			t.Errorf("llvmFunctionStorageName(%q, %d) = %q, want %q", test.name, test.cc, got, test.want)
		}
	}
}

func TestLLVMGoObjReferenceNames(t *testing.T) {
	oldLinkshared := base.Ctxt.Flag_linkshared
	oldLocalDefinitions := llvmGoObjLocalDefinitions
	base.Ctxt.Flag_linkshared = false
	llvmGoObjLocalDefinitions = make(map[llvmGoObjSymbolKey]bool)
	t.Cleanup(func() {
		base.Ctxt.Flag_linkshared = oldLinkshared
		llvmGoObjLocalDefinitions = oldLocalDefinitions
	})

	builtin := base.Ctxt.LookupABI("runtime.panicdivide", obj.ABIInternal)
	wantBuiltin, ok := goobj.BuiltinSymbolName(builtin.Name, int(builtin.ABI()))
	if !ok {
		t.Fatal("runtime.panicdivide is absent from GoObj builtin table")
	}
	if got := llvmGoObjReferenceName(builtin); got != wantBuiltin {
		t.Fatalf("builtin reference name = %q, want %q", got, wantBuiltin)
	}

	// Builtin and linkname are mutually exclusive reference encodings. The
	// builtin table wins when an implementation also carries a linkname bit.
	oldBuiltinLinkname := builtin.IsLinkname()
	builtin.Set(obj.AttrLinkname, true)
	t.Cleanup(func() { builtin.Set(obj.AttrLinkname, oldBuiltinLinkname) })
	if got := llvmGoObjReferenceName(builtin); got != wantBuiltin {
		t.Fatalf("linknamed builtin reference name = %q, want %q", got, wantBuiltin)
	}

	linkname := base.Ctxt.LookupABI("runtime.llvmLinknamePull", obj.ABIInternal)
	oldLinkname := linkname.IsLinkname()
	linkname.Set(obj.AttrLinkname, true)
	t.Cleanup(func() { linkname.Set(obj.AttrLinkname, oldLinkname) })
	if got, want := llvmGoObjReferenceName(linkname), linkname.Name+goobj.LinknameSymbolSuffix; got != want {
		t.Fatalf("linkname pull name = %q, want %q", got, want)
	}
	llvmGoObjLocalDefinitions[llvmGoObjSymbolKeyFor(linkname)] = true
	if got := llvmGoObjReferenceName(linkname); got != linkname.Name {
		t.Fatalf("local linkname definition name = %q, want %q", got, linkname.Name)
	}

	linknameStd := base.Ctxt.LookupABI("runtime.llvmLinknameStdPull", obj.ABIInternal)
	oldLinknameStd := linknameStd.IsLinknameStd()
	linknameStd.Set(obj.AttrLinknameStd, true)
	t.Cleanup(func() { linknameStd.Set(obj.AttrLinknameStd, oldLinknameStd) })
	if got, want := llvmGoObjReferenceName(linknameStd), linknameStd.Name+goobj.LinknameSymbolSuffix; got != want {
		t.Fatalf("linknamestd pull name = %q, want %q", got, want)
	}

	builtin.Set(obj.AttrLinkname, oldBuiltinLinkname)
	base.Ctxt.Flag_linkshared = true
	for _, s := range []*obj.LSym{builtin, linkname, linknameStd} {
		if got := llvmGoObjReferenceName(s); got != s.Name {
			t.Fatalf("linkshared reference name = %q, want %q", got, s.Name)
		}
	}
}

func TestEmitLateGoObjBuiltinDeclarations(t *testing.T) {
	oldModule := CurrentModule
	oldCompilerUsed := goObjCompilerUsed
	oldCompilerUsedNames := goObjCompilerUsedNames
	oldLinkshared := base.Ctxt.Flag_linkshared
	module := GlobalCtxt.NewModule("late_goobj_builtins")
	CurrentModule = module
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
	base.Ctxt.Flag_linkshared = false
	t.Cleanup(func() {
		base.Ctxt.Flag_linkshared = oldLinkshared
		goObjCompilerUsedNames = oldCompilerUsedNames
		goObjCompilerUsed = oldCompilerUsed
		CurrentModule = oldModule
		module.Dispose()
	})

	emitLateGoObjBuiltinDeclarations()
	lateCount := 0
	for i := 0; i < goobj.NBuiltin(); i++ {
		if !goobj.BuiltinIsLate(i) {
			continue
		}
		lateCount++
		name, abi := goobj.BuiltinName(i)
		storageName, ok := goobj.BuiltinSymbolName(name, abi)
		if !ok {
			t.Fatalf("late builtin %s has no encoded name", name)
		}
		storageName = llvmFunctionStorageName(storageName, llvmCallConv(obj.ABI(abi)))
		if fn := module.NamedFunction(storageName); fn.IsNil() {
			t.Errorf("late builtin declaration %q is absent", storageName)
		}
	}
	if got := len(goObjCompilerUsed); got != lateCount {
		t.Fatalf("compiler-used late builtin count = %d, want %d", got, lateCount)
	}
	memmove, ok := goobj.BuiltinSymbolName("runtime.memmove", int(obj.ABIInternal))
	if !ok {
		t.Fatal("runtime.memmove is absent from GoObj builtin table")
	}
	if fn := module.NamedFunction(memmove); !fn.IsNil() {
		t.Fatalf("ordinary builtin %q was declared eagerly", memmove)
	}
}

func TestLLVMGoObjImportsDeduplicateMidwayImports(t *testing.T) {
	simdFingerprint := goobj.FingerprintType{1, 2, 3, 4, 5, 6, 7, 8}
	bridgeFingerprint := goobj.FingerprintType{8, 7, 6, 5, 4, 3, 2, 1}
	imports := []goobj.ImportedPkg{
		{Pkg: "simd", Fingerprint: simdFingerprint},
		{Pkg: "simd/internal/bridge", Fingerprint: bridgeFingerprint},
		{Pkg: "simd", Fingerprint: simdFingerprint},
	}

	got, err := llvmGoObjImports(imports)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("unique import count = %d, want 2", len(got))
	}
	if got[0] != imports[0] || got[1] != imports[1] {
		t.Fatalf("unique imports = %v, want first-seen order %v", got, imports[:2])
	}

	imports[2].Fingerprint = bridgeFingerprint
	if _, err := llvmGoObjImports(imports); err == nil {
		t.Fatal("conflicting duplicate import was accepted")
	}
}

func TestLLVMAMD64MapPackedByteLowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = map[*types.Type]llvm.Type{
		types.Types[types.TUINT8]:  GlobalCtxt.Int8Type(),
		types.Types[types.TUINT64]: GlobalCtxt.Int64Type(),
		types.TypeInt128:           llvmAMD64ByteVectorType(),
	}
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	newContext := func(t *testing.T, name string, parameterCount int) (llvm.Module, *LLVMFuncContext, []*Value) {
		t.Helper()
		module := GlobalCtxt.NewModule(name)
		CurrentModule = module
		parameters := make([]llvm.Type, parameterCount)
		for i := range parameters {
			parameters[i] = GlobalCtxt.Int64Type()
		}
		function := llvm.AddFunction(module, name, llvm.FunctionType(GlobalCtxt.Int64Type(), parameters, false))
		block := llvm.AddBasicBlock(function, "entry")
		builder := GlobalCtxt.NewBuilder()
		t.Cleanup(module.Dispose)
		t.Cleanup(builder.Dispose)
		builder.SetInsertPointAtEnd(block)
		context := &LLVMFuncContext{
			Vs: make(map[ID]llvm.Value),
			b:  builder,
		}
		arguments := make([]*Value, parameterCount)
		for i := range arguments {
			arguments[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: types.Types[types.TUINT64]}
			context.Vs[arguments[i].ID] = function.Param(i)
		}
		return module, context, arguments
	}

	t.Run("GOAMD64v1", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v1", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		unpacked := &Value{ID: 5, Op: OpAMD64PUNPCKLBW, Type: types.TypeInt128, Args: []*Value{h2, h2}}
		broadcast := &Value{ID: 6, Op: OpAMD64PSHUFLW, Type: types.TypeInt128, AuxInt: 0, Args: []*Value{unpacked}}
		equal := &Value{ID: 7, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		mask := &Value{ID: 8, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT8], Args: []*Value{equal}}
		result := &Value{ID: 9, Op: OpZeroExt8to64, Type: types.Types[types.TUINT64], Args: []*Value{mask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v1 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"bitcast i64 %0 to <8 x i8>",
			"shufflevector <8 x i8>",
			"shufflevector <16 x i8>",
			"bitcast <16 x i8>",
			"icmp eq <16 x i8>",
			"sext <16 x i1>",
			"call i32 @llvm.x86.sse2.pmovmskb.128(<16 x i8>",
			"trunc i32",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v1 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v2", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v2", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		broadcast := &Value{ID: 5, Op: OpAMD64PSHUFBbroadcast, Type: types.TypeInt128, Args: []*Value{h2}}
		equal := &Value{ID: 6, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		equalMask := &Value{ID: 7, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{equal}}
		signed := &Value{ID: 8, Op: OpAMD64PSIGNB, Type: types.TypeInt128, Args: []*Value{group, group}}
		signMask := &Value{ID: 9, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{signed}}
		result := &Value{ID: 10, Op: OpOr64, Type: types.Types[types.TUINT64], Args: []*Value{equalMask, signMask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v2 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"insertelement <16 x i8>",
			"icmp sgt <16 x i8>",
			"icmp slt <16 x i8>",
			"select <16 x i1>",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v2 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v4", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v4", 1)
		broadcast := &Value{ID: 2, Op: OpAMD64VPBROADCASTB, Type: types.TypeInt128, Args: []*Value{args[0]}}
		mask := &Value{ID: 3, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{broadcast}}
		context.b.CreateRet(context.GenLV(mask))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v4 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		if !strings.Contains(ir, "trunc i64 %0 to i8") || !strings.Contains(ir, "shufflevector <16 x i8>") {
			t.Errorf("v4 LLVM IR does not broadcast the low input byte\n%s", ir)
		}
	})
}

func TestLLVMGenericVec128Lowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	for _, test := range []struct {
		name  string
		elem  *types.Type
		lanes int64
		op    Op
		wants []string
	}{
		{"add-int8", types.Types[types.TINT8], 16, OpAddInt8x16, []string{"add <16 x i8>"}},
		{"add-int16", types.Types[types.TINT16], 8, OpAddInt16x8, []string{"add <8 x i16>"}},
		{"add-int32", types.Types[types.TINT32], 4, OpAddInt32x4, []string{"add <4 x i32>"}},
		{"add-int64", types.Types[types.TINT64], 2, OpAddInt64x2, []string{"add <2 x i64>"}},
		{"add-float32", types.Types[types.TFLOAT32], 4, OpAddFloat32x4, []string{"fadd <4 x float>"}},
		{"add-float64", types.Types[types.TFLOAT64], 2, OpAddFloat64x2, []string{"fadd <2 x double>"}},
		{"sub-int16", types.Types[types.TUINT16], 8, OpSubUint16x8, []string{"sub <8 x i16>"}},
		{"sub-float64", types.Types[types.TFLOAT64], 2, OpSubFloat64x2, []string{"fsub <2 x double>"}},
		{"sat-add-int8", types.Types[types.TINT8], 16, OpAddSaturatedInt8x16, []string{"call <16 x i8> @llvm.sadd.sat.v16i8"}},
		{"sat-add-uint32", types.Types[types.TUINT32], 4, OpAddSaturatedUint32x4, []string{"call <4 x i32> @llvm.uadd.sat.v4i32"}},
		{"sat-sub-int64", types.Types[types.TINT64], 2, OpSubSaturatedInt64x2, []string{"call <2 x i64> @llvm.ssub.sat.v2i64"}},
		{"sat-sub-uint16", types.Types[types.TUINT16], 8, OpSubSaturatedUint16x8, []string{"call <8 x i16> @llvm.usub.sat.v8i16"}},
		{"mul-int8", types.Types[types.TINT8], 16, OpMulInt8x16, []string{"mul <16 x i8>"}},
		{"mul-float32", types.Types[types.TFLOAT32], 4, OpMulFloat32x4, []string{"fmul <4 x float>"}},
		{"div-float64", types.Types[types.TFLOAT64], 2, OpDivFloat64x2, []string{"fdiv <2 x double>"}},
		{"and", types.Types[types.TINT64], 2, OpAndInt64x2, []string{"and <2 x i64>"}},
		{"or", types.Types[types.TUINT32], 4, OpOrUint32x4, []string{"or <4 x i32>"}},
		{"xor", types.Types[types.TINT16], 8, OpXorInt16x8, []string{"xor <8 x i16>"}},
		{"and-not", types.Types[types.TUINT8], 16, OpAndNotUint8x16, []string{"xor <16 x i8>", "and <16 x i8>"}},
		{"or-not", types.Types[types.TINT32], 4, OpOrNotInt32x4, []string{"xor <4 x i32>", "or <4 x i32>"}},
		{"max-signed", types.Types[types.TINT16], 8, OpMaxInt16x8, []string{"call <8 x i16> @llvm.smax.v8i16"}},
		{"min-unsigned", types.Types[types.TUINT32], 4, OpMinUint32x4, []string{"call <4 x i32> @llvm.umin.v4i32"}},
		{"max-float32", types.Types[types.TFLOAT32], 4, OpMaxFloat32x4, []string{"call <4 x float> @llvm.maximum.v4f32"}},
		{"min-float64", types.Types[types.TFLOAT64], 2, OpMinFloat64x2, []string{"call <2 x double> @llvm.minimum.v2f64"}},
		{"average-signed", types.Types[types.TINT8], 16, OpAverageInt8x16, []string{"xor <16 x i8>", "ashr <16 x i8>", "or <16 x i8>", "sub <16 x i8>"}},
		{"average-unsigned", types.Types[types.TUINT16], 8, OpAverageUint16x8, []string{"xor <8 x i16>", "lshr <8 x i16>", "or <8 x i16>", "sub <8 x i16>"}},
		{"mul-high-signed", types.Types[types.TINT16], 8, OpMulHighInt16x8, []string{"sext <8 x i16>", "mul <8 x i32>", "ashr <8 x i32>", "trunc <8 x i32>"}},
		{"mul-high-unsigned", types.Types[types.TUINT16], 8, OpMulHighUint16x8, []string{"zext <8 x i16>", "mul <8 x i32>", "lshr <8 x i32>", "trunc <8 x i32>"}},
		{"mul-sign", types.Types[types.TINT32], 4, OpMulSignInt32x4, []string{"call <4 x i32> @llvm.x86.ssse3.psign.d.128"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			arch := "arm64"
			var entry *Block
			if strings.Contains(test.name, "mul-sign") {
				arch = "amd64"
				entry = &Block{CPUfeatures: CPUavx}
			}
			typ := llvmTestSIMDType(test.name, test.elem, test.lanes)
			vectorType := getLLVMType(typ)
			module := GlobalCtxt.NewModule("generic_vec128_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(vectorType, []llvm.Type{vectorType, vectorType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: arch}, Entry: entry},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: typ}
			y := &Value{ID: 2, Op: OpArg, Type: typ}
			context.Vs[x.ID] = function.Param(0)
			context.Vs[y.ID] = function.Param(1)
			result := &Value{ID: 3, Op: test.op, Type: typ, Args: []*Value{x, y}}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected generic Vec128 lowering: %v\n%s", err, module.String())
			}
			ir := module.String()
			for _, want := range append(test.wants, "ret "+llvmTestIRVectorType(vectorType)) {
				if !strings.Contains(ir, want) {
					t.Errorf("generic Vec128 IR does not contain %q\n%s", want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("generic Vec128 lowering introduced a carrier bitcast\n%s", ir)
			}
		})
	}

	t.Run("amd64-andnot-operand-order", func(t *testing.T) {
		typ := llvmTestSIMDType("andnot-int8", types.Types[types.TINT8], 16)
		vectorType := getLLVMType(typ)
		module := GlobalCtxt.NewModule("generic_vec128_amd64_andnot")
		CurrentModule = module
		builder := GlobalCtxt.NewBuilder()
		t.Cleanup(module.Dispose)
		t.Cleanup(builder.Dispose)

		function := llvm.AddFunction(module, "andnot", llvm.FunctionType(vectorType, []llvm.Type{vectorType, vectorType}, false))
		builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
		context := &LLVMFuncContext{
			F:  &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx}},
			Vs: make(map[ID]llvm.Value),
			b:  builder,
		}
		// simdgen's AMD64 intrinsic table passes method y before method x so
		// VPANDN receives the order required by that target instruction.
		methodY := &Value{ID: 1, Op: OpArg, Type: typ}
		methodX := &Value{ID: 2, Op: OpArg, Type: typ}
		context.Vs[methodY.ID] = function.Param(0)
		context.Vs[methodX.ID] = function.Param(1)
		result := &Value{ID: 3, Op: OpAndNotInt8x16, Type: typ, Args: []*Value{methodY, methodX}}
		builder.CreateRet(context.GenLV(result))

		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected AMD64 AndNot lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		if !strings.Contains(ir, "xor <16 x i8> %0") || !strings.Contains(ir, "and <16 x i8> %1") {
			t.Fatalf("AMD64 AndNot did not recover method-level x &^ y order\n%s", ir)
		}
	})

	for _, test := range []struct {
		name  string
		elem  *types.Type
		lanes int64
		op    Op
		wants []string
	}{
		{"not", types.Types[types.TUINT64], 2, OpNotUint64x2, []string{"xor <2 x i64>"}},
		{"neg-int32", types.Types[types.TINT32], 4, OpNegInt32x4, []string{"sub <4 x i32> zeroinitializer"}},
		{"neg-float32", types.Types[types.TFLOAT32], 4, OpNegFloat32x4, []string{"fneg <4 x float>"}},
		{"abs-int64", types.Types[types.TINT64], 2, OpAbsInt64x2, []string{"icmp slt <2 x i64>", "sub <2 x i64>", "select <2 x i1>"}},
		{"abs-float64", types.Types[types.TFLOAT64], 2, OpAbsFloat64x2, []string{"call <2 x double> @llvm.fabs.v2f64"}},
		{"sqrt-float32", types.Types[types.TFLOAT32], 4, OpSqrtFloat32x4, []string{"call <4 x float> @llvm.sqrt.v4f32"}},
		{"round-even-float64", types.Types[types.TFLOAT64], 2, OpRoundFloat64x2, []string{"call <2 x double> @llvm.roundeven.v2f64"}},
		{"floor-float32", types.Types[types.TFLOAT32], 4, OpFloorFloat32x4, []string{"call <4 x float> @llvm.floor.v4f32"}},
		{"ceil-float64", types.Types[types.TFLOAT64], 2, OpCeilFloat64x2, []string{"call <2 x double> @llvm.ceil.v2f64"}},
		{"trunc-float32", types.Types[types.TFLOAT32], 4, OpTruncFloat32x4, []string{"call <4 x float> @llvm.trunc.v4f32"}},
		{"ones-count-int8", types.Types[types.TINT8], 16, OpOnesCountInt8x16, []string{"call <16 x i8> @llvm.ctpop.v16i8"}},
		{"leading-zeros-uint32", types.Types[types.TUINT32], 4, OpLeadingZerosUint32x4, []string{"call <4 x i32> @llvm.ctlz.v4i32(<4 x i32> %0, i1 false)"}},
		{"leading-sign-bits", types.Types[types.TINT16], 8, OpLeadingSignBitsInt16x8, []string{"ashr <8 x i16>", "xor <8 x i16>", "call <8 x i16> @llvm.ctlz.v8i16", "sub <8 x i16>"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			typ := llvmTestSIMDType(test.name, test.elem, test.lanes)
			vectorType := getLLVMType(typ)
			module := GlobalCtxt.NewModule("generic_vec128_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(vectorType, []llvm.Type{vectorType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "arm64"}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: typ}
			context.Vs[x.ID] = function.Param(0)
			result := &Value{ID: 2, Op: test.op, Type: typ, Args: []*Value{x}}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected generic Vec128 lowering: %v\n%s", err, module.String())
			}
			ir := module.String()
			for _, want := range append(test.wants, "ret "+llvmTestIRVectorType(vectorType)) {
				if !strings.Contains(ir, want) {
					t.Errorf("generic Vec128 IR does not contain %q\n%s", want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("generic Vec128 lowering introduced a carrier bitcast\n%s", ir)
			}
		})
	}

	for _, test := range []struct {
		name     string
		elem     *types.Type
		maskElem *types.Type
		lanes    int64
		op       Op
		wants    []string
	}{
		{"equal-int8", types.Types[types.TINT8], types.Types[types.TINT8], 16, OpEqualInt8x16, []string{"icmp eq <16 x i8>", "sext <16 x i1>"}},
		{"greater-signed", types.Types[types.TINT16], types.Types[types.TINT16], 8, OpGreaterInt16x8, []string{"icmp sgt <8 x i16>", "sext <8 x i1>"}},
		{"greater-unsigned", types.Types[types.TUINT32], types.Types[types.TINT32], 4, OpGreaterUint32x4, []string{"icmp ugt <4 x i32>", "sext <4 x i1>"}},
		{"greater-equal-signed", types.Types[types.TINT64], types.Types[types.TINT64], 2, OpGreaterEqualInt64x2, []string{"icmp sge <2 x i64>", "sext <2 x i1>"}},
		{"greater-equal-unsigned", types.Types[types.TUINT8], types.Types[types.TINT8], 16, OpGreaterEqualUint8x16, []string{"icmp uge <16 x i8>", "sext <16 x i1>"}},
		{"equal-float32", types.Types[types.TFLOAT32], types.Types[types.TINT32], 4, OpEqualFloat32x4, []string{"fcmp oeq <4 x float>", "sext <4 x i1>"}},
		{"not-equal-float64", types.Types[types.TFLOAT64], types.Types[types.TINT64], 2, OpNotEqualFloat64x2, []string{"fcmp une <2 x double>", "sext <2 x i1>"}},
		{"greater-float32", types.Types[types.TFLOAT32], types.Types[types.TINT32], 4, OpGreaterFloat32x4, []string{"fcmp ogt <4 x float>", "sext <4 x i1>"}},
		{"greater-equal-float64", types.Types[types.TFLOAT64], types.Types[types.TINT64], 2, OpGreaterEqualFloat64x2, []string{"fcmp oge <2 x double>", "sext <2 x i1>"}},
		{"less-float32", types.Types[types.TFLOAT32], types.Types[types.TINT32], 4, OpLessFloat32x4, []string{"fcmp olt <4 x float>", "sext <4 x i1>"}},
		{"less-equal-float64", types.Types[types.TFLOAT64], types.Types[types.TINT64], 2, OpLessEqualFloat64x2, []string{"fcmp ole <2 x double>", "sext <2 x i1>"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			inputType := llvmTestSIMDType(test.name+"-input", test.elem, test.lanes)
			maskType := llvmTestSIMDType(test.name+"-mask", test.maskElem, test.lanes)
			inputVectorType := getLLVMType(inputType)
			maskVectorType := getLLVMType(maskType)
			module := GlobalCtxt.NewModule("generic_vec128_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(maskVectorType, []llvm.Type{inputVectorType, inputVectorType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "arm64"}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: inputType}
			y := &Value{ID: 2, Op: OpArg, Type: inputType}
			context.Vs[x.ID] = function.Param(0)
			context.Vs[y.ID] = function.Param(1)
			result := &Value{ID: 3, Op: test.op, Type: maskType, Args: []*Value{x, y}}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected Vec128 compare lowering: %v\n%s", err, module.String())
			}
			ir := module.String()
			for _, want := range append(test.wants, "ret "+llvmTestIRVectorType(maskVectorType)) {
				if !strings.Contains(ir, want) {
					t.Errorf("Vec128 compare IR does not contain %q\n%s", want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("Vec128 compare lowering introduced a carrier bitcast\n%s", ir)
			}
		})
	}

	for _, test := range []struct {
		name  string
		elem  *types.Type
		lanes int64
		op    Op
		wants []string
	}{
		{"bit-select", types.Types[types.TINT8], 16, OpbitSelectInt8x16, []string{"and <16 x i8> %0, %2", "xor <16 x i8> %2", "and <16 x i8> %1"}},
		{"bit-select-not", types.Types[types.TINT8], 16, OpbitSelectNotInt8x16, []string{"and <16 x i8> %1, %2", "xor <16 x i8> %2", "and <16 x i8> %0"}},
		{"amd64-blend", types.Types[types.TINT8], 16, OpblendInt8x16, []string{"icmp slt <16 x i8> %2", "select <16 x i1>"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			typ := llvmTestSIMDType(test.name, test.elem, test.lanes)
			vectorType := getLLVMType(typ)
			module := GlobalCtxt.NewModule("generic_vec128_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(vectorType, []llvm.Type{vectorType, vectorType, vectorType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "arm64"}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: typ}
			y := &Value{ID: 2, Op: OpArg, Type: typ}
			mask := &Value{ID: 3, Op: OpArg, Type: typ}
			context.Vs[x.ID] = function.Param(0)
			context.Vs[y.ID] = function.Param(1)
			context.Vs[mask.ID] = function.Param(2)
			result := &Value{ID: 4, Op: test.op, Type: typ, Args: []*Value{x, y, mask}}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected Vec128 select lowering: %v\n%s", err, module.String())
			}
			ir := module.String()
			for _, want := range append(test.wants, "ret "+llvmTestIRVectorType(vectorType)) {
				if !strings.Contains(ir, want) {
					t.Errorf("Vec128 select IR does not contain %q\n%s", want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("Vec128 select lowering introduced a carrier bitcast\n%s", ir)
			}
		})
	}

	t.Run("zero-load-store-alignment", func(t *testing.T) {
		typ := llvmTestSIMDType("memory-int32", types.Types[types.TINT32], 4)
		module := GlobalCtxt.NewModule("generic_vec128_memory")
		CurrentModule = module
		builder := GlobalCtxt.NewBuilder()
		t.Cleanup(module.Dispose)
		t.Cleanup(builder.Dispose)

		pointer := GlobalCtxt.PointerType(0)
		function := llvm.AddFunction(module, "memory", llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{pointer, pointer}, false))
		builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
		context := &LLVMFuncContext{
			F:  &Func{Config: &Config{arch: "arm64"}},
			Vs: make(map[ID]llvm.Value),
			b:  builder,
		}
		vecPointer := types.NewPtr(typ)
		src := &Value{ID: 1, Op: OpArg, Type: vecPointer}
		dst := &Value{ID: 2, Op: OpArg, Type: vecPointer}
		context.Vs[src.ID] = function.Param(0)
		context.Vs[dst.ID] = function.Param(1)
		loaded := &Value{ID: 3, Op: OpLoad, Type: typ, Args: []*Value{src}}
		zero := &Value{ID: 4, Op: OpZeroSIMD, Type: typ}
		sum := &Value{ID: 5, Op: OpAddInt32x4, Type: typ, Args: []*Value{loaded, zero}}
		store := &Value{ID: 6, Op: OpStore, Type: types.TypeMem, Args: []*Value{dst, sum}}
		context.GenLV(store)
		builder.CreateRetVoid()

		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected Vec128 memory lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"load <4 x i32>, ptr %0, align 8",
			"add <4 x i32>",
			"zeroinitializer",
			"store <4 x i32>",
			"ptr %1, align 8",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("Vec128 memory IR does not contain %q\n%s", want, ir)
			}
		}
		if strings.Contains(ir, "bitcast ") {
			t.Errorf("Vec128 memory lowering introduced a carrier bitcast\n%s", ir)
		}
	})
}

func TestLLVMGenericSIMDElementLowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	for _, test := range []struct {
		name        string
		elem        *types.Type
		llvmElement string
		lanes       int64
		index       int64
		get, set    Op
	}{
		{name: "int8", elem: types.Types[types.TINT8], llvmElement: "i8", lanes: 16, index: 11, get: OpGetElemInt8x16, set: OpSetElemInt8x16},
		{name: "uint32", elem: types.Types[types.TUINT32], llvmElement: "i32", lanes: 4, index: 2, get: OpGetElemUint32x4, set: OpSetElemUint32x4},
		{name: "float64", elem: types.Types[types.TFLOAT64], llvmElement: "double", lanes: 2, index: 1, get: OpGetElemFloat64x2, set: OpSetElemFloat64x2},
	} {
		t.Run(test.name, func(t *testing.T) {
			vectorGoType := llvmTestSIMDType("element-"+test.name, test.elem, test.lanes)
			vectorType := getLLVMType(vectorGoType)
			elementType := getLLVMType(test.elem)
			module := GlobalCtxt.NewModule("generic_element_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(vectorType, []llvm.Type{vectorType, elementType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: vectorGoType}
			y := &Value{ID: 2, Op: OpArg, Type: test.elem}
			context.Vs[x.ID] = function.Param(0)
			context.Vs[y.ID] = function.Param(1)
			extracted := &Value{ID: 3, Op: test.get, Type: test.elem, AuxInt: test.index, Args: []*Value{x}}
			context.GenLV(extracted)
			inserted := &Value{ID: 4, Op: test.set, Type: vectorGoType, AuxInt: test.index, Args: []*Value{x, y}}
			builder.CreateRet(context.GenLV(inserted))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected %s element lowering: %v\n%s", test.name, err, module.String())
			}
			ir := module.String()
			for _, want := range []string{
				fmt.Sprintf("extractelement %s %%0, i32 %d", llvmTestIRVectorType(vectorType), test.index),
				fmt.Sprintf("insertelement %s %%0, %s %%1, i32 %d", llvmTestIRVectorType(vectorType), test.llvmElement, test.index),
			} {
				if !strings.Contains(ir, want) {
					t.Errorf("element IR does not contain %q\n%s", want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("element lowering introduced a carrier bitcast\n%s", ir)
			}
		})
	}
}

func TestLLVMGenericWideSIMDLowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	for _, test := range []struct {
		name       string
		elem       *types.Type
		resultElem *types.Type
		lanes      int64
		op         Op
		arity      int
		wants      []string
	}{
		{name: "vec256-add-int8", elem: types.Types[types.TINT8], resultElem: types.Types[types.TINT8], lanes: 32, op: OpAddInt8x32, arity: 2, wants: []string{"add <32 x i8>"}},
		{name: "vec256-sub-int16", elem: types.Types[types.TINT16], resultElem: types.Types[types.TINT16], lanes: 16, op: OpSubInt16x16, arity: 2, wants: []string{"sub <16 x i16>"}},
		{name: "vec256-sat-add-uint8", elem: types.Types[types.TUINT8], resultElem: types.Types[types.TUINT8], lanes: 32, op: OpAddSaturatedUint8x32, arity: 2, wants: []string{"call <32 x i8> @llvm.uadd.sat.v32i8"}},
		{name: "vec256-mul-int32", elem: types.Types[types.TINT32], resultElem: types.Types[types.TINT32], lanes: 8, op: OpMulInt32x8, arity: 2, wants: []string{"mul <8 x i32>"}},
		{name: "vec256-div-float64", elem: types.Types[types.TFLOAT64], resultElem: types.Types[types.TFLOAT64], lanes: 4, op: OpDivFloat64x4, arity: 2, wants: []string{"fdiv <4 x double>"}},
		{name: "vec256-and", elem: types.Types[types.TINT64], resultElem: types.Types[types.TINT64], lanes: 4, op: OpAndInt64x4, arity: 2, wants: []string{"and <4 x i64>"}},
		{name: "vec256-abs-int64", elem: types.Types[types.TINT64], resultElem: types.Types[types.TINT64], lanes: 4, op: OpAbsInt64x4, arity: 1, wants: []string{"icmp slt <4 x i64>", "select <4 x i1>"}},
		{name: "vec256-greater-int16", elem: types.Types[types.TINT16], resultElem: types.Types[types.TINT16], lanes: 16, op: OpGreaterInt16x16, arity: 2, wants: []string{"icmp sgt <16 x i16>", "sext <16 x i1>"}},
		{name: "vec512-add-int8", elem: types.Types[types.TINT8], resultElem: types.Types[types.TINT8], lanes: 64, op: OpAddInt8x64, arity: 2, wants: []string{"add <64 x i8>"}},
		{name: "vec512-sub-int16", elem: types.Types[types.TINT16], resultElem: types.Types[types.TINT16], lanes: 32, op: OpSubInt16x32, arity: 2, wants: []string{"sub <32 x i16>"}},
		{name: "vec512-sat-sub-int16", elem: types.Types[types.TINT16], resultElem: types.Types[types.TINT16], lanes: 32, op: OpSubSaturatedInt16x32, arity: 2, wants: []string{"call <32 x i16> @llvm.ssub.sat.v32i16"}},
		{name: "vec512-mul-int32", elem: types.Types[types.TINT32], resultElem: types.Types[types.TINT32], lanes: 16, op: OpMulInt32x16, arity: 2, wants: []string{"mul <16 x i32>"}},
		{name: "vec512-div-float64", elem: types.Types[types.TFLOAT64], resultElem: types.Types[types.TFLOAT64], lanes: 8, op: OpDivFloat64x8, arity: 2, wants: []string{"fdiv <8 x double>"}},
		{name: "vec512-xor", elem: types.Types[types.TINT64], resultElem: types.Types[types.TINT64], lanes: 8, op: OpXorInt64x8, arity: 2, wants: []string{"xor <8 x i64>"}},
		{name: "vec512-abs-int64", elem: types.Types[types.TINT64], resultElem: types.Types[types.TINT64], lanes: 8, op: OpAbsInt64x8, arity: 1, wants: []string{"icmp slt <8 x i64>", "select <8 x i1>"}},
		{name: "vec512-less-equal-uint16", elem: types.Types[types.TUINT16], resultElem: types.Types[types.TINT16], lanes: 32, op: OpLessEqualUint16x32, arity: 2, wants: []string{"icmp ule <32 x i16>", "sext <32 x i1>"}},
		{name: "vec256-round-float32", elem: types.Types[types.TFLOAT32], resultElem: types.Types[types.TFLOAT32], lanes: 8, op: OpRoundFloat32x8, arity: 1, wants: []string{"call <8 x float> @llvm.roundeven.v8f32"}},
		{name: "vec512-leading-zeros-int64", elem: types.Types[types.TINT64], resultElem: types.Types[types.TINT64], lanes: 8, op: OpLeadingZerosInt64x8, arity: 1, wants: []string{"call <8 x i64> @llvm.ctlz.v8i64(<8 x i64> %0, i1 false)"}},
		{name: "vec512-max-float64", elem: types.Types[types.TFLOAT64], resultElem: types.Types[types.TFLOAT64], lanes: 8, op: OpMaxFloat64x8, arity: 2, wants: []string{"call <8 x double> @llvm.maximum.v8f64"}},
		{name: "vec256-average-uint8", elem: types.Types[types.TUINT8], resultElem: types.Types[types.TUINT8], lanes: 32, op: OpAverageUint8x32, arity: 2, wants: []string{"lshr <32 x i8>", "sub <32 x i8>"}},
		{name: "vec256-mul-high-int16", elem: types.Types[types.TINT16], resultElem: types.Types[types.TINT16], lanes: 16, op: OpMulHighInt16x16, arity: 2, wants: []string{"sext <16 x i16>", "mul <16 x i32>", "trunc <16 x i32>"}},
		{name: "vec256-mul-sign-int8", elem: types.Types[types.TINT8], resultElem: types.Types[types.TINT8], lanes: 32, op: OpMulSignInt8x32, arity: 2, wants: []string{"call <32 x i8> @llvm.x86.avx2.psign.b"}},
		{name: "vec512-average-uint16", elem: types.Types[types.TUINT16], resultElem: types.Types[types.TUINT16], lanes: 32, op: OpAverageUint16x32, arity: 2, wants: []string{"lshr <32 x i16>", "sub <32 x i16>"}},
		{name: "vec512-mul-high-uint16", elem: types.Types[types.TUINT16], resultElem: types.Types[types.TUINT16], lanes: 32, op: OpMulHighUint16x32, arity: 2, wants: []string{"zext <32 x i16>", "mul <32 x i32>", "trunc <32 x i32>"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			inputType := llvmTestSIMDType(test.name+"-input", test.elem, test.lanes)
			resultType := llvmTestSIMDType(test.name+"-result", test.resultElem, test.lanes)
			inputVectorType := getLLVMType(inputType)
			resultVectorType := getLLVMType(resultType)

			module := GlobalCtxt.NewModule("generic_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			params := make([]llvm.Type, test.arity)
			for i := range params {
				params[i] = inputVectorType
			}
			function := llvm.AddFunction(module, test.name, llvm.FunctionType(resultVectorType, params, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "amd64"}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			args := make([]*Value, test.arity)
			for i := range args {
				args[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: inputType}
				context.Vs[args[i].ID] = function.Param(i)
			}
			result := &Value{ID: ID(test.arity + 1), Op: test.op, Type: resultType, Args: args}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected %s lowering: %v\n%s", test.name, err, module.String())
			}
			ir := module.String()
			for _, want := range append(test.wants, "ret "+llvmTestIRVectorType(resultVectorType)) {
				if !strings.Contains(ir, want) {
					t.Errorf("%s IR does not contain %q\n%s", test.name, want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("%s lowering introduced a carrier bitcast\n%s", test.name, ir)
			}
		})
	}
}

func TestLLVMGeneratedSIMDReductions(t *testing.T) {
	if !buildcfg.Experiment.SIMD {
		t.Skip("requires GOEXPERIMENT=simd")
	}

	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	for _, test := range []struct {
		name  string
		elem  *types.Type
		lanes int64
		op    Op
		want  string
	}{
		{name: "add-int8", elem: types.Types[types.TINT8], lanes: 16, op: OpreduceSumInt8x16, want: "call i8 @llvm.vector.reduce.add.v16i8"},
		{name: "max-int16", elem: types.Types[types.TINT16], lanes: 8, op: OpreduceMaxInt16x8, want: "call i16 @llvm.vector.reduce.smax.v8i16"},
		{name: "min-uint32", elem: types.Types[types.TUINT32], lanes: 4, op: OpreduceMinUint32x4, want: "call i32 @llvm.vector.reduce.umin.v4i32"},
		{name: "max-float32", elem: types.Types[types.TFLOAT32], lanes: 4, op: OpreduceMaxFloat32x4, want: "call float @llvm.vector.reduce.fmaximum.v4f32"},
		{name: "min-float32", elem: types.Types[types.TFLOAT32], lanes: 4, op: OpreduceMinFloat32x4, want: "call float @llvm.vector.reduce.fminimum.v4f32"},
	} {
		t.Run(test.name, func(t *testing.T) {
			vectorGoType := llvmTestSIMDType("reduce-"+test.name, test.elem, test.lanes)
			vectorType := getLLVMType(vectorGoType)
			module := GlobalCtxt.NewModule("reduce_" + test.name)
			CurrentModule = module
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)

			function := llvm.AddFunction(module, test.name, llvm.FunctionType(vectorType, []llvm.Type{vectorType}, false))
			builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
			context := &LLVMFuncContext{
				F:  &Func{Config: &Config{arch: "arm64"}, Entry: &Block{}},
				Vs: make(map[ID]llvm.Value),
				b:  builder,
			}
			x := &Value{ID: 1, Op: OpArg, Type: vectorGoType}
			context.Vs[x.ID] = function.Param(0)
			result := &Value{ID: 2, Op: test.op, Type: types.TypeVec128, Args: []*Value{x}}
			builder.CreateRet(context.GenLV(result))

			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("LLVM verifier rejected %s reduction: %v\n%s", test.name, err, module.String())
			}
			ir := module.String()
			for _, want := range []string{
				test.want,
				"insertelement " + llvmTestIRVectorType(vectorType) + " zeroinitializer",
			} {
				if !strings.Contains(ir, want) {
					t.Errorf("%s reduction IR does not contain %q\n%s", test.name, want, ir)
				}
			}
			if strings.Contains(ir, "bitcast ") {
				t.Errorf("%s reduction introduced a carrier bitcast\n%s", test.name, ir)
			}
		})
	}
}

func TestGoALLCGeneratedSIMDDescriptors(t *testing.T) {
	for _, test := range []struct {
		name       string
		op         Op
		lowering   goALLCSIMDLowering
		lane       goALLCSIMDLane
		laneBits   uint8
		amdProfile string
	}{
		{
			name: "128-bit-add", op: OpAddInt8x16,
			lowering: goALLCSIMDLowerAdd,
			lane:     goALLCSIMDLaneInt, laneBits: 8,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "256-bit-avx2-add", op: OpAddInt8x32,
			lowering: goALLCSIMDLowerAdd,
			lane:     goALLCSIMDLaneInt, laneBits: 8,
			amdProfile: goCPUProfileX86AVX2,
		},
		{
			name: "512-bit-avx512-add", op: OpAddInt8x64,
			lowering: goALLCSIMDLowerAdd,
			lane:     goALLCSIMDLaneInt, laneBits: 8,
			amdProfile: goCPUProfileX86AVX512,
		},
		{
			name: "128-bit-arm64-saturated-add", op: OpAddSaturatedUint32x4,
			lowering: goALLCSIMDLowerAddSaturated,
			lane:     goALLCSIMDLaneUint, laneBits: 32,
		},
		{
			name: "512-bit-avx512-saturated-sub", op: OpSubSaturatedInt16x32,
			lowering: goALLCSIMDLowerSubSaturated,
			lane:     goALLCSIMDLaneInt, laneBits: 16,
			amdProfile: goCPUProfileX86AVX512,
		},
		{
			name: "128-bit-element-extract", op: OpGetElemFloat64x2,
			lowering: goALLCSIMDLowerExtractElement,
			lane:     goALLCSIMDLaneFloat, laneBits: 64,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "128-bit-element-insert", op: OpSetElemUint16x8,
			lowering: goALLCSIMDLowerInsertElement,
			lane:     goALLCSIMDLaneUint, laneBits: 16,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "128-bit-arm64-reduce-sum", op: OpreduceSumInt32x4,
			lowering: goALLCSIMDLowerReduceAdd,
			lane:     goALLCSIMDLaneInt, laneBits: 32,
		},
		{
			name: "128-bit-arm64-reduce-float-max", op: OpreduceMaxFloat32x4,
			lowering: goALLCSIMDLowerReduceMax,
			lane:     goALLCSIMDLaneFloat, laneBits: 32,
		},
		{
			name: "128-bit-arm64-signed-average", op: OpAverageInt8x16,
			lowering: goALLCSIMDLowerAverage,
			lane:     goALLCSIMDLaneInt, laneBits: 8,
		},
		{
			name: "256-bit-avx2-high-multiply", op: OpMulHighUint16x16,
			lowering: goALLCSIMDLowerMulHigh,
			lane:     goALLCSIMDLaneUint, laneBits: 16,
			amdProfile: goCPUProfileX86AVX2,
		},
		{
			name: "128-bit-arm64-leading-sign-bits", op: OpLeadingSignBitsUint32x4,
			lowering: goALLCSIMDLowerLeadingSignBits,
			lane:     goALLCSIMDLaneUint, laneBits: 32,
		},
		{
			name: "128-bit-avx-sign-multiply", op: OpMulSignInt32x4,
			lowering: goALLCSIMDLowerMulSign,
			lane:     goALLCSIMDLaneInt, laneBits: 32,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "128-bit-round-even", op: OpRoundFloat64x2,
			lowering: goALLCSIMDLowerRoundEven,
			lane:     goALLCSIMDLaneFloat, laneBits: 64,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "512-bit-leading-zeros", op: OpLeadingZerosUint64x8,
			lowering: goALLCSIMDLowerLeadingZeros,
			lane:     goALLCSIMDLaneUint, laneBits: 64,
			amdProfile: goCPUProfileX86AVX512,
		},
		{
			name: "128-bit-bitalg-ones-count", op: OpOnesCountUint8x16,
			lowering: goALLCSIMDLowerOnesCount,
			lane:     goALLCSIMDLaneUint, laneBits: 8,
			amdProfile: goCPUProfileX86AVX512BITALG,
		},
		{
			name: "128-bit-vpopcntdq-ones-count", op: OpOnesCountUint32x4,
			lowering: goALLCSIMDLowerOnesCount,
			lane:     goALLCSIMDLaneUint, laneBits: 32,
			amdProfile: goCPUProfileX86AVX512VPOPCNTDQ,
		},
		{
			name: "128-bit-float-max", op: OpMaxFloat32x4,
			lowering: goALLCSIMDLowerMax,
			lane:     goALLCSIMDLaneFloat, laneBits: 32,
			amdProfile: goCPUProfileX86AVX,
		},
		{
			name: "128-bit-signed-min", op: OpMinInt16x8,
			lowering: goALLCSIMDLowerMin,
			lane:     goALLCSIMDLaneInt, laneBits: 16,
			amdProfile: goCPUProfileX86AVX,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			info, ok := goALLCSIMDInfo(test.op)
			if !ok {
				t.Fatalf("missing generated descriptor for %s", test.op)
			}
			if info.lowering != test.lowering || info.lane != test.lane || info.laneBits != test.laneBits {
				t.Fatalf("generated descriptor for %s = %#v", test.op, info)
			}
			if info.amd64.cpuProfile != test.amdProfile {
				t.Fatalf("generated AMD64 profile for %s = %q, want %q", test.op, info.amd64.cpuProfile, test.amdProfile)
			}
		})
	}

	andNot, ok := goALLCSIMDInfo(OpAndNotInt8x16)
	if !ok {
		t.Fatal("missing generated AndNot descriptor")
	}
	if andNot.amd64.operandOrder != "21" || andNot.arm64.operandOrder != "" {
		t.Fatalf("generated AndNot operand orders = amd64:%q arm64:%q", andNot.amd64.operandOrder, andNot.arm64.operandOrder)
	}

	if _, ok := goALLCSIMDInfo(OpNotEqualUint64x2); ok {
		t.Fatal("wasm-only op unexpectedly received an LLVM lowering descriptor")
	}

}

func TestLLVMCPUProfileCoverage(t *testing.T) {
	for _, test := range []struct {
		name     string
		required string
		floor    string
		want     bool
	}{
		{name: "same", required: goCPUProfileX86AVX2, floor: goCPUProfileX86AVX2, want: true},
		{name: "avx2-covers-avx", required: goCPUProfileX86AVX, floor: goCPUProfileX86AVX2, want: true},
		{name: "avx512-covers-avx2", required: goCPUProfileX86AVX2, floor: goCPUProfileX86AVX512, want: true},
		{name: "avx512-does-not-cover-bitalg", required: goCPUProfileX86AVX512BITALG, floor: goCPUProfileX86AVX512, want: false},
		{name: "avx512-does-not-cover-vpopcntdq", required: goCPUProfileX86AVX512VPOPCNTDQ, floor: goCPUProfileX86AVX512, want: false},
		{name: "avx-does-not-cover-avx2", required: goCPUProfileX86AVX2, floor: goCPUProfileX86AVX, want: false},
		{name: "width-floor-does-not-cover-fma", required: goCPUProfileX86FMA, floor: goCPUProfileX86AVX2, want: false},
		{name: "no-floor", required: goCPUProfileX86AVX, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmCPUProfileCoveredByFloor(test.required, test.floor); got != test.want {
				t.Fatalf("llvmCPUProfileCoveredByFloor(%q, %q) = %v, want %v", test.required, test.floor, got, test.want)
			}
		})
	}
}

func TestLLVMWideVectorTypeWidth(t *testing.T) {
	vec256 := llvmTestSIMDType("Uint8x32", types.Types[types.TUINT8], 32)
	vec512 := llvmTestSIMDType("Uint8x64", types.Types[types.TUINT8], 64)
	aggregate := types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, nil, types.Types[types.TUINT64]),
		types.NewField(src.NoXPos, nil, types.NewArray(vec512, 2)),
	})
	types.CalcStructSize(aggregate)

	for _, test := range []struct {
		name string
		typ  *types.Type
		want int64
	}{
		{name: "scalar", typ: types.Types[types.TUINT64]},
		{name: "vec256", typ: vec256, want: 32},
		{name: "aggregate-vec512", typ: aggregate, want: 64},
		{name: "pointer-stops-walk", typ: types.NewPtr(vec512)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmWideVectorTypeWidth(test.typ); got != test.want {
				t.Fatalf("llvmWideVectorTypeWidth(%v) = %d, want %d", test.typ, got, test.want)
			}
		})
	}
}

func TestLLVMCPUProfileSuppliesWideVector(t *testing.T) {
	for _, test := range []struct {
		name     string
		profile  string
		required string
		want     bool
	}{
		{name: "avx", profile: goCPUProfileX86AVX, required: goCPUProfileX86AVX, want: true},
		{name: "avx2-supplies-avx", profile: goCPUProfileX86AVX2, required: goCPUProfileX86AVX, want: true},
		{name: "avx512-supplies-avx", profile: goCPUProfileX86AVX512, required: goCPUProfileX86AVX, want: true},
		{name: "bitalg-supplies-avx512", profile: goCPUProfileX86AVX512BITALG, required: goCPUProfileX86AVX512, want: true},
		{name: "vpopcntdq-supplies-avx512", profile: goCPUProfileX86AVX512VPOPCNTDQ, required: goCPUProfileX86AVX512, want: true},
		{name: "fma-does-not-supply-avx", profile: goCPUProfileX86FMA, required: goCPUProfileX86AVX},
		{name: "avx2-does-not-supply-avx512", profile: goCPUProfileX86AVX2, required: goCPUProfileX86AVX512},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmCPUProfileSuppliesWideVector(test.profile, test.required); got != test.want {
				t.Fatalf("llvmCPUProfileSuppliesWideVector(%q, %q) = %v, want %v", test.profile, test.required, got, test.want)
			}
		})
	}
}

func TestLLVMX86CPUFeatureProfile(t *testing.T) {
	for _, test := range []struct {
		field string
		want  string
	}{
		{field: "HasAVX", want: goCPUProfileX86AVX},
		{field: "HasAVX2", want: goCPUProfileX86AVX2},
		{field: "HasAVX512", want: goCPUProfileX86AVX512},
		{field: "HasAVX512BITALG", want: goCPUProfileX86AVX512BITALG},
		{field: "HasAVX512VPOPCNTDQ", want: goCPUProfileX86AVX512VPOPCNTDQ},
		{field: "HasFMA", want: goCPUProfileX86FMA},
		{field: "HasSSE41", want: goCPUProfileX86SSE41},
		{field: "HasPOPCNT", want: goCPUProfileX86POPCNT},
		{field: "HasAVX512GFNI"},
		{field: "HasAVXVNNI"},
	} {
		t.Run(test.field, func(t *testing.T) {
			if got := llvmX86CPUFeatureProfile(test.field); got != test.want {
				t.Fatalf("llvmX86CPUFeatureProfile(%q) = %q, want %q", test.field, got, test.want)
			}
		})
	}
}

func TestLLVMX86CPUFeatureGuard(t *testing.T) {
	pkg := types.NewPkg("internal/cpu", "cpu")
	x86Type := types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, pkg.Lookup("HasAVX2"), types.Types[types.TBOOL]),
	})
	types.CalcStructSize(x86Type)
	sb := &Value{Op: OpSB}
	addr := &Value{Op: OpAddr, Type: types.NewPtr(x86Type), Aux: &obj.LSym{Name: "internal/cpu.X86"}, Args: []*Value{sb}}
	offPtr := &Value{Op: OpOffPtr, AuxInt: x86Type.Field(0).Offset, Args: []*Value{addr}}
	load := &Value{Op: OpLoad, Args: []*Value{offPtr}}
	enabled := &Block{ID: 2}
	disabled := &Block{ID: 3}

	guard := &Block{
		Kind:     BlockIf,
		Controls: [2]*Value{load, nil},
		Succs:    []Edge{{b: enabled}, {b: disabled}},
	}
	profile, successor := llvmX86CPUFeatureGuard(guard)
	if profile != goCPUProfileX86AVX2 || successor != enabled {
		t.Fatalf("positive guard = (%q, %v), want (%q, %v)", profile, successor, goCPUProfileX86AVX2, enabled)
	}

	not := &Value{Op: OpNot, Args: []*Value{load}}
	guard.Controls[0] = not
	profile, successor = llvmX86CPUFeatureGuard(guard)
	if profile != goCPUProfileX86AVX2 || successor != disabled {
		t.Fatalf("negated guard = (%q, %v), want (%q, %v)", profile, successor, goCPUProfileX86AVX2, disabled)
	}
}

func TestLLVMSIMDFeatureFloor(t *testing.T) {
	for _, test := range []struct {
		name     string
		arch     string
		features CPUfeatures
		funcName string
		want     string
	}{
		{name: "amd64-avx", arch: "amd64", features: CPUavx, want: goCPUProfileX86AVX},
		{name: "amd64-avx2", arch: "amd64", features: CPUavx | CPUavx2, want: goCPUProfileX86AVX2},
		{name: "amd64-avx512", arch: "amd64", features: CPUavx | CPUavx2 | CPUavx512, want: goCPUProfileX86AVX512},
		{name: "amd64-midway-128", arch: "amd64", funcName: "simd.Int8s.Add@simd128", want: goCPUProfileX86AVX},
		{name: "amd64-midway-256", arch: "amd64", features: CPUavx, funcName: "simd.Int8s.Add@simd256", want: goCPUProfileX86AVX2},
		{name: "amd64-midway-512", arch: "amd64", features: CPUavx, funcName: "simd.Int8s.Add@simd512", want: goCPUProfileX86AVX512},
		{name: "amd64-none", arch: "amd64", features: CPUNone},
		{name: "arm64", arch: "arm64", features: CPUavx, funcName: "simd.Int8s.Add@simd512"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &Func{
				Config: &Config{arch: test.arch},
				Entry:  &Block{CPUfeatures: test.features},
				Name:   test.funcName,
			}
			if test.funcName != "" {
				f.OwnAux = &AuxCall{Fn: &obj.LSym{Name: test.funcName}}
			}
			if got := llvmSIMDFeatureFloor(f); got != test.want {
				t.Fatalf("llvmSIMDFeatureFloor() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLLVMTargetCPU(t *testing.T) {
	for _, test := range []struct {
		arch    string
		goamd64 int
		want    string
	}{
		{"arm64", 1, ""},
		{"amd64", 1, "x86-64"},
		{"amd64", 2, "x86-64-v2"},
		{"amd64", 3, "x86-64-v3"},
		{"amd64", 4, "x86-64-v4"},
	} {
		t.Run(test.arch+"/"+test.want, func(t *testing.T) {
			if got := llvmTargetCPU(test.arch, test.goamd64); got != test.want {
				t.Fatalf("llvmTargetCPU(%q, %d) = %q, want %q", test.arch, test.goamd64, got, test.want)
			}
		})
	}
}

func TestLLVMBaselineTargetFeatures(t *testing.T) {
	for _, test := range []struct {
		name    string
		arch    string
		goarm64 buildcfg.Goarm64Features
		want    string
	}{
		{"amd64", "amd64", buildcfg.Goarm64Features{}, ""},
		{"arm64-v8.0", "arm64", buildcfg.Goarm64Features{Version: "v8.0"}, ""},
		{"arm64-lse", "arm64", buildcfg.Goarm64Features{Version: "v8.0", LSE: true}, "+lse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmBaselineTargetFeatures(test.arch, test.goarm64); got != test.want {
				t.Fatalf("llvmBaselineTargetFeatures(%q, %+v) = %q, want %q", test.arch, test.goarm64, got, test.want)
			}
		})
	}
}

func TestLLVMRequiredCPUProfiles(t *testing.T) {
	for _, test := range []struct {
		name          string
		arch          string
		goamd64       int
		baselineLevel int
		profile       string
		want          string
	}{
		{"arm64-round", "arm64", 1, 2, goCPUProfileX86SSE41, ""},
		{"v1-round", "amd64", 1, 2, goCPUProfileX86SSE41, goCPUProfileX86SSE41},
		{"v2-round", "amd64", 2, 2, goCPUProfileX86SSE41, ""},
		{"v1-fma", "amd64", 1, 3, goCPUProfileX86FMA, goCPUProfileX86FMA},
		{"v2-fma", "amd64", 2, 3, goCPUProfileX86FMA, goCPUProfileX86FMA},
		{"v3-fma", "amd64", 3, 3, goCPUProfileX86FMA, ""},
		{"v4-fma", "amd64", 4, 3, goCPUProfileX86FMA, ""},
		{"v1-popcnt", "amd64", 1, 2, goCPUProfileX86POPCNT, goCPUProfileX86POPCNT},
		{"v2-popcnt", "amd64", 2, 2, goCPUProfileX86POPCNT, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmRequiredAMD64CPUProfile(test.arch, test.goamd64, test.baselineLevel, test.profile); got != test.want {
				t.Fatalf("llvmRequiredAMD64CPUProfile(%q, %d, %d, %q) = %q, want %q", test.arch, test.goamd64, test.baselineLevel, test.profile, got, test.want)
			}
		})
	}

	for _, test := range []struct {
		name               string
		arch               string
		baselineHasFeature bool
		want               string
	}{
		{"amd64", "amd64", false, ""},
		{"arm64-v8.0", "arm64", false, goCPUProfileARM64LSE},
		{"arm64-lse", "arm64", true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := llvmRequiredARM64CPUProfile(test.arch, test.baselineHasFeature, goCPUProfileARM64LSE); got != test.want {
				t.Fatalf("llvmRequiredARM64CPUProfile(%q, %t, %q) = %q, want %q", test.arch, test.baselineHasFeature, goCPUProfileARM64LSE, got, test.want)
			}
		})
	}
}

func TestLLVMRuntimeGorecoverUsesLinkSymbolName(t *testing.T) {
	recoverFn := &Func{
		Name:   "gorecover",
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "runtime.gorecover"}},
	}
	if !llvmIsRuntimeGorecover(recoverFn) {
		t.Fatal("gorecover definition was not recognized from its qualified link symbol")
	}

	unrelated := &Func{
		Name:   "gorecover",
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "other.gorecover"}},
	}
	if llvmIsRuntimeGorecover(unrelated) {
		t.Fatal("unqualified SSA function name incorrectly identified another package's gorecover")
	}

	caller := &Func{
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "runtime.preprintpanics.func1"}},
		Blocks: []*Block{{
			Values: []*Value{{Aux: &AuxCall{Fn: &obj.LSym{Name: "runtime.gorecover"}}}},
		}},
	}
	if llvmIsRuntimeGorecover(caller) {
		t.Fatal("direct gorecover caller was incorrectly identified as gorecover itself")
	}
}
