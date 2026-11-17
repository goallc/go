// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
// Exercise the generic LTO mutable hook with both Full LTO and ThinLTO.
#include "llvm/Analysis/ModuleSummaryAnalysis.h"
#include "llvm/Analysis/ProfileSummaryInfo.h"
#include "llvm/AsmParser/Parser.h"
#include "llvm/Bitcode/BitcodeWriter.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/Module.h"
#include "llvm/LTO/LTO.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Transforms/Utils/AssignGUID.h"
using namespace llvm;
int main(int argc, char **argv) {
  bool Thin = argc > 1 && StringRef(argv[1]) == "thin";
  bool Stop = argc > 2 && StringRef(argv[2]) == "stop";
  InitializeAllTargetInfos();
  InitializeAllTargets();
  InitializeAllTargetMCs();
  InitializeAllAsmPrinters();
  LLVMContext C;
  SMDiagnostic D;
  auto M = parseAssemblyString(R"(
    target triple = "x86_64-unknown-linux-gnu"
    target datalayout = "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"
    define i32 @answer() { ret i32 41 }
  )",
                               D, C);
  if (!M)
    return 1;
  AssignGUIDPass::runOnModule(*M);
  ProfileSummaryInfo PSI(*M);
  auto Index = buildModuleSummaryIndex(*M, nullptr, &PSI);
  SmallVector<char, 0> Input, Output;
  raw_svector_ostream In(Input);
  WriteBitcodeToFile(*M, In, false, Thin ? &Index : nullptr, true);
  auto File = lto::InputFile::create(MemoryBufferRef(
      StringRef(Input.data(), Input.size()), "memory:transform"));
  if (!File) {
    errs() << toString(File.takeError());
    return 1;
  }
  lto::Config Conf;
  Conf.CGFileType = CodeGenFileType::AssemblyFile;
  Conf.DiagHandler = [](const DiagnosticInfo &) {};
  bool Observed = false, Changed = false, Generated = false, Invalid = false;
  auto ReturnValue = [](const Module &Mod) -> const ConstantInt * {
    auto *F = Mod.getFunction("answer");
    auto *R =
        F ? dyn_cast<ReturnInst>(F->getEntryBlock().getTerminator()) : nullptr;
    return R ? dyn_cast<ConstantInt>(R->getReturnValue()) : nullptr;
  };
  Conf.PostOptModuleHook = [&](unsigned, const Module &Mod) {
    auto *V = ReturnValue(Mod);
    Observed = V && V->getZExtValue() == 41;
    Invalid |= !Observed;
    return Observed;
  };
  Conf.PostOptModuleTransform = [&](unsigned, Module &Mod) {
    Changed = Observed;
    auto *F = Mod.getFunction("answer");
    auto *R = cast<ReturnInst>(F->getEntryBlock().getTerminator());
    R->setOperand(0, ConstantInt::get(Type::getInt32Ty(Mod.getContext()), 42));
    return !Stop;
  };
  Conf.PreCodeGenModuleHook = [&](unsigned, const Module &Mod) {
    auto *V = ReturnValue(Mod);
    Generated = Changed && V && V->getZExtValue() == 42;
    Invalid |= !Generated;
    return Generated;
  };
  lto::LTO Driver(std::move(Conf));
  std::vector<lto::SymbolResolution> Resolutions;
  for (const auto &S : (*File)->symbols()) {
    lto::SymbolResolution R;
    R.Prevailing = !S.isUndefined();
    R.VisibleToRegularObj = true;
    Resolutions.push_back(R);
  }
  if (auto E = Driver.add(std::move(*File), Resolutions)) {
    errs() << toString(std::move(E));
    return 1;
  }
  Driver.getMaxTasks();
  auto AddStream =
      [&](unsigned,
          const Twine &) -> Expected<std::unique_ptr<CachedFileStream>> {
    return std::make_unique<CachedFileStream>(
        std::make_unique<raw_svector_ostream>(Output));
  };
  if (auto E = Driver.run(AddStream)) {
    errs() << toString(std::move(E));
    return 1;
  }
  if (Invalid || !Observed || !Changed || Generated == Stop ||
      Output.empty() != Stop)
    return 1;
  if (Stop)
    outs() << "stopped before codegen\n";
  else
    outs().write(Output.data(), Output.size());
}
