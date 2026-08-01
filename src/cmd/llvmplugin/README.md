# GoALLC LLVM pre-codegen plugin

This directory owns the GoALLC-specific LLVM pass pipeline. It deliberately
lives in the Go repository: LLVM only supplies the generic `llc`
`-load-pass-plugin` and pre-codegen callback mechanism.

`GoALLCStatepointPlugin.cpp` adapts that callback to
`goallc::runPreCodeGenPipeline`. GoALLC transformations belong in
`GoALLCPreCodeGen.cpp` or other sources in this directory, not in the LLVM
repository. The core entry point is separate from the plugin adapter so a
future in-process `cmd/compile` integration can call the same pipeline without
going through `llc`.

The SSA-to-LLVM lowering owns the function-level contract: Go ABI definitions
carry `gc "goallc"` and the `go-stack-growth-statepoint` attribute. Loading this
plugin registers the named GC strategy and its metadata printer; the rewrite
pass consumes the frontend markers rather than adding them.

Entry argument pointer maps do not use an IR marker or intrinsic. After LLVM
optimization and inlining, target formal-argument lowering recursively derives
pointer words from the surviving function's LLVM parameter types and combines
them with the Go calling-convention layout. Results, padding, integer words,
and the hidden `nest` closure context are not entry pointer bits. Keeping this
calculation at formal lowering prevents an inlined callee's entry contract from
being cloned into its caller.

The initial statepoint pass handles ordinary calls in Go ABI functions. It
computes pointer liveness with a backwards CFG dataflow analysis, assigns
stable callsite IDs, emits `gc.statepoint` and `gc.relocate`, and respects
LLVM's `gc-leaf-function` attribute on callees and individual call sites.
Definitions carrying that attribute are verified to contain only GC-leaf calls.
Pointer classification is conservative and independent of LLVM address spaces.
After all calls have been rewritten, the pass models each original pointer and
all of its relocates as definitions of a temporary promotable alloca. Loads at
the old uses make the required reaching definition explicit, and LLVM's public
`PromoteMemToReg` utility removes the temporary memory operations and forms the
relocation PHIs. This applies the same SSA construction to straight-line code,
conditional paths, loop backedges, and irreducible control flow.
Static `alloca` addresses and constant GEP/cast forms derived from them are
native frame addresses, not Go heap pointers, and are excluded from `gc-live`.
SelectionDAG rematerializes those addresses from their fixed frame indexes
after stack growth. Pointer values loaded from the allocation remain ordinary
tracked roots.

The current SSA value and CFG rewrite support matrix is:

| Value or control-flow shape | Status | Current contract |
| --- | --- | --- |
| Pointer arguments | AArch64 GoObj qualified; SelectionDAG home reuse also tested on X86 | Values live after a call use caller statepoints; exact stack inputs stay in their fixed ABI homes, while register inputs and transformed values use normal statepoint spills. Call-only arguments are described by the callee's type-derived entry map. |
| Static `alloca` addresses | Supported | Proven static-allocation addresses and constant GEP/cast forms are frame addresses and do not enter `gc-live`; a PHI/select is accepted only when every path resolves to the same zero-offset alloca. Pointers loaded from memory remain tracked. |
| `select`, GEP, and pointer casts | Supported | Each resulting pointer SSA value is tracked conservatively. |
| Pointer-valued call results | Supported | `gc.result` replaces the ordinary result and later safepoints relocate it. |
| Multiple ordinary calls | Supported | Stable IDs and live sets remain per call; the next statepoint consumes the current relocated SSA value. |
| Ordinary CFG merges | Supported | Call/skip and multiple-safepoint paths merge through pointer PHIs formed by `PromoteMemToReg`. |
| Loops and irreducible CFG | Supported | Relocation definitions are propagated through backedge and multi-entry PHIs without a shape-specific algorithm. |
| Fixed struct/array SSA aggregates | Supported | Pointer leaves are scalarized before liveness and reconstructed from the current relocated SSA leaves. |
| Aggregate arguments and call results | Supported for IR rewriting | The wrapped call keeps its real aggregate ABI type. Only leaves live after the call enter caller `gc-live`; supported fixed formal layouts also contribute pointer words to AArch64 entry maps. |
| Aggregate load results and store operands | Supported | First-class SSA values use aggregate normalization. Pointer leaves stored in a fixed alloca remain memory roots and are not converted to SSA roots. |
| Pointer-containing `alloca` storage | GoObj qualified for fixed layouts | Pointer slots in a single fixed entry-block alloca are zero-initialized once. Every safepoint carries the alloca address and a 64-bit-word pointer bitmap in a self-describing deopt suffix; no leaf preload, `gc-live`, `gc.relocate`, or post-call write-back is generated. |
| Fixed and scalable vectors | Unsupported | Vector lane and scalable-count semantics require a separate design; fails closed. |
| General moving-GC base/derived analysis | Unsupported | Base and derived indexes are identical in the current non-moving-heap phase. |
| `invoke`, `callbr`, non-deopt operand bundles, unsupported parameter attributes, and `musttail` | Unsupported | One ordinary deopt bundle is preserved before the alloca suffix. `nest`, `captures`, and `readonly` parameter attributes are preserved; other shapes fail closed. |

