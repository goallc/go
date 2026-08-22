// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/SmallPtrSet.h"
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

constexpr uint64_t GoObjInlineAnchorCookie = 0x476f414c4c43494eULL;

bool isInlinePlacementMarker(const MachineInstr &MI) {
  if (!MI.isInlineAsm())
    return false;
  const MDNode *Loc = MI.getLocCookieMD();
  const auto *Cookie =
      Loc && Loc->getNumOperands() != 0
          ? mdconst::dyn_extract<ConstantInt>(Loc->getOperand(0))
          : nullptr;
  return Cookie && Cookie->getZExtValue() == GoObjInlineAnchorCookie;
}

bool isSameInlineFrame(const DILocation *Loc, const DILocation *CallSite) {
  if (!Loc || !CallSite)
    return false;
  // Match the structural parent frame, not file/line coordinates. Optimizers
  // may clone or rewrite DILocations while preserving this inline context.
  const DISubprogram *LocSP = Loc->getScope()->getSubprogram();
  const DISubprogram *CallSiteSP = CallSite->getScope()->getSubprogram();
  return LocSP == CallSiteSP && Loc->getInlinedAt() == CallSite->getInlinedAt();
}

bool markerPrecedesSurvivingChild(const MachineInstr &Marker) {
  const DILocation *MarkerLoc = Marker.getDebugLoc().get();
  if (!MarkerLoc)
    return false;
  const MachineBasicBlock &MBB = *Marker.getParent();
  for (auto It = std::next(Marker.getIterator()); It != MBB.end(); ++It) {
    if (isInlinePlacementMarker(*It))
      break;
    if (It->isMetaInstruction())
      continue;
    for (const DILocation *Loc = It->getDebugLoc().get();
         Loc && Loc->getInlinedAt(); Loc = Loc->getInlinedAt())
      if (isSameInlineFrame(MarkerLoc, Loc->getInlinedAt()))
        return true;
  }
  return false;
}

MachineInstr *
findPlacementMarker(const SmallVectorImpl<MachineInstr *> &Markers,
                    const DILocation *InlineFrame,
                    const SmallPtrSetImpl<MachineInstr *> &ClaimedMarkers,
                    const SmallPtrSetImpl<MachineInstr *> &UsedMarkers) {
  MachineInstr *Best = nullptr;
  unsigned BestPriority = 5;
  for (MachineInstr *Candidate : Markers) {
    const bool SameFrame =
        isSameInlineFrame(Candidate->getDebugLoc().get(), InlineFrame);
    const bool Claimed = ClaimedMarkers.contains(Candidate);
    const bool Used = UsedMarkers.contains(Candidate);
    unsigned Priority = 4;
    if (SameFrame) {
      if (Used)
        Priority = 2;
      else if (Claimed)
        Priority = 1;
      else
        Priority = 0;
    } else if (!Claimed && !Used) {
      Priority = 3;
    }
    if (Priority < BestPriority) {
      Best = Candidate;
      BestPriority = Priority;
    }
  }
  return Best;
}

struct InlineInsertionPoint {
  MachineBasicBlock *Block = nullptr;
  MachineBasicBlock::iterator At;
};

InlineInsertionPoint fallbackInlineInsertionPoint(MachineFunction &MF) {
  // A source inline edge with no surviving placement marker cannot describe
  // an executable child range. Keep that compatibility fallback beside a
  // normal return, before frame teardown, instead of fabricating a child at
  // the function entry.
  for (MachineBasicBlock &MBB : MF) {
    for (MachineInstr &MI : MBB) {
      if (!MI.isReturn())
        continue;
      auto At = MI.getIterator();
      while (At != MBB.begin()) {
        auto Prev = std::prev(At);
        if (!Prev->isMetaInstruction() &&
            !Prev->getFlag(MachineInstr::FrameDestroy))
          break;
        At = Prev;
      }
      return {&MBB, At};
    }
  }

  for (MachineBasicBlock &MBB : llvm::reverse(MF))
    for (MachineInstr &MI : llvm::reverse(MBB))
      if (!MI.isMetaInstruction() && !isInlinePlacementMarker(MI))
        return {&MBB, MI.getIterator()};
  return {};
}

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

    SmallVector<MachineInstr *, 8> PlacementMarkers;
    SmallPtrSet<MachineInstr *, 8> ClaimedPlacementMarkers;
    for (MachineBasicBlock &MBB : MF)
      for (MachineInstr &MI : MBB)
        if (isInlinePlacementMarker(MI)) {
          PlacementMarkers.push_back(&MI);
          if (markerPrecedesSurvivingChild(MI))
            ClaimedPlacementMarkers.insert(&MI);
        }

    // LLVM may merge instructions from distinct frontend inline frames (for
    // example, SLP-vectorizing adjacent stores) and retain only one debug
    // location. Materialize a source NOP for each required inline edge that no
    // longer occurs in the optimized MachineFunction. Deeper chains are
    // considered first because one such marker also preserves all its parents.
    DenseSet<const DILocation *> SurvivingCallsites;
    for (MachineBasicBlock &MBB : MF)
      for (MachineInstr &MI : MBB)
        if (!MI.isMetaInstruction() && !isInlinePlacementMarker(MI))
          for (const DILocation *Loc = MI.getDebugLoc().get();
               Loc && Loc->getInlinedAt(); Loc = Loc->getInlinedAt())
            SurvivingCallsites.insert(Loc->getInlinedAt());

    bool Changed = false;
    if (!MF.empty()) {
      SmallPtrSet<MachineInstr *, 8> UsedPlacementMarkers;
      InlineInsertionPoint CompatibilityFallback;
      for (const DILocation *Loc : requiredInlineLocations(MF)) {
        const DILocation *Innermost = Loc->getInlinedAt();
        if (SurvivingCallsites.contains(Innermost))
          continue;

        MachineInstr *Placement =
            findPlacementMarker(PlacementMarkers, Innermost,
                                ClaimedPlacementMarkers, UsedPlacementMarkers);

        InlineInsertionPoint Insert;
        if (Placement) {
          UsedPlacementMarkers.insert(Placement);
          Insert = {Placement->getParent(), Placement->getIterator()};
        } else {
          if (!CompatibilityFallback.Block)
            CompatibilityFallback = fallbackInlineInsertionPoint(MF);
          Insert = CompatibilityFallback;
        }
        if (!Insert.Block)
          report_fatal_error(
              "GoALLC required inline location has no insertion point");
        TII.insertNoop(*Insert.Block, Insert.At);
        MachineInstr &Marker = *std::prev(Insert.At);
        Marker.setDebugLoc(DebugLoc(Loc));
        for (const DILocation *Site = Loc; Site && Site->getInlinedAt();
             Site = Site->getInlinedAt())
          SurvivingCallsites.insert(Site->getInlinedAt());
        Changed = true;
      }
    }

    for (MachineInstr *Marker : PlacementMarkers) {
      Marker->eraseFromParent();
      Changed = true;
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
