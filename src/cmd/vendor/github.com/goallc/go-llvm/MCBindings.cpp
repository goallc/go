// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Decode Go-encoded instructions without maintaining a second instruction map.
#include "MCBindings.h"
#include "llvm/MC/MCAsmInfo.h"
#include "llvm/MC/MCContext.h"
#include "llvm/MC/MCDisassembler/MCDisassembler.h"
#include "llvm/MC/MCDisassembler/MCRelocationInfo.h"
#include "llvm/MC/MCDisassembler/MCSymbolizer.h"
#include "llvm/MC/MCExpr.h"
#include "llvm/MC/MCInst.h"
#include "llvm/MC/MCInstPrinter.h"
#include "llvm/MC/MCInstrInfo.h"
#include "llvm/MC/MCRegisterInfo.h"
#include "llvm/MC/MCSubtargetInfo.h"
#include "llvm/MC/TargetRegistry.h"
#include "llvm/Support/TargetSelect.h"
#include "llvm/Support/raw_ostream.h"
#include <cstring>
using namespace llvm;
namespace {
// This expression is only printed, never assembled or evaluated. Mark the
// operand token so literal '$' characters from MCInstPrinter can be escaped
// without touching the LLVM inline-asm operand. No symbol-name text rewriting
// is involved, and target punctuation remains owned by the instruction printer.
class InlineAsmOperandExpr final : public MCTargetExpr {
  unsigned Operand;
  StringRef Prefix, Suffix;

public:
  explicit InlineAsmOperandExpr(const LLVMGoMCReloc &R)
      : Operand(R.Operand), Prefix(R.Prefix), Suffix(R.Suffix) {}
  void printImpl(raw_ostream &OS, const MCAsmInfo *) const override {
    OS << '\1' << Prefix << "${" << Operand << ":c}" << Suffix << '\2';
  }
  bool evaluateAsRelocatableImpl(MCValue &,
                                 const MCAssembler *) const override {
    return false;
  }
  void visitUsedExpr(MCStreamer &) const override {}
  MCFragment *findAssociatedFragment() const override { return nullptr; }
};

struct Decoder {
  Triple TT;
  std::unique_ptr<MCRegisterInfo> MRI;
  std::unique_ptr<MCAsmInfo> MAI;
  std::unique_ptr<MCInstrInfo> MII;
  std::unique_ptr<MCSubtargetInfo> STI;
  std::unique_ptr<MCContext> Ctx;
  std::unique_ptr<MCDisassembler> Dis;
  std::unique_ptr<MCInstPrinter> Printer;
  ArrayRef<LLVMGoMCReloc> Relocs;
  SmallVector<bool, 2> Used;
  std::string Error;
  explicit Decoder(const char *T) : TT(T) {}
};
class Symbolizer final : public MCSymbolizer {
  Decoder &D;

public:
  Symbolizer(Decoder &D) : MCSymbolizer(*D.Ctx, nullptr), D(D) {}
  bool tryAddingSymbolicOperand(MCInst &Inst, raw_ostream &, int64_t Value,
                                uint64_t Address, bool IsBranch,
                                uint64_t Offset, uint64_t OpSize,
                                uint64_t InstSize) override {
    for (size_t I = 0; I < D.Relocs.size(); ++I) {
      const auto &R = D.Relocs[I];
      if (R.Offset != Address + Offset || (D.TT.isX86() && R.Size != OpSize))
        continue;
      if (D.Used[I]) {
        D.Error = "relocation matched multiple operands";
        return false;
      }
      D.Used[I] = true;
      const MCExpr *E = new (*D.Ctx) InlineAsmOperandExpr(R);
      // Go x86 PC-relative relocations are relative to the relocation field's
      // end; the assembler's symbolic operand is relative to instruction end.
      int64_t Addend = R.Addend;
      if (R.PCRelative && D.TT.isX86())
        Addend += InstSize - Offset - OpSize;
      if (Addend)
        E = MCBinaryExpr::createAdd(E, MCConstantExpr::create(Addend, *D.Ctx),
                                    *D.Ctx);
      Inst.addOperand(MCOperand::createExpr(E));
      return true;
    }
    const auto &Desc = D.MII->get(Inst.getOpcode());
    bool PCRel = IsBranch;
    if (Inst.getNumOperands() < Desc.getNumOperands())
      PCRel |= Desc.operands()[Inst.getNumOperands()].OperandType ==
               MCOI::OPERAND_PCREL;
    if (D.TT.isX86() && Inst.getNumOperands() >= 3) {
      const auto &Base = Inst.getOperand(Inst.getNumOperands() - 3);
      PCRel |= Base.isReg() && Base.getReg() &&
               StringRef(D.MRI->getName(Base.getReg())) == "RIP";
    }
    if (!PCRel)
      return false;
    if (D.MII->getName(Inst.getOpcode()) == "ADRP") {
      D.Error =
          "ADRP without a symbolic relocation is not position independent";
      return false;
    }
    D.Error =
        "PC-relative operand without a relocation in symbolic instruction";
    return false;
  }
  void tryAddingPcLoadReferenceComment(raw_ostream &, int64_t,
                                       uint64_t) override {}
};
} // namespace
void *LLVMGoCreateMCDecoder(const char *T, const char *Features) {
  InitializeAllTargetInfos();
  InitializeAllTargetMCs();
  InitializeAllDisassemblers();
  auto D = std::make_unique<Decoder>(T);
  std::string Error;
  auto *Target = TargetRegistry::lookupTarget(D->TT, Error);
  if (!Target)
    return nullptr;
  D->MRI.reset(Target->createMCRegInfo(D->TT));
  D->MII.reset(Target->createMCInstrInfo());
  if (!D->MRI || !D->MII)
    return nullptr;
  D->MAI.reset(Target->createMCAsmInfo(*D->MRI, D->TT, MCTargetOptions()));
  // Go assembly can contain explicitly dispatched optional ISA instructions.
  // Use the same feature set for decoding and the naked inline-asm carrier.
  D->STI.reset(Target->createMCSubtargetInfo(D->TT, "", Features));
  if (!D->MAI || !D->STI)
    return nullptr;
  D->Ctx = std::make_unique<MCContext>(D->TT, *D->MAI, *D->MRI, *D->STI);
  D->Dis.reset(Target->createMCDisassembler(*D->STI, *D->Ctx));
  D->Printer.reset(
      Target->createMCInstPrinter(D->TT, 0, *D->MAI, *D->MII, *D->MRI));
  if (!D->Dis || !D->Printer)
    return nullptr;
  D->Dis->setSymbolizer(std::make_unique<Symbolizer>(*D));
  return D.release();
}
void LLVMGoDisposeMCDecoder(void *P) { delete static_cast<Decoder *>(P); }
char *LLVMGoDecodeMCInstruction(void *P, const uint8_t *Bytes, size_t Length,
                                uint64_t PC, const LLVMGoMCReloc *Relocs,
                                size_t NumRelocs, uint64_t *Size,
                                char **Error) {
  auto &D = *static_cast<Decoder *>(P);
  D.Relocs = ArrayRef(Relocs, NumRelocs);
  D.Used.assign(NumRelocs, false);
  D.Error.clear();
  MCInst Inst;
  auto Status =
      D.Dis->getInstruction(Inst, *Size, ArrayRef(Bytes, Length), PC, nulls());
  if (Status != MCDisassembler::Success)
    D.Error = "LLVM could not decode instruction";
  for (bool Used : D.Used)
    if (!Used)
      D.Error = "relocation did not match a decoded operand";
  D.Relocs = {};
  if (!D.Error.empty()) {
    *Error = strdup(D.Error.c_str());
    return nullptr;
  }
  std::string Text;
  raw_string_ostream OS(Text);
  // A decoded x86 near jump must not become an initially short, relaxable
  // jump when its mnemonic is parsed again. Besides preserving Go's encoding,
  // this makes the inline-asm size assertions absolute at parse time.
  if (D.TT.isX86() && D.MII->get(Inst.getOpcode()).isBranch())
    for (const auto &R : ArrayRef(Relocs, NumRelocs))
      if (R.PCRelative && R.Size == 4) {
        OS << "{disp32} ";
        break;
      }
  D.Printer->printInst(&Inst, 0, "", *D.STI, OS);
  std::string Template;
  bool InOperand = false;
  for (char C : Text) {
    if (C == '\1') {
      InOperand = true;
    } else if (C == '\2') {
      InOperand = false;
    } else {
      Template += C;
      if (C == '$' && !InOperand)
        Template += '$';
    }
  }
  return strdup(Template.c_str());
}

uint64_t LLVMGoMCInstructionSize(void *P, const uint8_t *Bytes, size_t Length,
                                 uint64_t PC) {
  auto &D = *static_cast<Decoder *>(P);
  D.Dis->setSymbolizer(nullptr);
  MCInst Inst;
  uint64_t Size = 0;
  auto Status =
      D.Dis->getInstruction(Inst, Size, ArrayRef(Bytes, Length), PC, nulls());
  D.Dis->setSymbolizer(std::make_unique<Symbolizer>(D));
  return Status == MCDisassembler::Success ? Size : 0;
}
