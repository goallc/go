// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"fmt"
	"internal/buildcfg"
	"strconv"
	"strings"

	"github.com/goallc/go-llvm"
)

type LLVMFuncContext struct {
	BBs               map[ID]llvm.BasicBlock
	Vs                map[ID]llvm.Value
	Locals            map[llvmLocalKey]llvmStackSlot
	AddressedResults  map[ID][]llvmAddressedResult
	ResultSlots       map[ID]llvm.Value
	CallResultSlots   map[llvmCallResultKey]llvmStackSlot
	ItabMethods       map[ID]bool
	ClosureCodeLoads  map[ID]bool
	DeferResults      map[llvmLocalKey]bool
	DeferResultKeys   map[ID]llvmLocalKey
	RequiredInlinePos map[int]bool
	OpenDeferBits     llvmLocalKey
	HasOpenDeferBits  bool
	OpenDeferSlots    map[llvmLocalKey]int
	F                 *Func
	LF                llvm.Value
	DISubprogram      llvm.Metadata
	Prologue          llvm.BasicBlock
	OpenDeferRecovery llvm.BasicBlock
	ClosureContext    llvm.Value
	ABI0FrameBase     llvm.Value
	b                 llvm.Builder
	ReturnType        llvm.Type
	ResultCount       int
	ReturnCount       int
	Params            []llvmParamSignature
	Results           []llvmResultSignature
}

// SSA may clone an ir.Name while retaining the same logical source
// declaration. Pointer identity is therefore not a stable stack-slot key.
// Package symbol plus declaration position distinguishes shadowed locals while
// merging those clones.
type llvmLocalKey struct {
	Sym *types.Sym
	Pos src.XPos
}

type llvmStackSlot struct {
	Value llvm.Value
	Type  *types.Type
}

type llvmAddressedResult struct {
	Index int64
	Slot  llvmStackSlot
	Owner *Value
}

type llvmCallResultKey struct {
	Call  ID
	Index int64
}

// LLVM's GoABIInternal calling convention has numeric ID 22. Keep the
// prototype lowering on the Go register ABI so llc emits GoObj symbols that
// the standard Go linker can call directly.
const goABIInternalCallConv llvm.CallConv = 22
const goABI0CallConv llvm.CallConv = 23
const goABI0SymbolSuffix = "<ABI0>"
const goResultsTupleAttr = "go_results_tuple"
const goGCStrategy = "goallc"
const goGCLeafFunctionAttr = "gc-leaf-function"
const goNoSplitAttr = "go-nosplit"
const goSystemStackAttr = "go-systemstack"
const goAsyncUnsafeAttr = "go-async-unsafe"
const goWriteBarrierIntrinsic = "llvm.go.gc.write.barrier"
const goDeferEdgeIntrinsic = "llvm.go.defer.edge"
const goPointerAddressObservation = "llvm.go.pointer.address"
const goPointerFromAddress = "llvm.go.pointer.from.address"
const goNotInHeapAddressMD = "goallc.notinheap"
const goDeferResultMD = "goallc.defer_result"
const goOpenDeferBitsMD = "goallc.open_defer_bits"
const goOpenDeferSlotsMD = "goallc.open_defer_slots"
const goObjMarkerRelocMD = "goobj.marker_reloc"
const goObjSymbolIndexMD = "goobj.symbol.index"
const goObjStaticRODataTypeMD = "goobj.static_rodata_type"
const goObjDebugInlineRequiredMD = "goobj.debug.inline.required"
const llvmFramePointerAttr = "frame-pointer"
const llvmFramePointerNonLeaf = "non-leaf"
const llvmTargetCPUAttr = "target-cpu"

// Keep fixed-size memmoves within the store expansion limits of the supported
// LLVM targets. Larger moves must use runtime.memmove rather than a libc symbol,
// which the GoObj pipeline cannot resolve.
const llvmInlineMemmoveLimit = 64
const llvmAttributeFunctionIndex = -1

type llvmFuncSignature struct {
	Type                llvm.Type
	ReturnType          llvm.Type
	ResultCount         int
	ReturnCount         int
	Params              []llvmParamSignature
	Results             []llvmResultSignature
	HasClosureContext   bool
	ClosureContextIndex int
}

type llvmParamSignature struct {
	ValueType llvm.Type
	Alignment int
	ByVal     bool
}

type llvmResultSignature struct {
	ValueType   llvm.Type
	Alignment   int
	InMemory    bool
	ReturnIndex int
	ParamIndex  int
}

func llvmCallConv(which obj.ABI) llvm.CallConv {
	switch which {
	case obj.ABI0:
		return goABI0CallConv
	case obj.ABIInternal:
		return goABIInternalCallConv
	default:
		base.Fatalf("unsupported Go ABI %v in LLVM lowering", which)
		return 0
	}
}

var llvmABIPadType llvm.Type

// getLLVMABIPadType is a one-byte storage carrier that GoCallingConv treats as
// padding rather than a register value. Its distinct identified type keeps the
// ABI exception in the LLVM type graph instead of encoding field paths in an
// attribute.
func getLLVMABIPadType() llvm.Type {
	if llvmABIPadType.IsNil() {
		llvmABIPadType = GlobalCtxt.StructCreateNamed("go.abi.pad")
		llvmABIPadType.StructSetBody([]llvm.Type{GlobalCtxt.Int8Type()}, false)
	}
	return llvmABIPadType
}

func llvmStructHasTailPad(typ *types.Type) bool {
	return typ.Kind() == types.TSTRUCT && typ.Size() != 0 && typ.NumFields() != 0 && typ.FieldType(typ.NumFields()-1).Size() == 0
}

func llvmTypeContainsABIPad(typ llvm.Type) bool {
	if typ == getLLVMABIPadType() {
		return true
	}
	switch typ.TypeKind() {
	case llvm.StructTypeKind:
		for _, field := range typ.StructElementTypes() {
			if llvmTypeContainsABIPad(field) {
				return true
			}
		}
	case llvm.ArrayTypeKind:
		return typ.ArrayLength() != 0 && llvmTypeContainsABIPad(typ.ElementType())
	}
	return false
}

// getLLVMABIType makes a non-empty carrier only at a top-level zero-sized ABI
// boundary. The original zero-sized layout remains in the wrapper, so
// DataLayout supplies its Go alignment without storing that alignment in an
// attribute.
func getLLVMABIType(typ *types.Type) llvm.Type {
	storage := getLLVMType(typ)
	if typ.Size() == 0 {
		return llvm.StructType([]llvm.Type{storage, getLLVMABIPadType()}, false)
	}
	return storage
}

func llvmSignature(aux *AuxCall) llvmFuncSignature {
	if aux == nil || aux.ABIInfo() == nil {
		base.Fatalf("missing ABI information in LLVM lowering")
	}

	params := make([]llvm.Type, 0, aux.NArgs())
	paramSignatures := make([]llvmParamSignature, 0, aux.NArgs())
	for i := int64(0); i < aux.NArgs(); i++ {
		goType := aux.TypeOfArg(i)
		valueType := getLLVMABIType(goType)
		paramType := valueType
		param := llvmParamSignature{ValueType: valueType}
		assignment := aux.ABIInfo().InParam(int(i))
		if len(assignment.Registers) == 0 && goType.Size() != 0 {
			if goType.Alignment() <= 0 {
				base.Fatalf("invalid alignment %d for stack argument %d of type %v", goType.Alignment(), i, goType)
			}
			param.ByVal = true
			param.Alignment = int(goType.Alignment())
			paramType = GlobalCtxt.PointerType(0)
		}
		params = append(params, paramType)
		paramSignatures = append(paramSignatures, param)
	}

	results := make([]llvm.Type, 0, aux.NResults())
	resultSignatures := make([]llvmResultSignature, 0, aux.NResults())
	for i := int64(0); i < aux.NResults(); i++ {
		goType := aux.TypeOfResult(i)
		result := llvmResultSignature{
			ValueType:   getLLVMABIType(goType),
			ReturnIndex: -1,
			ParamIndex:  -1,
		}
		assignment := aux.ABIInfo().OutParam(int(i))
		if len(assignment.Registers) == 0 && goType.Size() != 0 {
			if goType.Alignment() <= 0 {
				base.Fatalf("invalid alignment %d for stack result %d of type %v", goType.Alignment(), i, goType)
			}
			result.InMemory = true
			result.Alignment = int(goType.Alignment())
			result.ParamIndex = len(params)
			params = append(params, GlobalCtxt.PointerType(0))
		} else {
			result.ReturnIndex = len(results)
			results = append(results, result.ValueType)
		}
		resultSignatures = append(resultSignatures, result)
	}

	var ret llvm.Type
	switch len(results) {
	case 0:
		ret = GlobalCtxt.VoidType()
	case 1:
		ret = results[0]
	default:
		ret = llvm.StructType(results, false)
	}
	return llvmFuncSignature{
		Type:                llvm.FunctionType(ret, params, false),
		ReturnType:          ret,
		ResultCount:         len(resultSignatures),
		ReturnCount:         len(results),
		Params:              paramSignatures,
		Results:             resultSignatures,
		ClosureContextIndex: -1,
	}
}

func (sig llvmFuncSignature) withClosureContext() llvmFuncSignature {
	params := append([]llvm.Type(nil), sig.Type.ParamTypes()...)
	sig.ClosureContextIndex = len(params)
	sig.HasClosureContext = true
	params = append(params, GlobalCtxt.PointerType(0))
	sig.Type = llvm.FunctionType(sig.ReturnType, params, false)
	return sig
}

func llvmNestAttribute() llvm.Attribute {
	kind := llvm.AttributeKindID("nest")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the nest parameter attribute")
	}
	return GlobalCtxt.CreateEnumAttribute(kind, 0)
}

func llvmByValAttribute(t llvm.Type) llvm.Attribute {
	kind := llvm.AttributeKindID("byval")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the byval parameter attribute")
	}
	return GlobalCtxt.CreateTypeAttribute(kind, t)
}

func llvmGoRetAttribute(t llvm.Type) llvm.Attribute {
	kind := llvm.AttributeKindID("goret")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the goret parameter attribute")
	}
	return GlobalCtxt.CreateTypeAttribute(kind, t)
}

func llvmGoRetIndexAttribute(index int) llvm.Attribute {
	return GlobalCtxt.CreateStringAttribute("goretindex", strconv.Itoa(index))
}

func llvmNullPointerIsValidAttribute() llvm.Attribute {
	kind := llvm.AttributeKindID("null_pointer_is_valid")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the null_pointer_is_valid function attribute")
	}
	return GlobalCtxt.CreateEnumAttribute(kind, 0)
}

func llvmNoInlineAttribute() llvm.Attribute {
	kind := llvm.AttributeKindID("noinline")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the noinline function attribute")
	}
	return GlobalCtxt.CreateEnumAttribute(kind, 0)
}

func configureLLVMFunction(fn llvm.Value, sig llvmFuncSignature, cc llvm.CallConv) {
	fn.SetFunctionCallConv(cc)
	if sig.ReturnCount > 1 {
		fn.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goResultsTupleAttr, ""))
	}
	for i, param := range sig.Params {
		if !param.ByVal {
			continue
		}
		fn.AddAttributeAtIndex(i+1, llvmByValAttribute(param.ValueType))
		fn.Param(i).SetParamAlignment(param.Alignment)
	}
	for index, result := range sig.Results {
		if !result.InMemory {
			continue
		}
		fn.AddAttributeAtIndex(result.ParamIndex+1, llvmGoRetAttribute(result.ValueType))
		fn.AddAttributeAtIndex(result.ParamIndex+1, llvmGoRetIndexAttribute(index))
		fn.Param(result.ParamIndex).SetParamAlignment(result.Alignment)
	}
	if sig.HasClosureContext {
		// LLVM parameter attribute indexes are one-based. The closure context
		// is deliberately excluded from the Go ABI argument layout by the
		// target and carried in REGCTXT (RDX on amd64, X26 on arm64).
		fn.AddAttributeAtIndex(sig.ClosureContextIndex+1, llvmNestAttribute())
	}
}

func configureLLVMCall(call llvm.Value, sig llvmFuncSignature) {
	if sig.ReturnCount > 1 {
		call.AddCallSiteAttribute(llvmAttributeFunctionIndex, GlobalCtxt.CreateStringAttribute(goResultsTupleAttr, ""))
	}
	for i, param := range sig.Params {
		if !param.ByVal {
			continue
		}
		call.AddCallSiteAttribute(i+1, llvmByValAttribute(param.ValueType))
		call.SetInstrParamAlignment(i+1, param.Alignment)
	}
	for index, result := range sig.Results {
		if !result.InMemory {
			continue
		}
		call.AddCallSiteAttribute(result.ParamIndex+1, llvmGoRetAttribute(result.ValueType))
		call.AddCallSiteAttribute(result.ParamIndex+1, llvmGoRetIndexAttribute(index))
		call.SetInstrParamAlignment(result.ParamIndex+1, result.Alignment)
	}
}

func llvmFunctionStorageName(name string, cc llvm.CallConv) string {
	if strings.HasSuffix(name, goABI0SymbolSuffix) {
		base.Fatalf("Go symbol name %q uses reserved LLVM ABI suffix", name)
	}
	if cc == goABI0CallConv {
		return name + goABI0SymbolSuffix
	}
	return name
}

func getOrInsertLLVMFunction(name string, sig llvmFuncSignature, cc llvm.CallConv) llvm.Value {
	storageName := llvmFunctionStorageName(name, cc)
	fn := CurrentModule.NamedFunction(storageName)
	if fn.IsNil() {
		fn = llvm.AddFunction(CurrentModule, storageName+".goallc.final", sig.Type)
		if placeholder := CurrentModule.NamedGlobal(storageName); !placeholder.IsNil() {
			// An OpAddr may have needed the code address before this function
			// reached the compile queue. Opaque pointers let the provisional
			// global be replaced by the correctly typed function definition.
			placeholder.ReplaceAllUsesWith(fn)
			placeholder.EraseFromParentAsGlobal()
		}
		fn.SetName(storageName)
	} else if got := fn.GlobalValueType(); got != sig.Type {
		if fn.BasicBlocksCount() != 0 {
			base.Fatalf("conflicting LLVM function type for definition %s", name)
		}
		// Compiler data can refer to an ABI function before AuxCall exposes
		// its exact signature. Replace that provisional declaration now.
		replacement := llvm.AddFunction(CurrentModule, storageName+".goallc.final", sig.Type)
		fn.ReplaceAllUsesWith(replacement)
		fn.EraseFromParentAsFunction()
		replacement.SetName(storageName)
		fn = replacement
	}
	configureLLVMFunction(fn, sig, cc)
	return fn
}

func getOrInsertLLVMIntrinsic(name string, typ llvm.Type) llvm.Value {
	fn := CurrentModule.NamedFunction(name)
	if fn.IsNil() {
		return llvm.AddFunction(CurrentModule, name, typ)
	}
	if got := fn.GlobalValueType(); got != typ {
		base.Fatalf("conflicting LLVM intrinsic type for %s", name)
	}
	return fn
}

func getLLVMIntrinsicDeclaration(name string, overloadTypes ...llvm.Type) llvm.Value {
	id := llvm.LookupIntrinsicID(name)
	if id == 0 {
		base.Fatalf("unknown LLVM intrinsic %s", name)
	}
	return llvm.GetIntrinsicDeclaration(CurrentModule, id, overloadTypes)
}

func (lfc *LLVMFuncContext) llvmLifetimeStart(slot llvmStackSlot) {
	// Lifetime markers describe compiler-owned local allocations. ABI-defined
	// parameter/result frame addresses, such as llvm.go.abi0.frame-derived
	// slots, are owned by the caller and remain live for the whole activation.
	if slot.Value.IsAAllocaInst().IsNil() {
		return
	}
	sig := llvm.FunctionType(
		GlobalCtxt.VoidType(),
		[]llvm.Type{GlobalCtxt.PointerType(0)},
		false,
	)
	fn := getOrInsertLLVMIntrinsic("llvm.lifetime.start.p0", sig)
	lfc.b.CreateCall(sig, fn, []llvm.Value{slot.Value}, "")
}

func (lfc *LLVMFuncContext) llvmKeepAlive(value llvm.Value) {
	// An operand bundle is a real SSA use that follows the call through
	// inlining. llvm.donothing survives long enough for the statepoint pass to
	// consume that liveness, then code generation emits no instruction for it.
	fn := getLLVMIntrinsicDeclaration("llvm.donothing")
	bundle := llvm.NewOperandBundle("go.keepalive", []llvm.Value{value})
	lfc.b.CreateCallWithOperandBundles(fn.GlobalValueType(), fn, nil, []llvm.OperandBundle{bundle}, "")
	bundle.Dispose()
}

func llvmStoreDeferResultHome(value llvm.Value, home llvmStackSlot, before llvm.Value) {
	b := GlobalCtxt.NewBuilder()
	defer b.Dispose()
	b.SetInsertPointBefore(before)
	store := b.CreateStore(value, home.Value)
	store.SetAlignment(int(home.Type.Alignment()))
}

func (lfc *LLVMFuncContext) emitDeferResultHomeStores() {
	for _, bb := range lfc.F.Blocks {
		for _, v := range bb.Values {
			key, hasHome := lfc.DeferResultKeys[v.ID]
			if !hasHome {
				continue
			}
			value, ok := lfc.Vs[v.ID]
			if !ok || value.IsNil() {
				v.Fatalf("named defer result address has no LLVM value")
			}
			if value.Type().TypeKind() != llvm.PointerTypeKind {
				v.Fatalf("named defer result address has non-pointer LLVM type")
			}
			if !value.IsAConstant().IsNil() {
				v.Fatalf("heap output parameter address unexpectedly lowered to an LLVM constant")
			}
			home, ok := lfc.Locals[key]
			if !ok {
				v.Fatalf("named defer result has no LLVM memory home")
			}
			if value.Type() != getLLVMType(home.Type) {
				v.Fatalf("named defer result home changes LLVM representation")
			}

			instruction := value.IsAInstruction()
			if instruction.IsNil() {
				v.Fatalf("named defer result has unsupported LLVM value kind")
			}
			before := llvm.NextInstruction(instruction)
			if !instruction.IsAPHINode().IsNil() {
				before = instruction.InstructionParent().FirstInstruction()
				for !before.IsNil() && !before.IsAPHINode().IsNil() {
					before = llvm.NextInstruction(before)
				}
			}
			if before.IsNil() {
				v.Fatalf("named defer result has no insertion point after its LLVM definition")
			}
			llvmStoreDeferResultHome(value, home, before)
		}
	}
}

func (lfc *LLVMFuncContext) llvmUnaryIntrinsic(v *Value, name string) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	want := getLLVMType(v.Type)
	if x.Type() != want {
		v.Fatalf("%s has incompatible LLVM operand and result types", v.Op)
	}
	sig := llvm.FunctionType(want, []llvm.Type{want}, false)
	fn := getOrInsertLLVMIntrinsic(name, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x}, v.String())
}

func (lfc *LLVMFuncContext) llvmSaturatingFloatToInt(v *Value, signed bool) llvm.Value {
	// Go leaves an out-of-range conversion implementation-specific, but it must
	// still produce an integer value. LLVM's plain fptosi/fptoui produce poison
	// instead, so give the LLVM backend the defined saturating behavior used by
	// Go's conversion stress tests and by the native arm64 instructions.
	x := lfc.GenLV(v.Args[0])
	resultType := getLLVMType(v.Type)
	if resultType.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM result", v.Op)
	}

	sourceSuffix := ""
	switch x.Type().TypeKind() {
	case llvm.FloatTypeKind:
		sourceSuffix = "f32"
	case llvm.DoubleTypeKind:
		sourceSuffix = "f64"
	default:
		v.Fatalf("%s has a non-floating-point LLVM operand", v.Op)
	}

	name := "llvm.fptoui.sat"
	if signed {
		name = "llvm.fptosi.sat"
	}
	name += fmt.Sprintf(".i%d.%s", resultType.IntTypeWidth(), sourceSuffix)
	sig := llvm.FunctionType(resultType, []llvm.Type{x.Type()}, false)
	fn := getOrInsertLLVMIntrinsic(name, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x}, v.String())
}

func (lfc *LLVMFuncContext) llvmBinaryIntrinsic(v *Value, name string) llvm.Value {
	x, y := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	want := getLLVMType(v.Type)
	if x.Type() != want || y.Type() != want {
		v.Fatalf("%s has incompatible LLVM operand and result types", v.Op)
	}
	sig := llvm.FunctionType(want, []llvm.Type{want, want}, false)
	fn := getOrInsertLLVMIntrinsic(name, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x, y}, v.String())
}

func (lfc *LLVMFuncContext) llvmTernaryIntrinsic(v *Value, name string) llvm.Value {
	x, y, z := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), lfc.GenLV(v.Args[2])
	want := getLLVMType(v.Type)
	if x.Type() != want || y.Type() != want || z.Type() != want {
		v.Fatalf("%s has incompatible LLVM operand and result types", v.Op)
	}
	sig := llvm.FunctionType(want, []llvm.Type{want, want, want}, false)
	fn := getOrInsertLLVMIntrinsic(name, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x, y, z}, v.String())
}

func (lfc *LLVMFuncContext) llvmRoundIntrinsic(v *Value, genericName string, amd64Mode uint64) llvm.Value {
	if lfc.F.Config.arch != "amd64" || buildcfg.GOAMD64 >= 2 {
		return lfc.llvmUnaryIntrinsic(v, genericName)
	}

	// At GOAMD64=v1, Go SSA places these operations behind
	// runtime.x86HasSSE41. Preserve that path-sensitive contract in the
	// intrinsic instead of enabling SSE4.1 for the whole function. At v2 and
	// above, the function target-cpu guarantees SSE4.1 and the generic LLVM
	// intrinsic exposes the usual optimization opportunities.
	x := lfc.GenLV(v.Args[0])
	want := getLLVMType(v.Type)
	if x.Type() != want || want.TypeKind() != llvm.DoubleTypeKind {
		v.Fatalf("%s has incompatible LLVM operand and result types", v.Op)
	}
	i32 := GlobalCtxt.Int32Type()
	sig := llvm.FunctionType(want, []llvm.Type{want, i32}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.x86.go.sse41.round.f64", sig)
	mode := llvm.ConstInt(i32, amd64Mode, false)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x, mode}, v.String())
}

func (lfc *LLVMFuncContext) llvmFMA(v *Value) llvm.Value {
	if lfc.F.Config.arch != "amd64" || buildcfg.GOAMD64 >= 3 {
		return lfc.llvmTernaryIntrinsic(v, "llvm.fma.f64")
	}

	// At GOAMD64=v1 and v2, the AMD64 SSA control flow has already guarded this
	// operation with runtime.x86HasFMA. GOAMD64=v3 and above instead use the
	// generic intrinsic under a function target-cpu that guarantees FMA.
	return lfc.llvmTernaryIntrinsic(v, "llvm.x86.go.fma.f64")
}

// llvmContractionMul recognizes a product that the native target rules may
// contract with its parent. ARM64 first reduces these immediate negation forms
// to FMUL/FNMUL; AMD64 passes allowNeg=false because its contraction rule only
// matches MUL directly.
func llvmContractionMul(v *Value, mulOp, negOp Op, allowNeg bool) *Value {
	if allowNeg && v.Op == negOp {
		v = v.Args[0]
	}
	if v.Op != mulOp {
		return nil
	}
	return v
}

func setLLVMContract(v llvm.Value) {
	// LLVM's fast-math C API accepts only instructions. IRBuilder may fold a
	// floating-point operation to a constant, which needs no contraction flag.
	if v.IsAInstruction().IsNil() {
		return
	}
	v.SetFastMathFlags(llvm.FastMathAllowContract)
}

// allowLLVMContraction mirrors the native target rewrite eligibility but
// leaves the contraction choice to LLVM. Only the matched multiply and its
// Add/Sub root receive contract; no other fast-math flags are enabled.
//
// Round32F and Round64F, conversions, calls, and other explicit SSA values do
// not match a product here and therefore remain rounding boundaries.
func (lfc *LLVMFuncContext) allowLLVMContraction(v *Value, result llvm.Value) {
	var mulOp, negOp Op
	switch v.Op {
	case OpAdd32F, OpSub32F:
		mulOp, negOp = OpMul32F, OpNeg32F
	case OpAdd64F, OpSub64F:
		mulOp, negOp = OpMul64F, OpNeg64F
	default:
		return
	}

	var products []*Value
	switch lfc.F.Config.arch {
	case "arm64":
		// ARM64.rules contracts Add/Sub with FMUL/FNMUL for f32 and f64.
		for _, arg := range v.Args {
			if product := llvmContractionMul(arg, mulOp, negOp, true); product != nil {
				products = append(products, product)
			}
		}
	case "amd64":
		// AMD64.rules contracts only Add with MUL at GOAMD64 v3 and above.
		if buildcfg.GOAMD64 < 3 || (v.Op != OpAdd32F && v.Op != OpAdd64F) {
			return
		}
		for _, arg := range v.Args {
			if product := llvmContractionMul(arg, mulOp, negOp, false); product != nil {
				products = append(products, product)
			}
		}
	default:
		return
	}
	if len(products) == 0 || !v.Block.Func.useFMA(v) {
		return
	}

	setLLVMContract(result)
	for _, product := range products {
		setLLVMContract(lfc.GenLV(product))
	}
}

