//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"internal/buildcfg"

	"github.com/goallc/go-llvm"
)

type LLVMFuncContext struct {
	BBs         map[ID]llvm.BasicBlock
	Vs          map[ID]llvm.Value
	F           *Func
	LF          llvm.Value
	b           llvm.Builder
	ReturnType  llvm.Type
	ResultCount int
}

// LLVM's GoABIInternal calling convention has numeric ID 22. Keep the
// prototype lowering on the Go register ABI so llc emits GoObj symbols that
// the standard Go linker can call directly.
const goABIInternalCallConv llvm.CallConv = 22
const goABI0CallConv llvm.CallConv = 23
const goResultsTupleAttr = "go_results_tuple"

type llvmFuncSignature struct {
	Type        llvm.Type
	ReturnType  llvm.Type
	ResultCount int
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
		Type:        llvm.FunctionType(ret, params, false),
		ReturnType:  ret,
		ResultCount: len(results),
	}
}

func configureLLVMFunction(fn llvm.Value, sig llvmFuncSignature, cc llvm.CallConv) {
	fn.SetFunctionCallConv(cc)
	if sig.ResultCount > 1 {
		fn.AddFunctionAttr(GlobalCtxt.CreateStringAttribute(goResultsTupleAttr, ""))
	}
}

func getOrInsertLLVMFunction(name string, sig llvmFuncSignature, cc llvm.CallConv) llvm.Value {
	fn := CurrentModule.NamedFunction(name)
	if fn.IsNil() {
		fn = llvm.AddFunction(CurrentModule, name, sig.Type)
	} else if got := fn.GlobalValueType(); got != sig.Type {
		base.Fatalf("conflicting LLVM function type for %s", name)
	}
	configureLLVMFunction(fn, sig, cc)
	return fn
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
	if auxIntToBool(v.AuxInt) {
		shifted.SetName(v.String())
		return shifted
	}

	width := llvm.ConstInt(count.Type(), uint64(xBits), false)
	inRange := lfc.b.CreateICmp(llvm.IntULT, count, width, v.String()+".inrange")
	var outOfRange llvm.Value
	if kind == llvmShiftRightSigned {
		lastBit := llvm.ConstInt(x.Type(), uint64(xBits-1), false)
		outOfRange = lfc.b.CreateAShr(x, lastBit, v.String()+".sign")
	} else {
		outOfRange = llvm.ConstNull(x.Type())
	}
	return lfc.b.CreateSelect(inRange, shifted, outOfRange, v.String())
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
	assignment := ParamAssignmentForArgName(lfc.F, v.Aux.(*ir.Name))
	for i, param := range lfc.F.OwnAux.ABIInfo().InParams() {
		if param.Name == assignment.Name {
			return lfc.LF.Param(i)
		}
	}
	v.Fatalf("could not find LLVM parameter for %v", assignment.Name)
	return llvm.Value{}
}

func (lfc *LLVMFuncContext) aggregate(v *Value, args []*Value) llvm.Value {
	result := llvm.Undef(getLLVMType(v.Type))
	for i, arg := range args {
		result = lfc.b.CreateInsertValue(result, lfc.GenLV(arg), i, "")
	}
	result.SetName(v.String())
	return result
}

