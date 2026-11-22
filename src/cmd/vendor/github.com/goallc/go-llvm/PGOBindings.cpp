#include "PGOBindings.h"
#include "llvm/IR/DiagnosticHandler.h"
#include "llvm/Passes/PassBuilder.h"
#include "llvm/Passes/StandardInstrumentations.h"
#include "llvm/Target/TargetMachine.h"
#include "llvm/Transforms/Instrumentation/InstrProfiling.h"
#include "llvm/Transforms/Instrumentation/PGOInstrumentation.h"

using namespace llvm;

namespace {
// These LLVM 23 pipeline aliases schedule IR PGO at their native site.
static bool hasNativeSite(StringRef Spec) {
  auto [Base, Level] = Spec.split('<');
  return !Level.empty() &&
         (Base == "default" || Base == "thinlto-pre-link" ||
          Base == "lto-pre-link" || Base == "fatlto-pre-link");
}

// Inspect only the caller's textual pipeline, before LLVM expands aliases.
// Let PassBuilder parse the grammar (including nested pass parameters).
struct ProfileSites {
  unsigned Gen = 0, Use = 0, Lower = 0;
  SmallVector<std::pair<StringRef, StringRef>> Replacements;

  Error inspect(ArrayRef<PassBuilder::PipelineElement> Elements, bool Generate,
                bool Nested = false) {
    for (const auto &E : Elements) {
      StringRef Name = E.Name;
      if (!E.InnerPipeline.empty()) {
        if (Error Err =
                inspect(E.InnerPipeline, Generate, Nested || Name != "module"))
          return Err;
        continue;
      }
      if (hasNativeSite(Name)) {
        if (Generate) {
          ++Gen;
          ++Lower;
        } else {
          ++Use;
        }
      } else if (Name == "pgo-instr-gen") {
        ++Gen;
      } else if (Name == "instrprof") {
        if (!Gen)
          return createStringError(
              "instrprof requires preceding pgo-instr-gen");
        ++Lower;
        Replacements.emplace_back(Name, "go-atomic-instrprof");
      } else if (Name == "pgo-instr-use") {
        ++Use;
        Replacements.emplace_back(Name, "go-profile-use");
      } else {
        continue;
      }
      if (Nested)
        return createStringError(
            "nested LLVM PGO passes in optimization pipeline");
    }
    return Error::success();
  }
};

static Error buildPipeline(PassBuilder &PB, ModulePassManager &MPM,
                           StringRef Text, StringRef Profile) {
  // The regular compiler and pre-link pipelines go straight to LLVM.
  if (!Text.contains(',') && !Text.contains('(') && hasNativeSite(Text))
    return PB.parsePassPipeline(MPM, Text);
  if (Text.contains("go-profile-plan") || Text.contains("go-profile-use") ||
      Text.contains("go-atomic-instrprof"))
    return createStringError("reserved name in LLVM PGO pipeline");

  InstrProfOptions Instr;
  Instr.Atomic = true;
  PB.registerPipelineParsingCallback(
      [&](StringRef Name, ModulePassManager &PM,
          ArrayRef<PassBuilder::PipelineElement>) {
        if (Name == "go-profile-use")
          PM.addPass(PGOInstrumentationUse(Profile.str()));
        else if (Name == "go-atomic-instrprof")
          PM.addPass(InstrProfilingLoweringPass(Instr));
        else
          return false;
        return true;
      });
  ProfileSites Sites;
  std::string Wrapped = ("go-profile-plan(" + Text + ")").str();
  std::string SiteError;
  PB.registerPipelineParsingCallback(
      [&](StringRef Name, ModulePassManager &,
          ArrayRef<PassBuilder::PipelineElement> Elements) {
        if (Name != "go-profile-plan")
          return false;
        if (!Elements.empty())
          if (Error Err = Sites.inspect(Elements, Profile.empty()))
            SiteError = toString(std::move(Err));
        return true;
      });
  if (!Text.empty()) {
    ModulePassManager Plan;
    if (Error Err = PB.parsePassPipeline(Plan, Wrapped))
      return Err;
    if (!SiteError.empty())
      return createStringError(SiteError);
  }
  if (Sites.Gen != Sites.Lower || Sites.Gen > 1 || Sites.Use > 1 ||
      (Profile.empty() ? Sites.Use : Sites.Gen))
    return createStringError(
        "conflicting LLVM PGO passes in optimization pipeline");
  if (!Sites.Gen && !Sites.Use) {
    if (Profile.empty()) {
      MPM.addPass(PGOInstrumentationGen());
      MPM.addPass(InstrProfilingLoweringPass(Instr));
    } else {
      MPM.addPass(PGOInstrumentationUse(Profile.str()));
    }
  }
  if (Text.empty())
    return Error::success();
  std::string Pipeline = Text.str();
  const char *Start = Wrapped.data() + StringRef("go-profile-plan(").size();
  for (auto [Token, Replacement] : reverse(Sites.Replacements))
    Pipeline.replace(Token.data() - Start, Token.size(), Replacement.str());
  return PB.parsePassPipeline(MPM, Pipeline);
}
} // namespace

LLVMErrorRef LLVMGoRunPassesWithPGO(LLVMModuleRef Mod,
                                    LLVMTargetMachineRef Machine,
                                    const char *Pipeline, const char *Profile) {
  Module &M = *unwrap(Mod);
  auto *TM = reinterpret_cast<TargetMachine *>(Machine);
  PGOOptions Options(Profile, "", "", "",
                     *Profile ? PGOOptions::IRUse : PGOOptions::IRInstr);
  Options.AtomicCounterUpdate = true;
  PassInstrumentationCallbacks PIC;
  PassBuilder PB(TM, PipelineTuningOptions(), Options, &PIC);
  ModulePassManager MPM;
  if (Error Err = buildPipeline(PB, MPM, Pipeline, Profile))
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
