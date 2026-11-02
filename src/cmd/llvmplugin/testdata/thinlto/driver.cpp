// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include "GoALLCThinLTO.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/ToolOutputFile.h"
using namespace llvm;
static cl::opt<std::string> Out("output-dir", cl::Required);
static cl::opt<bool> Summarized("summarized", cl::init(false));
static cl::opt<unsigned> Jobs("jobs", cl::init(2));
static cl::list<std::string> Inputs(cl::Positional, cl::OneOrMore);
int main(int argc, char **argv) {
  cl::ParseCommandLineOptions(argc, argv);
  if (auto EC = sys::fs::create_directories(Out)) {
    errs() << EC.message();
    return 1;
  }
  std::vector<std::string> Paths;
  for (unsigned I = 0; I < Inputs.size(); ++I) {
    if (Summarized) {
      Paths.push_back(Inputs[I]);
      continue;
    }
    LLVMContext C;
    SMDiagnostic D;
    auto M = parseIRFile(Inputs[I], D, C);
    if (!M) {
      D.print(argv[0], errs());
      return 1;
    }
    std::string Path = Out + "/" + std::to_string(I) + ".input.bc";
    std::error_code EC;
    ToolOutputFile F(Path, EC, sys::fs::OF_None);
    if (EC) {
      errs() << EC.message();
      return 1;
    }
    if (auto E = goallc::writeThinBitcode(*M, F.os())) {
      errs() << toString(std::move(E));
      return 1;
    }
    F.keep();
    Paths.push_back(Path);
  }
  if (auto E = goallc::runThinLTO(Paths, Out, true, Jobs)) {
    errs() << "goallc-thinlto: " << toString(std::move(E)) << '\n';
    return 1;
  }
}