func llvmTargetCPU(arch string, goamd64 int) string {
	if arch != "amd64" {
		return ""
	}
	switch goamd64 {
	case 1:
		return "x86-64"
	case 2, 3, 4:
		return fmt.Sprintf("x86-64-v%d", goamd64)
	default:
		base.Fatalf("LLVM target CPU is not configured for GOAMD64=v%d", goamd64)
		return ""
	}
}

func (lfc *LLVMFuncContext) buildPureTuple(v *Value, values ...llvm.Value) llvm.Value {
	resultType := getLLVMType(v.Type)
	if resultType.TypeKind() != llvm.StructTypeKind {
		v.Fatalf("%s has a non-struct LLVM result", v.Op)
	}
	fields := resultType.StructElementTypes()
	if len(fields) != len(values) {
		v.Fatalf("%s has %d LLVM result fields for %d values", v.Op, len(fields), len(values))
	}
	result := llvm.Undef(resultType)
	for i, value := range values {
		if value.Type() != fields[i] {
			v.Fatalf("%s result field %d has incompatible LLVM type", v.Op, i)
		}
		result = lfc.b.CreateInsertValue(result, value, i, "")
	}
	result.SetName(v.String())
	return result
}

func (lfc *LLVMFuncContext) carryBorrow(v *Value, intrinsic string) llvm.Value {
	x, y, carryIn := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), lfc.GenLV(v.Args[2])
	i64 := GlobalCtxt.Int64Type()
	if x.Type() != i64 || y.Type() != i64 || carryIn.Type() != i64 {
		v.Fatalf("%s requires three i64 operands", v.Op)
	}

	i1 := GlobalCtxt.Int1Type()
	overflowType := llvm.StructType([]llvm.Type{i64, i1}, false)
	sig := llvm.FunctionType(overflowType, []llvm.Type{i64, i64}, false)
	fn := getOrInsertLLVMIntrinsic(intrinsic, sig)
	first := lfc.b.CreateCall(sig, fn, []llvm.Value{x, y}, v.String()+".first")
	value := lfc.b.CreateExtractValue(first, 0, v.String()+".value1")
	overflow1 := lfc.b.CreateExtractValue(first, 1, v.String()+".overflow1")
	second := lfc.b.CreateCall(sig, fn, []llvm.Value{value, carryIn}, v.String()+".second")
	value = lfc.b.CreateExtractValue(second, 0, v.String()+".value")
	overflow2 := lfc.b.CreateExtractValue(second, 1, v.String()+".overflow2")
	overflow := lfc.b.CreateOr(overflow1, overflow2, v.String()+".overflow")
	carry := lfc.b.CreateZExt(overflow, i64, v.String()+".carry")

	return lfc.buildPureTuple(v, value, carry)
}

func (lfc *LLVMFuncContext) unsignedMul64HiLo(v *Value) llvm.Value {
	x, y := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	i64 := GlobalCtxt.Int64Type()
	if x.Type() != i64 || y.Type() != i64 {
		v.Fatalf("%s requires two i64 operands", v.Op)
	}
	i128 := GlobalCtxt.IntType(128)
	x = lfc.b.CreateZExt(x, i128, v.String()+".x")
	y = lfc.b.CreateZExt(y, i128, v.String()+".y")
	product := lfc.b.CreateMul(x, y, v.String()+".wide")
	low := lfc.b.CreateTrunc(product, i64, v.String()+".low")
	highWide := lfc.b.CreateLShr(product, llvm.ConstInt(i128, 64, false), v.String()+".high.wide")
	high := lfc.b.CreateTrunc(highWide, i64, v.String()+".high")
	return lfc.buildPureTuple(v, high, low)
}

func (lfc *LLVMFuncContext) unsignedMulOver(v *Value) llvm.Value {
	x, y := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	if x.Type() != y.Type() || x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has incompatible LLVM operand types", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}

	i1 := GlobalCtxt.Int1Type()
	overflowType := llvm.StructType([]llvm.Type{x.Type(), i1}, false)
	sig := llvm.FunctionType(overflowType, []llvm.Type{x.Type(), x.Type()}, false)
	fn := getOrInsertLLVMIntrinsic(fmt.Sprintf("llvm.umul.with.overflow.i%d", bits), sig)
	result := lfc.b.CreateCall(sig, fn, []llvm.Value{x, y}, v.String())
	product := lfc.b.CreateExtractValue(result, 0, v.String()+".product")
	overflow := lfc.b.CreateExtractValue(result, 1, v.String()+".overflow.i1")
	overflow = lfc.b.CreateZExt(overflow, getLLVMType(v.Type.FieldType(1)), v.String()+".overflow")
	return lfc.buildPureTuple(v, product, overflow)
}

func (lfc *LLVMFuncContext) currentG(v *Value) llvm.Value {
	abi := lfc.F.OwnAux.ABI().Which()
	register, ok := llvmCurrentGRegister(lfc.F.Config.arch, abi)
	if !ok {
		if lfc.F.Config.arch == "amd64" && abi != obj.ABIInternal {
			v.Fatalf("GetG is unsupported for LLVM amd64 ABI %v; ABI0 must load g from TLS", abi)
		}
		v.Fatalf("GetG is unsupported for LLVM target %s", lfc.F.Config.arch)
	}

	i64 := GlobalCtxt.Int64Type()
	// llvm.read_register requires a metadata node whose first operand is the
	// register-name string. A bare MDString passes IR verification but crashes
	// SelectionDAG's intrinsic lowering when it casts the operand to MDNode.
	registerName := GlobalCtxt.MetadataAsValue(GlobalCtxt.MDNode([]llvm.Metadata{
		GlobalCtxt.MDString(register),
	}))
	sig := llvm.FunctionType(i64, []llvm.Type{registerName.Type()}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.read_register.i64", sig)
	raw := lfc.b.CreateCall(sig, fn, []llvm.Value{registerName}, v.String()+".register")
	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.PointerTypeKind {
		v.Fatalf("GetG has non-pointer LLVM result type")
	}
	return lfc.b.CreateIntToPtr(raw, want, v.String())
}

func llvmCurrentGRegister(arch string, abi obj.ABI) (string, bool) {
	switch arch {
	case "amd64":
		// The native amd64 backend only treats R14 as g under ABIInternal.
		// ABI0 loads g from TLS and explicitly repairs R14 at ABI crossings;
		// LLVM does not model those transitions yet, so fail closed.
		if abi != obj.ABIInternal {
			return "", false
		}
		return "r14", true
	case "arm64":
		return "x28", true
	default:
		return "", false
	}
}

func (lfc *LLVMFuncContext) publicationBarrier(v *Value) llvm.Value {
	if len(v.Args) != 1 || !v.Type.IsMemory() || !v.Args[0].Type.IsMemory() {
		v.Fatalf("PubBarrier has an invalid memory operand")
	}
	lfc.GenLV(v.Args[0])
	switch lfc.F.Config.arch {
	case "amd64":
		// Stores are already ordered on x86. A release fence is a compiler
		// barrier and lowers without a machine instruction on this target.
		return lfc.b.CreateFence(llvm.AtomicOrderingRelease, false, "")
	case "arm64":
		i32 := GlobalCtxt.Int32Type()
		sig := llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{i32}, false)
		fn := getOrInsertLLVMIntrinsic("llvm.aarch64.dmb", sig)
		// Match the native ARM64 lowering's DMB ST option exactly. A generic
		// LLVM release fence lowers to the stronger DMB ISH and loses this
		// target-level publication-barrier contract.
		return lfc.b.CreateCall(sig, fn, []llvm.Value{llvm.ConstInt(i32, 0xe, false)}, "")
	default:
		v.Fatalf("PubBarrier is unsupported for LLVM target %s", lfc.F.Config.arch)
		return llvm.Value{}
	}
}

func (lfc *LLVMFuncContext) prefetch(v *Value, locality uint64) llvm.Value {
	if len(v.Args) != 2 || !v.Type.IsMemory() || !v.Args[1].Type.IsMemory() {
		v.Fatalf("%s has invalid address or memory operands", v.Op)
	}
	address := lfc.llvmAddressPointer(v, lfc.GenLV(v.Args[0]), v.Args[0].Type, v.String()+".address")
	// Force the incoming Go memory dependency before emitting the side effect.
	lfc.GenLV(v.Args[1])

	i32 := GlobalCtxt.Int32Type()
	fn := getLLVMIntrinsicDeclaration("llvm.prefetch", address.Type())
	return lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{
		address,
		llvm.ConstInt(i32, 0, false),        // read
		llvm.ConstInt(i32, locality, false), // 3 = keep, 0 = non-temporal
		llvm.ConstInt(i32, 1, false),        // data cache
	}, "")
}

func (lfc *LLVMFuncContext) unsignedDiv128By64(v *Value) llvm.Value {
	hi, lo, divisor := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), lfc.GenLV(v.Args[2])
	i64 := GlobalCtxt.Int64Type()
	if hi.Type() != i64 || lo.Type() != i64 || divisor.Type() != i64 {
		v.Fatalf("%s requires three i64 operands", v.Op)
	}
	if lfc.F.Config.arch != "amd64" {
		v.Fatalf("%s is unsupported for LLVM target %s", v.Op, lfc.F.Config.arch)
	}

	resultType := getLLVMType(v.Type)
	sig := llvm.FunctionType(resultType, []llvm.Type{i64, i64, i64}, false)
	// Keep the exact 128-by-64 operation visible to the X86 backend. A generic
	// i128 udiv would instead legalize to the hosted __udivti3 compiler-runtime
	// call, which GoObj deliberately does not provide.
	fn := getOrInsertLLVMIntrinsic("llvm.x86.go.udivrem.i128.i64", sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{hi, lo, divisor}, v.String())
}

func llvmTupleFieldCount(typ *types.Type) int {
	switch typ.Kind() {
	case types.TTUPLE:
		return 2
	case types.TRESULTS:
		return typ.NumFields()
	default:
		base.Fatalf("LLVM tuple field count requested for %v", typ)
		return 0
	}
}

func (lfc *LLVMFuncContext) selectPureTuple(v, src *Value, sel int) llvm.Value {
	if src.Type.Kind() != types.TTUPLE && src.Type.Kind() != types.TRESULTS {
		v.Fatalf("%s selects unsupported tuple source %s", v.Op, src.Op)
	}
	fields := llvmTupleFieldCount(src.Type)
	for i := 0; i < fields; i++ {
		if src.Type.FieldType(i).IsMemory() {
			v.Fatalf("%s selects unsupported memory tuple source %s", v.Op, src.Op)
		}
	}
	if sel < 0 || sel >= fields {
		v.Fatalf("%s selects tuple field %d from %d fields", v.Op, sel, fields)
	}
	tuple := lfc.GenLV(src)
	if tuple.Type().TypeKind() != llvm.StructTypeKind || len(tuple.Type().StructElementTypes()) != fields {
		v.Fatalf("%s source %s has an incompatible LLVM tuple", v.Op, src.Op)
	}
	result := lfc.b.CreateExtractValue(tuple, sel, v.String()+".raw")
	if result.Type() == getLLVMType(v.Type) {
		result.SetName(v.String())
		return result
	}

	// The MulUintptr intrinsic deliberately gives Mul{32,64}uover a
	// (uint, uint) carrier tuple, while the Select1 produced for the source
	// function's second result has Go type bool. Native backends turn the
	// overflow flag into that bool while lowering the select. Do the same here
	// rather than letting the carrier type escape into subsequent bool phis.
	if sel == 1 && (src.Op == OpMul32uover || src.Op == OpMul64uover) && v.Type.IsBoolean() && result.Type().TypeKind() == llvm.IntegerTypeKind {
		return lfc.goBool(lfc.llvmCondition(result, v.String()+".i1"), v.String())
	}
	v.Fatalf("%s source %s field %d has LLVM kind %v, want %v for Go type %v", v.Op, src.Op, sel, result.Type().TypeKind(), getLLVMType(v.Type).TypeKind(), v.Type)
	return llvm.Value{}
}

func (lfc *LLVMFuncContext) bitLen(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	if x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM operand", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}

	i1 := GlobalCtxt.Int1Type()
	sig := llvm.FunctionType(x.Type(), []llvm.Type{x.Type(), i1}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.ctlz.i"+fmt.Sprint(bits), sig)
	leading := lfc.b.CreateCall(sig, fn, []llvm.Value{
		x,
		llvm.ConstInt(i1, 0, false), // ctlz(0, false) is the operand width.
	}, v.String()+".leading")
	length := lfc.b.CreateSub(llvm.ConstInt(x.Type(), uint64(bits), false), leading, v.String()+".width")

	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM result", v.Op)
	}
	switch {
	case bits < want.IntTypeWidth():
		return lfc.b.CreateZExt(length, want, v.String())
	case bits > want.IntTypeWidth():
		return lfc.b.CreateTrunc(length, want, v.String())
	default:
		length.SetName(v.String())
		return length
	}
}

func (lfc *LLVMFuncContext) trailingZeros(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	if x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM operand", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}

	i1 := GlobalCtxt.Int1Type()
	sig := llvm.FunctionType(x.Type(), []llvm.Type{x.Type(), i1}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.cttz.i"+fmt.Sprint(bits), sig)
	isZeroPoison := v.Op == OpCtz8NonZero || v.Op == OpCtz16NonZero ||
		v.Op == OpCtz32NonZero || v.Op == OpCtz64NonZero
	zeroPoisonFlag := uint64(0)
	if isZeroPoison {
		zeroPoisonFlag = 1
	}
	trailing := lfc.b.CreateCall(sig, fn, []llvm.Value{
		x,
		llvm.ConstInt(i1, zeroPoisonFlag, false),
	}, v.String()+".trailing")

	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM result", v.Op)
	}
	switch {
	case bits < want.IntTypeWidth():
		return lfc.b.CreateZExt(trailing, want, v.String())
	case bits > want.IntTypeWidth():
		return lfc.b.CreateTrunc(trailing, want, v.String())
	default:
		trailing.SetName(v.String())
		return trailing
	}
}

func (lfc *LLVMFuncContext) populationCount(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	if x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM operand", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 8 && bits != 16 && bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}

	sig := llvm.FunctionType(x.Type(), []llvm.Type{x.Type()}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.ctpop.i"+fmt.Sprint(bits), sig)
	count := lfc.b.CreateCall(sig, fn, []llvm.Value{x}, v.String()+".count")
	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM result", v.Op)
	}
	switch {
	case bits < want.IntTypeWidth():
		return lfc.b.CreateZExt(count, want, v.String())
	case bits > want.IntTypeWidth():
		return lfc.b.CreateTrunc(count, want, v.String())
	default:
		count.SetName(v.String())
		return count
	}
}

func (lfc *LLVMFuncContext) byteSwap(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	if x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM operand", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 16 && bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}
	sig := llvm.FunctionType(x.Type(), []llvm.Type{x.Type()}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.bswap.i"+fmt.Sprint(bits), sig)
	result := lfc.b.CreateCall(sig, fn, []llvm.Value{x}, v.String()+".swapped")
	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has a non-integer LLVM result", v.Op)
	}
	switch {
	case bits < want.IntTypeWidth():
		return lfc.b.CreateZExt(result, want, v.String())
	case bits > want.IntTypeWidth():
		return lfc.b.CreateTrunc(result, want, v.String())
	default:
		result.SetName(v.String())
		return result
	}
}

func (lfc *LLVMFuncContext) callerPC(v *Value) llvm.Value {
	// LLVM's caller-state intrinsics do not themselves prevent inlining. Go
	// already made its inlining decision before this point, so preserve that
	// established frame boundary through the LLVM pipeline.
	lfc.LF.AddFunctionAttr(llvmNoInlineAttribute())

	ptr := GlobalCtxt.PointerType(0)
	i32 := GlobalCtxt.Int32Type()
	fn := getLLVMIntrinsicDeclaration("llvm.returnaddress", ptr)
	pc := lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{llvm.ConstInt(i32, 0, false)}, v.String()+".ptr")
	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind || want.IntTypeWidth() != int(lfc.F.Config.PtrSize*8) {
		v.Fatalf("GetCallerPC has incompatible LLVM result type")
	}
	return lfc.b.CreatePtrToInt(pc, want, v.String())
}

func (lfc *LLVMFuncContext) callerSP(v *Value) llvm.Value {
	lfc.LF.AddFunctionAttr(llvmNoInlineAttribute())

	ptr := GlobalCtxt.PointerType(0)
	var sp llvm.Value
	switch lfc.F.Config.arch {
	case "arm64":
		// AArch64 implements sponentry as a fixed frame object at offset zero,
		// which remains the entry SP after fixed or dynamically chosen frames.
		fn := getLLVMIntrinsicDeclaration("llvm.sponentry", ptr)
		sp = lfc.b.CreateCall(fn.GlobalValueType(), fn, nil, v.String()+".ptr")
	case "amd64", "386":
		// On x86 the call instruction stores the return PC immediately below
		// the caller's SP. addressofreturnaddress is frame-layout aware, unlike
		// reading SP and adding the current frame size.
		fn := getLLVMIntrinsicDeclaration("llvm.addressofreturnaddress", ptr)
		returnSlot := lfc.b.CreateCall(fn.GlobalValueType(), fn, nil, v.String()+".returnslot")
		offset := llvm.ConstInt(getLLVMType(types.Types[types.TUINTPTR]), uint64(lfc.F.Config.PtrSize), false)
		sp = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), returnSlot, []llvm.Value{offset}, v.String()+".ptr")
	default:
		v.Fatalf("GetCallerSP is unsupported for LLVM target %s", lfc.F.Config.arch)
	}

	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind || want.IntTypeWidth() != int(lfc.F.Config.PtrSize*8) {
		v.Fatalf("GetCallerSP has incompatible LLVM result type")
	}
	return lfc.observePointerAddress(sp, want, v.String())
}

func (lfc *LLVMFuncContext) stackAddress(v *Value) llvm.Value {
	ptr := GlobalCtxt.PointerType(0)
	sig := llvm.FunctionType(ptr, nil, false)
	fn := getOrInsertLLVMIntrinsic("llvm.stackaddress.p0", sig)
	sp := lfc.b.CreateCall(sig, fn, nil, v.String()+".ptr")
	want := getLLVMType(v.Type)
	if want.TypeKind() != llvm.IntegerTypeKind || want.IntTypeWidth() != int(lfc.F.Config.PtrSize*8) {
		v.Fatalf("SP has incompatible LLVM result type")
	}
	return lfc.observePointerAddress(sp, want, v.String())
}

func (lfc *LLVMFuncContext) observePointerAddress(pointer llvm.Value, resultType llvm.Type, name string) llvm.Value {
	// Go stack pointers can change when a call grows the goroutine stack. Keep
	// the physical-address observation ordered through LLVM optimization; the
	// statepoint plugin lowers it to ptrtoint immediately before it computes
	// pointer liveness and relocation SSA.
	fn := getLLVMIntrinsicDeclaration(
		goPointerAddressObservation, resultType, pointer.Type())
	sig := fn.GlobalValueType()
	return lfc.b.CreateCall(sig, fn, []llvm.Value{pointer}, name)
}

func (lfc *LLVMFuncContext) pointerFromAddress(address llvm.Value, resultType llvm.Type, name string) llvm.Value {
	// Keep a pointer reconstructed from uintptr visible through LLVM
	// optimization. The statepoint plugin lowers this to inttoptr immediately
	// before computing pointer liveness, so a stack-growing call can relocate
	// the reconstructed pointer when it remains live across the call.
	fn := getLLVMIntrinsicDeclaration(
		goPointerFromAddress, resultType, address.Type())
	sig := fn.GlobalValueType()
	return lfc.b.CreateCall(sig, fn, []llvm.Value{address}, name)
}

// materializeAddressPointer distinguishes unmanaged addresses from integers
// that may reconstruct movable Go pointers. Not-in-heap addresses are stable
// across stack movement, so emit a marked plain inttoptr that statepoint
// liveness excludes. Other address conversions retain the frontend marker so
// their pointer identity and relocation semantics survive LLVM optimization.
func (lfc *LLVMFuncContext) materializeAddressPointer(address llvm.Value, addressType *types.Type, resultType llvm.Type, name string) llvm.Value {
	if llvmNotInHeapPointer(addressType) {
		pointer := lfc.b.CreateIntToPtr(address, resultType, name)
		// IRBuilder constant-folds a nil integer address into a pointer constant;
		// constants never participate in statepoint liveness and cannot carry
		// instruction metadata.
		if !pointer.IsAInstruction().IsNil() {
			pointer.SetMetadata(GlobalCtxt.MDKindID(goNotInHeapAddressMD), GlobalCtxt.MDNode(nil))
		}
		return pointer
	}
	return lfc.pointerFromAddress(address, resultType, name)
}

// llvmAddressPointer materializes an integer address only at its terminal
// memory use. In particular, a pointer to a not-in-heap object remains an
// integer while it is live across calls, so stack copying cannot mistake it
// for a movable Go stack pointer.
func (lfc *LLVMFuncContext) llvmAddressPointer(v *Value, address llvm.Value, addressType *types.Type, name string) llvm.Value {
	switch address.Type().TypeKind() {
	case llvm.PointerTypeKind:
		return address
	case llvm.IntegerTypeKind:
		if address.Type().IntTypeWidth() != types.PtrSize*8 {
			v.Fatalf("%s integer address has width %d, want %d", v.Op, address.Type().IntTypeWidth(), types.PtrSize*8)
		}
		return lfc.materializeAddressPointer(address, addressType, GlobalCtxt.PointerType(0), name)
	default:
		v.Fatalf("%s address has unsupported LLVM type %s", v.Op, address.Type())
		return llvm.Value{}
	}
}

func (lfc *LLVMFuncContext) cgoUnsafeArgAddress(name *ir.Name, llvmName string) llvm.Value {
	if lfc.F.OwnAux.ABI().Which() != obj.ABI0 {
		lfc.F.fe.Fatalf(name.Pos(), "cgo unsafe argument frame requires ABI0")
	}

	if lfc.ABI0FrameBase.IsNil() {
		fn := getLLVMIntrinsicDeclaration("llvm.go.abi0.frame")
		lfc.ABI0FrameBase = lfc.b.CreateCall(fn.GlobalValueType(), fn, nil, "abi0.frame")
	}
	offset := name.FrameOffset()
	if offset < 0 {
		lfc.F.fe.Fatalf(name.Pos(), "cgo unsafe argument %v has negative ABI0 frame offset %d", name, offset)
	}
	if offset == 0 {
		return lfc.ABI0FrameBase
	}
	index := llvmConstInt(types.Types[types.TUINTPTR], offset)
	return lfc.b.CreateGEP(GlobalCtxt.Int8Type(), lfc.ABI0FrameBase, []llvm.Value{index}, llvmName+".frame")
}

func markLLVMGCLeafCall(call llvm.Value) {
	attr := GlobalCtxt.CreateStringAttribute(goGCLeafFunctionAttr, "")
	call.AddCallSiteAttribute(llvmAttributeFunctionIndex, attr)
}

func llvmMemoryOpInfo(v *Value) (int64, int) {
	t, ok := v.Aux.(*types.Type)
	if !ok || t == nil {
		v.Fatalf("%s has no memory type", v.Op)
	}
	// LLVM lowering runs after the complete writebarrier pass. At this point a
	// pointer-containing heap Zero/Move is the raw memory operation that follows
	// its wbZero/wbMove helper; stack destinations need no helper. The unexpanded
	// OpZeroWB/OpMoveWB forms remain unsupported by GenLV and therefore fail
	// closed rather than bypassing the write barrier.
	size := auxIntToInt64(v.AuxInt)
	if size < 0 {
		v.Fatalf("%s has negative size %d", v.Op, size)
	}
	align := t.Alignment()
	if align <= 0 || align&(align-1) != 0 {
		v.Fatalf("%s has invalid alignment %d for %v", v.Op, align, t)
	}
	return size, int(align)
}

func (lfc *LLVMFuncContext) llvmMemoryLength(v *Value, size int64) llvm.Value {
	t := getLLVMType(types.Types[types.TUINTPTR])
	bits := t.IntTypeWidth()
	if bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported uintptr width %d", v.Op, bits)
	}
	if bits == 32 && uint64(size) > uint64(^uint32(0)) {
		v.Fatalf("%s size %d overflows uintptr", v.Op, size)
	}
	return llvm.ConstInt(t, uint64(size), false)
}

func (lfc *LLVMFuncContext) llvmMemoryPointer(v *Value, arg int) llvm.Value {
	return lfc.llvmAddressPointer(v, lfc.GenLV(v.Args[arg]), v.Args[arg].Type, fmt.Sprintf("%s.arg%d.address", v, arg))
}

func (lfc *LLVMFuncContext) isDeferResultAddress(v *Value) bool {
	for v != nil {
		switch v.Op {
		case OpLocalAddr:
			_, key := llvmLocalName(v)
			return lfc.DeferResults[key]
		case OpOffPtr, OpAddPtr, OpPtrIndex, OpCopy:
			if len(v.Args) == 0 {
				return false
			}
			v = v.Args[0]
		default:
			return false
		}
	}
	return false
}

func (lfc *LLVMFuncContext) isOpenDeferAddress(v *Value) bool {
	for v != nil {
		switch v.Op {
		case OpLocalAddr:
			_, key := llvmLocalName(v)
			return (lfc.HasOpenDeferBits && key == lfc.OpenDeferBits) ||
				lfc.OpenDeferSlots[key] != 0
		case OpOffPtr, OpAddPtr, OpPtrIndex, OpCopy:
			if len(v.Args) == 0 {
				return false
			}
			v = v.Args[0]
		default:
			return false
		}
	}
	return false
}

