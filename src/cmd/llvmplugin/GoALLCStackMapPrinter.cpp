// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/SmallVector.h"
#include "llvm/CodeGen/AsmPrinter.h"
#include "llvm/CodeGen/GCMetadataPrinter.h"
#include "llvm/CodeGen/GoCallingConv.h"
#include "llvm/CodeGen/StackMaps.h"
#include "llvm/MC/MCContext.h"
#include "llvm/Support/ErrorHandling.h"

#include <limits>
#include <optional>

using namespace llvm;

namespace {

class GoALLCStackMapPrinter final : public GCMetadataPrinter {
public:
  bool emitStackMaps(StackMaps &SM, AsmPrinter &AP) override;
};

MCContext::GoObjStackMapLocation::LocationType
convertLocationType(StackMaps::Location::LocationType Type) {
  using GoLocation = MCContext::GoObjStackMapLocation;
  switch (Type) {
  case StackMaps::Location::Unprocessed:
    return GoLocation::Unprocessed;
  case StackMaps::Location::Register:
    return GoLocation::Register;
  case StackMaps::Location::Direct:
    return GoLocation::Direct;
  case StackMaps::Location::Indirect:
    return GoLocation::Indirect;
  case StackMaps::Location::Constant:
    return GoLocation::Constant;
  case StackMaps::Location::ConstantIndex:
    return GoLocation::ConstantIndex;
  }
  llvm_unreachable("unknown StackMaps location type");
}

uint64_t getNonnegativeConstant(const StackMaps::Location &Location,
                                StringRef Description) {
  if (Location.Type != StackMaps::Location::Constant || Location.Offset < 0)
    report_fatal_error("malformed GoALLC stackmap " + Description);
  return static_cast<uint64_t>(Location.Offset);
}

} // namespace

static GCMetadataPrinterRegistry::Add<GoALLCStackMapPrinter>
    GoALLCStackMapPrinterRegistration("goallc",
                                      "GoALLC GoObj Machine StackMaps bridge");

bool GoALLCStackMapPrinter::emitStackMaps(StackMaps &SM, AsmPrinter &AP) {
  // Other object formats can continue to use LLVM's standard stackmap
  // serialization while the GoObj bridge is being brought up.
  if (!AP.OutContext.isGoObj())
    return false;

  auto &Callsites = SM.getCSInfos();
  auto Callsite = Callsites.begin();
  uint32_t PointerSize = AP.getPointerSize();
  if (!PointerSize)
    report_fatal_error("GoALLC statepoint target has no pointer size");

  for (const auto &[Function, Info] : SM.getFnInfos()) {
    for (uint64_t I = 0; I != Info.RecordCount; ++I) {
      if (Callsite == Callsites.end())
        report_fatal_error(
            "GoALLC statepoint function record count exceeds callsites");

      // LLVM's statepoint parser flattens CC, flags, deopt count, deopt
      // operands, GC base/derived pairs, and GC allocas into Locations. The
      // first three entries and the deopt operands are not GC roots.
      const StackMaps::CallsiteInfo &CSI = *Callsite++;
      if (CSI.Locations.size() < 3)
        report_fatal_error("malformed GoALLC statepoint location list");
      (void)getNonnegativeConstant(CSI.Locations[0], "calling convention");
      (void)getNonnegativeConstant(CSI.Locations[1], "flags");
      uint64_t NumDeopts =
          getNonnegativeConstant(CSI.Locations[2], "deopt count");
      if (NumDeopts > CSI.Locations.size() - 3)
        report_fatal_error("malformed GoALLC statepoint deopt operands");
      if (NumDeopts > std::numeric_limits<uint32_t>::max())
        report_fatal_error("GoALLC statepoint has too many deopt operands");

      MCContext::GoObjStackMapEntry Entry{CSI.CSOffsetExpr,   CSI.ID,
                                          CSI.IsIndirectCall, Info.StackSize,
                                          PointerSize,
                                          static_cast<uint32_t>(NumDeopts),
                                          {}};
      Entry.Locations.reserve(CSI.Locations.size() - 3);
      for (const StackMaps::Location &Location :
           ArrayRef(CSI.Locations).drop_front(3)) {
        auto Type = convertLocationType(Location.Type);
        int64_t Offset = Location.Offset;
        if (Location.Type == StackMaps::Location::Constant ||
            Location.Type == StackMaps::Location::ConstantIndex) {
          std::optional<int64_t> Constant = SM.getConstantValue(Location);
          if (!Constant)
            report_fatal_error(
                "GoALLC statepoint contains an invalid constant-pool index");
          Type = MCContext::GoObjStackMapLocation::Constant;
          Offset = *Constant;
        }
        Entry.Locations.push_back(
            {Type, Location.Size, Location.Reg, Offset});
      }
      AP.OutContext.addGoObjSymbolStackMapEntry(Function, std::move(Entry));
    }
  }

  if (Callsite != Callsites.end())
    report_fatal_error(
        "GoALLC statepoint callsites exceed function record counts");

  SM.reset();
  return true;
}
