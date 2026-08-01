//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"

	"github.com/goallc/go-llvm"
)

const llvmPanicmemName = "runtime.panicmem"

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
	return getOrInsertLLVMFunction(sym.Name, sig, goABIInternalCallConv), sig
}

func (lfc *LLVMFuncContext) explicitNilCheck(v *Value) llvm.Value {
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

	current := lfc.BlockEnds[v.Block.ID]
	if current.IsNil() || lfc.b.GetInsertBlock() != current {
		v.Fatalf("NilCheck is not emitted at the current LLVM block tail")
	}
	isNil := lfc.b.CreateICmp(llvm.IntEQ, checked, llvm.ConstNull(checked.Type()), v.String()+".isnil")
	panicBlock := GlobalCtxt.AddBasicBlock(lfc.LF, v.String()+".nil")
	continueBlock := GlobalCtxt.AddBasicBlock(lfc.LF, v.String()+".notnil")
	lfc.b.CreateCondBr(isNil, panicBlock, continueBlock)

	lfc.b.SetInsertPointAtEnd(panicBlock)
	panicmem, sig := llvmPanicmem()
	call := lfc.b.CreateCall(sig.Type, panicmem, nil, "")
	call.SetInstructionCallConv(goABIInternalCallConv)
	// panicmem enters the runtime panic path and may reach GC while this frame is
	// suspended, so it must remain an ordinary non-leaf call for statepoint and
	// stack-map construction.
	// runtime.panicmem ends in the Go panic builtin. A recovered panic resumes
	// in the caller's deferred recovery path, never after this call. Retain an
	// explicit edge to the continuation because the LLVM declaration does not
	// claim noreturn. This first implementation deliberately accepts the
	// conservative live set on that artificial edge; a future noreturn form
	// must also keep the call return PC inside a valid frame/PCSP range.
	lfc.b.CreateBr(continueBlock)

	// TODO(goallc): A later target-aware optimization may fold this explicit
	// branch and panic call into an implicit faulting nil check. It must first
	// prove Go panic ordering and recover semantics, a target-valid fault
	// offset, memory dependence, and the target's fault classification rules.
	lfc.BlockEnds[v.Block.ID] = continueBlock
	lfc.b.SetInsertPointAtEnd(continueBlock)
	return p
}