This matrix describes verified IR rewriting, not full runtime qualification.
The Darwin/arm64 Go execution whitelist additionally covers one unconditional
`runtime.GC`, a conditional call/skip merge, two branch-local GC safepoints,
and a nested conditional GC. Sequential safepoints in one activation, loop
backedges, multiple simultaneously live pointers, and pointer-valued call
results have IR and verifier coverage, but are not yet runtime-qualified:
focused execution trials currently expose invalid post-call values or
traceback/stack-map failures in those shapes.

The statepoint pass first computes aggregate-only liveness to find each
supported pointer-containing aggregate that is live after a safepoint. Every
explicit leaf is extracted next to the aggregate definition, and each
aggregate use is rebuilt next to that use from `poison` with every required
leaf inserted. Rebuilding from the original aggregate would keep that
aggregate live across the safepoint and is forbidden. Exact `extractvalue`
uses consume the corresponding leaf directly.

After normalization, a second liveness computation tracks scalar pointers
only. Both computations inspect instructions strictly after the current call:
the caller statepoint describes values live across that call, while values used
only as call arguments belong to the callee's `FUNCDATA_ArgsPointerMaps`.
No rebuilt aggregate or side table participates in final liveness.

This normalization has four invariants:

1. `gc-live` contains only scalar pointers, never an aggregate.
2. The original aggregate is used only by definition-local extracts and cannot
   cross a safepoint.
3. A use-local rebuilt aggregate cannot cross a safepoint. A call-only
   aggregate argument stays as the real ABI operand and is neither scalarized
   nor recorded in caller `gc-live`.
4. Post-safepoint reconstruction uses the current SSA definition produced by
   `gc.relocate` and the whole-function relocation PHIs.

Nested fixed structs and arrays are enumerated by leaf index path. Extracting
after an aggregate `freeze` preserves its correlated choice; rebuilding all
explicit leaves preserves `undef` and `poison` field semantics. LLVM struct
padding is not a first-class leaf. LLVM 23 permits an aggregate `gc.result`, so
an aggregate call result is projected first and its pointer leaves become roots
at later safepoints.

An aggregate loaded from memory can be normalized as an independent SSA value
and an aggregate store can consume a rebuilt value. A fixed pointer-containing
alloca follows a different memory-root contract. The plugin enumerates its
pointer offsets, zeroes those pointer slots once, and appends one record per
alloca to every ordinary statepoint's deopt operands. The record carries the
alloca address, whole-object size and alignment, pointer size, valid bitmap bit
count, and 64-bit bitmap words. The envelope and every record carry explicit
lengths, and a trailing duplicate envelope length makes the suffix recoverable
after ordinary deopt operands. There is deliberately no contract version in
the first grammar.

