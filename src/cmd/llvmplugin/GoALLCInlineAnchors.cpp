// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/CodeGen/MachineBasicBlock.h"
#include "llvm/CodeGen/MachineFunction.h"
#include "llvm/CodeGen/MachineFunctionPass.h"
#include "llvm/CodeGen/MachineInstr.h"
#include "llvm/CodeGen/TargetInstrInfo.h"
#include "llvm/IR/DebugInfoMetadata.h"
#include "llvm/IR/DebugLoc.h"
#include "llvm/MC/MCContext.h"
#include "llvm/MC/MCSymbol.h"
#include "llvm/Support/ErrorHandling.h"
#include "llvm/Target/TargetMachine.h"
#include <algorithm>
#include <iterator>

using namespace llvm;

namespace {

bool isSameInlineFrame(const DILocation *Loc, const DILocation *CallSite) {
  if (!Loc || !CallSite)
    return false;
  // Match the structural parent frame, not file/line coordinates. Optimizers
  // may clone or rewrite DILocations while preserving this inline context.
  const DISubprogram *LocSP = Loc->getScope()->getSubprogram();
  const DISubprogram *CallSiteSP = CallSite->getScope()->getSubprogram();
  return LocSP == CallSiteSP && Loc->getInlinedAt() == CallSite->getInlinedAt();
}

bool isSameSourcePosition(const DILocation *Loc, const DILocation *CallSite) {
  if (!Loc || !CallSite)
    return false;
  // Native Go normalizes inline marks and instructions to column one before
  // matching them. Keep the same file-and-line identity without scanning the
  // machine function for another source position.
  return Loc->getFile() == CallSite->getFile() &&
         Loc->getLine() == CallSite->getLine();
}

using InlineEdge = std::pair<const DILocation *, const DISubprogram *>;

SmallVector<InlineEdge, 4> inlineEdges(const DILocation *Loc) {
  SmallVector<InlineEdge, 4> Edges;
  while (const DILocation *CallSite = Loc->getInlinedAt()) {
    const DISubprogram *Callee = Loc->getScope()->getSubprogram();
    if (!Callee)
      report_fatal_error("GoALLC inline location has no callee subprogram");
    Edges.emplace_back(CallSite, Callee);
    Loc = CallSite;
  }
  std::reverse(Edges.begin(), Edges.end());
  return Edges;
}

bool containsInlineEdge(const DILocation *Loc, InlineEdge Edge) {
  if (!Loc)
    return false;
  for (InlineEdge Candidate : inlineEdges(Loc))
    if (Candidate == Edge)
      return true;
  return false;
}

bool isGoInlineMark(const MachineInstr &MI) {
  if (!MI.isDebugLabel())
    return false;
  const DILabel *Label = MI.getDebugLabel();
  return Label && Label->isArtificial() &&
         Label->getName().starts_with("$go.inlmark.");
}

/// Record one real parent instruction for every inline edge that remains after
/// all generic machine layout passes. Go's inline unwinder needs a final PC in
/// the parent frame; reuse an adjacent instruction when possible and insert a
/// NOP only when the surviving child has no suitable predecessor.
class GoALLCInlineAnchorPass final : public MachineFunctionPass {
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
    bool Changed = false;

    DenseSet<InlineEdge> SurvivingEdges;
    for (MachineBasicBlock &MBB : MF)
      for (MachineInstr &MI : MBB)
        if (!MI.isMetaInstruction() && MI.getDebugLoc())
          for (InlineEdge Edge : inlineEdges(MI.getDebugLoc().get()))
            SurvivingEdges.insert(Edge);