func (lfc *LLVMFuncContext) llvmZero(v *Value) llvm.Value {
	size, align := llvmMemoryOpInfo(v)
	dst := lfc.llvmMemoryPointer(v, 0)
	length := lfc.llvmMemoryLength(v, size)
	volatile := uint64(0)
	if lfc.isDeferResultAddress(v.Args[0]) || lfc.isOpenDeferAddress(v.Args[0]) {
		volatile = 1
	}
	sig := llvm.FunctionType(
		GlobalCtxt.VoidType(),
		[]llvm.Type{dst.Type(), GlobalCtxt.Int8Type(), length.Type(), GlobalCtxt.Int1Type()},
		false,
	)
	name := "llvm.memset.inline.p0.i64"
	if length.Type().IntTypeWidth() == 32 {
		name = "llvm.memset.inline.p0.i32"
	}
	fn := getOrInsertLLVMIntrinsic(name, sig)
	call := lfc.b.CreateCall(sig, fn, []llvm.Value{
		dst,
		llvm.ConstInt(GlobalCtxt.Int8Type(), 0, false),
		length,
		llvm.ConstInt(GlobalCtxt.Int1Type(), volatile, false),
	}, "")
	call.SetInstrParamAlignment(1, align)
	return call
}

func (lfc *LLVMFuncContext) llvmRuntimeMemmove(dst, src, length llvm.Value) llvm.Value {
	sig := llvmFuncSignature{
		Type: llvm.FunctionType(
			GlobalCtxt.VoidType(),
			[]llvm.Type{dst.Type(), src.Type(), length.Type()},
			false,
		),
		ReturnType:          GlobalCtxt.VoidType(),
		ClosureContextIndex: -1,
	}
	fn := getOrInsertLLVMABISymbolRef("runtime.memmove", obj.ABIInternal, sig, goABIInternalCallConv)
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{dst, src, length}, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	markLLVMGCLeafCall(call)
	return call
}

func (lfc *LLVMFuncContext) llvmCopyFixedMemory(dst, src llvm.Value, size int64, align int) llvm.Value {
	lengthType := getLLVMType(types.Types[types.TUINTPTR])
	length := llvm.ConstInt(lengthType, uint64(size), false)
	if size > llvmInlineMemmoveLimit {
		return lfc.llvmRuntimeMemmove(dst, src, length)
	}
	sig := llvm.FunctionType(
		GlobalCtxt.VoidType(),
		[]llvm.Type{dst.Type(), src.Type(), length.Type(), GlobalCtxt.Int1Type()},
		false,
	)
	name := "llvm.memmove.p0.p0.i64"
	if length.Type().IntTypeWidth() == 32 {
		name = "llvm.memmove.p0.p0.i32"
	}
	fn := getOrInsertLLVMIntrinsic(name, sig)
	call := lfc.b.CreateCall(sig, fn, []llvm.Value{
		dst,
		src,
		length,
		llvm.ConstInt(GlobalCtxt.Int1Type(), 0, false),
	}, "")
	call.SetInstrParamAlignment(1, align)
	call.SetInstrParamAlignment(2, align)
	return call
}

func (lfc *LLVMFuncContext) llvmMove(v *Value) llvm.Value {
	size, align := llvmMemoryOpInfo(v)
	dst := lfc.llvmMemoryPointer(v, 0)
	src := lfc.llvmMemoryPointer(v, 1)
	length := lfc.llvmMemoryLength(v, size)
	if size > llvmInlineMemmoveLimit {
		return lfc.llvmRuntimeMemmove(dst, src, length)
	}

	sig := llvm.FunctionType(
		GlobalCtxt.VoidType(),
		[]llvm.Type{dst.Type(), src.Type(), length.Type(), GlobalCtxt.Int1Type()},
		false,
	)
	name := "llvm.memmove.p0.p0.i64"
	if length.Type().IntTypeWidth() == 32 {
		name = "llvm.memmove.p0.p0.i32"
	}
	fn := getOrInsertLLVMIntrinsic(name, sig)
	volatile := uint64(0)
	if lfc.isDeferResultAddress(v.Args[0]) || lfc.isOpenDeferAddress(v.Args[0]) {
		volatile = 1
	}
	call := lfc.b.CreateCall(sig, fn, []llvm.Value{
		dst,
		src,
		length,
		llvm.ConstInt(GlobalCtxt.Int1Type(), volatile, false),
	}, "")
	call.SetInstrParamAlignment(1, align)
	call.SetInstrParamAlignment(2, align)
	return call
}

func (lfc *LLVMFuncContext) llvmMemEq(v *Value) llvm.Value {
	left := lfc.llvmMemoryPointer(v, 0)
	right := lfc.llvmMemoryPointer(v, 1)
	size := lfc.GenLV(v.Args[2])
	uintptrType := getLLVMType(types.Types[types.TUINTPTR])
	if size.Type() != uintptrType {
		v.Fatalf("MemEq size has incompatible LLVM type")
	}
	boolType := getLLVMType(types.Types[types.TBOOL])
	if getLLVMType(v.Type) != boolType {
		v.Fatalf("MemEq result has incompatible LLVM type")
	}
	sig := llvmFuncSignature{
		Type:                llvm.FunctionType(boolType, []llvm.Type{left.Type(), right.Type(), uintptrType}, false),
		ReturnType:          boolType,
		ResultCount:         1,
		ReturnCount:         1,
		ClosureContextIndex: -1,
	}
	fn := getOrInsertLLVMABISymbolRef("runtime.memequal", obj.ABIInternal, sig, goABIInternalCallConv)
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{left, right, size}, v.String())
	call.SetInstructionCallConv(goABIInternalCallConv)
	markLLVMGCLeafCall(call)
	return call
}

func (lfc *LLVMFuncContext) llvmSlicemask(v *Value) llvm.Value {
	if v.Type != types.Types[types.TINT] {
		v.Fatalf("Slicemask has non-native-int type %v", v.Type)
	}
	x := lfc.GenLV(v.Args[0])
	t := getLLVMType(v.Type)
	if x.Type() != t || t.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("Slicemask has incompatible LLVM operand type")
	}
	bits := t.IntTypeWidth()
	if bits != int(lfc.F.Config.PtrSize*8) {
		v.Fatalf("Slicemask has width %d, want native int width %d", bits, lfc.F.Config.PtrSize*8)
	}
	neg := lfc.b.CreateSub(llvm.ConstInt(t, 0, false), x, v.String()+".neg")
	shift := llvm.ConstInt(t, uint64(bits-1), false)
	return lfc.b.CreateAShr(neg, shift, v.String())
}

func (lfc *LLVMFuncContext) goBool(cond llvm.Value, name string) llvm.Value {
	if cond.Type().IntTypeWidth() == getLLVMType(types.Types[types.TBOOL]).IntTypeWidth() {
		cond.SetName(name)
		return cond
	}
	return lfc.b.CreateZExt(cond, getLLVMType(types.Types[types.TBOOL]), name)
}

func (lfc *LLVMFuncContext) llvmCondition(v llvm.Value, name string) llvm.Value {
	if v.Type().IntTypeWidth() == 1 {
		return v
	}
	zero := llvm.ConstInt(v.Type(), 0, false)
	return lfc.b.CreateICmp(llvm.IntNE, v, zero, name)
}

func llvmConstInt(t *types.Type, value int64) llvm.Value {
	bits := uint64(t.Size() * 8)
	raw := uint64(value)
	if bits < 64 {
		raw &= uint64(1)<<bits - 1
	}
	return llvm.ConstInt(getLLVMType(t), raw, false)
}

func (lfc *LLVMFuncContext) pointerComparisonOperands(v *Value) (llvm.Value, llvm.Value) {
	return lfc.normalizePointerComparisonOperands(v, lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]))
}

func (lfc *LLVMFuncContext) normalizePointerComparisonOperands(v *Value, x, y llvm.Value) (llvm.Value, llvm.Value) {
	if x.Type() == y.Type() {
		return x, y
	}
	switch {
	case x.Type().TypeKind() == llvm.PointerTypeKind && y.Type().TypeKind() == llvm.IntegerTypeKind:
		x = lfc.b.CreatePtrToInt(x, y.Type(), v.String()+".ptr")
	case x.Type().TypeKind() == llvm.IntegerTypeKind && y.Type().TypeKind() == llvm.PointerTypeKind:
		y = lfc.b.CreatePtrToInt(y, x.Type(), v.String()+".ptr")
	default:
		v.Fatalf("pointer comparison has incompatible LLVM operand types")
	}
	return x, y
}

type llvmShiftKind uint8

const (
	llvmShiftLeft llvmShiftKind = iota
	llvmShiftRightSigned
	llvmShiftRightUnsigned
)

func (lfc *LLVMFuncContext) shift(v *Value, kind llvmShiftKind) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	count := lfc.GenLV(v.Args[1])
	xBits := x.Type().IntTypeWidth()
	countBits := count.Type().IntTypeWidth()

	var normalized llvm.Value
	switch {
	case countBits < xBits:
		normalized = lfc.b.CreateZExt(count, x.Type(), v.String()+".count")
	case countBits > xBits:
		normalized = lfc.b.CreateTrunc(count, x.Type(), v.String()+".count")
	default:
		normalized = count
	}

	var shifted llvm.Value
	switch kind {
	case llvmShiftLeft:
		shifted = lfc.b.CreateShl(x, normalized, v.String()+".shift")
	case llvmShiftRightSigned:
		shifted = lfc.b.CreateAShr(x, normalized, v.String()+".shift")
	case llvmShiftRightUnsigned:
		shifted = lfc.b.CreateLShr(x, normalized, v.String()+".shift")
	}
	result := shifted
	if !auxIntToBool(v.AuxInt) {
		width := llvm.ConstInt(count.Type(), uint64(xBits), false)
		inRange := lfc.b.CreateICmp(llvm.IntULT, count, width, v.String()+".inrange")
		var outOfRange llvm.Value
		if kind == llvmShiftRightSigned {
			lastBit := llvm.ConstInt(x.Type(), uint64(xBits-1), false)
			outOfRange = lfc.b.CreateAShr(x, lastBit, v.String()+".sign")
		} else {
			outOfRange = llvm.ConstNull(x.Type())
		}
		result = lfc.b.CreateSelect(inRange, shifted, outOfRange, v.String()+".selected")
	}
	want := getLLVMType(v.Type)
	if result.Type() != want {
		if result.Type().TypeKind() != llvm.IntegerTypeKind || want.TypeKind() != llvm.IntegerTypeKind ||
			result.Type().IntTypeWidth() < want.IntTypeWidth() {
			v.Fatalf("%s changes LLVM representation", v.Op)
		}
		return lfc.b.CreateTrunc(result, want, v.String())
	}
	result.SetName(v.String())
	return result
}

func (lfc *LLVMFuncContext) rotateLeft(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	count := lfc.GenLV(v.Args[1])
	if x.Type().TypeKind() != llvm.IntegerTypeKind || count.Type().TypeKind() != llvm.IntegerTypeKind || x.Type() != getLLVMType(v.Type) {
		v.Fatalf("%s has incompatible LLVM operand types", v.Op)
	}
	switch {
	case count.Type().IntTypeWidth() < x.Type().IntTypeWidth():
		count = lfc.b.CreateZExt(count, x.Type(), v.String()+".count")
	case count.Type().IntTypeWidth() > x.Type().IntTypeWidth():
		count = lfc.b.CreateTrunc(count, x.Type(), v.String()+".count")
	}
	sig := llvm.FunctionType(x.Type(), []llvm.Type{x.Type(), x.Type(), x.Type()}, false)
	fn := getOrInsertLLVMIntrinsic("llvm.fshl.i"+fmt.Sprint(x.Type().IntTypeWidth()), sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x, x, count}, v.String())
}

func (lfc *LLVMFuncContext) integerDiv(v *Value, signed, remainder bool) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	y := lfc.GenLV(v.Args[1])
	safeY := y
	if signed {
		bits := x.Type().IntTypeWidth()
		min := llvm.ConstInt(x.Type(), uint64(1)<<(bits-1), false)
		minusOne := llvm.ConstAllOnes(x.Type())
		isMin := lfc.b.CreateICmp(llvm.IntEQ, x, min, v.String()+".ismin")
		isMinusOne := lfc.b.CreateICmp(llvm.IntEQ, y, minusOne, v.String()+".isminusone")
		overflow := lfc.b.CreateAnd(isMin, isMinusOne, v.String()+".overflow")
		// LLVM makes signed min/-1 poison. Go defines the quotient as min
		// and the remainder as zero; using 1 as the divisor on that path
		// produces exactly those values.
		safeY = lfc.b.CreateSelect(overflow, llvm.ConstInt(y.Type(), 1, false), y, v.String()+".divisor")
	}
	switch {
	case signed && remainder:
		return lfc.b.CreateSRem(x, safeY, v.String())
	case signed:
		return lfc.b.CreateSDiv(x, safeY, v.String())
	case remainder:
		return lfc.b.CreateURem(x, safeY, v.String())
	default:
		return lfc.b.CreateUDiv(x, safeY, v.String())
	}
}

func (lfc *LLVMFuncContext) highMultiply(v *Value, signed bool) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	y := lfc.GenLV(v.Args[1])
	if x.Type() != y.Type() || x.Type().TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s has incompatible LLVM operand types", v.Op)
	}
	bits := x.Type().IntTypeWidth()
	if bits != 32 && bits != 64 {
		v.Fatalf("%s has unsupported operand width %d", v.Op, bits)
	}
	wide := GlobalCtxt.IntType(bits * 2)
	if signed {
		x = lfc.b.CreateSExt(x, wide, v.String()+".x")
		y = lfc.b.CreateSExt(y, wide, v.String()+".y")
	} else {
		x = lfc.b.CreateZExt(x, wide, v.String()+".x")
		y = lfc.b.CreateZExt(y, wide, v.String()+".y")
	}
	product := lfc.b.CreateMul(x, y, v.String()+".wide")
	high := lfc.b.CreateLShr(product, llvm.ConstInt(wide, uint64(bits), false), v.String()+".high")
	return lfc.b.CreateTrunc(high, getLLVMType(v.Type), v.String())
}

func (lfc *LLVMFuncContext) unsignedAverage(v *Value) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	y := lfc.GenLV(v.Args[1])
	if x.Type() != y.Type() || x.Type().TypeKind() != llvm.IntegerTypeKind || x.Type() != getLLVMType(v.Type) {
		v.Fatalf("%s has incompatible LLVM operand types", v.Op)
	}
	difference := lfc.b.CreateSub(x, y, v.String()+".difference")
	half := lfc.b.CreateLShr(difference, llvm.ConstInt(x.Type(), 1, false), v.String()+".half")
	return lfc.b.CreateAdd(half, y, v.String())
}

func llvmAMD64ByteVectorType() llvm.Type {
	return llvm.VectorType(GlobalCtxt.Int8Type(), 16)
}

func llvmVectorShuffleMask(indices ...uint64) llvm.Value {
	elements := make([]llvm.Value, len(indices))
	for i, index := range indices {
		elements[i] = llvm.ConstInt(GlobalCtxt.Int32Type(), index, false)
	}
	return llvm.ConstVector(elements, false)
}

// amd64MoveQIntToFP lowers MOVQi2f as the bitwise register transfer that the
// native backend emits, rather than as an integer-to-floating-point
// conversion. The map-group intrinsics deliberately give this operation the
// pseudo-type int128: MOVQ initializes the low 64 bits of the XMM register and
// clears its high 64 bits, producing the byte vector consumed by the following
// packed operations.
func (lfc *LLVMFuncContext) amd64MoveQIntToFP(v *Value) llvm.Value {
	src := lfc.GenLV(v.Args[0])
	if src.Type() != GlobalCtxt.Int64Type() {
		v.Fatalf("%s source has LLVM type %v, want i64", v.Op, src.Type())
	}

	want := getLLVMType(v.Type)
	if want == GlobalCtxt.DoubleType() {
		return lfc.b.CreateBitCast(src, want, v.String())
	}
	if want != llvmAMD64ByteVectorType() {
		v.Fatalf("%s result has LLVM type %v, want double or <16 x i8>", v.Op, want)
	}

	lowBytes := lfc.b.CreateBitCast(src, llvm.VectorType(GlobalCtxt.Int8Type(), 8), v.String()+".low")
	zeroBytes := llvm.ConstNull(lowBytes.Type())
	return lfc.b.CreateShuffleVector(
		lowBytes,
		zeroBytes,
		llvmVectorShuffleMask(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
		v.String(),
	)
}

func (lfc *LLVMFuncContext) amd64BroadcastByte(v *Value) llvm.Value {
	src := lfc.GenLV(v.Args[0])
	want := llvmAMD64ByteVectorType()
	if getLLVMType(v.Type) != want {
		v.Fatalf("%s result has LLVM type %v, want <16 x i8>", v.Op, getLLVMType(v.Type))
	}

	var byte0 llvm.Value
	switch src.Type() {
	case GlobalCtxt.Int64Type():
		byte0 = lfc.b.CreateTrunc(src, GlobalCtxt.Int8Type(), v.String()+".byte")
	case want:
		byte0 = lfc.b.CreateExtractElement(src, llvm.ConstInt(GlobalCtxt.Int32Type(), 0, false), v.String()+".byte")
	default:
		v.Fatalf("%s source has LLVM type %v, want i64 or <16 x i8>", v.Op, src.Type())
	}

	seed := lfc.b.CreateInsertElement(
		llvm.Undef(want),
		byte0,
		llvm.ConstInt(GlobalCtxt.Int32Type(), 0, false),
		v.String()+".seed",
	)
	return lfc.b.CreateShuffleVector(
		seed,
		llvm.Undef(want),
		llvmVectorShuffleMask(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0),
		v.String(),
	)
}

func (lfc *LLVMFuncContext) amd64UnpackLowBytes(v *Value) llvm.Value {
	x, y := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	want := llvmAMD64ByteVectorType()
	if x.Type() != want || y.Type() != want || getLLVMType(v.Type) != want {
		v.Fatalf("%s requires <16 x i8> operands and result", v.Op)
	}
	return lfc.b.CreateShuffleVector(
		x,
		y,
		llvmVectorShuffleMask(0, 16, 1, 17, 2, 18, 3, 19, 4, 20, 5, 21, 6, 22, 7, 23),
		v.String(),
	)
}

func (lfc *LLVMFuncContext) amd64ShuffleLowWords(v *Value) llvm.Value {
	src := lfc.GenLV(v.Args[0])
	want := llvmAMD64ByteVectorType()
	if src.Type() != want || getLLVMType(v.Type) != want {
		v.Fatalf("%s requires a <16 x i8> operand and result", v.Op)
	}
	wordsType := llvm.VectorType(GlobalCtxt.Int16Type(), 8)
	words := lfc.b.CreateBitCast(src, wordsType, v.String()+".words")
	control := uint8(auxIntToInt64(v.AuxInt))
	shuffled := lfc.b.CreateShuffleVector(
		words,
		llvm.Undef(wordsType),
		llvmVectorShuffleMask(
			uint64(control&3),
			uint64((control>>2)&3),
			uint64((control>>4)&3),
			uint64((control>>6)&3),
			4, 5, 6, 7,
		),
		v.String()+".words.shuffled",
	)
	return lfc.b.CreateBitCast(shuffled, want, v.String())
}

func (lfc *LLVMFuncContext) amd64SignBytes(v *Value) llvm.Value {
	x, signs := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	want := llvmAMD64ByteVectorType()
	if x.Type() != want || signs.Type() != want || getLLVMType(v.Type) != want {
		v.Fatalf("%s requires <16 x i8> operands and result", v.Op)
	}
	zero := llvm.ConstNull(want)
	positive := lfc.b.CreateICmp(llvm.IntSGT, signs, zero, v.String()+".positive")
	negative := lfc.b.CreateICmp(llvm.IntSLT, signs, zero, v.String()+".negative")
	negated := lfc.b.CreateSub(zero, x, v.String()+".negated")
	nonPositive := lfc.b.CreateSelect(negative, negated, zero, v.String()+".nonpositive")
	return lfc.b.CreateSelect(positive, x, nonPositive, v.String())
}

func (lfc *LLVMFuncContext) amd64CompareEqualBytes(v *Value) llvm.Value {
	x, y := lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1])
	want := llvmAMD64ByteVectorType()
	if x.Type() != want || y.Type() != want || getLLVMType(v.Type) != want {
		v.Fatalf("%s requires <16 x i8> operands and result", v.Op)
	}
	equal := lfc.b.CreateICmp(llvm.IntEQ, x, y, v.String()+".equal")
	return lfc.b.CreateSExt(equal, want, v.String())
}

func (lfc *LLVMFuncContext) amd64MoveByteMask(v *Value) llvm.Value {
	src := lfc.GenLV(v.Args[0])
	if src.Type() != llvmAMD64ByteVectorType() {
		v.Fatalf("%s source has LLVM type %v, want <16 x i8>", v.Op, src.Type())
	}
	resultType := getLLVMType(v.Type)
	if resultType.TypeKind() != llvm.IntegerTypeKind {
		v.Fatalf("%s result has non-integer LLVM type %v", v.Op, resultType)
	}
	switch resultType.IntTypeWidth() {
	case 8, 16, 32, 64:
	default:
		v.Fatalf("%s result has unsupported LLVM integer width %d", v.Op, resultType.IntTypeWidth())
	}

	// PMOVMSKB always forms a 16-bit mask from all XMM byte lanes. Use LLVM's
	// exact SSE2 intrinsic so instruction selection retains that operation
	// rather than synthesizing a scalar lane-reduction sequence. The intrinsic
	// returns i32, with the complete mask in bits 0..15. Some Go intrinsics
	// deliberately assign the SSA operation uint8 and thereby keep only the low
	// eight lane bits; make that truncation explicit below.
	fn := getLLVMIntrinsicDeclaration("llvm.x86.sse2.pmovmskb.128")
	maskType := GlobalCtxt.Int32Type()
	if got := fn.GlobalValueType(); got != llvm.FunctionType(maskType, []llvm.Type{llvmAMD64ByteVectorType()}, false) {
		v.Fatalf("%s intrinsic has unexpected LLVM type %v", v.Op, got)
	}
	result := lfc.b.CreateCall(fn.GlobalValueType(), fn, []llvm.Value{src}, v.String()+".mask")
	switch {
	case resultType.IntTypeWidth() < 32:
		result = lfc.b.CreateTrunc(result, resultType, v.String()+".trunc")
	case resultType.IntTypeWidth() > 32:
		result = lfc.b.CreateZExt(result, resultType, v.String()+".wide")
	}
	result.SetName(v.String())
	return result
}

func (lfc *LLVMFuncContext) FinishPhi() {
	savedBlock := lfc.b.GetInsertBlock()
	savedLocation := lfc.b.CurrentDebugLocationMetadata()
	defer func() {
		lfc.b.SetCurrentDebugLocationMetadata(savedLocation)
		if !savedBlock.IsNil() {
			lfc.b.SetInsertPointAtEnd(savedBlock)
		}
	}()

	for _, BB := range lfc.F.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpPhi {
				continue
			}
			if v.Type.IsMemory() {
				continue
			}
			if len(v.Args) != len(BB.Preds) {
				v.Fatalf("phi has %d inputs for %d predecessors", len(v.Args), len(BB.Preds))
			}
			incomingLVals := make([]llvm.Value, 0, len(v.Args))
			predecessors := make([]llvm.BasicBlock, 0, len(BB.Preds))
			for i, incoming := range v.Args {
				incomingLVal, ok := lfc.Vs[incoming.ID]
				if !ok {
					v.Fatalf("phi input %s was not emitted in its defining block", incoming)
				}
				if incomingLVal.IsNil() {
					v.Fatalf("phi input %s produced no LLVM value", incoming.LongString())
				}
				predecessor := lfc.BBs[BB.Preds[i].Block().ID]
				terminator := predecessor.LastInstruction()
				if terminator.IsNil() {
					v.Fatalf("phi predecessor %s has no LLVM terminator", BB.Preds[i].Block())
				}
				lfc.b.SetInsertPointBefore(terminator)
				lfc.setDebugLocation(v.Pos)
				incomingLVal = lfc.reshapeLLVMValue(v, incomingLVal, incoming.Type, v.Type, v.String()+".incoming")
				if got, want := incomingLVal.Type(), lfc.Vs[v.ID].Type(); got != want {
					v.Fatalf("phi input %s has LLVM kind %s, want %s", incoming.LongString(), got.TypeKind(), want.TypeKind())
				}
				incomingLVals = append(incomingLVals, incomingLVal)
				predecessors = append(predecessors, predecessor)
			}
			lfc.Vs[v.ID].AddIncoming(incomingLVals, predecessors)
		}
	}
}

func (lfc *LLVMFuncContext) paramForArg(v *Value) llvm.Value {
	param, typ := lfc.paramForArgNameAndType(v.Aux.(*ir.Name))
	return lfc.llvmValueFromABI(v, param, typ, v.Type, v.String()+".arg")
}