func (lfc *LLVMFuncContext) staticCall(v *Value) llvm.Value {
	aux := auxToCall(v.Aux)
	if aux == nil || aux.Fn == nil {
		v.Fatalf("static call has no target")
	}
	if got, want := len(v.Args)-1, int(aux.NArgs()); got != want {
		v.Fatalf("static call to %s has %d LLVM arguments, want %d", aux.Fn.Name, got, want)
	}

	sig := llvmSignature(aux)
	cc := llvmCallConv(aux.ABI().Which())
	fn := getOrInsertLLVMFunction(aux.Fn.Name, sig, cc)
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
	return call
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
	call := lfc.b.CreateCall(sig.Type, fn, []llvm.Value{x, y}, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	return call
}

func (lfc *LLVMFuncContext) GenLV(v *Value) llvm.Value {
	if lv, ok := lfc.Vs[v.ID]; ok {
		return lv
	}
	var lVal llvm.Value
	arg0 := func() llvm.Value { return lfc.GenLV(v.Args[0]) }
	arg1 := func() llvm.Value { return lfc.GenLV(v.Args[1]) }
	switch v.Op {
	case OpInitMem, OpSP, OpSB, OpInlMark:
		// LLVM models memory ordering through instruction dependencies, not an
		// explicit SSA memory value. SP/SB are only address-space tokens here.
	case OpUnknown:
		if !v.Type.IsMemory() {
			v.Fatalf("non-memory Unknown value in LLVM lowering: %s", v.LongString())
		}
	case OpVarDef, OpVarLive:
		lVal = arg0()
	case OpKeepAlive:
		lVal = arg1()
	case OpLocalAddr:
		lVal = lfc.b.CreateAlloca(getLLVMType(v.Type.Elem()), v.String())
	case OpArg:
		lVal = lfc.paramForArg(v)
		lVal.SetName(v.Aux.(*ir.Name).Sym().Name)
	case OpConst8, OpConst16, OpConst32, OpConst64:
		lVal = llvm.ConstInt(getLLVMType(v.Type), uint64(auxIntToInt64(v.AuxInt)), v.Type.IsSigned())
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
	case OpEq64, OpEq32, OpEq16, OpEq8, OpEqB, OpEqPtr:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntEQ, arg0(), arg1(), v.String()+".i1"), v.String())
	case OpNeq64, OpNeq32, OpNeq16, OpNeq8, OpNeqB, OpNeqPtr:
		lVal = lfc.goBool(lfc.b.CreateICmp(llvm.IntNE, arg0(), arg1(), v.String()+".i1"), v.String())
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
	case OpCopy:
		lVal = arg0()
		if v.Type.IsMemory() || v.Type.IsVoid() {
			break
		}
		if got, want := lVal.Type(), getLLVMType(v.Type); got != want {
			v.Fatalf("%s changes LLVM representation", v.Op)
		}
	case OpCvtBoolToUint8, OpConvert:
		lVal = arg0()
		if got, want := lVal.Type(), getLLVMType(v.Type); got != want {
			v.Fatalf("%s changes LLVM representation", v.Op)
		}
	case OpOffPtr:
		off := llvm.ConstInt(getLLVMType(types.Types[types.TINT]), uint64(auxIntToInt64(v.AuxInt)), true)
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
	case OpPanicBounds:
		lVal = lfc.panicBounds(v)
	case OpSelectN:
		sel := int(auxIntToInt64(v.AuxInt))
		src := v.Args[0]
		switch src.Op {
		case OpStaticCall, OpStaticLECall:
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
	case OpLoad:
		lVal = lfc.b.CreateLoad(getLLVMType(v.Type), arg0(), v.String())
	case OpStore:
		lVal = lfc.b.CreateStore(arg1(), arg0())
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
	case OpStringPtr, OpSlicePtr, OpComplexReal, OpITab:
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

func (lfc *LLVMFuncContext) CompileBlock(BB *Block) {
	lfc.b.SetInsertPointAtEnd(lfc.BBs[BB.ID])
	for _, v := range BB.Values {
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
	cc := llvmCallConv(f.OwnAux.ABI().Which())
	FCtxt := &LLVMFuncContext{
		BBs:         map[ID]llvm.BasicBlock{},
		Vs:          map[ID]llvm.Value{},
		F:           f,
		b:           GlobalCtxt.NewBuilder(),
		ReturnType:  sig.ReturnType,
		ResultCount: sig.ResultCount,
	}
	defer FCtxt.b.Dispose()

	FCtxt.LF = getOrInsertLLVMFunction(f.OwnAux.Fn.Name, sig, cc)
	if FCtxt.LF.BasicBlocksCount() != 0 {
		f.fe.Fatalf(f.Entry.Pos, "duplicate LLVM definition for %s", f.OwnAux.Fn.Name)
	}
	for _, BB := range f.Blocks {
		FCtxt.BBs[BB.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, BB.String())
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
	for _, BB := range f.Blocks {
		FCtxt.CompileBlock(BB)
	}
	FCtxt.FinishPhi()
	FCtxt.MappingName()

	err := llvm.VerifyFunction(FCtxt.LF, llvm.PrintMessageAction)
	if err != nil {
		f.fe.Fatalf(f.Entry.Pos, "LLVM verifier failed for %s: %v", f.Name, err)
	}
}

var CurrentModule llvm.Module
var type2lTypes = map[*types.Type]llvm.Type{}
var goObjConfigWritten bool

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
	return llvm.LLVMPrintModuleToFile(CurrentModule, fileName)
}
