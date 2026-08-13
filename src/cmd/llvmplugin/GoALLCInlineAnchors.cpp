// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/CodeGen/MachineBasicBlock.h"
#include "llvm/CodeGen/MachineFunction.h"
#include "llvm/CodeGen/MachineFunctionPass.h"
#include "llvm/CodeGen/MachineInstr.h"
#include "llvm/CodeGen/TargetInstrInfo.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/DebugLoc.h"
#include "llvm/IR/Module.h"
#include "llvm/MC/MCContext.h"
#include "llvm/MC/MCSymbol.h"
#include "llvm/Support/ErrorHandling.h"
#include "llvm/Target/TargetMachine.h"

using namespace llvm;

namespace {

struct CanonicalInlineSite {
  const DILocation *Original = nullptr;
  const DISubprogram *Callee = nullptr;
  const DILocation *Parent = nullptr;
  DILocation *Canonical = nullptr;
};

/// Materialize one real instruction for every source inline edge that remains
/// after all generic machine layout passes. Go's inline unwinder needs a PC in
/// the parent frame; a zero-width label alone cannot create that PC range.
class GoALLCInlineAnchorPass final : public MachineFunctionPass {
  SmallVector<CanonicalInlineSite, 16> CanonicalSites;

  DILocation *canonicalizeInlineSite(const DILocation *CallSite,
                                     const DISubprogram *Callee) {
    if (!CallSite || !Callee)
      report_fatal_error("GoALLC inline site has no callsite or callee");

    DILocation *Parent = nullptr;
    if (const DILocation *Outer = CallSite->getInlinedAt())
      Parent =
          canonicalizeInlineSite(Outer, CallSite->getScope()->getSubprogram());

    for (const CanonicalInlineSite &Site : CanonicalSites)
      if (Site.Original == CallSite && Site.Callee == Callee &&
          Site.Parent == Parent)
        return Site.Canonical;

    // GoObj identifies a surviving inline edge by its DILocation pointer.
    // LLVM may otherwise share one callsite node between different inlinees,
    // so give every (callsite, callee, parent) edge a stable distinct node.
    DILocation *Canonical = DILocation::getDistinct(
        CallSite->getContext(), CallSite->getLine(), CallSite->getColumn(),
        CallSite->getScope(), Parent, CallSite->isImplicitCode(),
        CallSite->getAtomGroup(), CallSite->getAtomRank());
    CanonicalSites.push_back({CallSite, Callee, Parent, Canonical});
    return Canonical;
  }

  DILocation *canonicalizeLocation(const DILocation *Loc) {
    const DILocation *CallSite = Loc->getInlinedAt();
    if (!CallSite)
      return const_cast<DILocation *>(Loc);
    const DISubprogram *Callee = Loc->getScope()->getSubprogram();
    DILocation *CanonicalCallSite = canonicalizeInlineSite(CallSite, Callee);
    return DILocation::get(Loc->getContext(), Loc->getLine(), Loc->getColumn(),
                           Loc->getScope(), CanonicalCallSite,
                           Loc->isImplicitCode(), Loc->getAtomGroup(),
                           Loc->getAtomRank());
  }

  SmallVector<const DILocation *, 16>
  requiredInlineLocations(const MachineFunction &MF) const {
    SmallVector<const DILocation *, 16> Required;
    const Module *M = MF.getFunction().getParent();
    const NamedMDNode *Locations =
        M ? M->getNamedMetadata("goobj.debug.inline.required") : nullptr;
    if (!Locations)
      return Required;

    for (const MDNode *Entry : Locations->operands()) {
      if (Entry->getNumOperands() != 2)
        report_fatal_error("expected !goobj.debug.inline.required entries to "
                           "have two operands");
      const auto *CAM =
          dyn_cast_or_null<ConstantAsMetadata>(Entry->getOperand(0));
      const auto *GV = CAM ? dyn_cast<GlobalValue>(CAM->getValue()) : nullptr;
      const auto *Loc = dyn_cast_or_null<DILocation>(Entry->getOperand(1));
      if (!GV || !Loc || !Loc->getInlinedAt())
        report_fatal_error("invalid !goobj.debug.inline.required entry");
      if (GV == &MF.getFunction())
        Required.push_back(Loc);
    }

    llvm::stable_sort(
        Required, [](const DILocation *LHS, const DILocation *RHS) {
          auto Depth = [](const DILocation *Loc) {
            unsigned Result = 0;
            for (; Loc && Loc->getInlinedAt(); Loc = Loc->getInlinedAt())
              ++Result;
            return Result;
          };
          return Depth(LHS) > Depth(RHS);
        });
    return Required;
  }

public:
  static char ID;

  GoALLCInlineAnchorPass() : MachineFunctionPass(ID) {}

  StringRef getPassName() const override {
    return "GoALLC final inline unwind anchors";
  }