func (lfc *LLVMFuncContext) paramForArgNameAndType(name *ir.Name) (llvm.Value, *types.Type) {
	key := llvmLocalKeyForName(name)
	for i, param := range lfc.F.OwnAux.ABIInfo().InParams() {
		if param.Name != nil && llvmLocalKeyForName(param.Name) == key {
			value := lfc.LF.Param(i)
			if lfc.Params[i].ByVal {
				value = lfc.b.CreateLoad(lfc.Params[i].ValueType, value, name.Sym().Name+".byval")
				value.SetAlignment(lfc.Params[i].Alignment)
			}
			return value, lfc.F.OwnAux.TypeOfArg(int64(i))
		}
	}
	lfc.F.fe.Fatalf(name.Pos(), "could not find LLVM parameter for %v", name)
	return llvm.Value{}, nil
}

func (lfc *LLVMFuncContext) llvmByValCallArgument(v, argValue *Value, index int, logical *types.Type, param llvmParamSignature) llvm.Value {
	if !param.ByVal || logical.Size() == 0 {
		v.Fatalf("argument %d is not a non-empty byval parameter", index)
	}

	// Preserve an existing Go memory source when no memory effect separates its
	// load from the call. Ordinary LLVM byval lowering will copy these bytes
	// into the callee's Go ABI stack home; manufacturing an intermediate
	// aggregate value and a second alloca would only obscure that source from
	// ordinary memcpy forwarding. A different memory state requires the SSA
	// value path below so later argument evaluation cannot change the snapshot.
	if types.Identical(argValue.Type, logical) &&
		(argValue.Op == OpLoad || argValue.Op == OpDereference) && len(argValue.Args) != 0 &&
		argValue.MemoryArg() == v.MemoryArg() {
		address := lfc.GenLV(argValue.Args[0])
		if address.Type().TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("byval argument %d has non-pointer source address", index)
		}
		return address
	}

	value := lfc.GenLV(argValue)
	value = lfc.llvmValueToABI(v, value, argValue.Type, logical, param.ValueType, fmt.Sprintf("%s.arg%d", v, index))
	if value.Type() != param.ValueType {
		v.Fatalf("byval argument %d has incompatible LLVM value type", index)
	}

	// LLVM byval takes an address. A pure SSA value therefore needs a canonical
	// source object in IR. It is intentionally an ordinary alloca-backed byval
	// source: generic DAG combines may fold individual copies, while ordinary
	// byval copy semantics remain the correctness path.
	entryBuilder := GlobalCtxt.NewBuilder()
	defer entryBuilder.Dispose()
	entry := lfc.LF.EntryBasicBlock()
	if first := entry.FirstInstruction(); first.IsNil() {
		entryBuilder.SetInsertPointAtEnd(entry)
	} else {
		entryBuilder.SetInsertPointBefore(first)
	}
	address := entryBuilder.CreateAlloca(param.ValueType, fmt.Sprintf("%s.arg%d.byval", v, index))
	address.SetAlignment(param.Alignment)
	store := lfc.b.CreateStore(value, address)
	store.SetAlignment(param.Alignment)
	return address
}

func (lfc *LLVMFuncContext) llvmMemoryResultCallArguments(v *Value, sig llvmFuncSignature, aux *AuxCall) []llvm.Value {
	args := make([]llvm.Value, 0, sig.ResultCount-sig.ReturnCount)
	for index, result := range sig.Results {
		if !result.InMemory {
			continue
		}
		slot, ok := lfc.CallResultSlots[llvmCallResultKey{Call: v.ID, Index: int64(index)}]
		if !ok || !types.Identical(slot.Type, aux.TypeOfResult(int64(index))) {
			v.Fatalf("memory result %d has no compatible caller-owned home", index)
		}
		// The call writes this object. Start its source lifetime before the
		// statepoint; the statepoint pass zero-initializes pointer words that are
		// visible to the caller stack map before the callee has produced them.
		if slot.Type.HasPointers() {
			lfc.llvmLifetimeStart(slot)
		}
		args = append(args, slot.Value)
	}
	return args
}

func (lfc *LLVMFuncContext) storeMemoryResult(v, value *Value, index int) {
	result := lfc.Results[index]
	if !result.InMemory || result.ParamIndex < 0 {
		v.Fatalf("result %d is not assigned to memory", index)
	}
	logical := lfc.F.OwnAux.TypeOfResult(int64(index))
	dst := lfc.LF.Param(result.ParamIndex)
	if types.Identical(value.Type, logical) &&
		(value.Op == OpLoad || value.Op == OpDereference) && len(value.Args) != 0 &&
		value.MemoryArg() == v.MemoryArg() {
		src := lfc.GenLV(value.Args[0])
		if src.Type().TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("memory result %d has a non-pointer source address", index)
		}
		if src.C != dst.C {
			lfc.llvmCopyFixedMemory(dst, src, logical.Size(), result.Alignment)
		}
		return
	}

	lVal := lfc.GenLV(value)
	lVal = lfc.llvmValueToABI(v, lVal, value.Type, logical, result.ValueType, fmt.Sprintf("%s.result%d", v, index))
	if lVal.Type() != result.ValueType {
		v.Fatalf("memory result %d has incompatible LLVM value type", index)
	}
	store := lfc.b.CreateStore(lVal, dst)
	store.SetAlignment(result.Alignment)
}

func (lfc *LLVMFuncContext) registerArgument(v *Value) llvm.Value {
	aux, ok := v.Aux.(*AuxNameOffset)
	if !ok || aux.Name == nil {
		v.Fatalf("%s has invalid argument name/offset auxiliary %T", v.Op, v.Aux)
	}
	key := llvmLocalKeyForName(aux.Name)
	slot, ok := lfc.Locals[key]
	if !ok {
		v.Fatalf("%s parameter %v has no LLVM memory home", v.Op, aux.Name)
	}
	if aux.Offset < 0 || v.Type.Size() < 0 || aux.Offset+v.Type.Size() > slot.Type.Size() {
		v.Fatalf("%s parameter piece [%d,%d) is outside memory home %v", v.Op, aux.Offset, aux.Offset+v.Type.Size(), slot.Type)
	}

	addr := slot.Value
	if aux.Offset != 0 {
		off := llvmConstInt(types.Types[types.TINT], aux.Offset)
		addr = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), addr, []llvm.Value{off}, v.String()+".addr")
	}
	value := lfc.b.CreateLoad(getLLVMType(v.Type), addr, v.String()+".home")
	value.SetAlignment(int(v.Type.Alignment()))
	return value
}

func (lfc *LLVMFuncContext) aggregate(v *Value, args []*Value) llvm.Value {
	result := llvm.Undef(getLLVMType(v.Type))
	var elementTypes []llvm.Type
	switch result.Type().TypeKind() {
	case llvm.StructTypeKind:
		elementTypes = result.Type().StructElementTypes()
	case llvm.ArrayTypeKind:
		elementTypes = make([]llvm.Type, len(args))
		for i := range elementTypes {
			elementTypes[i] = result.Type().ElementType()
		}
	default:
		v.Fatalf("%s has non-aggregate LLVM kind %s", v.Op, result.Type().TypeKind())
	}
	wantElements := len(args)
	if v.Type.Kind() == types.TSTRUCT && llvmStructHasTailPad(v.Type) {
		wantElements++
	}
	if len(elementTypes) != wantElements {
		v.Fatalf("%s has %d LLVM aggregate elements for %d SSA arguments", v.Op, len(elementTypes), len(args))
	}
	for i, arg := range args {
		value := lfc.GenLV(arg)
		if value.IsNil() {
			v.Fatalf("aggregate field %d from %s produced no LLVM value", i, arg.LongString())
		}
		if value.Type() != elementTypes[i] {
			var elementType *types.Type
			switch v.Type.Kind() {
			case types.TSTRUCT:
				elementType = v.Type.FieldType(i)
			case types.TARRAY:
				elementType = v.Type.Elem()
			case types.TINTER:
				// Go SSA represents an interface's itab/type word as uintptr,
				// but OpAddr may still expose a semantic pointer. Keep the
				// interface carrier integer-typed so statepoint liveness does
				// not mistake immutable runtime metadata for a movable GC root.
				value = lfc.reshapeLLVMValueToType(value, elementTypes[i], fmt.Sprintf("%s.field%d", v, i))
			default:
				v.Fatalf("aggregate field %d changes LLVM representation for Go type %v", i, v.Type)
			}
			if elementType != nil {
				value = lfc.reshapeLLVMValue(v, value, arg.Type, elementType, fmt.Sprintf("%s.field%d", v, i))
			}
		}
		if got, want := value.Type(), elementTypes[i]; got != want {
			v.Fatalf("aggregate field %d from %s has LLVM kind %s for Go type %v, want %s in Go aggregate %v", i, arg.LongString(), got.TypeKind(), arg.Type, want.TypeKind(), v.Type)
		}
		result = lfc.b.CreateInsertValue(result, value, i, "")
	}
	result.SetName(v.String())
	return result
}

type llvmDirectIfaceCarrier uint8

const (
	llvmDirectIfaceInvalid llvmDirectIfaceCarrier = iota
	llvmDirectIfaceEmpty
	llvmDirectIfaceHasPointer
)

// llvmDirectIfacePath finds the single pointer carrier in an LLVM aggregate.
// Direct-interface aggregates contain exactly one pointer leaf; every other
// element is a zero-sized struct or array. Derive the path from the LLVM type
// graph so this conversion is independent of the source aggregate shape.
func llvmDirectIfacePath(t llvm.Type) ([]int, llvmDirectIfaceCarrier) {
	switch t.TypeKind() {
	case llvm.PointerTypeKind:
		return nil, llvmDirectIfaceHasPointer
	case llvm.StructTypeKind:
		var path []int
		carrier := llvmDirectIfaceEmpty
		for i, element := range t.StructElementTypes() {
			elementPath, elementCarrier := llvmDirectIfacePath(element)
			switch elementCarrier {
			case llvmDirectIfaceEmpty:
				continue
			case llvmDirectIfaceHasPointer:
				if carrier == llvmDirectIfaceHasPointer {
					return nil, llvmDirectIfaceInvalid
				}
				path = append([]int{i}, elementPath...)
				carrier = llvmDirectIfaceHasPointer
			default:
				return nil, llvmDirectIfaceInvalid
			}
		}
		return path, carrier
	case llvm.ArrayTypeKind:
		if t.ArrayLength() == 0 {
			return nil, llvmDirectIfaceEmpty
		}
		elementPath, carrier := llvmDirectIfacePath(t.ElementType())
		switch carrier {
		case llvmDirectIfaceEmpty:
			return nil, llvmDirectIfaceEmpty
		case llvmDirectIfaceHasPointer:
			if t.ArrayLength() == 1 {
				return append([]int{0}, elementPath...), llvmDirectIfaceHasPointer
			}
		}
	}
	return nil, llvmDirectIfaceInvalid
}

func (lfc *LLVMFuncContext) insertLLVMValueAtPath(v *Value, value llvm.Value, target llvm.Type, path []int) llvm.Value {
	if len(path) == 0 {
		if value.Type() != target {
			v.Fatalf("direct interface pointer carrier has incompatible LLVM type")
		}
		return value
	}

	index := path[0]
	var element llvm.Type
	switch target.TypeKind() {
	case llvm.StructTypeKind:
		elements := target.StructElementTypes()
		if index < 0 || index >= len(elements) {
			v.Fatalf("direct interface struct carrier index %d is out of range", index)
		}
		element = elements[index]
	case llvm.ArrayTypeKind:
		if index < 0 || index >= target.ArrayLength() {
			v.Fatalf("direct interface array carrier index %d is out of range", index)
		}
		element = target.ElementType()
	default:
		v.Fatalf("direct interface carrier path enters non-aggregate LLVM type")
	}

	elementValue := lfc.insertLLVMValueAtPath(v, value, element, path[1:])
	return lfc.b.CreateInsertValue(llvm.Undef(target), elementValue, index, v.String()+".carrier")
}

func (lfc *LLVMFuncContext) llvmIData(v *Value) llvm.Value {
	data := lfc.b.CreateExtractValue(lfc.GenLV(v.Args[0]), 1, v.String()+".data")
	want := getLLVMType(v.Type)
	if data.Type() == want {
		// CreateExtractValue may constant-fold an IMake and return its global
		// data operand directly. Renaming that value would rename the referenced
		// Go symbol (for example runtime.zeroVal) to this SSA value's temporary
		// name and leave an undefined GoObj relocation.
		if !data.IsAInstruction().IsNil() {
			data.SetName(v.String())
		}
		return data
	}
	if !types.IsDirectIface(v.Type) {
		v.Fatalf("IData has LLVM pointer carrier for non-direct Go type %v", v.Type)
	}
	path, carrier := llvmDirectIfacePath(want)
	if carrier != llvmDirectIfaceHasPointer {
		v.Fatalf("direct Go interface type %v has no unique LLVM pointer carrier", v.Type)
	}
	result := lfc.insertLLVMValueAtPath(v, data, want, path)
	result.SetName(v.String())
	return result
}

// reshapeLLVMValue converts between LLVM representations after Go SSA has
// already established that the value can flow to the destination type. LLVM
// identified structs retain Go's named aggregate identity, so rebuild the
// first-class value when the source and destination names differ.
func (lfc *LLVMFuncContext) reshapeLLVMValue(v *Value, value llvm.Value, from, to *types.Type, name string) llvm.Value {
	if value.IsNil() {
		v.Fatalf("cannot reshape an empty LLVM value")
	}
	want := getLLVMType(to)
	if value.Type() == want {
		return value
	}
	// The soft-float pass represents every float32/float64 SSA value with the
	// same-width uint32/uint64 bit pattern. Function ABI types remain the Go
	// source types, so calls and entries must reinterpret the carrier without a
	// numeric conversion.
	if lfc.F.Config.SoftFloat && from != nil && to != nil && from.Size() == to.Size() &&
		((from.IsFloat() && to.IsInteger()) || (from.IsInteger() && to.IsFloat())) {
		fromKind, toKind := value.Type().TypeKind(), want.TypeKind()
		if (fromKind == llvm.FloatTypeKind || fromKind == llvm.DoubleTypeKind || fromKind == llvm.IntegerTypeKind) &&
			(toKind == llvm.FloatTypeKind || toKind == llvm.DoubleTypeKind || toKind == llvm.IntegerTypeKind) {
			return lfc.b.CreateBitCast(value, want, name)
		}
	}
	return lfc.reshapeLLVMValueToType(value, want, name)
}

// reshapeLLVMValueToType is the LLVM-type half of reshapeLLVMValue. Call
// boundaries also use it for the physical ABI carrier after selecting the
// callee's semantic signature. Both relationships come from Go SSA and ABI
// analysis, so this routine only performs the required reconstruction.
func (lfc *LLVMFuncContext) reshapeLLVMValueToType(value llvm.Value, target llvm.Type, name string) llvm.Value {
	if value.Type() == target {
		return value
	}

	source := value.Type()
	// Go ABI analysis may describe a promoted method receiver using its single
	// physical register carrier while the generated wrapper definition retains
	// the named aggregate receiver type. Peel and rebuild singleton aggregates
	// at that caller boundary. This keeps the callee signature semantic without
	// introducing an anonymous aggregate signature shared by both sides.
	if source.TypeKind() != target.TypeKind() {
		switch source.TypeKind() {
		case llvm.StructTypeKind:
			fields := source.StructElementTypes()
			if len(fields) == 1 {
				field := lfc.b.CreateExtractValue(value, 0, name+".abi.unwrap")
				return lfc.reshapeLLVMValueToType(field, target, name)
			}
		case llvm.ArrayTypeKind:
			if source.ArrayLength() == 1 {
				element := lfc.b.CreateExtractValue(value, 0, name+".abi.unwrap")
				return lfc.reshapeLLVMValueToType(element, target, name)
			}
		}

		switch target.TypeKind() {
		case llvm.StructTypeKind:
			fields := target.StructElementTypes()
			if len(fields) == 1 {
				field := lfc.reshapeLLVMValueToType(value, fields[0], name)
				return lfc.b.CreateInsertValue(llvm.Undef(target), field, 0, name+".abi.wrap")
			}
		case llvm.ArrayTypeKind:
			if target.ArrayLength() == 1 {
				element := lfc.reshapeLLVMValueToType(value, target.ElementType(), name)
				return lfc.b.CreateInsertValue(llvm.Undef(target), element, 0, name+".abi.wrap")
			}
		}
	}

	switch target.TypeKind() {
	case llvm.StructTypeKind:
		targetFields := target.StructElementTypes()
		result := llvm.Undef(target)
		for i := range targetFields {
			fieldName := fmt.Sprintf("%s.abi.field%d", name, i)
			field := lfc.b.CreateExtractValue(value, i, fieldName+".extract")
			field = lfc.reshapeLLVMValueToType(field, targetFields[i], fieldName)
			result = lfc.b.CreateInsertValue(result, field, i, fieldName+".insert")
		}
		return result

	case llvm.ArrayTypeKind:
		result := llvm.Undef(target)
		for i := 0; i < target.ArrayLength(); i++ {
			elementName := fmt.Sprintf("%s.abi.element%d", name, i)
			element := lfc.b.CreateExtractValue(value, i, elementName+".extract")
			element = lfc.reshapeLLVMValueToType(element, target.ElementType(), elementName)
			result = lfc.b.CreateInsertValue(result, element, i, elementName+".insert")
		}
		return result

	default:
		switch {
		case source.TypeKind() == llvm.PointerTypeKind && target.TypeKind() == llvm.IntegerTypeKind:
			return lfc.b.CreatePtrToInt(value, target, name)
		case source.TypeKind() == llvm.IntegerTypeKind && target.TypeKind() == llvm.PointerTypeKind:
			return lfc.b.CreateIntToPtr(value, target, name)
		default:
			return lfc.b.CreateBitCast(value, target, name)
		}
	}
}

func (lfc *LLVMFuncContext) llvmValueToABI(v *Value, value llvm.Value, from, logical *types.Type, abiType llvm.Type, name string) llvm.Value {
	if logical.Size() == 0 {
		if from.Size() != 0 || !llvmTypeContainsABIPad(abiType) {
			v.Fatalf("zero-sized Go ABI value has incompatible pad carrier")
		}
		return llvm.Undef(abiType)
	}
	value = lfc.reshapeLLVMValue(v, value, from, logical, name)
	return lfc.reshapeLLVMValueToType(value, abiType, name)
}

func (lfc *LLVMFuncContext) llvmValueFromABI(v *Value, value llvm.Value, logical, to *types.Type, name string) llvm.Value {
	if logical.Size() == 0 {
		if to.Size() != 0 || !llvmTypeContainsABIPad(value.Type()) {
			v.Fatalf("zero-sized Go ABI pad carrier has incompatible type")
		}
		return llvm.Undef(getLLVMType(to))
	}
	value = lfc.reshapeLLVMValueToType(value, getLLVMType(logical), name)
	return lfc.reshapeLLVMValue(v, value, logical, to, name)
}

// llvmStaticCallSignature restores semantic pointer types for compiler-built
// runtime calls whose AuxCall uses uintptr only to compute physical ABI
// assignments. When compiling runtime itself, an ordinary source call to the
// same helper already has its semantic pointer type and needs no rewrite.
// AuxCall remains the physical ABI authority in both cases; the LLVM operands
// and runtime helper parameters are pointers.
func llvmStaticCallSignature(aux *AuxCall, sig llvmFuncSignature) llvmFuncSignature {
	pointerArgs := int64(0)
	switch aux.Fn {
	case ir.Syms.Newproc, ir.Syms.Deferproc, ir.Syms.DeferprocStack:
		pointerArgs = 1
	case ir.Syms.Deferprocat:
		pointerArgs = 1
	case ir.Syms.WBZero:
		pointerArgs = 2
	case ir.Syms.WBMove:
		pointerArgs = 3
	default:
		return sig
	}
	params := append([]llvm.Type(nil), sig.Type.ParamTypes()...)
	for i := int64(0); i < pointerArgs; i++ {
		if aux.TypeOfArg(i).IsUintptr() {
			params[i] = GlobalCtxt.PointerType(0)
		}
	}
	sig.Type = llvm.FunctionType(sig.ReturnType, params, false)
	return sig
}

func (lfc *LLVMFuncContext) staticCall(v *Value) llvm.Value {
	aux := auxToCall(v.Aux)
	if aux == nil || aux.Fn == nil {
		v.Fatalf("static call has no target")
	}
	if got, want := len(v.Args)-1, int(aux.NArgs()); got != want {
		v.Fatalf("static call to %s has %d LLVM arguments, want %d", aux.Fn.Name, got, want)
	}

	sig := llvmStaticCallSignature(aux, llvmSignature(aux))
	cc := llvmCallConv(aux.ABI().Which())
	fn := getOrInsertLLVMFunctionRef(aux.Fn, sig, cc)
	// AMD64 rewrites some Move and Eq operations to static runtime calls before
	// LLVM emission. Keep the same leaf contract as the dedicated LLVM lowering
	// paths so RewriteStatepointsForGC does not turn these raw helpers into
	// statepoints.
	llvmGCLeaf := aux.Fn == ir.Syms.WBZero || aux.Fn == ir.Syms.WBMove ||
		aux.Fn == ir.Syms.Memmove || aux.Fn == ir.Syms.Memequal
	args := make([]llvm.Value, 0, len(sig.Type.ParamTypes()))
	for i := int64(0); i < aux.NArgs(); i++ {
		var arg llvm.Value
		if sig.Params[i].ByVal {
			arg = lfc.llvmByValCallArgument(v, v.Args[i], int(i), aux.TypeOfArg(i), sig.Params[i])
		} else {
			arg = lfc.GenLV(v.Args[i])
			if arg.Type() != sig.Type.ParamTypes()[i] {
				arg = lfc.llvmValueToABI(v, arg, v.Args[i].Type, aux.TypeOfArg(i), sig.Type.ParamTypes()[i], fmt.Sprintf("%s.arg%d", v, i))
			}
		}
		if got, want := arg.Type(), sig.Type.ParamTypes()[i]; got != want {
			v.Fatalf("argument %d to %s has incompatible LLVM type", i, aux.Fn.Name)
		}
		args = append(args, arg)
	}
	args = append(args, lfc.llvmMemoryResultCallArguments(v, sig, aux)...)
	name := v.String()
	if sig.ReturnCount == 0 {
		name = ""
	}
	call := lfc.b.CreateCall(sig.Type, fn, args, name)
	call.SetInstructionCallConv(cc)
	configureLLVMCall(call, sig)
	lfc.materializeAddressedResults(v, call, aux)
	if llvmGCLeaf {
		markLLVMGCLeafCall(call)
	}
	return call
}

func (lfc *LLVMFuncContext) indirectCall(v *Value, argStart int, closureContext bool) llvm.Value {
	aux := auxToCall(v.Aux)
	if aux == nil {
		v.Fatalf("indirect call has no ABI information")
	}
	if got, want := len(v.Args)-argStart-1, int(aux.NArgs()); got != want {
		v.Fatalf("indirect call has %d LLVM arguments, want %d", got, want)
	}

	sig := llvmSignature(aux)
	if closureContext {
		if argStart != 2 {
			v.Fatalf("closure call has invalid argument start %d", argStart)
		}
		if aux.ABI().Which() != obj.ABIInternal {
			v.Fatalf("closure call uses unsupported ABI %v", aux.ABI().Which())
		}
		sig = sig.withClosureContext()
	}
	cc := llvmCallConv(aux.ABI().Which())
	code := lfc.GenLV(v.Args[0])
	if code.Type().TypeKind() == llvm.IntegerTypeKind {
		code = lfc.b.CreateIntToPtr(code, GlobalCtxt.PointerType(0), v.String()+".code")
	}
	if code.Type().TypeKind() != llvm.PointerTypeKind {
		v.Fatalf("indirect callee has non-pointer LLVM type")
	}
	args := make([]llvm.Value, 0, len(sig.Type.ParamTypes()))
	for i := int64(0); i < aux.NArgs(); i++ {
		argValue := v.Args[argStart+int(i)]
		var arg llvm.Value
		if sig.Params[i].ByVal {
			arg = lfc.llvmByValCallArgument(v, argValue, int(i), aux.TypeOfArg(i), sig.Params[i])
		} else {
			arg = lfc.GenLV(argValue)
			if arg.Type() != sig.Type.ParamTypes()[i] {
				arg = lfc.llvmValueToABI(v, arg, argValue.Type, aux.TypeOfArg(i), sig.Type.ParamTypes()[i], fmt.Sprintf("%s.arg%d", v, i))
			}
		}
		if got, want := arg.Type(), sig.Type.ParamTypes()[i]; got != want {
			v.Fatalf("argument %d to indirect call has incompatible LLVM type", i)
		}
		args = append(args, arg)
	}
	args = append(args, lfc.llvmMemoryResultCallArguments(v, sig, aux)...)
	if closureContext {
		context := lfc.GenLV(v.Args[1])
		if context.Type().TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("closure context has non-pointer LLVM type")
		}
		args = append(args, context)
	}
	name := v.String()
	if sig.ReturnCount == 0 {
		name = ""
	}
	call := lfc.b.CreateCall(sig.Type, code, args, name)
	call.SetInstructionCallConv(cc)
	configureLLVMCall(call, sig)
	lfc.materializeAddressedResults(v, call, aux)
	if closureContext {
		call.AddCallSiteAttribute(sig.ClosureContextIndex+1, llvmNestAttribute())
	}
	return call
}

