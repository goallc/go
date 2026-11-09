// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.
#include "GoALLCThinLTO.h"
#include "GoALLCThinLTOC.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/MemoryBuffer.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/TargetSelect.h"
using namespace llvm;
static cl::opt<std::string> Out("output-dir", cl::Required);
static cl::opt<bool> SaveTemps("save-temps", cl::init(true));
static cl::opt<bool> EmitObjects("emit-objects", cl::init(false));
static cl::opt<bool> Summarized("summarized", cl::init(false));
static cl::opt<unsigned> Jobs("jobs", cl::init(2));
static cl::list<std::string> Inputs(cl::Positional, cl::OneOrMore);
int main(int argc, char **argv) {
  cl::ParseCommandLineOptions(argc, argv);
  // Summary writing parses module assembly, just as the compiler does.
  InitializeAllTargetInfos();
  InitializeAllTargets();
  InitializeAllTargetMCs();
  InitializeAllAsmParsers();
  std::vector<std::unique_ptr<MemoryBuffer>> Buffers;
  SmallVector<MemoryBufferRef> Modules;
  for (unsigned I = 0; I < Inputs.size(); ++I) {
    if (Summarized) {
      auto B = MemoryBuffer::getFile(Inputs[I]);
      if (!B) {
        errs() << B.getError().message();
        return 1;
      }
      Buffers.push_back(std::move(*B));
    } else {
      LLVMContext C;
      SMDiagnostic D;
      auto M = parseIRFile(Inputs[I], D, C);
      if (!M) {
        D.print(argv[0], errs());
        return 1;
      }
      SmallVector<char, 0> Bytes;
      raw_svector_ostream OS(Bytes);
      if (auto E = goallc::writeThinBitcode(*M, OS)) {
        errs() << toString(std::move(E));
        return 1;
      }
      Buffers.push_back(MemoryBuffer::getMemBufferCopy(
          StringRef(Bytes.data(), Bytes.size()), "memory:" + Inputs[I]));
    }
    Modules.push_back(Buffers.back()->getMemBufferRef());
  }
  if (EmitObjects) {
    std::vector<LLVMMemoryBufferRef> Inputs;
    std::vector<LLVMMemoryBufferRef> Outputs(Modules.size());
    for (auto &B : Buffers)
      Inputs.push_back(wrap(B.get()));
    LLVMErrorRef Error = LLVMGoALLCThinLTO(
        Inputs.data(), Inputs.size(), SaveTemps ? Out.c_str() : "", 0, Jobs,
        Outputs.data());
    bool Valid = Error == nullptr;
    for (size_t I = 0; I != Outputs.size(); ++I) {
      if (!Outputs[I]) {
        Valid = false;
        continue;
      }
      StringRef Object = unwrap(Outputs[I])->getBuffer();
      if (!Error && !Object.starts_with("go object "))
        Valid = false;
      if (!Error)
        outs() << "object " << I << ": " << Object.take_front(9) << '\n';
      LLVMDisposeMemoryBuffer(Outputs[I]);
    }
    if (Error) {
      char *Message = LLVMGetErrorMessage(Error);
      errs() << Message << '\n';
      LLVMDisposeErrorMessage(Message);
    }
    if (!SaveTemps && sys::fs::exists(Out)) {
      errs() << "unexpected intermediate output directory\n";
      Valid = false;
    }
    return Valid ? 0 : 1;
  }
  if (auto E = goallc::runThinLTO(
          Modules, SaveTemps ? StringRef(Out) : StringRef(), Jobs)) {
    errs() << "goallc-thinlto: " << toString(std::move(E)) << '\n';
    return 1;
  }
}