LowerStatepoint lowers the deopt alloca address through its normal direct
FrameIndex path and preserves the adjacent constants in Machine StackMaps. It
does not create a root spill. The Go-owned StackMaps bridge retains the deopt
prefix and resolves both inline constants and `ConstantIndex` values. The
GoObj writer strictly parses the suffix, maps each direct frame location plus
bitmap bit to `LocalsPointerMaps`, and fails closed on malformed lengths,
non-direct bases, inconsistent layout, nonzero padding, duplicates, overlaps,
or locations outside the GC locals range. Runtime and the Go linker see only
ordinary locals pointer maps and `PCDATA_StackMapIndex`; the deopt grammar does
not escape the object writer.

The alloca memory remains the source of truth while the call executes. A
callee may clear or replace fields through an address argument, and the caller
observes those changes because no relocated pre-call SSA value is written back.

This first implementation conservatively records every pointer leaf at every
safepoint for the lifetime of the frame. It may therefore retain otherwise dead
heap objects longer than native Go's reachability-sensitive stack-object
metadata, but its locals pointer map is functionally sufficient for scanning
and relocation. Constants are not roots in the general SSA model. Alloca
records describe the memory layout even when a pointer slot currently contains
null.
Address passing to a callee is supported. A volatile byte load carrying the
compiler-owned empty `!goallc.nilcheck` marker is recognized as an SSA
`OpNilCheck` and remains in place for its faulting semantics; this does not
represent a volatile read of the pointer storage. Dynamic, multiple-element,
scalable, or realigned allocas, pointer vectors, lifetime markers, every
unmarked volatile access, and every atomic access fail closed until their
frame-home and update semantics are explicit.

LLVM's generic `RewriteStatepointsForGC` pass is the design reference for
liveness and relocation SSA formation. GoALLC does not run it directly:
that pass also owns base-pointer inference, EH/invoke rewriting, deopt bundles,
and module-wide attribute cleanup. Those policies exceed the deliberately
narrow Go ABI contract above.

The final combined order is:

1. **Fixed alloca memory-root description.** Enumerate fixed pointer leaves,
   zero them at object initialization, and construct a whole-alloca bitmap
   record without inserting per-call memory traffic.
2. **Aggregate normalization.** Use aggregate-only liveness to find and
   decompose supported live first-class struct/array values, then rebuild
   aggregates immediately before their uses.
3. **Scalar statepoint insertion.** Compute liveness, build scalar-only
   `gc-live` bundles, append the alloca records to deopt, and emit `gc.result`
   and `gc.relocate` only for ordinary SSA roots.
4. **Whole-function relocation SSA.** Model the original scalar definition and
   every relocate as stores to temporary promotable allocas, rewrite old uses
   through loads, and call the public `PromoteMemToReg` utility. This constructs
   conditional, loop/backedge, and irreducible-CFG PHIs and removes all
   temporary memory traffic.

Future stages remain:

1. **Base/derived pointers.** Adapt the scalar GEP/cast path of LLVM's
   `findBasePointer`, then add `select` and PHI conflict handling. Ambiguous
   relationships continue to fail closed.
2. **Rematerialization.** Consider LLVM's GEP/cast rematerialization only after
   base/derived correctness.
3. **Additional call shapes only when required by Go.** Do not import generic
   deopt, transition-bundle, invoke/EH edge normalization, or module-wide
   attribute stripping merely because the LLVM pass supports them.

Most useful LLVM implementation helpers are private to
`RewriteStatepointsForGC.cpp`, so GoALLC will reuse their algorithms through
public LLVM IR and utility APIs rather than depending on private symbols. If a
non-trivial implementation block is copied verbatim, it must live in a
separately attributed source file retaining LLVM's Apache-2.0-with-exception
notice; BSD-only Go source files should not silently absorb copied code.