// materializeAddressedResults gives SelectNAddr a stable caller-owned memory
// home. Native expandCalls can address the physical outgoing result slot, but
// LLVM emission deliberately runs before that pass and models even stack ABI
// results as first-class return values. Store only the address-selected results
// immediately after the call, preserving the call result's Go type and
// alignment while allowing ordinary LLVM promotion to remove unnecessary
// homes.
func (lfc *LLVMFuncContext) materializeAddressedResults(v *Value, call llvm.Value, aux *AuxCall) {
	sig := llvmSignature(aux)
	for _, result := range lfc.AddressedResults[v.ID] {
		if result.Slot.Type.HasPointers() {
			lfc.llvmLifetimeStart(result.Slot)
		}
		resultSig := sig.Results[result.Index]
		if resultSig.InMemory || resultSig.ReturnIndex < 0 {
			result.Owner.Fatalf("addressed register result was assigned to memory")
		}
		value := call
		if sig.ReturnCount > 1 {
			value = lfc.b.CreateExtractValue(call, resultSig.ReturnIndex, result.Owner.String()+".value")
		}
		value = lfc.llvmValueFromABI(result.Owner, value, aux.TypeOfResult(result.Index), result.Slot.Type, result.Owner.String()+".reshape")
		if got, want := value.Type(), getLLVMType(result.Slot.Type); got != want {
			result.Owner.Fatalf("addressed call result has incompatible LLVM type")
		}
		store := lfc.b.CreateStore(value, result.Slot.Value)
		store.SetAlignment(int(result.Slot.Type.Alignment()))
	}
}

func llvmFunctionUsesClosureContext(f *Func) bool {
	usesContext := false
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op == OpGetClosurePtr {
				usesContext = true
			}
		}
	}
	needContext := f.OwnAux.Fn.NeedCtxt()
	// A live OpGetClosurePtr reads REGCTXT directly and therefore requires the
	// hidden LLVM nest parameter even for the intrinsic's own body. NeedCtxt is
	// still authoritative when early deadcode removes an otherwise unused op.
	hasContext := usesContext || needContext
	if hasContext && f.OwnAux.ABI().Which() != obj.ABIInternal {
		f.fe.Fatalf(f.Entry.Pos, "closure context on unsupported ABI %v for %s", f.OwnAux.ABI().Which(), f.Name)
	}
	return hasContext
}

func llvmLocalName(v *Value) (*ir.Name, llvmLocalKey) {
	sym := auxToSym(v.Aux)
	name, ok := sym.(*ir.Name)
	if !ok {
		v.Fatalf("local address has no stack symbol")
	}
	return name, llvmLocalKeyForName(name)
}

func llvmLocalKeyForName(name *ir.Name) llvmLocalKey {
	return llvmLocalKey{Sym: name.Sym(), Pos: name.Pos()}
}

var llvmBoundsPanicNames = [...]string{
	BoundsIndex:       "runtime.goPanicIndex",
	BoundsIndexU:      "runtime.goPanicIndexU",
	BoundsSliceAlen:   "runtime.goPanicSliceAlen",
	BoundsSliceAlenU:  "runtime.goPanicSliceAlenU",
	BoundsSliceAcap:   "runtime.goPanicSliceAcap",
	BoundsSliceAcapU:  "runtime.goPanicSliceAcapU",
	BoundsSliceB:      "runtime.goPanicSliceB",
	BoundsSliceBU:     "runtime.goPanicSliceBU",
	BoundsSlice3Alen:  "runtime.goPanicSlice3Alen",
	BoundsSlice3AlenU: "runtime.goPanicSlice3AlenU",
	BoundsSlice3Acap:  "runtime.goPanicSlice3Acap",
	BoundsSlice3AcapU: "runtime.goPanicSlice3AcapU",
	BoundsSlice3B:     "runtime.goPanicSlice3B",
	BoundsSlice3BU:    "runtime.goPanicSlice3BU",
	BoundsSlice3C:     "runtime.goPanicSlice3C",
	BoundsSlice3CU:    "runtime.goPanicSlice3CU",
	BoundsConvert:     "runtime.goPanicSliceConvert",
}

func (lfc *LLVMFuncContext) panicBounds(v *Value) llvm.Value {
	kind := BoundsKind(v.AuxInt)
	if kind >= BoundsKindCount {
		v.Fatalf("invalid bounds panic kind %d", kind)
	}
	x := lfc.GenLV(v.Args[0])
	y := lfc.GenLV(v.Args[1])
	sig := llvmFuncSignature{
		Type:       llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{x.Type(), y.Type()}, false),
		ReturnType: GlobalCtxt.VoidType(),
	}
	fn := getOrInsertLLVMABISymbolRef(llvmBoundsPanicNames[kind], obj.ABIInternal, sig, goABIInternalCallConv)
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{x, y}, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	return call
}

func (lfc *LLVMFuncContext) llvmWriteBarrier(v *Value) llvm.Value {
	entries := auxIntToInt64(v.AuxInt)
	if entries < 1 || entries > 8 {
		v.Fatalf("write barrier requests %d buffer entries", entries)
	}
	i32 := GlobalCtxt.Int32Type()
	sig := llvm.FunctionType(GlobalCtxt.PointerType(0), []llvm.Type{i32}, false)
	fn := getOrInsertLLVMIntrinsic(goWriteBarrierIntrinsic, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{llvm.ConstInt(i32, uint64(entries), false)}, v.String())
}

