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

	"github.com/goallc/go-llvm"
)

type LLVMFuncContext struct {
	BBs              map[ID]llvm.BasicBlock
	Vs               map[ID]llvm.Value
	Locals           map[llvmLocalKey]llvmStackSlot
	ItabMethods      map[ID]bool
	ClosureCodeLoads map[ID]bool
	F                *Func
	LF               llvm.Value
	ClosureContext   llvm.Value
	b                llvm.Builder
	ReturnType       llvm.Type
	ResultCount      int
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

// LLVM's GoABIInternal calling convention has numeric ID 22. Keep the
// prototype lowering on the Go register ABI so llc emits GoObj symbols that
// the standard Go linker can call directly.
const goABIInternalCallConv llvm.CallConv = 22
const goABI0CallConv llvm.CallConv = 23
const goResultsTupleAttr = "go_results_tuple"
const goGCStrategy = "goallc"
const goGCLeafFunctionAttr = "gc-leaf-function"
const goStackGrowthStatepointAttr = "go-stack-growth-statepoint"
const goAsyncUnsafeAttr = "go-async-unsafe"
const goWriteBarrierIntrinsic = "llvm.go.gc.write.barrier"
const llvmFramePointerAttr = "frame-pointer"
const llvmFramePointerNonLeaf = "non-leaf"

// Keep fixed-size memmoves within the store expansion limits of the supported
// LLVM targets. Larger moves must use runtime.memmove rather than a libc symbol,
// which the GoObj pipeline cannot resolve.
const llvmInlineMemmoveLimit = 64
const llvmAttributeFunctionIndex = -1

type llvmFuncSignature struct {
	Type                llvm.Type
	ReturnType          llvm.Type
	ResultCount         int
	HasClosureContext   bool
	ClosureContextIndex int
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

func llvmSignature(aux *AuxCall) llvmFuncSignature {
	if aux == nil || aux.ABIInfo() == nil {
		base.Fatalf("missing ABI information in LLVM lowering")
	}

	params := make([]llvm.Type, 0, aux.NArgs())
	for i := int64(0); i < aux.NArgs(); i++ {
		params = append(params, getLLVMType(aux.TypeOfArg(i)))
	}

	results := make([]llvm.Type, 0, aux.NResults())
	for i := int64(0); i < aux.NResults(); i++ {
		results = append(results, getLLVMType(aux.TypeOfResult(i)))
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
		ResultCount:         len(results),
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

func llvmNullPointerIsValidAttribute() llvm.Attribute {
	kind := llvm.AttributeKindID("null_pointer_is_valid")
	if kind == 0 {
		base.Fatalf("LLVM does not provide the null_pointer_is_valid function attribute")
	}
	return GlobalCtxt.CreateEnumAttribute(kind, 0)
}

func configureLLVMFunction(fn llvm.Value, sig llvmFuncSignature, cc llvm.CallConv) {
	fn.SetFunctionCallConv(cc)
	if sig.ResultCount > 1 {
		fn.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goResultsTupleAttr, ""))
	}
	if sig.HasClosureContext {
		// LLVM parameter attribute indexes are one-based. The closure context
		// is deliberately excluded from the Go ABI argument layout by the
		// target and carried in REGCTXT (RDX on amd64, X26 on arm64).
		fn.AddAttributeAtIndex(sig.ClosureContextIndex+1, llvmNestAttribute())
	}
}

func getOrInsertLLVMFunction(name string, sig llvmFuncSignature, cc llvm.CallConv) llvm.Value {
	fn := CurrentModule.NamedFunction(name)
	if fn.IsNil() {
		fn = llvm.AddFunction(CurrentModule, name+".goallc.final", sig.Type)
		if placeholder := CurrentModule.NamedGlobal(name); !placeholder.IsNil() {
			// An OpAddr may have needed the code address before this function
			// reached the compile queue. Opaque pointers let the provisional
			// global be replaced by the correctly typed function definition.
			placeholder.ReplaceAllUsesWith(fn)
			placeholder.EraseFromParentAsGlobal()
		}
		fn.SetName(name)
	} else if got := fn.GlobalValueType(); got != sig.Type {
		if fn.BasicBlocksCount() != 0 {
			base.Fatalf("conflicting LLVM function type for definition %s", name)
		}
		// Compiler data can refer to an ABI function before AuxCall exposes
		// its exact signature. Replace that provisional declaration now.
		replacement := llvm.AddFunction(CurrentModule, name+".goallc.final", sig.Type)
		fn.ReplaceAllUsesWith(replacement)
		fn.EraseFromParentAsFunction()
		replacement.SetName(name)
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

func (lfc *LLVMFuncContext) llvmUnaryFloat64Intrinsic(v *Value, name string) llvm.Value {
	x := lfc.GenLV(v.Args[0])
	f64 := GlobalCtxt.DoubleType()
	if got, want := x.Type(), f64; got != want || getLLVMType(v.Type) != want {
		v.Fatalf("%s requires a float64 operand and result", v.Op)
	}
	sig := llvm.FunctionType(f64, []llvm.Type{f64}, false)
	fn := getOrInsertLLVMIntrinsic(name, sig)
	return lfc.b.CreateCall(sig, fn, []llvm.Value{x}, v.String())
}

func markLLVMGCLeaf(fn, call llvm.Value) {
	attr := GlobalCtxt.CreateStringAttribute(goGCLeafFunctionAttr, "")
	fn.AddFunctionAttr(attr)
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
	p := lfc.GenLV(v.Args[arg])
	if p.Type().TypeKind() != llvm.PointerTypeKind {
		v.Fatalf("%s argument %d has non-pointer LLVM type", v.Op, arg)
	}
	return p
}

func (lfc *LLVMFuncContext) llvmZero(v *Value) llvm.Value {
	size, align := llvmMemoryOpInfo(v)
	dst := lfc.llvmMemoryPointer(v, 0)
	length := lfc.llvmMemoryLength(v, size)
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
		llvm.ConstInt(GlobalCtxt.Int1Type(), 0, false),
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
	fn := getOrInsertLLVMFunction("runtime.memmove", sig, goABIInternalCallConv)
	attachGoObjABISymbolRef(fn, "runtime.memmove", obj.ABIInternal)
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{dst, src, length}, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	markLLVMGCLeaf(fn, call)
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
		ClosureContextIndex: -1,
	}
	fn := getOrInsertLLVMFunction("runtime.memequal", sig, goABIInternalCallConv)
	attachGoObjABISymbolRef(fn, "runtime.memequal", obj.ABIInternal)
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{left, right, size}, v.String())
	call.SetInstructionCallConv(goABIInternalCallConv)
	markLLVMGCLeaf(fn, call)
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
	if x.Type().TypeKind() != llvm.IntegerTypeKind || count.Type() != x.Type() || x.Type() != getLLVMType(v.Type) {
		v.Fatalf("%s has incompatible LLVM operand types", v.Op)
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

func (lfc *LLVMFuncContext) FinishPhi() {
	for _, BB := range lfc.F.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpPhi {
				continue
			}
			if v.Type.IsMemory() {
				continue
			}
			var incomingLVals []llvm.Value
			for _, incoming := range v.Args {
				incomingLVal, ok := lfc.Vs[incoming.ID]
				if !ok {
					v.Fatalf("phi input %s was not emitted in its defining block", incoming)
				}
				if incomingLVal.IsNil() {
					v.Fatalf("phi input %s produced no LLVM value", incoming.LongString())
				}
				if got, want := incomingLVal.Type(), lfc.Vs[v.ID].Type(); got != want {
					v.Fatalf("phi input %s has LLVM kind %s, want %s", incoming.LongString(), got.TypeKind(), want.TypeKind())
				}
				incomingLVals = append(incomingLVals, incomingLVal)
			}
			var predecessors []llvm.BasicBlock
			for _, pred := range BB.Preds {
				predecessors = append(predecessors, lfc.BBs[pred.Block().ID])
			}
			lfc.Vs[v.ID].AddIncoming(incomingLVals, predecessors)
		}
	}
}

func (lfc *LLVMFuncContext) paramForArg(v *Value) llvm.Value {
	return lfc.paramForArgName(v, v.Aux.(*ir.Name))
}

func (lfc *LLVMFuncContext) paramForArgName(v *Value, name *ir.Name) llvm.Value {
	key := llvmLocalKeyForName(name)
	for i, param := range lfc.F.OwnAux.ABIInfo().InParams() {
		if param.Name != nil && llvmLocalKeyForName(param.Name) == key {
			return lfc.LF.Param(i)
		}
	}
	v.Fatalf("could not find LLVM parameter for %v", name)
	return llvm.Value{}
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
	if len(elementTypes) != len(args) {
		v.Fatalf("%s has %d LLVM aggregate elements for %d SSA arguments", v.Op, len(elementTypes), len(args))
	}
	for i, arg := range args {
		value := lfc.GenLV(arg)
		if value.IsNil() {
			v.Fatalf("aggregate field %d from %s produced no LLVM value", i, arg.LongString())
		}
		if got, want := value.Type(), elementTypes[i]; got != want {
			v.Fatalf("aggregate field %d from %s has LLVM kind %s for Go type %v, want %s in Go aggregate %v", i, arg.LongString(), got.TypeKind(), arg.Type, want.TypeKind(), v.Type)
		}
		result = lfc.b.CreateInsertValue(result, value, i, "")
	}
	result.SetName(v.String())
	return result
}

// llvmStaticCallSignature restores semantic pointer types for compiler-built
// runtime calls whose AuxCall uses uintptr only to compute physical ABI
// assignments. AuxCall remains the physical ABI authority; the LLVM operands
// and runtime helper parameters are pointers.
func llvmStaticCallSignature(v *Value, aux *AuxCall, sig llvmFuncSignature) llvmFuncSignature {
	if aux == nil || aux.Fn == nil {
		return sig
	}
	wantArgs := int64(0)
	switch aux.Fn {
	case ir.Syms.Newproc:
		wantArgs = 1
	case ir.Syms.WBZero:
		wantArgs = 2
	case ir.Syms.WBMove:
		wantArgs = 3
	default:
		return sig
	}
	if aux.ABI().Which() != obj.ABIInternal {
		v.Fatalf("%s uses unsupported ABI %v", aux.Fn.Name, aux.ABI().Which())
	}
	if aux.NArgs() != wantArgs || aux.NResults() != 0 {
		v.Fatalf("%s has unexpected raw call signature: %d arguments, %d results", aux.Fn.Name, aux.NArgs(), aux.NResults())
	}
	if aux.Fn == ir.Syms.Newproc && (len(v.Args) != 2 || v.Args[0].Type == nil || !v.Args[0].Type.IsPtrShaped()) {
		v.Fatalf("runtime.newproc argument is not pointer-shaped")
	}
	for i := int64(0); i < aux.NArgs(); i++ {
		if typ := aux.TypeOfArg(i); typ == nil || !typ.IsUintptr() {
			v.Fatalf("argument %d to %s is not raw uintptr", i, aux.Fn.Name)
		}
	}
	params := append([]llvm.Type(nil), sig.Type.ParamTypes()...)
	for i := range params {
		params[i] = GlobalCtxt.PointerType(0)
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

	sig := llvmStaticCallSignature(v, aux, llvmSignature(aux))
	cc := llvmCallConv(aux.ABI().Which())
	fn := getOrInsertLLVMFunction(aux.Fn.Name, sig, cc)
	attachGoObjSymbolRef(fn, aux.Fn)
	// AMD64 rewrites some Move and Eq operations to static runtime calls before
	// LLVM emission. Keep the same leaf contract as the dedicated LLVM lowering
	// paths so RewriteStatepointsForGC does not turn these raw helpers into
	// statepoints.
	llvmGCLeaf := aux.Fn == ir.Syms.WBZero || aux.Fn == ir.Syms.WBMove ||
		aux.Fn == ir.Syms.Memmove || aux.Fn == ir.Syms.Memequal
	args := make([]llvm.Value, 0, aux.NArgs())
	for i := int64(0); i < aux.NArgs(); i++ {
		arg := lfc.GenLV(v.Args[i])
		if got, want := arg.Type(), sig.Type.ParamTypes()[i]; got != want {
			v.Fatalf("argument %d to %s has incompatible LLVM type", i, aux.Fn.Name)
		}
		args = append(args, arg)
	}
	name := v.String()
	if sig.ResultCount == 0 {
		name = ""
	}
	call := lfc.b.CreateCall(sig.Type, fn, args, name)
	call.SetInstructionCallConv(cc)
	if llvmGCLeaf {
		markLLVMGCLeaf(fn, call)
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
	args := make([]llvm.Value, 0, aux.NArgs())
	for i := int64(0); i < aux.NArgs(); i++ {
		arg := lfc.GenLV(v.Args[argStart+int(i)])
		if got, want := arg.Type(), sig.Type.ParamTypes()[i]; got != want {
			v.Fatalf("argument %d to indirect call has incompatible LLVM type", i)
		}
		args = append(args, arg)
	}
	if closureContext {
		context := lfc.GenLV(v.Args[1])
		if context.Type().TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("closure context has non-pointer LLVM type")
		}
		args = append(args, context)
	}
	name := v.String()
	if sig.ResultCount == 0 {
		name = ""
	}
	call := lfc.b.CreateCall(sig.Type, code, args, name)
	call.SetInstructionCallConv(cc)
	if closureContext {
		call.AddCallSiteAttribute(sig.ClosureContextIndex+1, llvmNestAttribute())
	}
	return call
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
	// NeedCtxt is the authoritative ABI property. Early deadcode may remove an
	// unused OpGetClosurePtr while the hidden closure-context parameter must
	// remain in the function signature.
	if usesContext && !needContext {
		f.fe.Fatalf(f.Entry.Pos, "closure context mismatch for %s: NEEDCTXT=%t, OpGetClosurePtr=%t", f.Name, needContext, usesContext)
	}
	if needContext && f.OwnAux.ABI().Which() != obj.ABIInternal {
		f.fe.Fatalf(f.Entry.Pos, "closure context on unsupported ABI %v for %s", f.OwnAux.ABI().Which(), f.Name)
	}
	return needContext
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
	fn := getOrInsertLLVMFunction(llvmBoundsPanicNames[kind], sig, goABIInternalCallConv)
	attachGoObjABISymbolRef(fn, llvmBoundsPanicNames[kind], obj.ABIInternal)
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
	if v.Block != nil {
		lfc.b.SetInsertPointAtEnd(lfc.BBs[v.Block.ID])
	}
	defer func() {
		if !savedBlock.IsNil() {
			lfc.b.SetInsertPointAtEnd(savedBlock)
		}
	}()
	var lVal llvm.Value
	arg0 := func() llvm.Value { return lfc.GenLV(v.Args[0]) }
	arg1 := func() llvm.Value { return lfc.GenLV(v.Args[1]) }
	switch v.Op {
	case OpInitMem, OpSP, OpSB, OpInlMark, OpWBend:
		// LLVM models memory ordering through instruction dependencies, not an
		// explicit SSA memory value. SP/SB are only address-space tokens here.
	case OpUnknown:
		// SSA construction leaves Unknown values only in dead code. Preserve
		// their "value does not matter" semantics as LLVM undef; live Go
		// values are resolved before this point.
		if !v.Type.IsMemory() && v.Type != types.TypeInvalid {
			lVal = llvm.Undef(getLLVMType(v.Type))
		}
	case OpVarDef, OpVarLive:
		lVal = arg0()
	case OpKeepAlive:
		lVal = arg1()
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
	case OpAddr:
		sym, ok := v.Aux.(*obj.LSym)
		if !ok {
			v.Fatalf("global address has non-LSym auxiliary %T", v.Aux)
		}
		lVal = llvmGoDataRef(sym)
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
	case OpAdd64, OpAdd32, OpAdd16, OpAdd8:
		lVal = lfc.b.CreateAdd(arg0(), arg1(), v.String())
	case OpAdd32F, OpAdd64F:
		lVal = lfc.b.CreateFAdd(arg0(), arg1(), v.String())
	case OpSub64, OpSub32, OpSub16, OpSub8:
		lVal = lfc.b.CreateSub(arg0(), arg1(), v.String())
	case OpSub32F, OpSub64F:
		lVal = lfc.b.CreateFSub(arg0(), arg1(), v.String())
	case OpMul64, OpMul32, OpMul16, OpMul8:
		lVal = lfc.b.CreateMul(arg0(), arg1(), v.String())
	case OpHmul32, OpHmul64:
		lVal = lfc.highMultiply(v, true)
	case OpHmul32u, OpHmul64u:
		lVal = lfc.highMultiply(v, false)
	case OpAvg32u, OpAvg64u:
		lVal = lfc.unsignedAverage(v)
	case OpMul32F, OpMul64F:
		lVal = lfc.b.CreateFMul(arg0(), arg1(), v.String())
	case OpDiv64, OpDiv32, OpDiv16, OpDiv8:
		lVal = lfc.integerDiv(v, true, false)
	case OpDiv64u, OpDiv32u, OpDiv16u, OpDiv8u:
		lVal = lfc.integerDiv(v, false, false)
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
		lVal = lfc.llvmUnaryFloat64Intrinsic(v, "llvm.sqrt.f64")
	case OpAbs:
		lVal = lfc.llvmUnaryFloat64Intrinsic(v, "llvm.fabs.f64")
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
		lVal = lfc.b.CreateFPToSI(arg0(), getLLVMType(v.Type), v.String())
	case OpCvt32Fto32U, OpCvt32Fto64U, OpCvt64Fto32U, OpCvt64Fto64U:
		lVal = lfc.b.CreateFPToUI(arg0(), getLLVMType(v.Type), v.String())
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
			lVal = lfc.b.CreatePtrToInt(lVal, want, v.String()+".coerce")
		case lVal.Type().TypeKind() == llvm.IntegerTypeKind &&
			lVal.Type().IntTypeWidth() == types.PtrSize*8 &&
			want.TypeKind() == llvm.PointerTypeKind:
			lVal = lfc.b.CreateIntToPtr(lVal, want, v.String()+".coerce")
		default:
			v.Fatalf("%s changes machine representation", v.Op)
		}
	case OpOffPtr:
		off := llvmConstInt(types.Types[types.TINT], auxIntToInt64(v.AuxInt))
		lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), arg0(), []llvm.Value{off}, v.String())
	case OpAddPtr:
		lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), arg0(), []llvm.Value{arg1()}, v.String())
	case OpSubPtr:
		neg := lfc.b.CreateNeg(arg1(), v.String()+".neg")
		lVal = lfc.b.CreateGEP(GlobalCtxt.Int8Type(), arg0(), []llvm.Value{neg}, v.String())
	case OpPtrIndex:
		lVal = lfc.b.CreateGEP(getLLVMType(v.Type.Elem()), arg0(), []llvm.Value{arg1()}, v.String())
	case OpStaticCall, OpStaticLECall:
		lVal = lfc.staticCall(v)
	case OpWB:
		lVal = lfc.llvmWriteBarrier(v)
	case OpClosureCall, OpClosureLECall:
		// arg0 is the code pointer loaded from the funcval, arg1 is the
		// funcval itself. The latter is a hidden REGCTXT input, not an
		// ordinary Go ABI argument.
		lVal = lfc.indirectCall(v, 2, true)
	case OpInterCall, OpInterLECall:
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
		case OpAtomicLoadPtr:
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
			v.Fatalf("%s selects unsupported tuple source %s", v.Op, src.Op)
		}
	case OpSelectN:
		sel := int(auxIntToInt64(v.AuxInt))
		src := v.Args[0]
		switch src.Op {
		case OpStaticCall, OpStaticLECall, OpClosureCall, OpClosureLECall, OpInterCall, OpInterLECall:
			aux := auxToCall(src.Aux)
			call := lfc.GenLV(src)
			switch {
			case sel >= int(aux.NResults()):
				// Selecting the trailing SSA memory dependency only forces the
				// call to be emitted; it has no LLVM value.
			case aux.NResults() == 1:
				lVal = call
			default:
				lVal = lfc.b.CreateExtractValue(call, sel, v.String())
			}
		case OpAtomicLoadPtr:
			load := lfc.GenLV(src)
			if sel == 0 {
				lVal = load
			}
		default:
			lVal = lfc.b.CreateExtractValue(lfc.GenLV(src), sel, v.String())
		}
	case OpMakeResult:
		switch lfc.ResultCount {
		case 0:
		case 1:
			lVal = lfc.GenLV(v.Args[0])
		default:
			lVal = llvm.Undef(lfc.ReturnType)
			for i := 0; i < lfc.ResultCount; i++ {
				lVal = lfc.b.CreateInsertValue(lVal, lfc.GenLV(v.Args[i]), i, "")
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
		if addr.Type().TypeKind() != llvm.PointerTypeKind {
			v.Fatalf("%s address has non-pointer LLVM type", v.Op)
		}
		lVal = lfc.b.CreateLoad(typ, addr, v.String())
		if v.Op == OpDereference {
			align := v.Type.Alignment()
			if align <= 0 || align&(align-1) != 0 {
				v.Fatalf("%s has invalid alignment %d for %v", v.Op, align, v.Type)
			}
			lVal.SetAlignment(int(align))
		}
	case OpAtomicLoadPtr:
		lVal = lfc.b.CreateLoad(GlobalCtxt.PointerType(0), arg0(), v.String())
		lVal.SetOrdering(llvm.AtomicOrderingSequentiallyConsistent)
	case OpNilCheck:
		lVal = lfc.emitNilCheckIntrinsic(v)
	case OpStore:
		lVal = lfc.b.CreateStore(arg1(), arg0())
	case OpZero:
		lVal = lfc.llvmZero(v)
	case OpMove:
		lVal = lfc.llvmMove(v)
	case OpMemEq:
		lVal = lfc.llvmMemEq(v)
	case OpSlicemask:
		lVal = lfc.llvmSlicemask(v)
	case OpStructSelect:
		lVal = lfc.b.CreateExtractValue(arg0(), int(auxIntToInt32(v.AuxInt)), v.String())
	case OpStructMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpArrayMake1:
		lVal = lfc.aggregate(v, v.Args)
	case OpArraySelect:
		lVal = lfc.b.CreateExtractValue(arg0(), int(auxIntToInt64(v.AuxInt)), v.String())
	case OpStringMake, OpComplexMake, OpIMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpSliceMake:
		lVal = lfc.aggregate(v, v.Args)
	case OpStringPtr, OpSlicePtr, OpSlicePtrUnchecked, OpComplexReal, OpITab:
		lVal = lfc.b.CreateExtractValue(arg0(), 0, v.String())
	case OpComplexImag, OpIData:
		lVal = lfc.b.CreateExtractValue(arg0(), 1, v.String())
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
	for _, v := range values {
		lfc.GenLV(v)
	}
	switch BB.Kind {
	case BlockRet:
		if lfc.ResultCount == 0 {
			lfc.b.CreateRetVoid()
		} else {
			lfc.b.CreateRet(lfc.GenLV(BB.Controls[0]))
		}
	case BlockIf:
		cond := lfc.llvmCondition(lfc.GenLV(BB.Controls[0]), BB.String()+".cond")
		lfc.b.CreateCondBr(cond, lfc.BBs[BB.Succs[0].Block().ID], lfc.BBs[BB.Succs[1].Block().ID])
	case BlockPlain:
		lfc.b.CreateBr(lfc.BBs[BB.Succs[0].Block().ID])
	case BlockExit:
		lfc.b.CreateUnreachable()
	case BlockJumpTable:
		index := lfc.GenLV(BB.Controls[0])
		table := lfc.b.CreateSwitch(index, lfc.BBs[BB.Succs[0].Block().ID], len(BB.Succs))
		for i, succ := range BB.Succs {
			table.AddCase(llvm.ConstInt(index.Type(), uint64(i), false), lfc.BBs[succ.Block().ID])
		}
	default:
		BB.Func.fe.Fatalf(BB.Pos, "unsupported SSA block kind in LLVM lowering: %s", BB.Kind)
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
		BBs:              map[ID]llvm.BasicBlock{},
		Vs:               map[ID]llvm.Value{},
		Locals:           map[llvmLocalKey]llvmStackSlot{},
		ItabMethods:      map[ID]bool{},
		ClosureCodeLoads: map[ID]bool{},
		F:                f,
		b:                GlobalCtxt.NewBuilder(),
		ReturnType:       sig.ReturnType,
		ResultCount:      sig.ResultCount,
	}
	defer FCtxt.b.Dispose()

	FCtxt.LF = getOrInsertLLVMFunction(f.OwnAux.Fn.Name, sig, cc)
	if FCtxt.LF.BasicBlocksCount() != 0 {
		f.fe.Fatalf(f.Entry.Pos, "duplicate LLVM definition for %s", f.OwnAux.Fn.Name)
	}
	FCtxt.LF.SetGC(goGCStrategy)
	inParams := f.OwnAux.ABIInfo().InParams()
	if got, want := len(inParams), int(f.OwnAux.NArgs()); got != want {
		f.fe.Fatalf(f.Entry.Pos, "LLVM parameter metadata count %d does not match signature count %d for %s", got, want, f.Name)
	}
	for i, param := range inParams {
		if param.Name != nil {
			FCtxt.LF.Param(i).SetName(param.Name.Sym().Name)
		}
	}
	FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goAsyncUnsafeAttr, ""))
	// TODO(goallc): Once LLVM lowering propagates the compiler's precise
	// morestack policy, attach this only to functions whose prologue can grow
	// the Go stack.
	FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goStackGrowthStatepointAttr, ""))
	FCtxt.LF.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(llvmFramePointerAttr, llvmFramePointerNonLeaf))
	setGoObjMarkerRelocMetadata(FCtxt.LF, f.OwnAux.Fn)
	if sig.HasClosureContext {
		FCtxt.ClosureContext = FCtxt.LF.Param(sig.ClosureContextIndex)
		FCtxt.ClosureContext.SetName(".closureptr")
	}
	// LLVM defines the first block in a function as its entry. Go passes may
	// reorder f.Blocks without changing f.Entry, so create the real entry first
	// and retain the relative order of every remaining block.
	FCtxt.BBs[f.Entry.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, f.Entry.String())
	for _, BB := range f.Blocks {
		if BB == f.Entry {
			continue
		}
		FCtxt.BBs[BB.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, BB.String())
	}
	for _, BB := range f.Blocks {
		for _, v := range BB.Values {
			if (v.Op == OpInterCall || v.Op == OpInterLECall) && len(v.Args) != 0 {
				code := v.Args[0]
				if code.Op == OpLoad && code.Type.IsUintptr() && code.Uses == 1 {
					FCtxt.ItabMethods[code.ID] = true
				}
			}
			if (v.Op == OpClosureCall || v.Op == OpClosureLECall) && len(v.Args) >= 2 {
				code := v.Args[0]
				switch code.Op {
				case OpLoad:
					if !code.Type.IsUintptr() || code.Uses != 1 {
						v.Fatalf("closure call code pointer is not a single-use uintptr load")
					}
					if len(code.Args) == 0 || code.Args[0] != v.Args[1] {
						v.Fatalf("closure call code pointer was not loaded from its funcval context")
					}
					FCtxt.ClosureCodeLoads[code.ID] = true
				case OpAddr:
					if !code.Type.IsUintptr() {
						v.Fatalf("direct closure call code address has type %v", code.Type)
					}
				default:
					v.Fatalf("closure call code pointer has unsupported form %s", code.Op)
				}
			} else if v.Op == OpClosureCall || v.Op == OpClosureLECall {
				v.Fatalf("closure call has %d SSA arguments, want at least code, context, and memory", len(v.Args))
			}
		}
	}
	if len(FCtxt.ClosureCodeLoads) != 0 {
		// Native Go relies on the funcval code-word load faulting when the
		// funcval is nil. Its result feeds the indirect call, so keeping null
		// dereferences defined is enough to retain the single ordinary load.
		FCtxt.LF.AddFunctionAttr(llvmNullPointerIsValidAttribute())
	}
	// LLVM only treats a constant-sized alloca as a fixed frame object when
	// it is in the entry block. Preallocate every Go stack slot before phi or
	// ordinary instruction emission so LocalAddr values in loops and branches
	// cannot become dynamic allocas (which Go stack growth cannot support).
	FCtxt.b.SetInsertPointAtEnd(FCtxt.BBs[f.Entry.ID])
	var parameterHomes []*Value
	for _, BB := range f.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpLocalAddr || v.Uses == 0 {
				continue
			}
			name, key := llvmLocalName(v)
			if slot, ok := FCtxt.Locals[key]; ok {
				if !types.Identical(slot.Type, name.Type()) {
					v.Fatalf("conflicting Go types for local stack slot %v", name)
				}
				continue
			}
			if name.Type().Alignment() <= 0 {
				v.Fatalf("invalid alignment %d for local stack slot %v", name.Type().Alignment(), name)
			}
			slot := FCtxt.b.CreateAlloca(getLLVMType(name.Type()), v.String())
			slot.SetAlignment(int(name.Type().Alignment()))
			FCtxt.Locals[key] = llvmStackSlot{Value: slot, Type: name.Type()}
			if name.Class == ir.PPARAM {
				parameterHomes = append(parameterHomes, v)
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
	// them. A future optimization may bind wholly stack-assigned parameters
	// directly to their incoming fixed stack slots.
	FCtxt.b.SetInsertPointAtEnd(FCtxt.BBs[f.Entry.ID])
	for _, v := range parameterHomes {
		name, key := llvmLocalName(v)
		slot := FCtxt.Locals[key]
		param := FCtxt.paramForArgName(v, name)
		if param.Type() != getLLVMType(slot.Type) {
			v.Fatalf("parameter home changes LLVM representation")
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
var currentLLVMDataLowerer *llvmDataLowerer
var goObjCompilerUsed []llvm.Value
var goObjCompilerUsedNames map[string]bool

var GlobalCtxt = llvm.GlobalContext()

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
	case types.TPTR, types.TUNSAFEPTR, types.TFUNC, types.TMAP, types.TCHAN:
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
			fieldTypes := make([]llvm.Type, typ.NumFields())
			for i := range fieldTypes {
				fieldTypes[i] = getLLVMType(typ.FieldType(i))
			}
			lType.StructSetBody(fieldTypes, false)
		} else {
			fieldTypes := make([]llvm.Type, typ.NumFields())
			for i := range fieldTypes {
				fieldTypes[i] = getLLVMType(typ.FieldType(i))
			}
			lType = llvm.StructType(fieldTypes, false)
		}
	case types.TSTRING:
		lType = llvm.StructType([]llvm.Type{ptrType(), getLLVMType(types.Types[types.TINT])}, false)
	case types.TSLICE:
		lType = llvm.StructType([]llvm.Type{
			ptrType(),
			getLLVMType(types.Types[types.TINT]),
			getLLVMType(types.Types[types.TINT]),
		}, false)
	case types.TINTER:
		lType = llvm.StructType([]llvm.Type{ptrType(), ptrType()}, false)
	case types.TCOMPLEX64:
		lType = llvm.StructType([]llvm.Type{GlobalCtxt.FloatType(), GlobalCtxt.FloatType()}, false)
	case types.TCOMPLEX128:
		lType = llvm.StructType([]llvm.Type{GlobalCtxt.DoubleType(), GlobalCtxt.DoubleType()}, false)
	case types.TTUPLE, types.TRESULTS:
		var fields []llvm.Type
		for i := 0; i < typ.NumFields(); i++ {
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

	CurrentModule = GlobalCtxt.NewModule(pkg.Path)
	CurrentModule.SetTarget(goObjTargetTriple())
	goObjConfigWritten = false
	goObjImportsWritten = false
	currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
}

// goObjTargetTriple identifies the GoObj target that llc should use when it
// consumes the IR produced by this compiler. Keep this in the IR rather than
// making the toolexec wrapper rediscover it from the build environment.
func goObjTargetTriple() string {
	switch buildcfg.GOOS + "/" + buildcfg.GOARCH {
	case "darwin/arm64":
		return "aarch64-apple-darwin-goobj"
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
		GlobalCtxt.MDNode(experimentMetadata),
	})
	CurrentModule.AddNamedMetadataOperand("goobj.config", config)
}

func Output(fileName string) error {
	if !goObjConfigWritten {
		addGoObjConfigMetadata(types.LocalPkg)
		goObjConfigWritten = true
	}
	if !goObjImportsWritten {
		emitGoObjImportMetadata()
		goObjImportsWritten = true
	}
	return llvm.LLVMPrintModuleToFile(CurrentModule, fileName)
}