  bool runOnMachineFunction(MachineFunction &MF) override {
    if (!MF.getTarget().getTargetTriple().isOSBinFormatGoObj() ||
        !MF.getFunction().getSubprogram())
      return false;

    const TargetInstrInfo &TII = *MF.getSubtarget().getInstrInfo();
    CanonicalSites.clear();

    // LLVM may merge instructions from distinct frontend inline frames (for
    // example, SLP-vectorizing adjacent stores) and retain only one debug
    // location. Materialize a source NOP for each required inline edge that no
    // longer occurs in the optimized MachineFunction. Deeper chains are
    // considered first because one such marker also preserves all its parents.
    DenseSet<const DILocation *> SurvivingCallsites;
    for (MachineBasicBlock &MBB : MF)
      for (MachineInstr &MI : MBB)
        if (!MI.isMetaInstruction())
          for (const DILocation *Loc = MI.getDebugLoc().get();
               Loc && Loc->getInlinedAt(); Loc = Loc->getInlinedAt())
            SurvivingCallsites.insert(Loc->getInlinedAt());

    bool Changed = false;
    if (!MF.empty()) {
      MachineBasicBlock *InsertBlock = nullptr;
      MachineBasicBlock::iterator InsertAt;
      for (MachineBasicBlock &MBB : MF) {
        auto It = MBB.begin();
        while (It != MBB.end() && (It->isMetaInstruction() ||
                                   It->getFlag(MachineInstr::FrameSetup)))
          ++It;
        if (It != MBB.end()) {
          InsertBlock = &MBB;
          InsertAt = It;
          break;
        }
      }
      for (const DILocation *Loc : requiredInlineLocations(MF)) {
        const DILocation *Innermost = Loc->getInlinedAt();
        if (SurvivingCallsites.contains(Innermost))
          continue;
        if (!InsertBlock)
          report_fatal_error("GoALLC required inline location has no "
                             "post-prologue insertion point");
        TII.insertNoop(*InsertBlock, InsertAt);
        MachineInstr &Marker = *std::prev(InsertAt);
        Marker.setDebugLoc(DebugLoc(Loc));
        for (const DILocation *Site = Loc; Site && Site->getInlinedAt();
             Site = Site->getInlinedAt())
          SurvivingCallsites.insert(Site->getInlinedAt());
        Changed = true;
      }
    }

    // Normalize complete inline chains before inspecting them. This keeps the
    // anchor pass and the later GoObj debug handler on the same edge identity.
    for (MachineBasicBlock &MBB : MF)
      for (MachineInstr &MI : MBB)
        if (!MI.isMetaInstruction() && MI.getDebugLoc())
          MI.setDebugLoc(
              DebugLoc(canonicalizeLocation(MI.getDebugLoc().get())));

    DenseSet<const DILocation *> AnchoredCallsites;

    for (MachineBasicBlock &MBB : MF) {
      for (auto It = MBB.begin(), End = MBB.end(); It != End; ++It) {
        MachineInstr &MI = *It;
        if (MI.isMetaInstruction() || !MI.getDebugLoc())
          continue;

        SmallVector<const DILocation *, 4> CallSites;
        for (const DILocation *Loc = MI.getDebugLoc().get();
             Loc && Loc->getInlinedAt(); Loc = Loc->getInlinedAt())
          CallSites.push_back(Loc->getInlinedAt());
        std::reverse(CallSites.begin(), CallSites.end());

        // Insert outer-to-inner before the first surviving instruction in the
        // child. Repeated insertion at It consequently leaves the anchors in
        // outer-to-inner byte order immediately before that instruction.
        for (const DILocation *CallSite : CallSites) {
          if (!AnchoredCallsites.insert(CallSite).second)
            continue;

          TII.insertNoop(MBB, It);
          MachineInstr &Anchor = *std::prev(It);

          // Compiler-generated wrappers can carry a real inline edge whose
          // callsite line is zero. GoObj still needs a ParentPC for that edge,
          // while the line-table collector deliberately ignores line zero.
          // Give only the artificial anchor the nearest existing caller line;
          // the inline-tree node retains the frontend's original line zero.
          unsigned AnchorLine = CallSite->getLine();
          for (const DILocation *Caller = CallSite->getInlinedAt();
               AnchorLine == 0 && Caller; Caller = Caller->getInlinedAt())
            AnchorLine = Caller->getLine();
          if (AnchorLine == 0)
            if (const DISubprogram *SP = CallSite->getScope()->getSubprogram())
              AnchorLine = SP->getLine();
          if (AnchorLine == 0)
            report_fatal_error(
                "GoALLC inline anchor has no usable caller source line");

          auto *Artificial = DILocation::get(
              MF.getFunction().getContext(), AnchorLine, CallSite->getColumn(),
              CallSite->getScope(), CallSite->getInlinedAt(),
              /*ImplicitCode=*/true, CallSite->getAtomGroup(),
              CallSite->getAtomRank());
          Anchor.setDebugLoc(DebugLoc(Artificial));

          MCSymbol *Label = MF.getContext().createTempSymbol();
          Anchor.setPreInstrSymbol(MF, Label);
          MF.getContext().markGoObjInlineAnchor(Label);
          Changed = true;
        }
      }
    }
    return Changed;
  }

  void getAnalysisUsage(AnalysisUsage &AU) const override {
    AU.setPreservesCFG();
    MachineFunctionPass::getAnalysisUsage(AU);
  }
};

char GoALLCInlineAnchorPass::ID = 0;

} // namespace

Pass *createGoALLCInlineAnchorPass() { return new GoALLCInlineAnchorPass(); }