func (lfc *LLVMFuncContext) GenLV(v *Value) llvm.Value {
	if lv, ok := lfc.Vs[v.ID]; ok {
		return lv
	}
	savedBlock := lfc.b.GetInsertBlock()
	savedLocation := lfc.b.CurrentDebugLocationMetadata()
	if v.Op == OpSP {
		// CompileBlock leaves SP lazy because LocalAddr uses it only as a
		// generic-SSA addressing token. A real consumer may request SP after
		// the entry block already has a terminator, so insert its definition at
		// the stable beginning of the entry rather than at the block end.
		entry := lfc.BBs[lfc.F.Entry.ID]
		before := entry.FirstInstruction()
		for !before.IsNil() && !before.IsAPHINode().IsNil() {
			before = llvm.NextInstruction(before)
		}
		if before.IsNil() {
			lfc.b.SetInsertPointAtEnd(entry)
		} else {
			lfc.b.SetInsertPointBefore(before)
		}
	} else if v.Block != nil {
		lfc.b.SetInsertPointAtEnd(lfc.BBs[v.Block.ID])
	}
	lfc.setDebugLocation(v.Pos)
	defer func() {
		lfc.b.SetCurrentDebugLocationMetadata(savedLocation)
		if !savedBlock.IsNil() {
			lfc.b.SetInsertPointAtEnd(savedBlock)
		}
	}()
	var lVal llvm.Value
	arg0 := func() llvm.Value { return lfc.GenLV(v.Args[0]) }
	arg1 := func() llvm.Value { return lfc.GenLV(v.Args[1]) }
	switch v.Op {
	case OpInitMem, OpSB, OpInlMark, OpWBend:
		// LLVM models memory ordering through instruction dependencies, not an
		// explicit SSA memory value. SB is only an address-space token here.
	case OpSP:
		// Unlike SB, SP can participate in integer expressions such as the
		// caller-frame-size calculation emitted for GetCallerSP.
		lVal = lfc.stackAddress(v)
	case OpUnknown:
		// SSA construction leaves Unknown values only in dead code. Preserve
		// their "value does not matter" semantics as LLVM undef; live Go
		// values are resolved before this point.
		if !v.Type.IsMemory() && v.Type != types.TypeInvalid {
			lVal = llvm.Undef(getLLVMType(v.Type))
		}
	case OpVarDef:
		lVal = arg0()
		if name, ok := v.Aux.(*ir.Name); ok {
			key := llvmLocalKeyForName(name)
			if slot, ok := lfc.Locals[key]; ok {
				if slot.Type.HasPointers() && !lfc.DeferResults[key] && lfc.OpenDeferSlots[key] == 0 {
					lfc.llvmLifetimeStart(slot)
				}
			}
		}
	case OpVarLive:
		lVal = arg0()
		if name, ok := v.Aux.(*ir.Name); ok {
			if slot, ok := lfc.Locals[llvmLocalKeyForName(name)]; ok && slot.Type.HasPointers() {
				lfc.llvmKeepAlive(slot.Value)
			}
		}
	case OpKeepAlive:
		lVal = arg1()
		lfc.llvmKeepAlive(arg0())
	case OpLocalAddr:
		if v.Uses == 0 {
			break
		}
		_, key := llvmLocalName(v)
		slot, ok := lfc.Locals[key]
		if !ok {
			v.Fatalf("local stack slot was not preallocated in the entry block")
		}
		lVal = slot.Value
	case OpGetClosurePtr:
		if lfc.ClosureContext.IsNil() {
			v.Fatalf("closure context requested by a function without a closure ABI parameter")
		}
		lVal = lfc.ClosureContext
		want := getLLVMType(v.Type)
		if want.TypeKind() == llvm.IntegerTypeKind {
			lVal = lfc.b.CreatePtrToInt(lVal, want, v.String())
		} else if want.TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("closure context has unsupported LLVM result type")
		}
	case OpGetG:
		lVal = lfc.currentG(v)
	case OpGetCallerPC:
		lVal = lfc.callerPC(v)
	case OpGetCallerSP:
		lVal = lfc.callerSP(v)
	case OpAddr:
		sym, ok := v.Aux.(*obj.LSym)
		if !ok {
			v.Fatalf("global address has non-LSym auxiliary %T", v.Aux)
		}
		lVal = llvmGoDataRef(sym)
		if llvmNotInHeapPointer(v.Type) {
			lVal = lfc.observePointerAddress(lVal, getLLVMType(v.Type), v.String())
		}
	case OpHasCPUFeature:
		// The generic op is deliberately emitted before architecture lowering.
		// Match AMD64LoweredHasCPUFeature by loading the runtime's byte flag and
		// normalizing it to a Go bool. Keeping this generic also lets LLVM choose
		// the surrounding feature and fallback control flow.
		sym, ok := v.Aux.(*obj.LSym)
		if !ok {
			v.Fatalf("CPU feature has non-LSym auxiliary %T", v.Aux)
		}
		flag := lfc.b.CreateLoad(GlobalCtxt.Int8Type(), llvmGoDataRef(sym), v.String()+".flag")
		flag.SetAlignment(1)
		cond := lfc.b.CreateICmp(llvm.IntNE, flag, llvm.ConstInt(flag.Type(), 0, false), v.String()+".i1")
		lVal = lfc.goBool(cond, v.String())
	case OpArg:
		lVal = lfc.paramForArg(v)
		lVal.SetName(v.Aux.(*ir.Name).Sym().Name)
	case OpArgIntReg, OpArgFloatReg:
		lVal = lfc.registerArgument(v)
	case OpEmpty:
		if v.Type.Size() != 0 || len(v.Args) != 0 || v.Type.IsMemory() || v.Type.IsVoid() {
			v.Fatalf("Empty has invalid type %v or %d arguments", v.Type, len(v.Args))
		}
		lVal = llvm.Undef(getLLVMType(v.Type))
	case OpConst8, OpConst16, OpConst32, OpConst64:
		// AuxInt already carries the two's-complement bit pattern. Asking LLVM
		// to sign-extend the uint64 representation of a negative value makes
		// APInt reject it as an out-of-range signed host integer. Masking to
		// the SSA type width also avoids presenting a sign-extended host value
		// as an out-of-range unsigned i8/i16/i32.
		lVal = llvmConstInt(v.Type, auxIntToInt64(v.AuxInt))
	case OpConstBool:
		lVal = llvm.ConstInt(getLLVMType(v.Type), uint64(v.AuxInt), false)
	case OpConst32F:
		lVal = llvm.ConstFloat(getLLVMType(v.Type), float64(auxIntToFloat32(v.AuxInt)))
	case OpConst64F:
		lVal = llvm.ConstFloat(getLLVMType(v.Type), auxIntToFloat64(v.AuxInt))
	case OpConstNil:
		lVal = llvm.ConstNull(getLLVMType(v.Type))
	case OpConstInterface, OpConstSlice:
		lVal = llvm.ConstNull(getLLVMType(v.Type))
	case OpConstString:
		str := auxToString(v.Aux)
		strData := llvm.ConstString(str, false)
		strVal := llvm.AddGlobal(CurrentModule, strData.Type(), ".str")
		strVal.SetInitializer(strData)
		strVal.SetUnnamedAddr(true)
		strVal.SetLinkage(llvm.PrivateLinkage)
		strVal.SetGlobalConstant(true)
		strLen := llvm.ConstInt(getLLVMType(types.Types[types.TINT]), uint64(len(str)), false)
		lVal = llvm.ConstStruct([]llvm.Value{strVal, strLen}, false)
	case OpMakeTuple:
		lVal = lfc.aggregate(v, v.Args)
	case OpAdd64, OpAdd32, OpAdd16, OpAdd8:
		lVal = lfc.b.CreateAdd(arg0(), arg1(), v.String())
	case OpAdd64carry:
		lVal = lfc.carryBorrow(v, "llvm.uadd.with.overflow.i64")
	case OpAdd32F, OpAdd64F:
		lVal = lfc.b.CreateFAdd(arg0(), arg1(), v.String())
		lfc.allowLLVMContraction(v, lVal)
	case OpSub64, OpSub32, OpSub16, OpSub8:
		lVal = lfc.b.CreateSub(arg0(), arg1(), v.String())
	case OpSub64borrow:
		lVal = lfc.carryBorrow(v, "llvm.usub.with.overflow.i64")
	case OpSub32F, OpSub64F:
		lVal = lfc.b.CreateFSub(arg0(), arg1(), v.String())
		lfc.allowLLVMContraction(v, lVal)
	case OpMul64, OpMul32, OpMul16, OpMul8:
		lVal = lfc.b.CreateMul(arg0(), arg1(), v.String())
	case OpMul64uhilo:
		lVal = lfc.unsignedMul64HiLo(v)
	case OpMul32uover, OpMul64uover:
		lVal = lfc.unsignedMulOver(v)
	case OpHmul32, OpHmul64:
		lVal = lfc.highMultiply(v, true)
	case OpHmul32u, OpHmul64u:
		lVal = lfc.highMultiply(v, false)
	case OpAvg32u, OpAvg64u:
		lVal = lfc.unsignedAverage(v)
	case OpAMD64MOVQi2f:
		lVal = lfc.amd64MoveQIntToFP(v)
	case OpAMD64MOVQf2i, OpAMD64MOVLi2f, OpAMD64MOVLf2i:
		// These are raw register-bank transfers, not numeric conversions.
		lVal = lfc.b.CreateBitCast(arg0(), getLLVMType(v.Type), v.String())
	case OpAMD64PUNPCKLBW:
		lVal = lfc.amd64UnpackLowBytes(v)
	case OpAMD64PSHUFLW:
		lVal = lfc.amd64ShuffleLowWords(v)
	case OpAMD64PSHUFBbroadcast, OpAMD64VPBROADCASTB:
		lVal = lfc.amd64BroadcastByte(v)
	case OpAMD64PSIGNB:
		lVal = lfc.amd64SignBytes(v)
	case OpAMD64PCMPEQB:
		lVal = lfc.amd64CompareEqualBytes(v)
	case OpAMD64PMOVMSKB:
		lVal = lfc.amd64MoveByteMask(v)
	case OpMul32F, OpMul64F:
		lVal = lfc.b.CreateFMul(arg0(), arg1(), v.String())
	case OpDiv64, OpDiv32, OpDiv16, OpDiv8:
		lVal = lfc.integerDiv(v, true, false)
	case OpDiv64u, OpDiv32u, OpDiv16u, OpDiv8u:
		lVal = lfc.integerDiv(v, false, false)
	case OpDiv128u:
		lVal = lfc.unsignedDiv128By64(v)
	case OpMod64, OpMod32, OpMod16, OpMod8:
		lVal = lfc.integerDiv(v, true, true)
	case OpMod64u, OpMod32u, OpMod16u, OpMod8u:
		lVal = lfc.integerDiv(v, false, true)
	case OpDiv32F, OpDiv64F:
		lVal = lfc.b.CreateFDiv(arg0(), arg1(), v.String())
	case OpAnd64, OpAnd32, OpAnd16, OpAnd8, OpAndB:
		lVal = lfc.b.CreateAnd(arg0(), arg1(), v.String())
	case OpOr64, OpOr32, OpOr16, OpOr8, OpOrB:
		lVal = lfc.b.CreateOr(arg0(), arg1(), v.String())
	case OpXor64, OpXor32, OpXor16, OpXor8:
		lVal = lfc.b.CreateXor(arg0(), arg1(), v.String())
	case OpCom64, OpCom32, OpCom16, OpCom8:
		lVal = lfc.b.CreateNot(arg0(), v.String())
	case OpNot:
		zero := llvm.ConstInt(arg0().Type(), 0, false)
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntEQ, arg0(), zero, v.String()+".i1"), v.String())
	case OpNeg64, OpNeg32, OpNeg16, OpNeg8:
		lVal = lfc.b.CreateNeg(arg0(), v.String())
	case OpNeg32F, OpNeg64F:
		lVal = lfc.b.CreateFNeg(arg0(), v.String())
	case OpSqrt:
		lVal = lfc.llvmUnaryIntrinsic(v, "llvm.sqrt.f64")
	case OpSqrt32:
		lVal = lfc.llvmUnaryIntrinsic(v, "llvm.sqrt.f32")
	case OpAbs:
		lVal = lfc.llvmUnaryIntrinsic(v, "llvm.fabs.f64")
	case OpFloor:
		lVal = lfc.llvmRoundIntrinsic(v, "llvm.floor.f64", 1)
	case OpCeil:
		lVal = lfc.llvmRoundIntrinsic(v, "llvm.ceil.f64", 2)
	case OpTrunc:
		lVal = lfc.llvmRoundIntrinsic(v, "llvm.trunc.f64", 3)
	case OpRound:
		lVal = lfc.llvmUnaryIntrinsic(v, "llvm.round.f64")
	case OpRoundToEven:
		lVal = lfc.llvmRoundIntrinsic(v, "llvm.roundeven.f64", 0)
	case OpMin64F:
		lVal = lfc.llvmBinaryIntrinsic(v, "llvm.minimum.f64")
	case OpMin32F:
		lVal = lfc.llvmBinaryIntrinsic(v, "llvm.minimum.f32")
	case OpMax64F:
		lVal = lfc.llvmBinaryIntrinsic(v, "llvm.maximum.f64")
	case OpMax32F:
		lVal = lfc.llvmBinaryIntrinsic(v, "llvm.maximum.f32")
	case OpFMA:
		lVal = lfc.llvmFMA(v)
	case OpEq64, OpEq32, OpEq16, OpEq8, OpEqB:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntEQ, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpEqPtr:
		x, y := lfc.pointerComparisonOperands(v)
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntEQ, x, y, v.String()+".i1"), v.String())
	case OpNeq64, OpNeq32, OpNeq16, OpNeq8, OpNeqB:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntNE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpNeqPtr:
		x, y := lfc.pointerComparisonOperands(v)
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntNE, x, y, v.String()+".i1"), v.String())
	case OpEqInter, OpNeqInter:
		x := lfc.b.CreateExtractValue(arg0(), 0, v.String()+".x")
		y := lfc.b.CreateExtractValue(arg1(), 0, v.String()+".y")
		x, y = lfc.normalizePointerComparisonOperands(v, x, y)
		pred := llvm.IntEQ
		if v.Op == OpNeqInter {
			pred = llvm.IntNE
		}
		lVal = lfc.goBool(lfc.b.CreateICmp(pred, x, y, v.String()+".i1"), v.String())
	case OpLess64, OpLess32, OpLess16, OpLess8:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntSLT, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpLess64U, OpLess32U, OpLess16U, OpLess8U:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntULT, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpLeq64, OpLeq32, OpLeq16, OpLeq8:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntSLE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpLeq64U, OpLeq32U, OpLeq16U, OpLeq8U:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntULE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpEq32F, OpEq64F:
		lVal = lfc.goBool(lfc.b.CreateFCmp(llvm.FloatOEQ, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpNeq32F, OpNeq64F:
		lVal = lfc.goBool(lfc.b.CreateFCmp(llvm.FloatUNE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpLess32F, OpLess64F:
		lVal = lfc.goBool(lfc.b.CreateFCmp(llvm.FloatOLT, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpLeq32F, OpLeq64F:
		lVal = lfc.goBool(lfc.b.CreateFCmp(llvm.FloatOLE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpIsInBounds:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntULT, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpIsSliceInBounds:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntULE, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpIsNonNil:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntNE, arg0(), llvm.ConstNull(arg0().Type()), v.String()+".i1"), v.String())
	case OpLsh64x64, OpLsh64x32, OpLsh64x16, OpLsh64x8,
		OpLsh32x64, OpLsh32x32, OpLsh32x16, OpLsh32x8,
		OpLsh16x64, OpLsh16x32, OpLsh16x16, OpLsh16x8,
		OpLsh8x64, OpLsh8x32, OpLsh8x16, OpLsh8x8:
		lVal = lfc.shift(v, llvmShiftLeft)
	case OpRsh64x64, OpRsh64x32, OpRsh64x16, OpRsh64x8,
		OpRsh32x64, OpRsh32x32, OpRsh32x16, OpRsh32x8,
		OpRsh16x64, OpRsh16x32, OpRsh16x16, OpRsh16x8,
		OpRsh8x64, OpRsh8x32, OpRsh8x16, OpRsh8x8:
		lVal = lfc.shift(v, llvmShiftRightSigned)
	case OpRsh64Ux64, OpRsh64Ux32, OpRsh64Ux16, OpRsh64Ux8,
		OpRsh32Ux64, OpRsh32Ux32, OpRsh32Ux16, OpRsh32Ux8,
		OpRsh16Ux64, OpRsh16Ux32, OpRsh16Ux16, OpRsh16Ux8,
		OpRsh8Ux64, OpRsh8Ux32, OpRsh8Ux16, OpRsh8Ux8:
		lVal = lfc.shift(v, llvmShiftRightUnsigned)
	case OpRotateLeft64, OpRotateLeft32, OpRotateLeft16, OpRotateLeft8:
		lVal = lfc.rotateLeft(v)
	case OpBswap64, OpBswap32, OpBswap16:
		lVal = lfc.byteSwap(v)
	case OpBitRev64, OpBitRev32, OpBitRev16, OpBitRev8:
		resultType := getLLVMType(v.Type)
		if resultType.TypeKind() != llvm.IntegerTypeKind {
			v.Fatalf("%s has a non-integer LLVM result", v.Op)
		}
		lVal = lfc.llvmUnaryIntrinsic(v, "llvm.bitreverse.i"+fmt.Sprint(resultType.IntTypeWidth()))
	case OpBitLen64, OpBitLen32, OpBitLen16, OpBitLen8:
		lVal = lfc.bitLen(v)
	case OpCtz64, OpCtz32, OpCtz16, OpCtz8,
		OpCtz64NonZero, OpCtz32NonZero, OpCtz16NonZero, OpCtz8NonZero:
		lVal = lfc.trailingZeros(v)
	case OpPopCount64, OpPopCount32, OpPopCount16, OpPopCount8:
		lVal = lfc.populationCount(v)
	case OpCondSelect:
		x, y := arg0(), arg1()
		if x.Type() != y.Type() || x.Type() != getLLVMType(v.Type) {
			v.Fatalf("%s has incompatible LLVM value types", v.Op)
		}
		cond := lfc.llvmCondition(lfc.GenLV(v.Args[2]), v.String()+".cond")
		lVal = lfc.b.CreateSelect(cond, x, y, v.String())
	case OpSignExt8to16, OpSignExt8to32, OpSignExt8to64,
		OpSignExt16to32, OpSignExt16to64, OpSignExt32to64:
		lVal = lfc.b.CreateSExt(arg0(), getLLVMType(v.Type), v.String())
	case OpZeroExt8to16, OpZeroExt8to32, OpZeroExt8to64,
		OpZeroExt16to32, OpZeroExt16to64, OpZeroExt32to64:
		lVal = lfc.b.CreateZExt(arg0(), getLLVMType(v.Type), v.String())
	case OpTrunc64to32, OpTrunc64to16, OpTrunc64to8,
		OpTrunc32to16, OpTrunc32to8, OpTrunc16to8:
		lVal = lfc.b.CreateTrunc(arg0(), getLLVMType(v.Type), v.String())
	case OpCvt32to32F, OpCvt32to64F, OpCvt64to32F, OpCvt64to64F:
		lVal = lfc.b.CreateSIToFP(arg0(), getLLVMType(v.Type), v.String())
	case OpCvt32Uto32F, OpCvt32Uto64F, OpCvt64Uto32F, OpCvt64Uto64F:
		lVal = lfc.b.CreateUIToFP(arg0(), getLLVMType(v.Type), v.String())
	case OpCvt32Fto32, OpCvt32Fto64, OpCvt64Fto32, OpCvt64Fto64:
		lVal = lfc.llvmSaturatingFloatToInt(v, true)
	case OpCvt32Fto32U, OpCvt32Fto64U, OpCvt64Fto32U, OpCvt64Fto64U:
		lVal = lfc.llvmSaturatingFloatToInt(v, false)
	case OpCvt32Fto64F, OpCvt64Fto32F:
		lVal = lfc.b.CreateFPCast(arg0(), getLLVMType(v.Type), v.String())
	case OpRound32F, OpRound64F:
		// LLVM float and double operations on the configured arm64 and amd64
		// targets already produce IEEE binary32 and binary64 values. The
		// frontend does not attach fast-math or contract flags, so preserving
		// the value also preserves Go's explicit rounding boundary against FMA
		// contraction.
		lVal = arg0()
		want := GlobalCtxt.FloatType()
		if v.Op == OpRound64F {
			want = GlobalCtxt.DoubleType()
		}
		if lVal.Type() != want || getLLVMType(v.Type) != want {
			v.Fatalf("%s has an unexpected LLVM operand or result type", v.Op)
		}
	case OpCopy:
		lVal = arg0()
		if v.Type.IsMemory() || v.Type.IsVoid() {
			break
		}
		lVal = lfc.reshapeLLVMValue(v, lVal, v.Args[0].Type, v.Type, v.String()+".reshape")
		if got, want := lVal.Type(), getLLVMType(v.Type); got != want {
			v.Fatalf("%s changes LLVM representation", v.Op)
		}
	case OpCvtBoolToUint8:
		lVal = arg0()
		if got, want := lVal.Type(), getLLVMType(v.Type); got != want {
			v.Fatalf("%s changes LLVM representation", v.Op)
		}
	case OpConvert:
		if len(v.Args) != 2 || !v.Args[1].Type.IsMemory() {
			v.Fatalf("Convert has invalid memory dependency")
		}
		lVal = arg0()
		want := getLLVMType(v.Type)
		if lVal.Type() == want {
			break
		}
		switch {
		case lVal.Type().TypeKind() == llvm.PointerTypeKind &&
			want.TypeKind() == llvm.IntegerTypeKind &&
			want.IntTypeWidth() == types.PtrSize*8:
			lVal = lfc.observePointerAddress(lVal, want, v.String()+".coerce")
		case lVal.Type().TypeKind() == llvm.IntegerTypeKind &&
			lVal.Type().IntTypeWidth() == types.PtrSize*8 &&
			want.TypeKind() == llvm.PointerTypeKind:
			lVal = lfc.materializeAddressPointer(lVal, v.Args[0].Type, want, v.String()+".coerce")
		default:
			v.Fatalf("%s changes machine representation", v.Op)
		}
	case OpOffPtr:
		off := llvmConstInt(types.Types[types.TINT], auxIntToInt64(v.AuxInt))
		base := arg0()
		if base.Type().TypeKind() == llvm.IntegerTypeKind {
			// Interface itab/type words and pointers to not-in-heap objects are
			// addresses, but keeping their arithmetic integer-typed prevents an
			// early inttoptr from becoming live across unrelated safepoints. A
			// terminal memory operation materializes the pointer immediately
			// before use.
			if base.Type() != off.Type() {
				v.Fatalf("%s integer address has incompatible offset type", v.Op)
			}
			lVal = lfc.b.CreateAdd(base, off, v.String())
			// A derived value whose Go type is no longer *NotInHeap must adopt
			// pointer representation, but its source address is still unmanaged.
			// Materialize it with a raw inttoptr rather than a relocation marker.
			// The integer itab carrier takes the other path because its source
			// type is uintptr rather than *NotInHeap.
			if llvmNotInHeapPointer(v.Args[0].Type) && !llvmNotInHeapPointer(v.Type) {
				lVal = lfc.materializeAddressPointer(lVal, v.Args[0].Type, getLLVMType(v.Type), v.String()+".pointer")
			}
		} else {
			lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), base, []llvm.Value{off}, v.String())
		}
	case OpAddPtr:
		base := arg0()
		if base.Type().TypeKind() == llvm.IntegerTypeKind {
			if base.Type() != arg1().Type() {
				v.Fatalf("%s integer address has incompatible offset type", v.Op)
			}
			lVal = lfc.b.CreateAdd(base, arg1(), v.String())
			if !llvmNotInHeapPointer(v.Type) {
				lVal = lfc.materializeAddressPointer(lVal, v.Args[0].Type, getLLVMType(v.Type), v.String()+".pointer")
			}
		} else {
			lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), base, []llvm.Value{arg1()}, v.String())
		}
	case OpSubPtr:
		base := arg0()
		neg := lfc.b.CreateNeg(arg1(), v.String()+".neg")
		if base.Type().TypeKind() == llvm.IntegerTypeKind {
			if base.Type() != neg.Type() {
				v.Fatalf("%s integer address has incompatible offset type", v.Op)
			}
			lVal = lfc.b.CreateAdd(base, neg, v.String())
			if !llvmNotInHeapPointer(v.Type) {
				lVal = lfc.materializeAddressPointer(lVal, v.Args[0].Type, getLLVMType(v.Type), v.String()+".pointer")
			}
		} else {
			lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), base, []llvm.Value{neg}, v.String())
		}
	case OpPtrIndex:
		base := arg0()
		if base.Type().TypeKind() == llvm.IntegerTypeKind {
			index := arg1()
			if base.Type() != index.Type() {
				v.Fatalf("%s integer address has incompatible index type", v.Op)
			}
			size := llvm.ConstInt(index.Type(), uint64(v.Type.Elem().Size()), false)
			offset := lfc.b.CreateMul(index, size, v.String()+".offset")
			lVal = lfc.b.CreateAdd(base, offset, v.String())
			if !llvmNotInHeapPointer(v.Type) {
				lVal = lfc.materializeAddressPointer(lVal, v.Args[0].Type, getLLVMType(v.Type), v.String()+".pointer")
			}
		} else {
			lVal = lfc.b.CreateGEP(getLLVMType(v.Type.Elem()), base, []llvm.Value{arg1()}, v.String())
		}
	case OpStaticCall, OpStaticLECall, OpTailLECall:
		lVal = lfc.staticCall(v)
	case OpWB:
		lVal = lfc.llvmWriteBarrier(v)
	case OpClosureCall, OpClosureLECall:
		// arg0 is the code pointer loaded from the funcval, arg1 is the
		// funcval itself. The latter is a hidden REGCTXT input, not an
		// ordinary Go ABI argument.
		lVal = lfc.indirectCall(v, 2, true)
	case OpInterCall, OpInterLECall, OpTailLECallInter:
		// arg0 is the code pointer loaded from the itab. Interface method
		// ABIs receive the interface data word as their first real argument.
		lVal = lfc.indirectCall(v, 1, false)
	case OpPanicBounds:
		lVal = lfc.panicBounds(v)
	case OpSelect0, OpSelect1:
		sel := 0
		if v.Op == OpSelect1 {
			sel = 1
		}
		src := v.Args[0]
		switch src.Op {
		case OpAtomicLoadPtr, OpAtomicLoad8, OpAtomicLoad32, OpAtomicLoad64,
			OpAtomicLoadAcq32, OpAtomicLoadAcq64,
			OpAtomicAdd32, OpAtomicAdd32Variant, OpAtomicAdd64, OpAtomicAdd64Variant,
			OpAtomicExchange8, OpAtomicExchange8Variant,
			OpAtomicExchange32, OpAtomicExchange32Variant,
			OpAtomicExchange64, OpAtomicExchange64Variant,
			OpAtomicAnd64value, OpAtomicAnd64valueVariant,
			OpAtomicAnd32value, OpAtomicAnd32valueVariant,
			OpAtomicAnd8value, OpAtomicAnd8valueVariant,
			OpAtomicOr64value, OpAtomicOr64valueVariant,
			OpAtomicOr32value, OpAtomicOr32valueVariant,
			OpAtomicOr8value, OpAtomicOr8valueVariant,
			OpAtomicCompareAndSwap32, OpAtomicCompareAndSwap32Variant,
			OpAtomicCompareAndSwap64, OpAtomicCompareAndSwap64Variant,
			OpAtomicCompareAndSwapRel32:
			load := lfc.GenLV(src)
			if sel == 0 {
				lVal = load
			}
		case OpWB:
			call := lfc.GenLV(src)
			if sel == 0 {
				lVal = call
			}
		default:
			lVal = lfc.selectPureTuple(v, src, sel)
		}
	case OpSelectN:
		sel := int(auxIntToInt64(v.AuxInt))
		src := v.Args[0]
		switch src.Op {
		case OpStaticCall, OpStaticLECall, OpClosureCall, OpClosureLECall, OpInterCall, OpInterLECall, OpTailLECall, OpTailLECallInter:
			aux := auxToCall(src.Aux)
			call := lfc.GenLV(src)
			switch {
			case sel >= int(aux.NResults()):
				// Selecting the trailing SSA memory dependency only forces the
				// call to be emitted; it has no LLVM value.
			default:
				sig := llvmSignature(aux)
				result := sig.Results[sel]
				if result.InMemory {
					slot, ok := lfc.CallResultSlots[llvmCallResultKey{Call: src.ID, Index: int64(sel)}]
					if !ok {
						v.Fatalf("memory result %d has no caller-owned home", sel)
					}
					lVal = lfc.b.CreateLoad(result.ValueType, slot.Value, v.String())
					lVal.SetAlignment(result.Alignment)
				} else if sig.ReturnCount == 1 {
					lVal = call
				} else {
					lVal = lfc.b.CreateExtractValue(call, result.ReturnIndex, v.String())
				}
			}
			if sel < int(aux.NResults()) {
				lVal = lfc.llvmValueFromABI(v, lVal, aux.TypeOfResult(int64(sel)), v.Type, v.String()+".reshape")
			}
		case OpAtomicLoadPtr, OpAtomicLoad8, OpAtomicLoad32, OpAtomicLoad64,
			OpAtomicLoadAcq32, OpAtomicLoadAcq64,
			OpAtomicAdd32, OpAtomicAdd32Variant, OpAtomicAdd64, OpAtomicAdd64Variant,
			OpAtomicExchange8, OpAtomicExchange8Variant,
			OpAtomicExchange32, OpAtomicExchange32Variant,
			OpAtomicExchange64, OpAtomicExchange64Variant,
			OpAtomicAnd64value, OpAtomicAnd64valueVariant,
			OpAtomicAnd32value, OpAtomicAnd32valueVariant,
			OpAtomicAnd8value, OpAtomicAnd8valueVariant,
			OpAtomicOr64value, OpAtomicOr64valueVariant,
			OpAtomicOr32value, OpAtomicOr32valueVariant,
			OpAtomicOr8value, OpAtomicOr8valueVariant,
			OpAtomicCompareAndSwap32, OpAtomicCompareAndSwap32Variant,
			OpAtomicCompareAndSwap64, OpAtomicCompareAndSwap64Variant,
			OpAtomicCompareAndSwapRel32:
			load := lfc.GenLV(src)
			if sel == 0 {
				lVal = load
			}
		default:
			lVal = lfc.b.CreateExtractValue(lfc.GenLV(src), sel, v.String())
		}
	case OpSelectNAddr:
		var ok bool
		lVal, ok = lfc.ResultSlots[v.ID]
		if !ok {
			v.Fatalf("addressed call result has no LLVM memory home")
		}
	case OpMakeResult:
		direct := make([]llvm.Value, lfc.ReturnCount)
		for i, result := range lfc.Results {
			if result.InMemory {
				lfc.storeMemoryResult(v, v.Args[i], i)
				continue
			}
			direct[result.ReturnIndex] = lfc.llvmValueToABI(
				v, lfc.GenLV(v.Args[i]), v.Args[i].Type,
				lfc.F.OwnAux.TypeOfResult(int64(i)), result.ValueType,
				fmt.Sprintf("%s.result%d", v, i),
			)
		}
		switch lfc.ReturnCount {
		case 0:
		case 1:
			lVal = direct[0]
		default:
			lVal = llvm.Undef(lfc.ReturnType)
			for i, result := range direct {
				lVal = lfc.b.CreateInsertValue(lVal, result, i, "")
			}
			lVal.SetName(v.String())
		}
	case OpPhi:
		if !v.Type.IsMemory() {
			lVal = lfc.b.CreatePHI(getLLVMType(v.Type), v.String())
		}
	case OpLoad, OpDereference:
		// LLVM lowering runs before expandCalls, where the native backend
		// normally rewrites Dereference to Load while decomposing call
		// arguments and results. Address-taken, non-SSA-able named results can
		// therefore still reach this point as Dereference.
		typ := getLLVMType(v.Type)
		if lfc.ItabMethods[v.ID] || lfc.ClosureCodeLoads[v.ID] {
			// Native SSA uses uintptr for an itab method slot. Preserve its
			// pointer-sized storage but expose the callable pointer to LLVM.
			typ = GlobalCtxt.PointerType(0)
		}
		addr := arg0()
		if v.Op == OpDereference {
			if key, ok := lfc.DeferResultKeys[v.Args[0].ID]; ok {
				home, ok := lfc.Locals[key]
				if !ok {
					v.Fatalf("named defer result has no LLVM memory home")
				}
				// Panic recovery resumes at this dereference without following
				// the normal LLVM edge from the suspended call. Reload the heap
				// result address from its stable stack home instead of relying on
				// an SSA register value that the recovery transfer bypassed.
				addr = lfc.b.CreateLoad(getLLVMType(home.Type), home.Value, v.String()+".defer.addr")
				addr.SetAlignment(int(home.Type.Alignment()))
				addr.SetVolatile(true)
			}
		}
		addr = lfc.llvmAddressPointer(v, addr, v.Args[0].Type, v.String()+".addr")
		lVal = lfc.b.CreateLoad(typ, addr, v.String())
		// The runtime may resume at the first deferreturn call recorded for the
		// function, which can be an ordinary exit rather than the fake recovery
		// successor. Reload named results from their stack homes at every such
		// exit so LLVM cannot replace the recovered value with its normal-path
		// SSA value.
		if lfc.isDeferResultAddress(v.Args[0]) || lfc.isOpenDeferAddress(v.Args[0]) {
			lVal.SetVolatile(true)
		}
		if v.Op == OpDereference {
			align := v.Type.Alignment()
			if align <= 0 || align&(align-1) != 0 {
				v.Fatalf("%s has invalid alignment %d for %v", v.Op, align, v.Type)
			}
			lVal.SetAlignment(int(align))
		}
	case OpAtomicLoadPtr:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateLoad(GlobalCtxt.PointerType(0), address, v.String())
		lVal.SetOrdering(llvm.AtomicOrderingSequentiallyConsistent)
	case OpAtomicLoad8, OpAtomicLoad32, OpAtomicLoad64, OpAtomicLoadAcq32, OpAtomicLoadAcq64:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateLoad(getLLVMType(v.Type.FieldType(0)), address, v.String())
		ordering := llvm.AtomicOrderingSequentiallyConsistent
		if v.Op == OpAtomicLoadAcq32 || v.Op == OpAtomicLoadAcq64 {
			ordering = llvm.AtomicOrderingAcquire
		}
		lVal.SetOrdering(ordering)
		lVal.SetAlignment(int(v.Type.FieldType(0).Alignment()))
	case OpAtomicStore8, OpAtomicStore8Variant,
		OpAtomicStore32, OpAtomicStore32Variant,
		OpAtomicStore64, OpAtomicStore64Variant,
		OpAtomicStorePtrNoWB,
		OpAtomicStoreRel32, OpAtomicStoreRel64:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateStore(arg1(), address)
		ordering := llvm.AtomicOrderingSequentiallyConsistent
		if v.Op == OpAtomicStoreRel32 || v.Op == OpAtomicStoreRel64 {
			ordering = llvm.AtomicOrderingRelease
		}
		lVal.SetOrdering(ordering)
		lVal.SetAlignment(int(v.Args[1].Type.Alignment()))
	case OpAtomicAdd32, OpAtomicAdd32Variant, OpAtomicAdd64, OpAtomicAdd64Variant:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		old := lfc.b.CreateAtomicRMW(
			llvm.AtomicRMWBinOpAdd, address, arg1(),
			llvm.AtomicOrderingSequentiallyConsistent,
			false,
		)
		lVal = lfc.b.CreateAdd(old, arg1(), v.String())
	case OpAtomicExchange8, OpAtomicExchange8Variant,
		OpAtomicExchange32, OpAtomicExchange32Variant,
		OpAtomicExchange64, OpAtomicExchange64Variant:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateAtomicRMW(
			llvm.AtomicRMWBinOpXchg, address, arg1(),
			llvm.AtomicOrderingSequentiallyConsistent,
			false,
		)
		lVal.SetName(v.String())
	case OpAtomicAnd8, OpAtomicAnd32,
		OpAtomicAnd64value, OpAtomicAnd64valueVariant,
		OpAtomicAnd32value, OpAtomicAnd32valueVariant,
		OpAtomicAnd8value, OpAtomicAnd8valueVariant:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateAtomicRMW(
			llvm.AtomicRMWBinOpAnd, address, arg1(),
			llvm.AtomicOrderingSequentiallyConsistent,
			false,
		)
		lVal.SetName(v.String())
	case OpAtomicOr8, OpAtomicOr32,
		OpAtomicOr64value, OpAtomicOr64valueVariant,
		OpAtomicOr32value, OpAtomicOr32valueVariant,
		OpAtomicOr8value, OpAtomicOr8valueVariant:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateAtomicRMW(
			llvm.AtomicRMWBinOpOr, address, arg1(),
			llvm.AtomicOrderingSequentiallyConsistent,
			false,
		)
		lVal.SetName(v.String())
	case OpAtomicCompareAndSwap32, OpAtomicCompareAndSwap32Variant,
		OpAtomicCompareAndSwap64, OpAtomicCompareAndSwap64Variant,
		OpAtomicCompareAndSwapRel32:
		successOrdering := llvm.AtomicOrderingSequentiallyConsistent
		failureOrdering := llvm.AtomicOrderingSequentiallyConsistent
		if v.Op == OpAtomicCompareAndSwapRel32 {
			successOrdering = llvm.AtomicOrderingRelease
			failureOrdering = llvm.AtomicOrderingMonotonic
		}
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		compare, replacement := arg1(), lfc.GenLV(v.Args[2])
		if compare.Type() != replacement.Type() {
			// Pointer atomics are represented by the same generic 64-bit SSA
			// operation as uintptr atomics. A generic atomic.Pointer method can
			// pair an unsafe.Pointer value with a *NotInHeap value, whose LLVM
			// carrier is integer-typed. Reconstruct only that terminal atomic
			// operand rather than making the NotInHeap pointer live as a GC root.
			switch {
			case compare.Type().TypeKind() == llvm.IntegerTypeKind &&
				compare.Type().IntTypeWidth() == types.PtrSize*8 &&
				replacement.Type().TypeKind() == llvm.PointerTypeKind:
				compare = lfc.materializeAddressPointer(compare, v.Args[1].Type, replacement.Type(), v.String()+".compare")
			case compare.Type().TypeKind() == llvm.PointerTypeKind &&
				replacement.Type().TypeKind() == llvm.IntegerTypeKind &&
				replacement.Type().IntTypeWidth() == types.PtrSize*8:
				replacement = lfc.materializeAddressPointer(replacement, v.Args[2].Type, compare.Type(), v.String()+".replacement")
			default:
				v.Fatalf("%s has incompatible compare and replacement LLVM types", v.Op)
			}
		}
		pair := lfc.b.CreateAtomicCmpXchg(
			address, compare, replacement,
			successOrdering,
			failureOrdering,
			false,
		)
		success := lfc.b.CreateExtractValue(pair, 1, v.String()+".success")
		lVal = lfc.b.CreateZExt(success, getLLVMType(v.Type.FieldType(0)), v.String())
	case OpPubBarrier:
		lVal = lfc.publicationBarrier(v)
	case OpPrefetchCache:
		lVal = lfc.prefetch(v, 3)
	case OpPrefetchCacheStreamed:
		lVal = lfc.prefetch(v, 0)
	case OpNilCheck:
		lVal = lfc.emitNilCheckIntrinsic(v)
	case OpStore:
		address := lfc.llvmAddressPointer(v, arg0(), v.Args[0].Type, v.String()+".address")
		lVal = lfc.b.CreateStore(arg1(), address)
		if lfc.isDeferResultAddress(v.Args[0]) || lfc.isOpenDeferAddress(v.Args[0]) {
			lVal.SetVolatile(true)
		}
	case OpZero:
		lVal = lfc.llvmZero(v)
	case OpMove:
		lVal = lfc.llvmMove(v)
	case OpMemEq:
		lVal = lfc.llvmMemEq(v)
	case OpSlicemask:
		lVal = lfc.llvmSlicemask(v)
	case OpStructSelect:
		field := int(auxIntToInt32(v.AuxInt))
		lVal = lfc.b.CreateExtractValue(arg0(), field, v.String())
		lVal = lfc.reshapeLLVMValue(v, lVal, v.Args[0].Type.FieldType(field), v.Type, v.String()+".reshape")
	case OpStructMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpArrayMake1:
		lVal = lfc.aggregate(v, v.Args)
	case OpArraySelect:
		lVal = lfc.b.CreateExtractValue(arg0(), int(auxIntToInt64(v.AuxInt)), v.String())
		lVal = lfc.reshapeLLVMValue(v, lVal, v.Args[0].Type.Elem(), v.Type, v.String()+".reshape")
	case OpStringMake, OpComplexMake, OpIMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpSliceMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpStringPtr, OpSlicePtr, OpSlicePtrUnchecked, OpComplexReal, OpITab:
		lVal = lfc.b.CreateExtractValue(arg0(), 0, v.String())
	case OpComplexImag:
		lVal = lfc.b.CreateExtractValue(arg0(), 1, v.String())
	case OpIData:
		lVal = lfc.llvmIData(v)
	case OpStringLen, OpSliceLen:
		lVal = lfc.b.CreateExtractValue(arg0(), 1, v.String())
	case OpSliceCap:
		lVal = lfc.b.CreateExtractValue(arg0(), 2, v.String())
	default:
		v.Fatalf("unsupported SSA operation in LLVM lowering: %s (%s)", v.Op, v.LongString())
	}
	lfc.Vs[v.ID] = lVal
	return lVal
}

func (lfc *LLVMFuncContext) CompileBlock(BB *Block, values []*Value) {
	lfc.b.SetInsertPointAtEnd(lfc.BBs[BB.ID])
	lfc.b.ClearCurrentDebugLocation()
	for _, v := range values {
		if v.Op == OpSP {
			continue
		}
		lfc.GenLV(v)
	}
	lfc.setDebugLocation(BB.Pos)
	defer lfc.b.ClearCurrentDebugLocation()
	switch BB.Kind {
	case BlockRet:
		if lfc.ResultCount == 0 {
			lfc.b.CreateRetVoid()
			break
		}
		result := lfc.GenLV(BB.Controls[0])
		if lfc.ReturnCount == 0 {
			lfc.b.CreateRetVoid()
		} else {
			lfc.b.CreateRet(result)
		}
	case BlockRetJmp:
		lfc.emitTailCallReturn(BB)
	case BlockIf:
		cond := lfc.llvmCondition(lfc.GenLV(BB.Controls[0]), BB.String()+".cond")
		lfc.b.CreateCondBr(cond, lfc.BBs[BB.Succs[0].Block().ID], lfc.BBs[BB.Succs[1].Block().ID])
	case BlockDefer:
		if len(BB.Succs) != 2 || BB.NumControls() != 1 || !BB.Controls[0].Type.IsMemory() {
			BB.Func.fe.Fatalf(BB.Pos, "invalid LLVM defer block %s", BB)
		}
		deferEdge := getLLVMIntrinsicDeclaration(goDeferEdgeIntrinsic)
		lfc.b.CreateCallBr(
			deferEdge.GlobalValueType(), deferEdge, nil,
			lfc.BBs[BB.Succs[0].Block().ID],
			[]llvm.BasicBlock{lfc.BBs[BB.Succs[1].Block().ID]},
			"",
		)
	case BlockPlain:
		lfc.b.CreateBr(lfc.BBs[BB.Succs[0].Block().ID])
	case BlockExit:
		lfc.b.CreateUnreachable()
	case BlockJumpTable:
		index := lfc.GenLV(BB.Controls[0])
		// BlockJumpTable's control is already proven to be in range, and every
		// Go SSA edge is represented by one indexed successor. A real successor
		// as LLVM's default would add an extra CFG edge and make PHIs on a
		// repeated target have one too few incoming values.
		defaultBlock := GlobalCtxt.AddBasicBlock(lfc.LF, BB.String()+".jump.default")
		table := lfc.b.CreateSwitch(index, defaultBlock, len(BB.Succs))
		for i, succ := range BB.Succs {
			table.AddCase(llvm.ConstInt(index.Type(), uint64(i), false), lfc.BBs[succ.Block().ID])
		}
		lfc.b.SetInsertPointAtEnd(defaultBlock)
		lfc.b.CreateUnreachable()
	default:
		BB.Func.fe.Fatalf(BB.Pos, "unsupported SSA block kind in LLVM lowering: %s", BB.Kind)
	}
}

