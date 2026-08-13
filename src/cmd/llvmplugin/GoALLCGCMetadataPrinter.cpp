// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "llvm/CodeGen/GCMetadataPrinter.h"

using namespace llvm;

namespace {

// AsmPrinter requires every GC strategy to have a registered metadata
// printer. GoObj itself consumes Machine StackMaps in LLVM's AsmPrinter path,
// so the Go-owned plugin only needs this marker registration.
class GoALLCGCMetadataPrinter final : public GCMetadataPrinter {};

} // namespace

static GCMetadataPrinterRegistry::Add<GoALLCGCMetadataPrinter>
    GoALLCGCMetadataPrinterRegistration("goallc",
                                        "GoALLC GC metadata marker");
