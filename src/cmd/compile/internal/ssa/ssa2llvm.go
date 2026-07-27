//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"fmt"

	"github.com/goallc/go-llvm"
)

type LLVMFuncContext struct {
	BBs        map[ID]llvm.BasicBlock
	Vs         map[ID]llvm.Value
	F          *Func
	LF         llvm.Value
	b          llvm.Builder
	ReturnType llvm.Type
	ArgIdx     int
}

// LLVM's GoABIInternal calling convention has numeric ID 22. Keep the
// prototype lowering on the Go register ABI so llc emits GoObj symbols that
// the standard Go linker can call directly.
const goABIInternalCallConv llvm.CallConv = 22

func (lfc *LLVMFuncContext) FinishPhi() {
	for _, BB := range lfc.F.Blocks {
		for _, v := range BB.Values {
			if v.Op != OpPhi {
				continue
			}
			var incomingLVals []llvm.Value
			for _, incoming := range v.Args {
				incomingLVals = append(incomingLVals, lfc.Vs[incoming.ID])
			}
			var predecessors []llvm.BasicBlock
			for _, pred := range BB.Preds {
				predecessors = append(predecessors, lfc.BBs[pred.Block().ID])
			}
			lfc.Vs[v.ID].AddIncoming(incomingLVals, predecessors)
		}
	}
}

func (lfc *LLVMFuncContext) GenLV(v *Value) llvm.Value {
	if lv, ok := lfc.Vs[v.ID]; ok {
		return lv
	}
	var lVal llvm.Value
	switch v.Op {
	case OpInitMem:
	case OpSP:
	case OpSB:
	case OpLocalAddr:
		lVal = lfc.b.CreateAlloca(getLLVMType(v.Type.Elem()), v.String())
	case OpArg:
		lVal = lfc.LF.Param(lfc.ArgIdx)
		// TODO Set arg name
		lfc.ArgIdx++
	case OpConst8, OpConst16, OpConst32, OpConst64:
		lVal = llvm.ConstInt(getLLVMType(v.Type), uint64(auxIntToInt64(v.AuxInt)), v.Type.IsSigned())
	case OpConstString:
		str := auxToString(v.Aux)
		strData := llvm.ConstString(auxToString(v.Aux), false)
		strVal := llvm.AddGlobal(CurrentModule, strData.Type(), str)
		strVal.SetInitializer(strData)
		strVal.SetUnnamedAddr(true)
		strVal.SetLinkage(llvm.LinkOnceAnyLinkage)
		strVal.SetGlobalConstant(true)
		strLen := llvm.ConstInt(getLLVMType(types.Types[types.TINT]), uint64(len(auxToString(v.Aux))), true)
		lVal = llvm.ConstNamedStruct(getLLVMType(v.Type), []llvm.Value{strVal, strLen})
	case OpAdd64, OpAdd32, OpAdd16, OpAdd8:
		lVal = lfc.b.CreateAdd(lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), v.String())
	case OpSub64, OpSub32, OpSub16, OpSub8:
		lVal = lfc.b.CreateSub(lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), v.String())
	case OpLess64:
		lVal = lfc.b.CreateICmp(llvm.IntSLT, lfc.GenLV(v.Args[0]), lfc.GenLV(v.Args[1]), v.String())
	case OpCopy:
		lVal = lfc.GenLV(v.Args[0]) // TODO ?
	case OpMakeResult:
		switch len(v.Args) {
		case 1:
		case 2:
			lVal = lfc.GenLV(v.Args[0])
		default:
			lVal = llvm.Undef(getLLVMType(lfc.F.Type).ReturnType())
			numRes := len(v.Args) - 1
			for i := 0; i < numRes; i++ {
				lVal = lfc.b.CreateInsertValue(lVal, lfc.GenLV(v.Args[i]), i, "")
			}
			lVal.SetName(v.String())
		}
	case OpPhi:
		lVal = lfc.b.CreatePHI(getLLVMType(v.Type), v.String())
	case OpLoad:
		lVal = lfc.b.CreateLoad(getLLVMType(v.Type), lfc.GenLV(v.Args[0]), v.String())
	case OpNilCheck:
		lVal = lfc.Vs[v.Args[0].ID] // TODO nil check
	case OpStore:
		lVal = lfc.b.CreateStore(lfc.GenLV(v.Args[1]), lfc.GenLV(v.Args[0]))
	case OpStructSelect:
		lVal = lfc.b.CreateExtractValue(lfc.GenLV(v.Args[0]), int(auxIntToInt32(v.AuxInt)), v.String())
	case OpStructMake:
		lVal = llvm.Undef(getLLVMType(v.Type))
		numFields := v.Type.NumFields()
		for i := 0; i < numFields; i++ {
			lVal = lfc.b.CreateInsertValue(lVal, lfc.GenLV(v.Args[i]), i, "")
		}
		lVal.SetName(v.String())
	case OpStringLen, OpSliceLen:
		lVal = lfc.b.CreateExtractValue(lfc.GenLV(v.Args[0]), 1, v.String())
	case OpSliceCap:
		lVal = lfc.b.CreateExtractValue(lfc.GenLV(v.Args[0]), 2, v.String())
	default:
		fmt.Println("skip value: ", v)
	}
	lfc.Vs[v.ID] = lVal
	return lVal
}