func (lfc *LLVMFuncContext) emitOpenDeferRecovery() {
	if lfc.OpenDeferRecovery.IsNil() {
		return
	}

	deferReturnSig := llvmFuncSignature{
		Type:                llvm.FunctionType(GlobalCtxt.VoidType(), nil, false),
		ReturnType:          GlobalCtxt.VoidType(),
		ClosureContextIndex: -1,
	}
	deferReturn := getOrInsertLLVMABISymbolRef("runtime.deferreturn", obj.ABIInternal, deferReturnSig, goABIInternalCallConv)

	lfc.b.SetInsertPointAtEnd(lfc.OpenDeferRecovery)
	frontendFunc := lfc.F.Frontend().Func()
	if frontendFunc == nil || !frontendFunc.Endlineno.IsKnown() {
		lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "open-coded defer recovery has no function-end source position")
	}
	// Match the native shared deferreturn convention: its synthetic call is
	// attributed to the function end, after every source-level defer. Besides
	// giving PCLN a stable line, a call in an LLVM debug-info function must
	// carry a !dbg location even when it lives in a disconnected recovery block.
	lfc.setDebugLocation(frontendFunc.Endlineno)
	call := lfc.b.CreateCall(deferReturnSig.Type, deferReturn, nil, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	lfc.b.ClearCurrentDebugLocation()

	outParams := lfc.F.OwnAux.ABIInfo().OutParams()
	if len(outParams) != lfc.ResultCount {
		lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "open-coded defer result count %d does not match LLVM signature result count %d", len(outParams), lfc.ResultCount)
	}
	results := make([]llvm.Value, lfc.ReturnCount)
	reshapeContext := &Value{Block: lfc.F.Entry, Pos: lfc.F.Entry.Pos}
	for i, result := range outParams {
		resultSig := lfc.Results[i]
		abiType := resultSig.ValueType
		if result.Type.Size() == 0 {
			results[resultSig.ReturnIndex] = llvm.Undef(abiType)
			continue
		}
		if result.Name == nil {
			lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "open-coded defer result %d has no stack name", i)
		}
		slot, ok := lfc.Locals[llvmLocalKeyForName(result.Name)]
		if !ok {
			lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "open-coded defer result %d has no stack home", i)
		}
		if resultSig.InMemory && slot.Value.C == lfc.LF.Param(resultSig.ParamIndex).C {
			// The recovery path has already updated the caller-owned result
			// object. Loading it only to store it back to the same goret home
			// would add no observable action, even for a volatile named result.
			continue
		}
		value := lfc.b.CreateLoad(getLLVMType(result.Type), slot.Value, fmt.Sprintf("open.defer.result%d", i))
		value.SetAlignment(int(result.Type.Alignment()))
		value.SetVolatile(true)
		value = lfc.llvmValueToABI(reshapeContext, value, result.Type, lfc.F.OwnAux.TypeOfResult(int64(i)), abiType, fmt.Sprintf("open.defer.result%d.abi", i))
		if value.Type() != abiType {
			lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "open-coded defer result %d has incompatible LLVM ABI type", i)
		}
		if resultSig.InMemory {
			store := lfc.b.CreateStore(value, lfc.LF.Param(resultSig.ParamIndex))
			store.SetAlignment(resultSig.Alignment)
		} else {
			results[resultSig.ReturnIndex] = value
		}
	}

	switch lfc.ReturnCount {
	case 0:
		lfc.b.CreateRetVoid()
	case 1:
		lfc.b.CreateRet(results[0])
	default:
		ret := llvm.Undef(lfc.ReturnType)
		for i, result := range results {
			ret = lfc.b.CreateInsertValue(ret, result, i, fmt.Sprintf("open.defer.return%d", i))
		}
		lfc.b.CreateRet(ret)
	}
}

// llvmCanEmitMustTail reports the first Go tail-call shape whose ABI frame
// contract LLVM can currently guarantee. Keep this decision in the Go
// compiler: LLVM implements the mechanical transfer between its Go calling
// conventions, while the frontend decides which Go wrappers may reuse their
// caller's frame. This intentionally starts with direct void(void) transfers;
// later revisions can widen the predicate as argument/result frame reuse is
// proven.
func (lfc *LLVMFuncContext) llvmCanEmitMustTail(call *Value, aux *AuxCall) bool {
	if call.Op != OpTailLECall || aux == nil || aux.Fn == nil ||
		lfc.F.OwnAux == nil {
		return false
	}
	callerABI := lfc.F.OwnAux.ABI().Which()
	calleeABI := aux.ABI().Which()
	compatibleABI := callerABI == calleeABI ||
		callerABI == obj.ABI0 && calleeABI == obj.ABIInternal
	return compatibleABI &&
		aux.NArgs() == 0 && aux.NResults() == 0 &&
		lfc.F.OwnAux.NArgs() == 0 && lfc.F.OwnAux.NResults() == 0
}

// emitTailCallReturn lowers the compiler's RetJmp terminator. Proven
// void(void) direct transfers are emitted as musttail so neither LLVM
// optimization nor statepoint rewriting may reintroduce a frame. Other
// compiler-generated tail calls retain the existing call-and-return fallback
// until their ABI frame shapes are supported.
func (lfc *LLVMFuncContext) emitTailCallReturn(b *Block) {
	mem := b.Controls[0]
	if mem == nil || mem.Op != OpSelectN || len(mem.Args) != 1 || !mem.Type.IsMemory() {
		b.Func.fe.Fatalf(b.Pos, "LLVM RetJmp control is not a tail-call memory selector")
	}
	call := mem.Args[0]
	if call.Op != OpTailLECall && call.Op != OpTailLECallInter {
		b.Func.fe.Fatalf(b.Pos, "LLVM RetJmp control selects non-tail call %s", call.Op)
	}
	aux := auxToCall(call.Aux)
	if aux == nil || int(aux.NResults()) != lfc.ResultCount {
		b.Func.fe.Fatalf(b.Pos, "LLVM tail-call result count does not match caller")
	}

	result := lfc.GenLV(call)
	if lfc.llvmCanEmitMustTail(call, aux) {
		result.SetTailCallKind(llvm.TailCallKindMustTail)
		lfc.b.CreateRetVoid()
		return
	}
	calleeSig := llvmSignature(aux)
	direct := make([]llvm.Value, lfc.ReturnCount)
	for i, callerResult := range lfc.Results {
		calleeResult := calleeSig.Results[i]
		if callerResult.InMemory && calleeResult.InMemory {
			src, ok := lfc.CallResultSlots[llvmCallResultKey{Call: call.ID, Index: int64(i)}]
			if !ok {
				call.Fatalf("memory result %d has no caller-owned home", i)
			}
			lfc.llvmCopyFixedMemory(
				lfc.LF.Param(callerResult.ParamIndex), src.Value,
				lfc.F.OwnAux.TypeOfResult(int64(i)).Size(), callerResult.Alignment,
			)
			continue
		}

		var field llvm.Value
		if calleeResult.InMemory {
			slot, ok := lfc.CallResultSlots[llvmCallResultKey{Call: call.ID, Index: int64(i)}]
			if !ok {
				call.Fatalf("memory result %d has no caller-owned home", i)
			}
			field = lfc.b.CreateLoad(calleeResult.ValueType, slot.Value, fmt.Sprintf("%s.return%d.load", call, i))
			field.SetAlignment(calleeResult.Alignment)
		} else if calleeSig.ReturnCount == 1 {
			field = result
		} else {
			field = lfc.b.CreateExtractValue(result, calleeResult.ReturnIndex, fmt.Sprintf("%s.return%d.extract", call, i))
		}
		field = lfc.llvmValueFromABI(call, field, aux.TypeOfResult(int64(i)), aux.TypeOfResult(int64(i)), fmt.Sprintf("%s.return%d.fromabi", call, i))
		field = lfc.llvmValueToABI(call, field, aux.TypeOfResult(int64(i)), lfc.F.OwnAux.TypeOfResult(int64(i)), callerResult.ValueType, fmt.Sprintf("%s.return%d", call, i))
		if callerResult.InMemory {
			store := lfc.b.CreateStore(field, lfc.LF.Param(callerResult.ParamIndex))
			store.SetAlignment(callerResult.Alignment)
		} else {
			direct[callerResult.ReturnIndex] = field
		}
	}

	switch lfc.ReturnCount {
	case 0:
		lfc.b.CreateRetVoid()
	case 1:
		lfc.b.CreateRet(direct[0])
	default:
		ret := llvm.Undef(lfc.ReturnType)
		for i, field := range direct {
			ret = lfc.b.CreateInsertValue(ret, field, i, fmt.Sprintf("%s.return%d.insert", call, i))
		}
		lfc.b.CreateRet(ret)
	}
}

func (lfc *LLVMFuncContext) MappingName() {
	for s, vs := range lfc.F.NamedValues {
		for _, v := range vs {
			if v.Op != OpArg {
				continue
			}
			if lv, ok := lfc.Vs[v.ID]; ok {
				llvm.LLVMSetValueName2(lv, s.N.Sym().Name)
			}
		}
	}
}

func llvmIsRuntimeGorecover(f *Func) bool {
	return f.OwnAux != nil && f.OwnAux.Fn != nil && f.OwnAux.Fn.Name == "runtime.gorecover"
}

func llvmIsAtomicMemoryOp(op Op) bool {
	switch op {
	case OpAtomicLoadPtr, OpAtomicLoad8, OpAtomicLoad32, OpAtomicLoad64,
		OpAtomicLoadAcq32, OpAtomicLoadAcq64,
		OpAtomicStore8, OpAtomicStore8Variant,
		OpAtomicStore32, OpAtomicStore32Variant,
		OpAtomicStore64, OpAtomicStore64Variant,
		OpAtomicStorePtrNoWB, OpAtomicStoreRel32, OpAtomicStoreRel64,
		OpAtomicAdd32, OpAtomicAdd32Variant, OpAtomicAdd64, OpAtomicAdd64Variant,
		OpAtomicExchange8, OpAtomicExchange8Variant,
		OpAtomicExchange32, OpAtomicExchange32Variant,
		OpAtomicExchange64, OpAtomicExchange64Variant,
		OpAtomicAnd8, OpAtomicAnd32,
		OpAtomicAnd64value, OpAtomicAnd64valueVariant,
		OpAtomicAnd32value, OpAtomicAnd32valueVariant,
		OpAtomicAnd8value, OpAtomicAnd8valueVariant,
		OpAtomicOr8, OpAtomicOr32,
		OpAtomicOr64value, OpAtomicOr64valueVariant,
		OpAtomicOr32value, OpAtomicOr32valueVariant,
		OpAtomicOr8value, OpAtomicOr8valueVariant,
		OpAtomicCompareAndSwap32, OpAtomicCompareAndSwap32Variant,
		OpAtomicCompareAndSwap64, OpAtomicCompareAndSwap64Variant,
		OpAtomicCompareAndSwapRel32:
		return true
	}
	return false
}

func LLVMCompile(f *Func) {
	if f.OwnAux == nil || f.OwnAux.Fn == nil || f.OwnAux.ABIInfo() == nil {
		f.fe.Fatalf(f.Entry.Pos, "missing function ABI information in LLVM lowering for %s", f.Name)
	}
	sig := llvmSignature(f.OwnAux)
	if llvmFunctionUsesClosureContext(f) {
		sig = sig.withClosureContext()
	}
	cc := llvmCallConv(f.OwnAux.ABI().Which())
	FCtxt := &LLVMFuncContext{
		BBs:               map[ID]llvm.BasicBlock{},
		Vs:                map[ID]llvm.Value{},
		Locals:            map[llvmLocalKey]llvmStackSlot{},
		AddressedResults:  map[ID][]llvmAddressedResult{},
		ResultSlots:       map[ID]llvm.Value{},
		CallResultSlots:   map[llvmCallResultKey]llvmStackSlot{},
		ItabMethods:       map[ID]bool{},
		ClosureCodeLoads:  map[ID]bool{},
		DeferResults:      map[llvmLocalKey]bool{},
		DeferResultKeys:   map[ID]llvmLocalKey{},
		RequiredInlinePos: map[int]bool{},
		OpenDeferSlots:    map[llvmLocalKey]int{},
		F:                 f,
		b:                 GlobalCtxt.NewBuilder(),
		ReturnType:        sig.ReturnType,
		ResultCount:       sig.ResultCount,
		ReturnCount:       sig.ReturnCount,
		Params:            sig.Params,
		Results:           sig.Results,
	}
	defer FCtxt.b.Dispose()

	FCtxt.LF = getOrInsertLLVMFunction(f.OwnAux.Fn.Name, sig, cc)
	if FCtxt.LF.BasicBlocksCount() != 0 {
		f.fe.Fatalf(f.Entry.Pos, "duplicate LLVM definition for %s", f.OwnAux.Fn.Name)
	}
	FCtxt.DISubprogram = llvmDebugSubprogram(
		f.OwnAux.Fn, llvmSourcePos(f.Entry.Pos), f)
	llvmDISubprogramVals[f.OwnAux.Fn] = FCtxt.LF
	FCtxt.LF.SetSubprogram(FCtxt.DISubprogram)
	FCtxt.LF.SetGC(goGCStrategy)
	if cpu := llvmTargetCPU(f.Config.arch, buildcfg.GOAMD64); cpu != "" {
		// GOAMD64 levels are the standard x86-64 microarchitecture levels.
		// Make the required instruction set visible to both LLVM optimization
		// and instruction selection without selecting a host-specific CPU.
		FCtxt.LF.AddTargetDependentFunctionAttr(llvmTargetCPUAttr, cpu)
	}
	// Go has already made its source-level inlining decision before LLVM
	// lowering. Preserve both explicit //go:noinline boundaries and the
	// frontend's implicit no-inline rules for functions containing defer or
	// recover.
	//
	// Inlining a classic-defer function would merge its recovery block into the
	// caller, but Go FuncInfo records only one deferreturn PC per function. It
	// would also make runtime.deferprocStack observe the caller's frame instead
	// of the frame selected by the Go frontend. Likewise, inlining a function
	// containing recover would change the frame checked by runtime.gorecover.
	frontendFunc := f.Frontend().Func()
	if frontendFunc != nil && frontendFunc.HasDefer() {
		for _, slot := range f.Names {
			// Heap-escaped output parameters are represented by a synthetic
			// PAUTO named &result. IsOutputParamHeapAddr retains the semantic
			// connection after the address becomes a mallocgc result rather than
			// an OpLocalAddr.
			if slot.N == nil || !slot.N.IsOutputParamHeapAddr() {
				continue
			}
			for _, value := range f.NamedValues[*slot] {
				if value.Type != nil && value.Type.IsPtr() {
					// Zero-sized escaped results use a constant zerobase
					// address, which remains available on the recovery path
					// without either GC liveness or a stable stack home.
					if value.Op == OpAddr {
						continue
					}
					FCtxt.DeferResultKeys[value.ID] = llvmLocalKeyForName(slot.N)
				}
			}
		}
	}
	cgoUnsafeArgs := frontendFunc != nil && frontendFunc.Pragma&ir.CgoUnsafeArgs != 0
	// TODO(goallc): Native Go permits nosplit functions to be inlined, and the
	// linker checks the final GoObj PCSP tables and call relocations against the
	// nosplit limit. This conservative fence currently keeps LLVM inlining from
	// increasing runtime's callRet chain beyond that limit. Replace it with a
	// targeted policy once the specific frame growth is understood.
	frontendNoInline := f.NoSplit || frontendFunc != nil && (frontendFunc.Pragma&ir.Noinline != 0 || frontendFunc.HasDefer() || cgoUnsafeArgs)
	// gorecover explicitly advances the physical unwinder past its own frame
	// before it walks the remaining physical and inline frames. Preserve that
	// one physical boundary. Direct callers may be represented by Go's inline
	// tree and are deliberately left available to LLVM's inliner.
	if frontendNoInline || llvmIsRuntimeGorecover(f) {
		FCtxt.LF.AddFunctionAttr(llvmNoInlineAttribute())
	}
	if f.OpenDeferBits != nil {
		if len(f.OpenDeferSlots) == 0 {
			f.fe.Fatalf(f.Entry.Pos, "open-coded defer has no function slots")
		}
		FCtxt.OpenDeferBits = llvmLocalKeyForName(f.OpenDeferBits)
		FCtxt.HasOpenDeferBits = true
		for i, slot := range f.OpenDeferSlots {
			// Store index+1 so map lookup also distinguishes slot zero from absence.
			FCtxt.OpenDeferSlots[llvmLocalKeyForName(slot)] = i + 1
		}
	}
	setLLVMSymbolLinkage(FCtxt.LF, f.OwnAux.Fn)
	if section := llvmFunctionSection(f.OwnAux.Fn); section != "" {
		FCtxt.LF.SetSection(section)
	}
	setGoObjFunctionFlags(FCtxt.LF, f.OwnAux.Fn)
	setGoObjFunctionInfo(FCtxt.LF, f.OwnAux.Fn)
	inParams := f.OwnAux.ABIInfo().InParams()
	if got, want := len(inParams), int(f.OwnAux.NArgs()); got != want {
		f.fe.Fatalf(f.Entry.Pos, "LLVM parameter metadata count %d does not match signature count %d for %s", got, want, f.Name)
	}
	outParams := f.OwnAux.ABIInfo().OutParams()
	if got, want := len(outParams), sig.ResultCount; got != want {
		f.fe.Fatalf(f.Entry.Pos, "LLVM result metadata count %d does not match signature count %d for %s", got, want, f.Name)
	}
	for i, param := range inParams {
		if param.Name != nil {
			FCtxt.LF.Param(i).SetName(param.Name.Sym().Name)
		}
	}
	for i, result := range sig.Results {
		if result.InMemory {
			FCtxt.LF.Param(result.ParamIndex).SetName(fmt.Sprintf(".result%d", i))
		}
	}
	FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goAsyncUnsafeAttr, ""))
	// A //go:nosplit function must not acquire that late morestack edge. Besides
	// violating the runtime's nosplit call graph, it would expose a safepoint the
	// frontend deliberately prohibited. Give target frame lowering the source
	// policy directly instead of asking it to infer a pragma from GoObj metadata.
	if f.NoSplit {
		FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goNoSplitAttr, ""))
	}
	// Native Go gives //go:systemstack functions a distinct stack-growth
	// prologue: it checks g.stackguard1 and calls runtime.morestackc. Carry the
	// source pragma through AttrCFunc so target frame lowering cannot silently
	// use the ordinary goroutine stack-growth protocol for runtime code.
	if f.OwnAux.Fn.CFunc() {
		FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goSystemStackAttr, ""))
	}
	FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(llvmFramePointerAttr, llvmFramePointerNonLeaf))
	if sig.HasClosureContext {
		FCtxt.ClosureContext = FCtxt.LF.Param(sig.ClosureContextIndex)
		FCtxt.ClosureContext.SetName(".closureptr")
	}
	// Open-coded defer needs a real CFG edge to its runtime recovery entry. Use
	// a zero-instruction callbr in a synthetic prologue, leaving the Go SSA entry
	// and all of its normal optimization semantics unchanged.
	if f.OpenDeferBits != nil {
		FCtxt.Prologue = GlobalCtxt.AddBasicBlock(FCtxt.LF, "open.defer.prologue")
	}
	FCtxt.BBs[f.Entry.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, f.Entry.String())
	if FCtxt.Prologue.IsNil() {
		FCtxt.Prologue = FCtxt.BBs[f.Entry.ID]
	}
	for _, BB := range f.Blocks {
		if BB == f.Entry {
			continue
		}
		FCtxt.BBs[BB.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, BB.String())
	}
	if f.OpenDeferBits != nil {
		FCtxt.OpenDeferRecovery = GlobalCtxt.AddBasicBlock(FCtxt.LF, "open.defer.recovery")
	}
	hasAtomicMemoryOp := false
	for _, BB := range f.Blocks {
		for _, v := range BB.Values {
			hasAtomicMemoryOp = hasAtomicMemoryOp || llvmIsAtomicMemoryOp(v.Op)
			if (v.Op == OpInterCall || v.Op == OpInterLECall || v.Op == OpTailLECallInter) && len(v.Args) != 0 {
				code := v.Args[0]
				if code.Op == OpLoad && code.Type.IsUintptr() && code.Uses == 1 {
					FCtxt.ItabMethods[code.ID] = true
				}
			}
			if (v.Op == OpClosureCall || v.Op == OpClosureLECall) && len(v.Args) >= 2 {
				code := v.Args[0]
				if code.Op == OpLoad {
					// A nil func value faults while loading its code word. Record
					// loads only to preserve that behavior in LLVM; ClosureCall
					// itself already defines the code/context contract and does not
					// constrain how either value was produced.
					FCtxt.ClosureCodeLoads[code.ID] = true
				}
			} else if v.Op == OpClosureCall || v.Op == OpClosureLECall {
				v.Fatalf("closure call has %d SSA arguments, want at least code, context, and memory", len(v.Args))
			}
		}
	}
	if len(FCtxt.ClosureCodeLoads) != 0 || hasAtomicMemoryOp {
		// A nil memory access in Go must remain a real faulting instruction so
		// the runtime can translate it into a recoverable panic. LLVM otherwise
		// treats a known-null atomic access as immediate undefined behavior and
		// may replace the function body with unreachable.
		// Native Go relies on the funcval code-word load faulting when the
		// funcval is nil. Its result feeds the indirect call, so keeping null
		// dereferences defined is enough to retain the single ordinary load.
		FCtxt.LF.AddFunctionAttr(llvmNullPointerIsValidAttribute())
	}
	// LLVM only treats a constant-sized alloca as a fixed frame object when
	// it is in the entry block. Preallocate every Go stack slot before phi or
	// ordinary instruction emission so LocalAddr values in loops and branches
	// cannot become dynamic allocas (which Go stack growth cannot support).
	FCtxt.b.SetInsertPointAtEnd(FCtxt.Prologue)
	emitGoObjFunctionMarkerRelocs(FCtxt.b, f.OwnAux.Fn)
	if len(f.OpenDeferSlots) != 0 {
		openDeferSlotsType := llvm.ArrayType(GlobalCtxt.PointerType(0), len(f.OpenDeferSlots))
		openDeferSlots := FCtxt.b.CreateAlloca(openDeferSlotsType, "open.defer.slots")
		openDeferSlots.SetAlignment(types.PtrSize)
		openDeferSlots.SetMetadata(GlobalCtxt.MDKindID(goOpenDeferSlotsMD), GlobalCtxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(len(f.OpenDeferSlots)), false).ConstantAsMetadata(),
		}))
		for i, name := range f.OpenDeferSlots {
			key := llvmLocalKeyForName(name)
			if index := FCtxt.OpenDeferSlots[key]; index != i+1 {
				f.fe.Fatalf(name.Pos(), "open-coded defer stack slot %v has index %d, want %d", name, index, i+1)
			}
			if name.Type().Size() != int64(types.PtrSize) {
				f.fe.Fatalf(name.Pos(), "invalid open-coded defer stack slot %v", name)
			}
			if _, exists := FCtxt.Locals[key]; exists {
				f.fe.Fatalf(name.Pos(), "duplicate open-coded defer stack slot %v", name)
			}
			offset := llvm.ConstInt(GlobalCtxt.Int64Type(), uint64(i)*uint64(types.PtrSize), false)
			value := FCtxt.b.CreateGEP(
				GlobalCtxt.Int8Type(), openDeferSlots, []llvm.Value{offset},
				fmt.Sprintf("open.defer.slot%d", i),
			)
			FCtxt.Locals[key] = llvmStackSlot{Value: value, Type: name.Type()}
		}
	}
	isDeferResultLocal := func(name *ir.Name) bool {
		return frontendFunc != nil && frontendFunc.HasDefer() &&
			(name.Class == ir.PPARAMOUT || name.IsOutputParamHeapAddr())
	}
	preallocateLocal := func(name *ir.Name, llvmName string) (llvmStackSlot, bool) {
		key := llvmLocalKeyForName(name)
		if slot, ok := FCtxt.Locals[key]; ok {
			if !types.Identical(slot.Type, name.Type()) {
				f.fe.Fatalf(name.Pos(), "conflicting Go types for local stack slot %v", name)
			}
			return slot, false
		}
		if name.Type().Alignment() <= 0 {
			f.fe.Fatalf(name.Pos(), "invalid alignment %d for local stack slot %v", name.Type().Alignment(), name)
		}
		if FCtxt.OpenDeferSlots[key] != 0 {
			f.fe.Fatalf(name.Pos(), "open-coded defer stack slot %v was not preallocated", name)
		}
		if cgoUnsafeArgs && (name.Class == ir.PPARAM || name.Class == ir.PPARAMOUT) {
			// CgoUnsafeArgs code may pass one ABI0 slot address to C, which can
			// then address the complete contiguous argument/result frame. Bind
			// every parameter/result LocalAddr to that physical frame.
			slot := llvmStackSlot{Value: FCtxt.cgoUnsafeArgAddress(name, llvmName), Type: name.Type()}
			FCtxt.Locals[key] = slot
			return slot, true
		}
		value := FCtxt.b.CreateAlloca(getLLVMType(name.Type()), llvmName)
		value.SetAlignment(int(name.Type().Alignment()))
		if FCtxt.HasOpenDeferBits && key == FCtxt.OpenDeferBits {
			value.SetMetadata(GlobalCtxt.MDKindID(goOpenDeferBitsMD), GlobalCtxt.MDNode(nil))
		}
		isDeferResult := isDeferResultLocal(name)
		if isDeferResult {
			// Defer recovery can resume without following the suspended call's
			// ordinary LLVM edge, so the result slot is live for the whole function.
			FCtxt.DeferResults[key] = true
		}
		if name.Type().HasPointers() {
			if isDeferResult {
				value.SetMetadata(GlobalCtxt.MDKindID(goDeferResultMD), GlobalCtxt.MDNode(nil))
			}
		}
		slot := llvmStackSlot{Value: value, Type: name.Type()}
		FCtxt.Locals[key] = slot
		return slot, true
	}
	// A typed goret parameter is the Go ABI home of a stack-assigned result.
	// Bind an ordinary, non-escaping PPARAMOUT directly to that caller-owned
	// home so named-result stores, address-taking, and defer recovery all see
	// the same object. Register results still need a callee local when they are
	// addressable, while a heap-escaped result must retain the heap object whose
	// address may outlive the caller's result area.
	for i, result := range sig.Results {
		if !result.InMemory || cgoUnsafeArgs {
			continue
		}
		assignment := outParams[i]
		if len(assignment.Registers) != 0 {
			f.fe.Fatalf(f.Entry.Pos, "LLVM goret result %d was assigned Go registers", i)
		}
		if assignment.Name == nil || !assignment.Name.OnStack() {
			continue
		}
		goType := f.OwnAux.TypeOfResult(int64(i))
		if goType.Size() == 0 || !types.Identical(goType, assignment.Name.Type()) {
			f.fe.Fatalf(assignment.Name.Pos(), "invalid Go type for LLVM goret result %v", assignment.Name)
		}
		key := llvmLocalKeyForName(assignment.Name)
		if _, exists := FCtxt.Locals[key]; exists {
			f.fe.Fatalf(assignment.Name.Pos(), "duplicate LLVM goret result home %v", assignment.Name)
		}
		FCtxt.Locals[key] = llvmStackSlot{Value: FCtxt.LF.Param(result.ParamIndex), Type: assignment.Name.Type()}
		if isDeferResultLocal(assignment.Name) {
			// Defer recovery can enter without following the suspended call's
			// ordinary edge. The goret formal is already a whole-activation fixed
			// home, but volatile accesses are still needed on recovery paths. Mark
			// pointer-containing homes so the statepoint pass keeps their contents
			// in every ArgsPointerMaps entry, matching native PPARAMOUT liveness.
			FCtxt.DeferResults[key] = true
			if assignment.Name.Type().HasPointers() {
				FCtxt.LF.AddAttributeAtIndex(
					result.ParamIndex+1,
					GlobalCtxt.CreateStringAttribute(goDeferResultMD, ""),
				)
			}
		}
	}
	// A typed byval parameter already denotes the callee-private Go ABI stack
	// copy. Bind addressable parameter uses directly to that incoming home;
	// creating a second alloca and copying the value again would defeat the ABI
	// model.
	for i, param := range sig.Params {
		if !param.ByVal {
			continue
		}
		// CgoUnsafeArgs exposes one parameter address as the base of the
		// complete contiguous ABI0 input/result frame. A typed byval argument
		// denotes only its own object, so keep addressable uses on the existing
		// llvm.go.abi0.frame-derived view below. The physical incoming home is
		// still the same slot and therefore needs no initialization copy.
		if cgoUnsafeArgs {
			continue
		}
		assignment := inParams[i]
		if len(assignment.Registers) != 0 {
			f.fe.Fatalf(f.Entry.Pos, "LLVM byval parameter %d was assigned Go registers", i)
		}
		if assignment.Name == nil {
			continue
		}
		goType := f.OwnAux.TypeOfArg(int64(i))
		if goType.Size() == 0 || !types.Identical(goType, assignment.Name.Type()) {
			f.fe.Fatalf(assignment.Name.Pos(), "invalid Go type for LLVM byval parameter %v", assignment.Name)
		}
		key := llvmLocalKeyForName(assignment.Name)
		if _, exists := FCtxt.Locals[key]; exists {
			f.fe.Fatalf(assignment.Name.Pos(), "duplicate LLVM byval parameter home %v", assignment.Name)
		}
		FCtxt.Locals[key] = llvmStackSlot{Value: FCtxt.LF.Param(i), Type: assignment.Name.Type()}
	}
	// Escape analysis names the pointer to a heap-backed result &result. SSA can
	// keep that pointer only in a register, but panic recovery needs the same
	// stable, whole-function local-slot semantics as an ordinary named result.
	// Materialize &result even when no OpLocalAddr survived SSA optimization.
	for _, named := range f.Names {
		if frontendFunc == nil || !frontendFunc.HasDefer() || named.N == nil || !named.N.IsOutputParamHeapAddr() {
			continue
		}
		needed := false
		for _, value := range f.NamedValues[*named] {
			if _, ok := FCtxt.DeferResultKeys[value.ID]; ok {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		name := named.N
		if !name.Type().IsPtr() {
			f.fe.Fatalf(name.Pos(), "heap output parameter address %v has non-pointer type %v", name, name.Type())
		}
		home, created := preallocateLocal(name, name.Sym().Name+".defer.home")
		if !created {
			continue
		}
		// This pointer home is live at every safepoint. Initialize it before
		// mallocgc can suspend and expose the frame to the stack scanner.
		init := FCtxt.b.CreateStore(llvm.ConstNull(getLLVMType(name.Type())), home.Value)
		init.SetAlignment(int(name.Type().Alignment()))
		init.SetVolatile(true)
	}
	var parameterHomes []*ir.Name
	var parameterLifetimeSlots []llvmStackSlot
	if cgoUnsafeArgs {
		// Reuse the ordinary parameter-home path established for the modeled
		// ABI0 frame. CgoUnsafeArgs exposes the complete contiguous input area
		// through one parameter address, so every non-empty input needs a home
		// even when Go SSA contains no OpLocalAddr for it.
		for _, param := range inParams {
			if param.Name == nil || param.Type.Size() == 0 {
				continue
			}
			if _, created := preallocateLocal(param.Name, param.Name.Sym().Name+".cgo"); created {
				// Keep an explicit IR data dependence between every formal input
				// and the complete frame address passed to C. Per-argument byval
				// semantics cannot describe CgoUnsafeArgs' permitted access beyond
				// the first parameter object into adjacent arguments/results. The
				// two addresses lower to the same incoming fixed home, so target
				// copy folding may remove the physical self-copy.
				parameterHomes = append(parameterHomes, param.Name)
			}
		}
	}
	for _, BB := range f.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpLocalAddr || v.Uses == 0 {
				continue
			}
			name, key := llvmLocalName(v)
			_, created := preallocateLocal(name, v.String())
			if !created {
				continue
			}
			if name.Class == ir.PPARAM {
				// A zero-sized parameter has a logical ABI0 address but no
				// bytes to copy into its home. In particular, do not turn the
				// go.abi.pad carrier into storage in the Go frame.
				if name.Type().Size() != 0 {
					parameterHomes = append(parameterHomes, name)
				}
				if name.Type().HasPointers() && !cgoUnsafeArgs {
					parameterLifetimeSlots = append(parameterLifetimeSlots, FCtxt.Locals[key])
				}
			}
		}
	}
	// Results assigned by Go to memory use caller-owned typed goret carriers.
	// Reserve every destination before call emission; ordinary SelectN loads
	// from the same object that SelectNAddr exposes by address.
	for _, BB := range f.Blocks {
		for _, call := range BB.Values {
			switch call.Op {
			case OpStaticCall, OpStaticLECall, OpTailLECall,
				OpClosureCall, OpClosureLECall,
				OpInterCall, OpInterLECall, OpTailLECallInter:
			default:
				continue
			}
			aux := auxToCall(call.Aux)
			if aux == nil {
				call.Fatalf("call has no ABI information")
			}
			callSig := llvmSignature(aux)
			for index, result := range callSig.Results {
				if !result.InMemory {
					continue
				}
				resultType := aux.TypeOfResult(int64(index))
				slot := llvmStackSlot{
					Value: FCtxt.b.CreateAlloca(result.ValueType, fmt.Sprintf("%s.result%d.home", call, index)),
					Type:  resultType,
				}
				slot.Value.SetAlignment(result.Alignment)
				FCtxt.CallResultSlots[llvmCallResultKey{Call: call.ID, Index: int64(index)}] = slot
			}
		}
	}

	// SelectNAddr for a register result still needs an addressable local copy.
	// Multiple selectors of one call result share the same slot.
	addressedResultSlots := make(map[llvmCallResultKey]llvmStackSlot)
	for _, BB := range f.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpSelectNAddr || v.Uses == 0 {
				continue
			}
			if len(v.Args) != 1 || v.Type == nil || !v.Type.IsPtr() {
				v.Fatalf("SelectNAddr has invalid arguments or type")
			}
			call := v.Args[0]
			aux := auxToCall(call.Aux)
			index := auxIntToInt64(v.AuxInt)
			if aux == nil || index < 0 || index >= aux.NResults() {
				v.Fatalf("SelectNAddr has invalid call result index %d", index)
			}
			resultType := v.Type.Elem()
			if resultType.Alignment() <= 0 || !types.Identical(resultType, aux.TypeOfResult(index)) {
				v.Fatalf("SelectNAddr result type %v does not match call result %v", resultType, aux.TypeOfResult(index))
			}
			key := llvmCallResultKey{Call: call.ID, Index: index}
			if memorySlot, ok := FCtxt.CallResultSlots[key]; ok {
				FCtxt.ResultSlots[v.ID] = memorySlot.Value
				continue
			}
			slot, ok := addressedResultSlots[key]
			if !ok {
				slot = llvmStackSlot{
					Value: FCtxt.b.CreateAlloca(getLLVMType(resultType), v.String()+".home"),
					Type:  resultType,
				}
				slot.Value.SetAlignment(int(resultType.Alignment()))
				addressedResultSlots[key] = slot
				FCtxt.AddressedResults[call.ID] = append(FCtxt.AddressedResults[call.ID], llvmAddressedResult{
					Index: index,
					Slot:  slot,
					Owner: v,
				})
			}
			FCtxt.ResultSlots[v.ID] = slot.Value
		}
	}
	for _, slot := range parameterLifetimeSlots {
		FCtxt.llvmLifetimeStart(slot)
	}
	if f.OpenDeferBits != nil {
		zeroSlot := func(name *ir.Name, volatile bool) {
			slot, ok := FCtxt.Locals[llvmLocalKeyForName(name)]
			if !ok {
				f.fe.Fatalf(f.Entry.Pos, "open-coded defer stack slot %v was not allocated", name)
			}
			store := FCtxt.b.CreateStore(llvm.ConstNull(getLLVMType(slot.Type)), slot.Value)
			store.SetAlignment(int(slot.Type.Alignment()))
			store.SetVolatile(volatile)
		}
		zeroSlot(f.OpenDeferBits, true)
		for _, slot := range f.OpenDeferSlots {
			zeroSlot(slot, true)
		}
		for _, result := range frontendFunc.Type().Results() {
			if result.Nname == nil || result.Type.Size() == 0 {
				continue
			}
			name := result.Nname.(*ir.Name)
			if _, ok := FCtxt.Locals[llvmLocalKeyForName(name)]; ok {
				zeroSlot(name, true)
			}
		}
	}
	// LLVM requires all phi nodes to precede non-phi instructions in a
	// block. Predeclare them before recursive value emission can insert any
	// other instruction into the block.
	for _, BB := range f.Blocks {
		FCtxt.b.SetInsertPointAtEnd(FCtxt.BBs[BB.ID])
		for _, v := range BB.Values {
			if v.Op == OpPhi && !v.Type.IsMemory() {
				FCtxt.Vs[v.ID] = FCtxt.b.CreatePHI(getLLVMType(v.Type), v.String())
			}
		}
	}
	// Go's ABI assigns each parameter either wholly to registers or wholly to
	// the stack. Give only parameters that already have an addressable Go SSA
	// LocalAddr a complete LLVM memory home. Ordinary register parameters remain
	// direct LLVM SSA values, while the backend remains responsible for the
	// physical Go ABI assignment.
	//
	// This intentionally differs from the native lowering, which stores each
	// incoming register piece separately and addresses stack-assigned parameters
	// in their incoming slots. The full aggregate store makes any existing piece
	// loads and stores redundant and lets normal LLVM memory optimization remove
	// them. Store the physical ABI carrier directly through the opaque pointer:
	// reconstructing its semantic named aggregate first obscures the formal
	// argument store from SelectionDAG and prevents a wholly stack-assigned
	// parameter home from being folded back to its incoming fixed stack slot.
	FCtxt.b.SetInsertPointAtEnd(FCtxt.BBs[f.Entry.ID])
	for _, name := range parameterHomes {
		key := llvmLocalKeyForName(name)
		slot := FCtxt.Locals[key]
		param, paramType := FCtxt.paramForArgNameAndType(name)
		if paramType.Size() != slot.Type.Size() || param.Type() != getLLVMABIType(slot.Type) {
			f.fe.Fatalf(name.Pos(), "parameter home has incompatible physical ABI carrier")
		}
		init := FCtxt.b.CreateStore(param, slot.Value)
		init.SetAlignment(int(slot.Type.Alignment()))
	}
	// Emit dominators before their uses in successor blocks. GenLV may
	// recursively request a value from another block, and inserting that value
	// into an as-yet empty defining block would otherwise move it ahead of the
	// defining block's memory operations. Go SSA represents memory state as a
	// token, but loads do not produce a new token. Order each block against its
	// store chain so reads of a token are emitted before the following memory
	// operation, without applying the full native instruction scheduler.
	sset := f.newSparseSet(f.NumValues())
	defer f.retSparseSet(sset)
	storeNumber := f.Cache.allocInt32Slice(f.NumValues())
	defer f.Cache.freeInt32Slice(storeNumber)
	postorder := f.Postorder()
	for i := len(postorder) - 1; i >= 0; i-- {
		BB := postorder[i]
		FCtxt.CompileBlock(BB, storeOrder(BB.Values, sset, storeNumber))
	}
	FCtxt.emitDeferResultHomeStores()
	if f.OpenDeferBits != nil {
		FCtxt.b.SetInsertPointAtEnd(FCtxt.Prologue)
		deferEdge := getLLVMIntrinsicDeclaration(goDeferEdgeIntrinsic)
		FCtxt.b.CreateCallBr(
			deferEdge.GlobalValueType(), deferEdge, nil,
			FCtxt.BBs[f.Entry.ID],
			[]llvm.BasicBlock{FCtxt.OpenDeferRecovery},
			"",
		)
		FCtxt.emitOpenDeferRecovery()
	}
	FCtxt.FinishPhi()
	FCtxt.expandNilCheckIntrinsics()
	FCtxt.MappingName()

	err := llvm.VerifyFunction(FCtxt.LF, llvm.PrintMessageAction)
	if err != nil {
		f.fe.Fatalf(f.Entry.Pos, "LLVM verifier failed for %s: %v", f.Name, err)
	}
}

