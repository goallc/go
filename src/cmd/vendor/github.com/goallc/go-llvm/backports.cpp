
#include "backports.h"
#include "llvm/IR/DIBuilder.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/Instructions.h"
#if LLVM_VERSION_MAJOR >= 16
#include "llvm/IR/PassManager.h"
#include "llvm/Analysis/LoopAnalysisManager.h"
#include "llvm/Analysis/CGSCCPassManager.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Transforms/IPO/ThinLTOBitcodeWriter.h"
#else
#include "llvm/IR/LegacyPassManager.h"
#include "llvm/Transforms/IPO/PassManagerBuilder.h"
#endif
#include "llvm/IR/Module.h"
#include "llvm/Pass.h"
#include "llvm/Support/MemoryBuffer.h"
#include "llvm/Transforms/IPO.h"
#include "llvm-c/DebugInfo.h"

void LLVMGlobalObjectAddMetadata(LLVMValueRef Global, unsigned KindID, LLVMMetadataRef MD) {
  llvm::MDNode *N = MD ? llvm::unwrap<llvm::MDNode>(MD) : nullptr;
  llvm::GlobalObject *O = llvm::unwrap<llvm::GlobalObject>(Global);
  O->addMetadata(KindID, *N);
}

// See https://reviews.llvm.org/D119431
LLVMMemoryBufferRef LLVMGoWriteThinLTOBitcodeToMemoryBuffer(LLVMModuleRef M) {
  std::string Data;
  llvm::raw_string_ostream OS(Data);
#if LLVM_VERSION_MAJOR >= 16
  llvm::LoopAnalysisManager LAM;
  llvm::FunctionAnalysisManager FAM;
  llvm::CGSCCAnalysisManager CGAM;
  llvm::ModuleAnalysisManager MAM;
  llvm::PassBuilder PB;
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);
  llvm::ModulePassManager MPM;
  MPM.addPass(llvm::ThinLTOBitcodeWriterPass(OS, nullptr));
  MPM.run(*llvm::unwrap(M), MAM);
#else
  llvm::legacy::PassManager PM;
  PM.add(createWriteThinLTOBitcodePass(OS));
  PM.run(*llvm::unwrap(M));
#endif
  return llvm::wrap(llvm::MemoryBuffer::getMemBufferCopy(OS.str()).release());
}

LLVMMetadataRef LLVMGoDIBuilderCreateAutoVariable(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File, unsigned LineNo, LLVMMetadataRef Type,
    LLVMBool AlwaysPreserve, LLVMDIFlags Flags, uint32_t AlignInBits,
    LLVMMetadataRef Annotations) {
  auto *D = llvm::unwrap(Builder);
  auto *S = llvm::unwrap<llvm::DILocalScope>(Scope);
  auto *F = llvm::unwrap<llvm::DIFile>(File);
  auto *T = llvm::unwrap<llvm::DIType>(Type);
  auto DIFlags = static_cast<llvm::DINode::DIFlags>(Flags);
  if (!Annotations)
    return llvm::wrap(D->createAutoVariable(S, llvm::StringRef(Name, NameLen),
                                           F, LineNo, T, AlwaysPreserve,
                                           DIFlags, AlignInBits));

  // DILocalVariable already supports annotations, but createAutoVariable
  // does not expose them. Use the native node API without changing DIBuilder.
  auto *Var = llvm::DILocalVariable::get(
      S->getContext(), S, llvm::StringRef(Name, NameLen), F, LineNo, T,
      /*Arg=*/0, DIFlags, AlignInBits,
      llvm::DINodeArray(llvm::unwrap<llvm::MDTuple>(Annotations)));
  if (AlwaysPreserve) {
    auto *SP = S->getSubprogram();
    // finalizeSubprogram merges existing retainedNodes with builder-tracked
    // nodes. Keep the variable there even if optimization removes all uses.
    if (!llvm::is_contained(SP->getRetainedNodes(), Var)) {
      llvm::Metadata *Node = Var;
      SP->retainNodes(&Node, &Node + 1);
    }
  }
  return llvm::wrap(Var);
}