    struct CoalescedInlineChild {
      MachineInstr *Before;
      DebugLoc Location;
    };
    SmallVector<CoalescedInlineChild, 4> CoalescedInlineChildren;
    for (MachineBasicBlock &MBB : MF) {
      for (auto It = MBB.begin(), End = MBB.end(); It != End; ++It) {
        if (!isGoInlineMark(*It) || !It->getDebugLoc() ||
            !It->getDebugLoc()->getInlinedAt())
          continue;
        const DISubprogram *Callee =
            It->getDebugLabel()->getScope()->getSubprogram();
        if (!Callee)
          report_fatal_error("GoALLC inline mark has no callee subprogram");
        SmallVector<InlineEdge, 4> MarkedEdges =
            inlineEdges(It->getDebugLoc().get());
        InlineEdge Edge = MarkedEdges.back();
        auto Next = std::next(It);
        while (Next != End && Next->isMetaInstruction())
          ++Next;
        if (Next == End || !Next->getDebugLoc() ||
            containsInlineEdge(Next->getDebugLoc().get(), Edge))
          continue;

        const DILocation *NextLoc = Next->getDebugLoc().get();
        SmallVector<InlineEdge, 4> NextEdges = inlineEdges(NextLoc);
        if (NextLoc->getScope()->getSubprogram() != Callee ||
            !NextLoc->getInlinedAt() || MarkedEdges.size() <= NextEdges.size())
          continue;
        bool ParentSurvives = true;
        for (auto Parent = MarkedEdges.begin();
             Parent != std::prev(MarkedEdges.end()); ++Parent)
          ParentSurvives &= SurvivingEdges.contains(*Parent);
        if (!ParentSurvives)
          continue;

        // LLVM may coalesce equivalent instructions from differently nested
        // instances of the same inline callee and retain only the shallower
        // instance's inlinedAt chain on the combined instruction. A surviving
        // parent chain, the adjacent preserved label, and the final
        // instruction's identical callee together prove that the deeper
        // instance contributed code. Recreate only its location on a final
        // NOP; ambiguous same-depth labels and fully optimized-out bodies do
        // not manufacture Go frames.
        auto *Location = DILocation::get(
            MF.getFunction().getContext(), NextLoc->getLine(),
            NextLoc->getColumn(), NextLoc->getScope(),
            It->getDebugLoc()->getInlinedAt(), /*ImplicitCode=*/true,
            NextLoc->getAtomGroup(), NextLoc->getAtomRank());
        CoalescedInlineChildren.push_back({&*Next, DebugLoc(Location)});
      }
    }
    for (const CoalescedInlineChild &Child : CoalescedInlineChildren) {
      MachineBasicBlock &MBB = *Child.Before->getParent();
      auto Before = Child.Before->getIterator();
      TII.insertNoop(MBB, Before);
      std::prev(Before)->setDebugLoc(Child.Location);
      Changed = true;
    }

    DenseMap<InlineEdge, MachineInstr *> PreferredChildren;
    for (MachineBasicBlock &MBB : MF) {
      for (auto It = MBB.begin(), End = MBB.end(); It != End; ++It) {
        if (!isGoInlineMark(*It) || !It->getDebugLoc() ||
            !It->getDebugLoc()->getInlinedAt())
          continue;
        const DISubprogram *Callee =
            It->getDebugLabel()->getScope()->getSubprogram();
        if (!Callee)
          report_fatal_error("GoALLC inline mark has no callee subprogram");
        InlineEdge Edge{It->getDebugLoc()->getInlinedAt(), Callee};
        auto Next = std::next(It);
        while (Next != End && Next->isMetaInstruction())
          ++Next;
        // A preserved debug label is only a preferred boundary when final real
        // code still proves the same inline edge. Labels for fully optimized-
        // out bodies remain debug history and must not manufacture Go frames.
        if (Next != End && Next->getDebugLoc() &&
            containsInlineEdge(Next->getDebugLoc().get(), Edge))
          PreferredChildren.try_emplace(Edge, &*Next);
      }
    }

    DenseSet<InlineEdge> AnchoredEdges;
    DenseSet<MachineInstr *> AnchorInstructions;