var CurrentModule llvm.Module
var type2lTypes = map[*types.Type]llvm.Type{}
var goObjConfigWritten bool
var goObjImportsWritten bool
var llvmModuleFinalized bool
var currentLLVMDataLowerer *llvmDataLowerer
var goObjCompilerUsed []llvm.Value
var goObjCompilerUsedNames map[string]bool

var GlobalCtxt = llvm.GlobalContext()

func llvmNotInHeapPointer(typ *types.Type) bool {
	return typ != nil && typ.IsPtr() && typ.Elem().NotInHeap()
}

func getLLVMType(typ *types.Type) llvm.Type {
	if t, ok := type2lTypes[typ]; ok {
		return t
	}

	ptrType := func() llvm.Type {
		return GlobalCtxt.PointerType(0)
	}
	var lType llvm.Type
	switch typ.Kind() {
	case types.TINT8, types.TUINT8:
		lType = GlobalCtxt.Int8Type()
	case types.TINT16, types.TUINT16:
		lType = GlobalCtxt.Int16Type()
	case types.TINT32, types.TUINT32:
		lType = GlobalCtxt.Int32Type()
	case types.TINT64, types.TUINT64:
		lType = GlobalCtxt.Int64Type()
	case types.TINT, types.TUINT, types.TUINTPTR:
		if types.PtrSize == 8 {
			lType = GlobalCtxt.Int64Type()
		} else {
			lType = GlobalCtxt.Int32Type()
		}
	case types.TBOOL:
		lType = GlobalCtxt.Int8Type()
	case types.TFLOAT32:
		lType = GlobalCtxt.FloatType()
	case types.TFLOAT64:
		lType = GlobalCtxt.DoubleType()
	case types.TPTR:
		if llvmNotInHeapPointer(typ) {
			// The runtime must not relocate pointers to unmanaged storage when a
			// goroutine stack moves. Carry their pointer-sized bits as integers so
			// LLVM statepoint liveness cannot classify them as movable GC roots.
			lType = getLLVMType(types.Types[types.TUINTPTR])
		} else {
			lType = ptrType()
		}
	case types.TUNSAFEPTR, types.TFUNC, types.TMAP, types.TCHAN:
		// LLVM uses opaque pointers. A Go func value is a pointer to its
		// closure object; a callable LLVM function type is built from AuxCall
		// ABI information instead.
		lType = ptrType()
	case types.TARRAY:
		lType = llvm.ArrayType(getLLVMType(typ.Elem()), int(typ.NumElem()))
	case types.TSTRUCT:
		if typ.Sym() != nil {
			// Cache the opaque shell before descending so recursive types
			// through named aggregates cannot recurse indefinitely.
			lType = GlobalCtxt.StructCreateNamed(typ.LinkString())
			type2lTypes[typ] = lType
			fieldTypes := make([]llvm.Type, typ.NumFields(), typ.NumFields()+1)
			for i := 0; i < typ.NumFields(); i++ {
				fieldTypes[i] = getLLVMType(typ.FieldType(i))
			}
			if llvmStructHasTailPad(typ) {
				fieldTypes = append(fieldTypes, getLLVMABIPadType())
			}
			lType.StructSetBody(fieldTypes, false)
		} else {
			fieldTypes := make([]llvm.Type, typ.NumFields(), typ.NumFields()+1)
			for i := 0; i < typ.NumFields(); i++ {
				fieldTypes[i] = getLLVMType(typ.FieldType(i))
			}
			if llvmStructHasTailPad(typ) {
				fieldTypes = append(fieldTypes, getLLVMABIPadType())
			}
			lType = llvm.StructType(fieldTypes, false)
		}
	case types.TSTRING:
		lType = llvm.StructType([]llvm.Type{ptrType(), getLLVMType(types.Types[types.TINT])}, false)
	case types.TSLICE:
		data := ptrType()
		if typ.Elem().NotInHeap() {
			// A slice of not-in-heap elements has the same non-GC data-word
			// semantics as *NotInHeap.
			data = getLLVMType(types.Types[types.TUINTPTR])
		}
		lType = llvm.StructType([]llvm.Type{
			data,
			getLLVMType(types.Types[types.TINT]),
			getLLVMType(types.Types[types.TINT]),
		}, false)
	case types.TINTER:
		// The itab/type word addresses immutable runtime metadata and is not a
		// movable GC pointer. Preserve its pointer-sized ABI bits as an integer
		// so LLVM statepoint liveness only sees the interface data word.
		lType = llvm.StructType([]llvm.Type{getLLVMType(types.Types[types.TINT]), ptrType()}, false)
	case types.TCOMPLEX64:
		lType = llvm.StructType([]llvm.Type{GlobalCtxt.FloatType(), GlobalCtxt.FloatType()}, false)
	case types.TCOMPLEX128:
		lType = llvm.StructType([]llvm.Type{GlobalCtxt.DoubleType(), GlobalCtxt.DoubleType()}, false)
	case types.TTUPLE, types.TRESULTS:
		var fields []llvm.Type
		for i, count := 0, llvmTupleFieldCount(typ); i < count; i++ {
			if field := typ.FieldType(i); !field.IsMemory() {
				fields = append(fields, getLLVMType(field))
			}
		}
		switch len(fields) {
		case 0:
			lType = GlobalCtxt.VoidType()
		case 1:
			lType = fields[0]
		default:
			lType = llvm.StructType(fields, false)
		}
	default:
		base.Fatalf("unsupported Go type in LLVM lowering: %v (kind %v)", typ, typ.Kind())
	}
	type2lTypes[typ] = lType
	return lType
}

func InitModule(pkg *types.Pkg) {
	type2lTypes = map[*types.Type]llvm.Type{
		types.Types[types.TINT8]:   GlobalCtxt.Int8Type(),
		types.Types[types.TUINT8]:  GlobalCtxt.Int8Type(),
		types.Types[types.TINT16]:  GlobalCtxt.Int16Type(),
		types.Types[types.TUINT16]: GlobalCtxt.Int16Type(),
		types.Types[types.TINT32]:  GlobalCtxt.Int32Type(),
		types.Types[types.TUINT32]: GlobalCtxt.Int32Type(),
		types.Types[types.TINT64]:  GlobalCtxt.Int64Type(),
		types.Types[types.TUINT64]: GlobalCtxt.Int64Type(),
		// The Go ABI represents bool in an 8-bit integer slot. LLVM
		// comparisons are widened to this type before crossing SSA or ABI
		// boundaries.
		types.Types[types.TBOOL]:      GlobalCtxt.Int8Type(),
		types.Types[types.TFLOAT32]:   GlobalCtxt.FloatType(),
		types.Types[types.TFLOAT64]:   GlobalCtxt.DoubleType(),
		types.Types[types.TUNSAFEPTR]: GlobalCtxt.PointerType(0),
	}
	if types.PtrSize == 8 {
		type2lTypes[types.Types[types.TINT]] = GlobalCtxt.Int64Type()
		type2lTypes[types.Types[types.TUINT]] = GlobalCtxt.Int64Type()
		type2lTypes[types.Types[types.TUINTPTR]] = GlobalCtxt.Int64Type()
	} else {
		type2lTypes[types.Types[types.TINT]] = GlobalCtxt.Int32Type()
		type2lTypes[types.Types[types.TUINT]] = GlobalCtxt.Int32Type()
		type2lTypes[types.Types[types.TUINTPTR]] = GlobalCtxt.Int32Type()
	}
	type2lTypes[types.ByteType] = GlobalCtxt.Int8Type()
	type2lTypes[types.RuneType] = GlobalCtxt.Int32Type()
	// AMD64's TypeInt128 is an untyped 128-bit XMM carrier. Model it as
	// bytes so packed byte operations retain their lane semantics in LLVM IR.
	type2lTypes[types.TypeInt128] = llvmAMD64ByteVectorType()

	CurrentModule = GlobalCtxt.NewModule(pkg.Path)
	CurrentModule.SetTarget(goObjTargetTriple())
	goObjConfigWritten = false
	goObjImportsWritten = false
	llvmModuleFinalized = false
	currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
	initLLVMGoObjLocalDefinitions()
	emitLateGoObjBuiltinDeclarations()
	initLLVMDebugInfo(pkg)
}

// goObjTargetTriple identifies the GoObj target that llc should use when it
// consumes the IR produced by this compiler. Keep this in the IR rather than
// making the toolexec wrapper rediscover it from the build environment.
func goObjTargetTriple() string {
	switch buildcfg.GOOS + "/" + buildcfg.GOARCH {
	case "darwin/arm64":
		return "aarch64-apple-darwin-goobj"
	case "linux/arm64":
		return "aarch64-unknown-linux-goobj"
	case "linux/amd64":
		return "x86_64-unknown-linux-goobj"
	default:
		base.Fatalf("LLVM GoObj target is not configured for %s/%s", buildcfg.GOOS, buildcfg.GOARCH)
		return ""
	}
}

// addGoObjConfigMetadata makes an LLVM IR file self-describing for llc's
// GoObj writer. Keep every Go object-header field structurally separate so llc
// never needs to parse a Go header or a comma-separated experiment list.
func addGoObjConfigMetadata(pkg *types.Pkg) {
	goarchKey, goarchValue := buildcfg.GOGOARCH()
	experiments := buildcfg.Experiment.Enabled()
	experimentMetadata := make([]llvm.Metadata, len(experiments))
	for i, experiment := range experiments {
		experimentMetadata[i] = GlobalCtxt.MDString(experiment)
	}
	shared := "0"
	if *base.Flag.Shared {
		shared = "1"
	}
	main := "0"
	if pkg.Name == "main" {
		main = "1"
	}
	std := "0"
	if base.Ctxt.Std {
		std = "1"
	}
	config := GlobalCtxt.MDNode([]llvm.Metadata{
		GlobalCtxt.MDString("goallc.goobj"),
		GlobalCtxt.MDString(buildcfg.GOOS),
		GlobalCtxt.MDString(buildcfg.GOARCH),
		GlobalCtxt.MDString(buildcfg.Version),
		GlobalCtxt.MDString(goarchKey),
		GlobalCtxt.MDString(goarchValue),
		GlobalCtxt.MDString(base.Flag.BuildID),
		GlobalCtxt.MDString(pkg.Path),
		GlobalCtxt.MDString(main),
		GlobalCtxt.MDString(shared),
		GlobalCtxt.MDString(std),
		GlobalCtxt.MDNode(experimentMetadata),
	})
	CurrentModule.AddNamedMetadataOperand("goobj.config", config)
	emitGoObjStaticRODataType()
}

func finalizeLLVMModule() {
	if llvmModuleFinalized {
		return
	}
	if !goObjConfigWritten {
		addGoObjConfigMetadata(types.LocalPkg)
		goObjConfigWritten = true
	}
	if !goObjImportsWritten {
		emitGoObjImportMetadata()
		emitGoObjCgoModuleAsm()
		goObjImportsWritten = true
	}
	finalizeLLVMDebugInfo()
	llvmModuleFinalized = true
}

func Output(fileName string) error {
	finalizeLLVMModule()
	return llvm.LLVMPrintModuleToFile(CurrentModule, fileName)
}