`GoALLCStackMapPrinter.cpp` is the Go-owned boundary between LLVM Machine
StackMaps and GoObj. It uses the standard
`AsmPrinter -> GCMetadataPrinter::emitStackMaps` hook, retains deopt and GC
locations in `MCContext`, and resolves StackMaps constant-pool indexes without
adding a parallel serializer. LLVM's generic `StackMaps.cpp` only exposes the
constant resolver; it has no GoALLC grammar or GoObj policy. LLVM records GoObj
statepoint callsites at the CALL
start, matching Go's `PCDATA_StackMapIndex` convention without a command-line
mode. The frontend's stack-growth attribute
asks LLVM to express the late-generated `runtime.morestack` call as a physical
MIR `STATEPOINT` with empty deopt and GC-alloca sections. Before frame
allocation, AArch64 formal lowering maps every type-derived input pointer word
onto the existing fixed home reserved for that ABI input. Frame lowering encodes
those homes in the morestack statepoint's GC pointer section as indirect
`SP+offset` locations with base-equals-derived pairs; it never asks the
statepoint machinery to allocate another spill. The GoObj writer recognizes
the stack-growth ID, interprets those offsets in the entry-SP geometry, and
selects pair 0: non-empty `EntryArgs` when present and empty locals. Ordinary
and stack-growth calls use the same Machine StackMaps pipeline without relying
on a return-PC convention. GoObj functions that already contain a Machine
`STATEPOINT` but lack the frontend stack-growth attribute fail closed; there is
no slow-path reset-label fallback for a raw morestack call.
GoObj emits the currently constant safe `PCDATA_UnsafePoint` table first and
the statepoint-derived `PCDATA_StackMapIndex` table second, as required by
their Go ABI indexes 0 and 1.
The GoObj writer interprets locations after final layout. An
`Indirect [SP+offset]` location in the current frame contributes a locals
pointer bit; one in the post-prologue caller-owned argument/result area
contributes an args pointer bit. A `Direct SP+offset` stack address contributes
to neither bitmap as an ordinary GC root because the address itself, rather
than the slot contents, is the pointer. Inside a validated alloca deopt record,
the same direct location is instead the frame base whose bitmap selects memory
slots. Static alloca addresses are excluded from `gc-live` before this point.

Ordinary stack inputs use the same statepoint path. SelectionDAG formal
lowering records a value home only when a Go ABI pointer part is a direct,
non-extending load from an immutable fixed object and its IR aggregate offset,
ABI part offset, load size, and object size all agree. If `gc-live` later
contains that argument or an exact `extractvalue` leaf, statepoint lowering
uses the existing fixed frame index as its indirect memory location, makes the
slot mutable for GC relocation, and reloads `gc.relocate` from that same frame
index. No pre-call copy to a local spill is emitted. A merged, derived,
size-mismatched, or otherwise unproven value falls back to LLVM's normal local
statepoint spill. This is a SelectionDAG contract; it does not introduce
`byval`/`sret`, change GoALLC's LLVM IR emission point, or bypass the standard
statepoint operand format.

For AArch64 GoObj, target frame lowering uses Go's frame-chain layout instead
of the platform ABI frame record: LR is at `SP+0`, this function writes its FP
link at `SP-8` for a future callee, and the caller's existing FP-link word is at
the top of this function's physical frame. The writer derives
`_func.locals = StackSize - 8` and the locals bitmap range
`[SP+8, SP+StackSize-8)` from the final physical `StackSize`. No separate
lowering-to-writer locals-size channel is part of the contract. As in the
native arm64 backend, small frames atomically save LR while updating SP with a
pre-indexed store; frames above `0xf0` compute NewSP and save `(FP, LR)` before
moving SP so asynchronous traceback cannot observe a half-built frame.

The ArgsPointerMaps phase supports scalar LLVM pointer inputs and receivers,
plus pointer leaves in supported fixed struct/array formal layouts, in
ABIInternal register homes, ABIInternal stack-input slots, and ABI0 stack-input
slots on AArch64. Pair 0 is always `(EntryArgs, empty locals)`. Ordinary
statepoints use their actual final machine locations: indirect pointer slots in
the current frame become locals bits, while exact fixed input homes and stack
result slots above the final frame become args bits. The writer jointly
deduplicates each complete `(Args, locals)` pair, so the two tables always have
the same count. It does not eagerly mark declared result slots; a result slot
is an args root only when a statepoint records a live pointer in that physical
slot. A direct stack address is not a bitmap root.