    for (MachineBasicBlock &MBB : MF) {
      for (auto It = MBB.begin(), End = MBB.end(); It != End; ++It) {
        MachineInstr &MI = *It;
        if (MI.isMetaInstruction() || !MI.getDebugLoc())
          continue;

        SmallVector<InlineEdge, 4> Edges = inlineEdges(MI.getDebugLoc().get());

        // A function value points at the first machine byte. Never let a
        // later preferred debug label move the outer function's only parent
        // range past a child instruction scheduled at byte zero: FuncForPC
        // must still resolve the entry PC to the concrete function. A real
        // prologue or any earlier emitted instruction already provides that
        // range and keeps the ordinary preferred-boundary behavior.
        bool AtFunctionEntry = &MBB == &MF.front();
        if (AtFunctionEntry)
          for (auto Prev = MBB.begin(); Prev != It; ++Prev)
            if (!Prev->isMetaInstruction() &&
                (Prev->getFlag(MachineInstr::FrameSetup) ||
                 (Prev->getDebugLoc() && Prev->getDebugLoc().getLine() != 0)) &&
                TII.getInstSizeInBytes(*Prev) != 0) {
              AtFunctionEntry = false;
              break;
            }

        // Handle outer-to-inner before the first surviving instruction in the
        // child. Native Go reuses an emitted instruction for an inline mark
        // whenever possible. Do the same at the structural child boundary:
        // reuse the immediately preceding non-zero-width parent instruction
        // only when it also carries the callsite's source position.
        for (InlineEdge Edge : Edges) {
          if (AnchoredEdges.contains(Edge))
            continue;
          if (auto Preferred = PreferredChildren.find(Edge);
              Preferred != PreferredChildren.end() &&
              Preferred->second != &MI && !AtFunctionEntry)
            continue;
          AnchoredEdges.insert(Edge);

          const DILocation *CallSite = Edge.first;

          MachineInstr *Anchor = nullptr;
          bool InsertedAnchor = false;
          auto Prev = It;
          while (Prev != MBB.begin()) {
            --Prev;
            if (Prev->isMetaInstruction())
              continue;
            if (!Prev->getFlag(MachineInstr::FrameSetup) &&
                Prev->getDebugLoc() && Prev->getDebugLoc().getLine() != 0 &&
                TII.getInstSizeInBytes(*Prev) != 0 &&
                !AnchorInstructions.contains(&*Prev) &&
                isSameInlineFrame(Prev->getDebugLoc().get(), CallSite) &&
                isSameSourcePosition(Prev->getDebugLoc().get(), CallSite))
              Anchor = &*Prev;
            break;
          }
          if (!Anchor) {
            TII.insertNoop(MBB, It);
            Anchor = &*std::prev(It);
            InsertedAnchor = true;
          }
          AnchorInstructions.insert(Anchor);

          if (InsertedAnchor) {
            // Compiler-generated wrappers can carry a real inline edge whose
            // callsite line is zero. GoObj still needs a ParentPC for that
            // edge, while the line-table collector deliberately ignores line
            // zero. Give only a synthetic anchor the nearest existing caller
            // line; a reused instruction keeps its original source location.
            unsigned AnchorLine = CallSite->getLine();
            for (const DILocation *Caller = CallSite->getInlinedAt();
                 AnchorLine == 0 && Caller; Caller = Caller->getInlinedAt())
              AnchorLine = Caller->getLine();
            if (AnchorLine == 0)
              if (const DISubprogram *SP =
                      CallSite->getScope()->getSubprogram())
                AnchorLine = SP->getLine();
            if (AnchorLine == 0)
              report_fatal_error(
                  "GoALLC inline anchor has no usable caller source line");

            auto *Artificial = DILocation::get(
                MF.getFunction().getContext(), AnchorLine,
                CallSite->getColumn(), CallSite->getScope(),
                CallSite->getInlinedAt(), /*ImplicitCode=*/true,
                CallSite->getAtomGroup(), CallSite->getAtomRank());
            Anchor->setDebugLoc(DebugLoc(Artificial));
          }

          MCSymbol *Label = Anchor->getPreInstrSymbol();
          if (!Label) {
            Label = MF.getContext().createTempSymbol();
            Anchor->setPreInstrSymbol(MF, Label);
          }
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
