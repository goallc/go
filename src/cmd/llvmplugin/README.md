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
The initial implementation keeps live `alloca` addresses and alloca-derived
pointer values in `gc-live`; it does not try to predict whether instruction
selection will rematerialize a frame address or spill a materialized pointer
value.

The current SSA value and CFG rewrite support matrix is:

| Value or control-flow shape | Status | Current contract |
| --- | --- | --- |
| Pointer arguments | Supported | Tracked independently at every safepoint. |
| `alloca` and alloca-derived pointers | Supported | The pointer value is live; pointer fields stored in the allocation are not described. |
| `select`, GEP, and pointer casts | Supported | Each resulting pointer SSA value is tracked conservatively. |
| Pointer-valued call results | Supported | `gc.result` replaces the ordinary result and later safepoints relocate it. |
| Multiple ordinary calls | Supported | Stable IDs and live sets remain per call; the next statepoint consumes the current relocated SSA value. |
| Ordinary CFG merges | Supported | Call/skip and multiple-safepoint paths merge through pointer PHIs formed by `PromoteMemToReg`. |
| Loops and irreducible CFG | Supported | Relocation definitions are propagated through backedge and multi-entry PHIs without a shape-specific algorithm. |
| Pointer-containing aggregates | Unsupported | Fails closed before rewriting. |
| General moving-GC base/derived analysis | Unsupported | Base and derived indexes are identical in the current non-moving-heap phase. |
| `invoke`, `callbr`, operand bundles, non-`nest` parameter attributes, and `musttail` | Unsupported | Fails closed rather than widening the call contract. |

This matrix describes verified IR rewriting, not full runtime qualification.
The Darwin/arm64 Go execution whitelist additionally covers one unconditional
`runtime.GC`, a conditional call/skip merge, two branch-local GC safepoints,
and a nested conditional GC. Sequential safepoints in one activation, loop
backedges, multiple simultaneously live pointers, and pointer-valued call
results have IR and verifier coverage, but are not yet runtime-qualified:
focused execution trials currently expose invalid post-call values or
traceback/stack-map failures in those shapes.

LLVM's generic `RewriteStatepointsForGC` pass is the design reference for
liveness and relocation SSA formation. GoALLC does not run it directly:
that pass also owns base-pointer inference, EH/invoke rewriting, deopt bundles,
and module-wide attribute cleanup. Those policies exceed the deliberately
narrow Go ABI contract above.

The staged reuse plan is:

1. **Global relocation SSA (current).** Keep GoALLC's call eligibility,
   stable IDs, per-safepoint live sets, and base-equals-derived convention.
   Adapt the structure of LLVM's
   `relocationViaAlloca`: model the original definition and every relocate as
   stores to temporary promotable allocas, rewrite old uses through loads, and
   call the public `PromoteMemToReg` utility. This handles loop/backedge PHIs,
   multiple relocation definitions in one block, and irreducible CFG without
   importing the generic pass's broader call policy. GoALLC validation remains
   in front of the transform.
2. **Base/derived pointers.** Adapt the scalar GEP/cast path of LLVM's
   `findBasePointer` first, then add `select` and PHI conflict handling. Inserted
   base PHIs become explicit definitions, and liveness must be recomputed before
   finalizing each statepoint live set. Ambiguous relationships continue to
   fail closed.
3. **Aggregates and rematerialization.** Add pointer aggregate decomposition
   and only then consider LLVM's GEP/cast rematerialization optimization.
   `ArgsPointerMaps`, `StackObjects`, and write barriers remain separate Go
   runtime metadata projects.
4. **Additional call shapes only when required by Go.** Do not import generic
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
`AsmPrinter -> GCMetadataPrinter::emitStackMaps` hook and copies only raw
machine locations into `MCContext`. LLVM's generic `StackMaps.cpp` has no
GoALLC or GoObj branch. LLVM records GoObj statepoint callsites at the CALL
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
The GoObj writer interprets locations after final layout: an
`Indirect [SP+offset]` location contributes a locals pointer bit, while a
`Direct SP+offset` stack address does not. This permits conservative IR
tracking without confusing an alloca's address with pointer data stored in the
alloca.

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

The first ArgsPointerMaps phase supports scalar LLVM pointer inputs and
receivers in ABIInternal register homes, ABIInternal stack-input slots, and
ABI0 stack-input slots on AArch64. Pair 0 is always
`(EntryArgs, empty locals)`. Ordinary
statepoints classify their post-prologue indirect stack roots as locals and
jointly deduplicate the complete `(Args, locals)` pair; the Args and Locals
tables therefore always have the same count. A direct stack address is not a
bitmap root.

This phase fails closed for unsupported pointer-containing aggregate layouts
and aggregates live at an ordinary statepoint; dynamic or realigned frames;
raw register roots; pointer vectors or ABI layouts whose pointer words do not
map to fixed homes; and an ordinary statepoint path that would recover a
pointer from an unadjusted original argument home. `FUNCDATA_StackObjects` and
`PCDATA_ArgLiveIndex` are not implemented. Tracking an alloca address across a
statepoint does not describe pointer fields stored inside that object, and
ArgLive is not a replacement for the entry argument bitmap. Precise handling
of pointer-containing address-taken stack storage belongs to the separate
StackObjects work rather than this ArgsPointerMaps change.

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

The installed file is `lib/GoALLCStatepoints.dylib` on Darwin or
`lib/GoALLCStatepoints.so` on Linux. Do not build the plugin against a different
LLVM installation and copy it into the payload.
