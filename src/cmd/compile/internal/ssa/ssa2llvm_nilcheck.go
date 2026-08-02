//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"

	"github.com/goallc/go-llvm"
)

const llvmPanicmemName = "runtime.panicmem"
const llvmNilCheckIntrinsicName = "llvm.goallc.nilcheck"

// llvmPanicmem returns the compiler-owned Go object symbol and the only LLVM
// signature accepted for runtime.panicmem. Unlike ordinary compiler-generated
// runtime calls, OpNilCheck has no AuxCall and panicmem is not part of the
// compiler builtin table, so keep this adaptation local to LLVM lowering.
func llvmPanicmem() (llvm.Value, llvmFuncSignature) {
	sym := base.Ctxt.LookupABI(llvmPanicmemName, obj.ABIInternal)
	if sym == nil || sym.Name != llvmPanicmemName || sym.ABI() != obj.ABIInternal {
		base.Fatalf("invalid LLVM runtime symbol model for %s", llvmPanicmemName)
	}
	sig := llvmFuncSignature{
		Type:       llvm.FunctionType(GlobalCtxt.VoidType(), nil, false),
		ReturnType: GlobalCtxt.VoidType(),
	}
	fn := getOrInsertLLVMFunction(sym.Name, sig, goABIInternalCallConv)
	attachGoObjSymbolRef(fn, sym)
	return fn, sig
}

// emitNilCheckIntrinsic preserves the Go nil-check side effect without
// changing the CFG while ordinary SSA values and block terminators are being
// translated. expandNilCheckIntrinsics removes every marker before LLVM IR is
// verified, optimized, or emitted.
func (lfc *LLVMFuncContext) emitNilCheckIntrinsic(v *Value) llvm.Value {
	if len(v.Args) != 2 || !v.Args[1].Type.IsMemory() {
		v.Fatalf("NilCheck has invalid arguments")
	}

	p := lfc.GenLV(v.Args[0])
	checked := p
	switch p.Type().TypeKind() {
	case llvm.PointerTypeKind:
	case llvm.IntegerTypeKind:
		if p.Type().IntTypeWidth() != types.PtrSize*8 {
			v.Fatalf("NilCheck integer address has width %d, want %d", p.Type().IntTypeWidth(), types.PtrSize*8)
		}
		checked = lfc.b.CreateIntToPtr(p, GlobalCtxt.PointerType(0), v.String()+".ptr")
	default:
		v.Fatalf("NilCheck address has unsupported LLVM type")
	}
	sig := llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{checked.Type()}, false)
	intrinsic := getOrInsertLLVMIntrinsic(llvmNilCheckIntrinsicName, sig)
	lfc.b.CreateCall(sig, intrinsic, []llvm.Value{checked}, "")
	return p
}

func (lfc *LLVMFuncContext) nilCheckMarkers(intrinsic llvm.Value) []llvm.Value {
	var markers []llvm.Value
	for _, bb := range lfc.F.Blocks {
		for inst := lfc.BBs[bb.ID].FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
			if inst.IsACallInst().IsNil() || inst.CalledValue() != intrinsic {
				continue
			}
			markers = append(markers, inst)
		}
	}
	return markers
}

// replacePhiPredecessor repairs successor phi nodes after a nil-check marker
// splits an already complete LLVM block.
func (lfc *LLVMFuncContext) replacePhiPredecessor(from, to llvm.BasicBlock) {
	for _, bb := range lfc.F.Blocks {
		for _, v := range bb.Values {
			if v.Op != OpPhi || v.Type.IsMemory() {
				continue
			}
			lfc.Vs[v.ID].ReplaceIncomingBlock(from, to)
		}
	}
}

// expandNilCheckIntrinsics lowers all temporary nil-check markers after the
// original LLVM CFG and phi nodes are complete.
func (lfc *LLVMFuncContext) expandNilCheckIntrinsics() {
	intrinsic := CurrentModule.NamedFunction(llvmNilCheckIntrinsicName)
	if intrinsic.IsNil() {
		return
	}
	if intrinsic.BasicBlocksCount() != 0 {
		lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "LLVM nil-check intrinsic has a definition")
	}
	expectedType := llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{GlobalCtxt.PointerType(0)}, false)
	if intrinsic.GlobalValueType() != expectedType {
		lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "LLVM nil-check intrinsic has an invalid type")
	}

	markers := lfc.nilCheckMarkers(intrinsic)
	b := GlobalCtxt.NewBuilder()
	defer b.Dispose()

	for _, call := range markers {
		if call.OperandsCount() != 2 || call.Operand(0).Type() != GlobalCtxt.PointerType(0) {
			lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "LLVM nil-check intrinsic call has invalid operands")
		}
		before := call.InstructionParent()
		if before.IsNil() || llvm.NextInstruction(call).IsNil() {
			lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "LLVM nil-check intrinsic is not followed by a block terminator")
		}

		panicBlock := GlobalCtxt.AddBasicBlock(lfc.LF, "nilcheck.nil")
		continueBlock := GlobalCtxt.AddBasicBlock(lfc.LF, "nilcheck.notnil")
		b.SetInsertPointAtEnd(continueBlock)
		for inst := llvm.NextInstruction(call); !inst.IsNil(); {
			next := llvm.NextInstruction(inst)
			inst.RemoveFromParentAsInstruction()
			b.Insert(inst)
			inst = next
		}

		checked := call.Operand(0)
		call.EraseFromParentAsInstruction()
		b.SetInsertPointAtEnd(before)
		isNil := b.CreateICmp(llvm.IntEQ, checked, llvm.ConstNull(checked.Type()), "nilcheck.isnil")
		b.CreateCondBr(isNil, panicBlock, continueBlock)

		b.SetInsertPointAtEnd(panicBlock)
		panicmem, sig := llvmPanicmem()
		panicCall := b.CreateCall(sig.Type, panicmem, nil, "")
		panicCall.SetInstructionCallConv(goABIInternalCallConv)
		// panicmem enters the runtime panic path and may reach GC while this
		// frame is suspended, so it remains an ordinary non-leaf call for
		// statepoint and stack-map construction.
		//
		// A recovered panic resumes in the caller, never after this call. Keep
		// the conservative continuation edge until a future noreturn form can
		// also guarantee a valid call return PC and PCSP range.
		b.CreateBr(continueBlock)

		lfc.replacePhiPredecessor(before, continueBlock)
	}

	if !intrinsic.FirstUse().IsNil() {
		lfc.F.fe.Fatalf(lfc.F.Entry.Pos, "unexpanded LLVM nil-check intrinsic use")
	}
	intrinsic.EraseFromParentAsFunction()

	// TODO(goallc): A later target-aware optimization may fold this explicit
	// branch and panic call into an implicit faulting nil check. It must first
	// prove Go panic ordering and recover semantics, a target-valid fault
	// offset, memory dependence, and the target's fault classification rules.
}