This phase fails closed for unsupported formal aggregate layouts; dynamic or
realigned frames; raw register roots; pointer vectors or ABI layouts whose
pointer words do not map to fixed homes; and ordinary statepoint stack
locations outside either the GC locals range or the adjusted caller-owned
argument/result range. Pointer-containing allocas additionally fail closed for
dynamic or multiple-element allocation, scalable or realigned layout,
non-default-address-space pointers, pointer vectors, lifetime markers,
volatile or atomic access, and any record that does not resolve to one unique
direct frame object at offset zero.
`FUNCDATA_StackObjects` and `PCDATA_ArgLiveIndex` are not implemented.
Pointer-containing fixed allocas are represented conservatively in
`LocalsPointerMaps`; the alloca address itself is never treated as the root.
ArgLive is not a replacement for the entry argument bitmap.

TODO: add native-style `FUNCDATA_StackObjects` reachability so pointer leaves of
dead address-taken objects stop retaining heap objects. This is an optimization
of the conservative locals-map implementation, not a prerequisite for correct
scanning or relocation.

TODO: LLVM opaque pointer types currently make the first itab/type word of a
Go interface and pointers to `NotInHeap` types look like ordinary GC pointers.
The type-derived entry map conservatively marks those words for now. A later
change should give non-GC address words a distinct IR representation or a
signature-level GC-shape attribute.

Build, test, and install it into the LLVM payload that contains `llc`:

```sh
LLVM_PAYLOAD=/path/to/goallc-llvm
PLUGIN_BUILD=/path/to/empty/plugin-build

cmake -S "$GOROOT/src/cmd/llvmplugin" -B "$PLUGIN_BUILD" -G Ninja \
  -DLLVM_DIR="$LLVM_PAYLOAD/lib/cmake/llvm" \
  -DCMAKE_INSTALL_PREFIX="$LLVM_PAYLOAD"
cmake --build "$PLUGIN_BUILD"
ctest --test-dir "$PLUGIN_BUILD" --output-on-failure
cmake --install "$PLUGIN_BUILD"
```

Build the canonical `cmd/objview` from this checkout and pass
`-DGOALLC_OBJVIEW_EXECUTABLE=/path/to/objview` at configure time. This enables
the structured multiple-call test, which verifies both numbered PCDATA tables,
the map selected at each CALL, and the corresponding locals pointer bitmaps.
The Go test fixture
`src/cmd/internal/testdir/testdata/llvm_args_pointer_maps.mir` additionally
forces an entry input home and an ordinary stack-result root into different
ArgsPointerMaps entries, then checks their exact objview bitmaps and
`PCDATA_StackMapIndex` sequence. The identical-source Go fixture
`test/abi/llvm_args_pointer_maps.go` separately forces a scalar pointer and a
three-word pointer aggregate onto the incoming stack. It checks native
assembly stack loads, scalar-only rewritten `gc-live`/`gc.relocate`, fixed-home
MIR with no local statepoint spill, and exact Args/Locals/PCDATA objview data.
The executable identical-source fixture `test/abi/llvm_args_results.go` repeats
those checks with `runtime.GC` for a scalar stack pointer, two pointer stack
arguments separated by a scalar, a pointer-containing stack aggregate, and the
same aggregate combined with overflowing pointer results. The caller checks
pointer identity, pointee values, and scalar payloads after another GC; the ABI
differential still asserts the exact native and GoALLC metadata and MIR for
these functions.

The installed file is `lib/GoALLCStatepoints.dylib` on Darwin or
`lib/GoALLCStatepoints.so` on Linux. Do not build the plugin against a different
LLVM installation and copy it into the payload.
