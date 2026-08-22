// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "GoALLCPreCodeGen.h"
#include "GoALLCStatepoints.h"
#include "llvm/Analysis/AssumptionCache.h"
#include "llvm/CodeGen/StackProtector.h"
#include "llvm/CodeGen/TargetPassConfig.h"
#include "llvm/Config/llvm-config.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/IRBuilder.h"
#include "llvm/IR/InlineAsm.h"
#include "llvm/IR/InstIterator.h"
#include "llvm/IR/IntrinsicInst.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/Pass.h"
#include "llvm/Plugins/PassPlugin.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/ErrorHandling.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/Target/RegisterTargetPassConfigCallback.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

Pass *createGoALLCInlineAnchorPass();

namespace {

constexpr StringLiteral GoObjInlineAnchorBundle = "goobj.debug.inline.anchor";
constexpr uint64_t GoObjInlineAnchorCookie = 0x476f414c4c43494eULL;

cl::opt<bool> ReportInvocation(
    "goallc-pass-plugin-report", cl::Hidden,
    cl::desc("Report invocation of the GoALLC pre-codegen plugin"),
    cl::init(false));

cl::opt<bool>
    EmitIR("goallc-pass-plugin-emit-ir", cl::Hidden,
           cl::desc("Emit IR after the GoALLC pre-codegen pipeline and stop"),
           cl::init(false));

bool runPreCodeGenCallback(Module &M, TargetMachine &TM, CodeGenFileType,
                           raw_pwrite_stream &) {
  // Keep the early callback only as an IR-emission test facility. Production
  // compilation performs only module-wide preparation here, then rewrites
  // each function from GoALLCPreISelPass after standard codegen IR preparation.
  Error Err = EmitIR ? goallc::runPreCodeGenPipeline(M, TM)
                     : goallc::prepareStatepointModule(M);
  if (Err) {
    M.getContext().emitError(toString(std::move(Err)));
    return true;
  }

  if (!EmitIR)
    return false;

  if (ReportInvocation)
    errs() << "GoALLCStatepoints: ran pre-codegen pipeline for "
           << M.getModuleIdentifier() << '\n';
  if (EmitIR) {
    M.print(errs(), nullptr);
    return true;
  }
  return false;
}

void lowerInlineAnchorMarkers(Function &F) {
  SmallVector<CallInst *, 8> Markers;
  for (Instruction &I : instructions(F)) {
    auto *Call = dyn_cast<CallInst>(&I);
    auto *II = Call ? dyn_cast<IntrinsicInst>(Call) : nullptr;
    if (!II || II->getIntrinsicID() != Intrinsic::sideeffect)
      continue;
    std::optional<OperandBundleUse> Bundle =
        Call->getOperandBundle(GoObjInlineAnchorBundle);
    if (!Bundle)
      continue;
    if (!Bundle->Inputs.empty() || Call->getNumOperandBundles() != 1)
      report_fatal_error("invalid GoALLC inline anchor operand bundle");
    Markers.push_back(Call);
  }

  if (Markers.empty())
    return;

  LLVMContext &Context = F.getContext();
  FunctionType *AsmType = FunctionType::get(Type::getVoidTy(Context), false);
  InlineAsm *Asm = InlineAsm::get(AsmType, "", "", /*hasSideEffects=*/true);
  Metadata *Cookie = ConstantAsMetadata::get(
      ConstantInt::get(Type::getInt64Ty(Context), GoObjInlineAnchorCookie));
  MDNode *CookieNode = MDNode::get(Context, Cookie);
  for (CallInst *Marker : Markers) {
    IRBuilder<> Builder(Marker);
    CallInst *Lowered = Builder.CreateCall(AsmType, Asm);
    Lowered->setDebugLoc(Marker->getDebugLoc());
    Lowered->setMetadata("srcloc", CookieNode);
    Marker->eraseFromParent();
  }
}

class GoALLCPreISelPass final : public FunctionPass {
public:
  static char ID;

  GoALLCPreISelPass() : FunctionPass(ID) {}
  explicit GoALLCPreISelPass(TargetMachine &TM) : FunctionPass(ID), TM(&TM) {}

  bool runOnFunction(Function &F) override {
    assert(TM && "GoALLC pre-isel pass requires a target machine");
    if (Error Err = goallc::rewriteStatepoints(F, *TM)) {
      std::string Message = toString(std::move(Err));
      F.getContext().emitError(Message);
      // Validation can fail after canonicalization has already changed local
      // IR, so conservatively invalidate analyses even though code generation
      // will stop on the emitted diagnostic.
      return true;
    }
    // Keep the intrinsic marker out of statepoint liveness: lowering it to an
    // InlineAsm value earlier would make the callee pointer look GC-live.
    lowerInlineAnchorMarkers(F);

    if (ReportInvocation)
      errs() << "GoALLCStatepoints: ran late pre-isel pipeline for "
             << F.getName() << '\n';
    return true;
  }

  void getAnalysisUsage(AnalysisUsage &AU) const override {
    // rewriteStatepoints changes both instructions and the CFG. Let the legacy
    // pass manager invalidate and rebuild CFG-dependent analyses before
    // instruction selection instead of carrying pre-rewrite DT/AA state across
    // this pass. The rewrite does not add or remove assumptions, and its
    // temporary allocas are promoted before returning, so the already-computed
    // stack-protector layout remains valid.
    AU.addPreserved<AssumptionCacheTracker>();
    AU.addPreserved<StackProtector>();
  }

  StringRef getPassName() const override { return "GoALLC late statepoints"; }

private:
  TargetMachine *TM = nullptr;
};

char GoALLCPreISelPass::ID = 0;
static RegisterPass<GoALLCPreISelPass>
    RegisterGoALLCPreISelPass("goallc-late-statepoints",
                              "GoALLC late statepoints", false, false);

RegisterTargetPassConfigCallback RegisterGoALLCTargetPasses(
    [](TargetMachine &TM, PassManagerBase &, TargetPassConfig *TPC) {
      if (!TPC)
        return;

      TPC->addPreISelPass(
          [TMPtr = &TM]() { return new GoALLCPreISelPass(*TMPtr); });
      if (TM.getTargetTriple().isOSBinFormatGoObj())
        TPC->addPreBranchRelaxationPass(
            []() { return createGoALLCInlineAnchorPass(); });
    });

} // namespace

extern "C" LLVM_ATTRIBUTE_WEAK ::llvm::PassPluginLibraryInfo
llvmGetPassPluginInfo() {
  return {LLVM_PLUGIN_API_VERSION, "GoALLCStatepoints", LLVM_VERSION_STRING,
          nullptr, runPreCodeGenCallback};
}
