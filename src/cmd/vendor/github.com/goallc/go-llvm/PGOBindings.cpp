#include "PGOBindings.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Passes/StandardInstrumentations.h"
#include "llvm/Target/TargetMachine.h"
#include "llvm/Transforms/Instrumentation/InstrProfiling.h"
#include "llvm/Transforms/Instrumentation/PGOInstrumentation.h"

using namespace llvm;

namespace {
class CallbackLowering : public RequiredPassInfoMixin<CallbackLowering> {
  LLVMGoInstrProfLowering Lower;

public:
  explicit CallbackLowering(LLVMGoInstrProfLowering Lower) : Lower(Lower) {}
  PreservedAnalyses run(Module &M, ModuleAnalysisManager &) {
    if (LLVMErrorRef Err = Lower(wrap(&M)))
      M.getContext().emitError(toString(unwrap(Err)));
    return PreservedAnalyses::none();
  }
};

// LLVM 23 adds the IR profiling passes to the module pipeline. Replace the
// lowering in its original slot, after any intervening loop transforms.
class PGOPipeline : public ModulePassManager {
  template <typename PassT>
  static std::unique_ptr<PassConceptT> boxed(PassT Pass) {
    PGOPipeline Single;
    Single.addPass(std::move(Pass));
    return std::move(Single.Passes.front());
  }

public:
  Error configure(StringRef Profile, LLVMGoInstrProfLowering Lower,
                  StringRef Pipeline) {
    bool Generate = Profile.empty();
    unsigned GenSites = 0, LowerSites = 0, UseSites = 0;
    for (auto &P : Passes) {
      if (P->name() == PGOInstrumentationGen::name()) {
        ++GenSites;
      } else if (P->name() == InstrProfilingLoweringPass::name()) {
        if (!GenSites)
          return createStringError(
              "instrprof requires preceding pgo-instr-gen");
        ++LowerSites;
        if (Lower)
          P = boxed(CallbackLowering(Lower));
      } else if (P->name() == PGOInstrumentationUse::name()) {
        ++UseSites;
        // LLVM's text parser takes the filename for explicit pgo-instr-use
        // from opt's command-line options. Supply the binding caller's file.
        if (!Generate && Pipeline.contains("pgo-instr-use"))
          P = boxed(PGOInstrumentationUse(Profile.str()));
      } else if (Pipeline.contains('(')) {
        // A repeated or conditional subpipeline cannot own the profiling site.
        bool Nested = false;
        P->printPipeline(nulls(), [&](StringRef Name) {
          Nested |= Name == PGOInstrumentationGen::name() ||
                    Name == PGOInstrumentationUse::name() ||
                    Name == InstrProfilingLoweringPass::name();
          return Name;
        });
        if (Nested)
          return createStringError(
              "nested LLVM PGO passes in optimization pipeline");
      }
    }
    if (GenSites != LowerSites || GenSites > 1 || UseSites > 1 ||
        (Generate ? UseSites : GenSites))
      return createStringError(
          "conflicting LLVM PGO passes in optimization pipeline");
    if (!GenSites && !UseSites) {
      // Explicit pass lists with no profiling site train and use at the input.
      ModulePassManager Tail(std::move(*this));
      if (Generate) {
        addPass(PGOInstrumentationGen());
        if (Lower)
          addPass(CallbackLowering(Lower));
        else
          addPass(InstrProfilingLoweringPass());
      } else {
        addPass(PGOInstrumentationUse(Profile.str()));
      }
      addPass(std::move(Tail));
    }
    return Error::success();
  }
};
} // namespace

LLVMErrorRef LLVMGoRunPassesWithPGO(LLVMModuleRef Mod,
                                    LLVMTargetMachineRef Machine,
                                    const char *Pipeline, const char *Profile,
                                    LLVMGoInstrProfLowering Lower) {
  Module &M = *unwrap(Mod);
  auto *TM = reinterpret_cast<TargetMachine *>(Machine);
  PGOOptions Options(Profile, "", "", "",
                     *Profile ? PGOOptions::IRUse : PGOOptions::IRInstr);
  PassInstrumentationCallbacks PIC;
  PassBuilder PB(TM, PipelineTuningOptions(), Options, &PIC);
  PGOPipeline MPM;
  if (*Pipeline)
    if (Error Err = PB.parsePassPipeline(MPM, Pipeline))
      return wrap(std::move(Err));
  if (Error Err = MPM.configure(Profile, Lower, Pipeline))
    return wrap(std::move(Err));
  if (TM)
    TM->setPGOOption(Options);

  LoopAnalysisManager LAM;
  FunctionAnalysisManager FAM;
  CGSCCAnalysisManager CGAM;
  ModuleAnalysisManager MAM;
  PB.registerModuleAnalyses(MAM);
  PB.registerCGSCCAnalyses(CGAM);
  PB.registerFunctionAnalyses(FAM);
  PB.registerLoopAnalyses(LAM);
  PB.crossRegisterProxies(LAM, FAM, CGAM, MAM);
  StandardInstrumentations SI(M.getContext(), false);
  SI.registerCallbacks(PIC, &MAM);
  MPM.run(M, MAM);
  if (M.getContext().getDiagHandlerPtr()->HasErrors)
    return LLVMCreateStringError("LLVM PGO pipeline reported an error");
  return LLVMErrorSuccess;
}