LLVMMetadataRef LLVMGoDIBuilderCreateParameterVariable(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, unsigned ArgNo, LLVMMetadataRef File, unsigned LineNo,
    LLVMMetadataRef Type, LLVMBool AlwaysPreserve, LLVMDIFlags Flags,
    LLVMMetadataRef Annotations) {
  return llvm::wrap(llvm::unwrap(Builder)->createParameterVariable(
      llvm::unwrap<llvm::DIScope>(Scope), llvm::StringRef(Name, NameLen), ArgNo,
      llvm::unwrap<llvm::DIFile>(File), LineNo, llvm::unwrap<llvm::DIType>(Type),
      AlwaysPreserve, static_cast<llvm::DINode::DIFlags>(Flags),
      llvm::DINodeArray(Annotations ? llvm::unwrap<llvm::MDTuple>(Annotations)
                                   : nullptr)));
}

LLVMMetadataRef LLVMGoDIBuilderCreateArrayType(
    LLVMDIBuilderRef Builder, const char *Name, size_t NameLen, uint64_t Size,
    uint32_t AlignInBits, LLVMMetadataRef ElementType,
    LLVMMetadataRef *Subscripts, unsigned NumSubscripts) {
  auto *D = llvm::unwrap(Builder);
  return llvm::wrap(D->createArrayType(
      nullptr, llvm::StringRef(Name, NameLen), nullptr, 0, Size, AlignInBits,
      llvm::unwrap<llvm::DIType>(ElementType),
      D->getOrCreateArray({llvm::unwrap(Subscripts), NumSubscripts})));
}

LLVMMetadataRef LLVMGoDIBuilderCreateModule(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File) {
  return llvm::wrap(llvm::unwrap(Builder)->createModule(
      llvm::unwrap<llvm::DIScope>(Scope), llvm::StringRef(Name, NameLen), {}, {},
      {}, llvm::unwrap<llvm::DIFile>(File)));
}

void LLVMGoDIBuilderInsertDbgValueRecordAtEnd(
    LLVMDIBuilderRef Builder, LLVMValueRef Val, LLVMMetadataRef VarInfo,
    LLVMMetadataRef Expr, LLVMMetadataRef DebugLoc, LLVMBasicBlockRef Block) {
#if LLVM_VERSION_MAJOR >= 19
  // Note: this returns a LLVMDbgRecordRef. Previously, InsertValueAtEnd would
  // return a Value. But since the type changed, and I'd like to keep the API
  // consistent across LLVM versions, I decided to drop the return value.
  LLVMDIBuilderInsertDbgValueRecordAtEnd(Builder, Val, VarInfo, Expr, DebugLoc, Block);
#else
  // Old llvm.dbg.* API.
  LLVMDIBuilderInsertDbgValueAtEnd(Builder, Val, VarInfo, Expr, DebugLoc, Block);
#endif
}

void LLVMGoDIBuilderInsertDeclareRecordAtEnd(
    LLVMDIBuilderRef Builder, LLVMValueRef Storage, LLVMMetadataRef VarInfo,
    LLVMMetadataRef Expr, LLVMMetadataRef DebugLoc, LLVMBasicBlockRef Block) {
#if LLVM_VERSION_MAJOR >= 19
  LLVMDIBuilderInsertDeclareRecordAtEnd(Builder, Storage, VarInfo, Expr,
                                        DebugLoc, Block);
#else
  LLVMDIBuilderInsertDeclareAtEnd(Builder, Storage, VarInfo, Expr, DebugLoc,
                                  Block);
#endif
}

LLVMMetadataRef LLVMGoDIBuilderCreateLabel(
    LLVMDIBuilderRef Builder, LLVMMetadataRef Scope, const char *Name,
    size_t NameLen, LLVMMetadataRef File, unsigned LineNo,
    LLVMBool IsArtificial, LLVMBool AlwaysPreserve) {
  return llvm::wrap(llvm::unwrap(Builder)->createLabel(
      llvm::unwrap<llvm::DIScope>(Scope), llvm::StringRef(Name, NameLen),
      llvm::unwrap<llvm::DIFile>(File), LineNo, /*Column=*/0, IsArtificial,
      /*CoroSuspendIdx=*/std::nullopt, AlwaysPreserve));
}
