// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/ADT/ArrayRef.h"
#include "llvm/BinaryFormat/GoObj.h"
#include "llvm/CodeGen/AsmPrinter.h"
#include "llvm/CodeGen/GCMetadataPrinter.h"
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

// The statically linked compiler calls this anchor so the archive linker pulls
// in this translation unit and runs the GCMetadataPrinter registration above.
void linkGoALLCStackMapPrinter() {}

bool GoALLCStackMapPrinter::emitStackMaps(StackMaps &SM, AsmPrinter &AP) {
  // Other object formats continue to use LLVM's standard stackmap format.
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
            "GoALLC stackmap function record count exceeds callsites");

      const StackMaps::CallsiteInfo &CSI = *Callsite++;
      const bool IsEntryArgs = CSI.ID == GoObj::EntryArgsStackMapID;
      uint64_t NumDeopts = 0;
      ArrayRef<StackMaps::Location> Locations = CSI.Locations;
      if (!IsEntryArgs) {
        // Statepoint records start with the calling convention, flags, deopt
        // count, and then the deopt operands. EntryArgsStackMapID is a plain
        // STACKMAP whose locations are only function argument pointer homes.
        if (Locations.size() < 3)
          report_fatal_error("malformed GoALLC statepoint location list");
        (void)getNonnegativeConstant(Locations[0], "calling convention");
        (void)getNonnegativeConstant(Locations[1], "flags");
        NumDeopts = getNonnegativeConstant(Locations[2], "deopt count");
        if (NumDeopts > Locations.size() - 3)
          report_fatal_error("malformed GoALLC statepoint deopt operands");
        if (NumDeopts > std::numeric_limits<uint32_t>::max())
          report_fatal_error("GoALLC statepoint has too many deopt operands");
        Locations = Locations.drop_front(3);
      }

      MCContext::GoObjStackMapEntry Entry{CSI.CSOffsetExpr,
                                          CSI.ID,
                                          CSI.IsIndirectCall,
                                          Info.StackSize,
                                          PointerSize,
                                          static_cast<uint32_t>(NumDeopts),
                                          {}};
      Entry.Locations.reserve(Locations.size());
      for (const StackMaps::Location &Location : Locations) {
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
        Entry.Locations.push_back({Type, Location.Size, Location.Reg, Offset});
      }
      AP.OutContext.addGoObjSymbolStackMapEntry(Function, std::move(Entry));
    }
  }

  if (Callsite != Callsites.end())
    report_fatal_error(
        "GoALLC stackmap callsites exceed function record counts");

  SM.reset();
  return true;
}