func (lfc *LLVMFuncContext) CompileBlock(BB *Block) {
	lfc.b.SetInsertPointAtEnd(lfc.BBs[BB.ID])
	//fmt.Println("compiling: ", BB.String())
	//for _, v := range BB.Values {
	//	fmt.Println("value: ", v)
	//}
	//fmt.Println()
	for _, v := range BB.Values {
		lfc.GenLV(v)
	}
	switch BB.Kind {
	case BlockRet:
		if BB.Controls[0].Type.NumFields() == 1 { // TODO type like int,mem
			lfc.b.CreateRetVoid()
		} else {
			lfc.b.CreateRet(lfc.Vs[BB.Controls[0].ID])
		}
	case BlockIf:
		lfc.b.CreateCondBr(lfc.Vs[BB.Controls[0].ID], lfc.BBs[BB.Succs[0].Block().ID], lfc.BBs[BB.Succs[1].Block().ID])
	case BlockPlain:
		lfc.b.CreateBr(lfc.BBs[BB.Succs[0].Block().ID])
	default:

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
	FCtxt := &LLVMFuncContext{
		BBs: map[ID]llvm.BasicBlock{},
		Vs:  map[ID]llvm.Value{},
		F:   f,
		b:   GlobalCtxt.NewBuilder(),
	}

	FCtxt.LF = llvm.AddFunction(CurrentModule, base.Ctxt.Pkgpath+"."+f.Name, getLLVMType(f.Type))
	FCtxt.LF.SetFunctionCallConv(goABIInternalCallConv)
	for _, BB := range f.Blocks {
		FCtxt.BBs[BB.ID] = GlobalCtxt.AddBasicBlock(FCtxt.LF, BB.String())
	}
	for _, BB := range f.Blocks {
		FCtxt.CompileBlock(BB)
	}
	FCtxt.FinishPhi()
	FCtxt.MappingName()

	err := llvm.VerifyFunction(FCtxt.LF, llvm.PrintMessageAction)
	if err != nil {
		//TODO handle error
	}
}

var CurrentModule llvm.Module
var type2lTypes = map[*types.Type]llvm.Type{}

var GlobalCtxt = llvm.GlobalContext()

func getLLVMType(typ *types.Type) llvm.Type {
	if t, ok := type2lTypes[typ]; ok {
		return t
	}
	var lType llvm.Type
	switch typ.Kind() {
	case types.TFUNC:
		var llvmRetType llvm.Type
		var llvmParamTypes []llvm.Type
		switch typ.NumResults() {
		case 0:
			llvmRetType = GlobalCtxt.VoidType()
		case 1:
			llvmRetType = getLLVMType(typ.Result(0).Type)
		default:
			var llvmResTypes []llvm.Type
			for i := 0; i < typ.NumResults(); i++ {
				llvmResTypes = append(llvmResTypes, getLLVMType(typ.Result(i).Type))
			}
			llvmRetType = llvm.StructType(llvmResTypes, false)
			// TODO need add attribute to distinguish with real struct type
		}
		numParam := typ.NumParams()
		for i := 0; i < numParam; i++ {
			llvmParamTypes = append(llvmParamTypes, getLLVMType(typ.Param(i).Type))
		}
		lType = llvm.FunctionType(llvmRetType, llvmParamTypes, false)
	case types.TPTR:
		lType = llvm.PointerType(getLLVMType(typ.Elem()), 0)
	case types.TSTRUCT:
		var llvmFieldTypes []llvm.Type
		numField := typ.NumFields()
		for i := 0; i < numField; i++ {
			llvmFieldTypes = append(llvmFieldTypes, getLLVMType(typ.FieldType(i)))
		}
		if typ.Sym() != nil {
			lType = GlobalCtxt.StructCreateNamed(typ.NameString())
			lType.StructSetBody(llvmFieldTypes, false)
		} else {
			lType = llvm.StructType(llvmFieldTypes, false)
		}
	case types.TSTRING:
		stringFieldTypes := []llvm.Type{GlobalCtxt.PointerType(0), getLLVMType(types.Types[types.TINT])}
		lType = GlobalCtxt.StructCreateNamed("string")
		lType.StructSetBody(stringFieldTypes, false)
	case types.TSLICE:
		sliceFieldTypes := []llvm.Type{GlobalCtxt.PointerType(0),
			getLLVMType(types.Types[types.TINT]),
			getLLVMType(types.Types[types.TINT])}
		lType = GlobalCtxt.StructCreateNamed("slice")
		lType.StructSetBody(sliceFieldTypes, false)
	default:
		fmt.Println("unknown type", typ, ", kind", typ.Kind())
		panic("unhandled default case")
	}
	type2lTypes[typ] = lType
	return lType
}

func InitModule(pkg *types.Pkg) {
	type2lTypes[types.Types[types.TINT8]] = GlobalCtxt.Int8Type()
	type2lTypes[types.Types[types.TUINT8]] = GlobalCtxt.Int8Type()
	type2lTypes[types.Types[types.TINT16]] = GlobalCtxt.Int16Type()
	type2lTypes[types.Types[types.TUINT16]] = GlobalCtxt.Int16Type()
	type2lTypes[types.Types[types.TUINT32]] = GlobalCtxt.Int32Type()
	type2lTypes[types.Types[types.TUINT32]] = GlobalCtxt.Int32Type()
	type2lTypes[types.Types[types.TINT64]] = GlobalCtxt.Int64Type()
	type2lTypes[types.Types[types.TUINT8]] = GlobalCtxt.Int64Type()
	if types.PtrSize == 8 {
		type2lTypes[types.Types[types.TINT]] = GlobalCtxt.Int64Type()
		type2lTypes[types.Types[types.TUINT]] = GlobalCtxt.Int64Type()
		type2lTypes[types.Types[types.TUINTPTR]] = GlobalCtxt.Int64Type()
	} else {
		type2lTypes[types.Types[types.TINT]] = GlobalCtxt.Int32Type()
		type2lTypes[types.Types[types.TUINT]] = GlobalCtxt.Int32Type()
		type2lTypes[types.Types[types.TUINTPTR]] = GlobalCtxt.Int32Type()
	}

	type2lTypes[types.Types[types.TUNSAFEPTR]] = GlobalCtxt.PointerType(0)
	type2lTypes[types.Types[types.TFLOAT32]] = GlobalCtxt.FloatType()
	type2lTypes[types.Types[types.TFLOAT64]] = GlobalCtxt.DoubleType()
	//type2lTypes[types.Types[types.TCOMPLEX64]] =
	//type2lTypes[types.Types[types.TCOMPLEX128]] =
	type2lTypes[types.Types[types.TBOOL]] = GlobalCtxt.Int1Type()

	type2lTypes[types.ByteType] = GlobalCtxt.Int8Type()
	type2lTypes[types.RuneType] = GlobalCtxt.Int32Type()

	CurrentModule = GlobalCtxt.NewModule(pkg.Path)
}

func Output(fileName string) error {
	return llvm.LLVMPrintModuleToFile(CurrentModule, fileName)
}
