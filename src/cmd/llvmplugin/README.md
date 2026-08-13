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
carry `gc "goallc"`, and only source-level exceptions to the native Go stack
policy need target attributes such as `go-nosplit` or `go-systemstack`. Loading
this plugin registers the named GC strategy and the no-op metadata-printer
marker required by AsmPrinter; the rewrite pass consumes the frontend markers
rather than adding them. GoObj stack-map adaptation lives in LLVM core.

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
Raw static `alloca` values are native frame identities, not Go heap pointers.
Pointer-containing allocas are explicit `gc-live` values while their storage
is active so the adjacent alloca ptrmap describes their contents. Direct
loads, stores, lifetime markers, and address derivations retain that identity.
An observable use of the alloca address itself is rebuilt immediately before
that first-class use. Direct loads and stores keep their original derived
address and SelectionDAG lowers it from the current FrameIndex; putting such an
address through whole-function relocation SSA could spill a cached pre-growth
stack address into an ordinary non-root slot. PHI/select values which cannot be
rebuilt from one alloca remain ordinary scalar roots. Pointer values loaded
from the allocation remain ordinary tracked roots.

The current SSA value and CFG rewrite support matrix is:

| Value or control-flow shape | Status | Current contract |
| --- | --- | --- |
| Pointer arguments | AArch64 GoObj qualified; SelectionDAG home reuse also tested on X86 | Values live after a call use caller statepoints; exact stack inputs stay in their fixed ABI homes, while register inputs and transformed values use normal statepoint spills. Call-only arguments are described by the callee's type-derived entry map. |
| Static `alloca` addresses | Supported | The raw alloca is the storage identity and the only extra `gc-live` root. First-class GEP/cast uses are rebuilt immediately at the use; direct memory uses stay on the FrameIndex and never enter relocation SSA. Pointers loaded from memory remain tracked. |
| `select`, GEP, and pointer casts | Supported | Fixed-allocation GEP/no-op cast chains stay as direct FrameIndex uses or are rebuilt immediately at first-class uses. Merged or non-stack pointer values remain ordinary scalar roots. |
| Pointer-valued call results | Supported | `gc.result` replaces the ordinary result and later safepoints relocate it. |
| Multiple ordinary calls | Supported | Stable IDs and live sets remain per call; the next statepoint consumes the current relocated SSA value. |
| Ordinary CFG merges | Supported | Call/skip and multiple-safepoint paths merge through pointer PHIs formed by `PromoteMemToReg`. |
| Loops and irreducible CFG | Supported | Relocation definitions are propagated through backedge and multi-entry PHIs without a shape-specific algorithm. |
| Fixed struct/array SSA aggregates | Supported | Pointer and fixed pointer-vector leaves are extracted before liveness and reconstructed from the current relocated SSA leaves. |
| Fixed-width pointer-vector SSA values | Supported | The vector remains one `gc-live` operand and one same-typed `gc.relocate`; it is not split into lanes. Pointer vectors in allocas remain unsupported. |
| Aggregate arguments and call results | Supported for IR rewriting | The wrapped call keeps its real aggregate ABI type. Only leaves live after the call enter caller `gc-live`; supported fixed formal layouts also contribute pointer words to AArch64 entry maps. |
| Aggregate load results and store operands | Supported | First-class SSA values use aggregate normalization. Pointer leaves in surviving fixed allocas remain memory roots described by the alloca deopt layout protocol. |
| Pointer-containing `alloca` storage | GoObj qualified for fixed layouts | Go `VarDef` emits `llvm.lifetime.start`; parameter homes and addressed result homes have explicit starts at their initialization sites. The statepoint pass uses starts as backward liveness kills and real address uses as gens, so the last use supplies the implicit end. Active contents contribute callsite `LocalsPointerMaps`; address-observable objects additionally get function-wide `FUNCDATA_StackObjects` and one entry initialization because stack growth adjusts them even while source-dead. Locals-only storage is not initialized by the plugin, and no lifetime ends are emitted. |
| Scalable vectors | Unsupported | The generic LLVM statepoint rewrite assumes a fixed vector width when constructing relocates; fails closed. |
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

Nested fixed structs and arrays are enumerated by leaf index path. A fixed-width
pointer vector is one leaf: LLVM's statepoint verifier and rewrite preserve its
type in `gc-live` and `gc.relocate`, so there is no lane-wise scalarization.
Extracting after an aggregate `freeze` preserves its correlated choice;
rebuilding all explicit leaves preserves `undef` and `poison` field semantics.
LLVM struct padding is not a first-class leaf. LLVM 23 permits an aggregate
`gc.result`, so an aggregate call result is projected first and its pointer
leaves become roots at later safepoints.

An aggregate loaded from memory can be normalized as an independent SSA value
and an aggregate store can consume a rebuilt value. For each surviving fixed
pointer-containing alloca, the plugin first inspects the optimized LLVM use
graph. Calls, stored or returned addresses, ptr-to-int, volatile/atomic access,
and unknown uses make the object address-observable. Direct load/store,
constant address derivation, and local PHI/select uses remain compiler
controlled. The frontend Addrtaken metadata is provenance only and never keeps
an alloca alive or overrides this final structural classification.

The Go frontend emits `llvm.lifetime.start` at each pointer-containing
`OpVarDef`. Repeated starts are intentional: `VarDef` denotes complete
reinitialization, including in branches and loops. Parameter homes start before
their synthetic incoming-value store, and addressed result homes start after
the producing call and before their result store. `OpVarLive` remains an
`llvm.fake.use` of the home. The producer's existing Zero, Store, or Move owns
source-lifetime initialization; the frontend marker helper adds no pointer
clears, and neither side emits `llvm.lifetime.end`.

For every surviving fixed object, statepoint rewriting enumerates pointer
offsets and performs a backward, path-sensitive live-out dataflow over its
optimized address-use graph. A lifetime start is a kill and a real load, store,
call use, comparison, `llvm.fake.use`, or other terminal address use is a gen.
The callsite is sampled before applying that call's use transfer, so an address
used only as a current call argument is live-in but not caller `gc-live`.
Direct GEP/cast chains retain the alloca base; PHI/select/freeze results become
independent scalar roots and do not make every possible incoming alloca live
after the merge. This makes the final real use the implicit lifetime end
without modifying IR. Unclassifiable objects fail closed. The original
lifetime starts remain in IR for the normal code-generation pipeline.
Address-observable objects are different: Go's runtime adjusts every pointer
word in every `StackObjects` record during stack growth, whether the object is
source-live or not. The plugin therefore zeroes only those pointer leaves once
at frame entry. This is independent of a later VarDef reinitialization. The
record carries one generic tag, the alloca address, whole-object size and
alignment, pointer size, valid bitmap bit count, and 64-bit bitmap words. The
envelope and every record carry explicit lengths, and a trailing duplicate
envelope length makes the suffix recoverable after ordinary deopt operands.
There is deliberately no contract version in the first grammar.

LowerStatepoint lowers both the deopt alloca address and explicit `gc-live`
alloca through their normal direct FrameIndex paths and preserves the adjacent
constants in Machine StackMaps. For `gc "goallc"`, a static alloca used only
as a deopt layout carrier is not implicitly promoted to a GC root; explicit
`gc-live` is the activity signal. The resulting `gc.relocate(alloca)` is
`NoRelocate` and rematerializes the same frame index, so no root spill is
created. LLVM's GoObj StackMaps bridge retains the deopt prefix and resolves
both inline constants and `ConstantIndex` values. The GoObj writer strictly
parses the suffix and maps every layout with a matching direct `gc-live` alloca
plus bitmap bit to that callsite's `LocalsPointerMaps`. An unmatched layout is
a function-level StackObject candidate; the writer requires that same layout
at every ordinary statepoint before emitting native-layout
`FUNCDATA_StackObjects` plus content-addressed GC bitmaps. A record may
therefore contribute locals bits at active callsites and still establish a
function-level StackObject at an inactive callsite. The writer fails closed on
malformed lengths, non-direct bases, incomplete function-level layouts,
nonzero padding, duplicates, overlaps, or locations outside the GC locals
range. The deopt grammar does not escape the object writer.

Because a StackObject record describes one function-wide frame object, the
plugin marks that alloca with `llvm.stackcoloring.no_merge`. Stack coloring
keeps that object's frame identity while remaining enabled for unrelated
locals.

The alloca memory remains the source of truth while the call executes. A
callee may clear or replace fields through an address argument, and the caller
observes those changes because no relocated pre-call SSA value is written back.

StackObjects provide native object identity and layout, but do not themselves
make an object live. The lifetime-specific Locals bits retain active pointer
contents, while inactive calls omit them. Constants are not roots in the
general SSA model. Alloca records describe memory layout even when a slot
contains null. Dynamic, multiple-element, scalable, or realigned allocas and
pointer vectors still fail closed.

LLVM's generic `RewriteStatepointsForGC` pass is the design reference for
liveness and relocation SSA formation. GoALLC does not run it directly:
that pass also owns base-pointer inference, EH/invoke rewriting, deopt bundles,
and module-wide attribute cleanup. Those policies exceed the deliberately
narrow Go ABI contract above.

The final combined order is:

1. **Optimized alloca classification and activity.** Enumerate pointer leaves, classify
   final IR uses as address-observable stack objects or direct-only locals,
   then compute callsite live-out from frontend lifetime starts and terminal
   address uses. Address-observable objects alone receive function-entry
   pointer initialization.
2. **Aggregate normalization.** Use aggregate-only liveness to find and
   decompose supported live first-class struct/array values, then rebuild
   aggregates immediately before their uses.
3. **Scalar statepoint insertion.** Split observable direct alloca-address uses
   from the raw storage identity, compute scalar and derived-address liveness,
   put each active base alloca and ordinary scalar root in `gc-live`, append the
   unchanged alloca ptrmap records to deopt, and emit `gc.result` plus
   `gc.relocate`.
4. **Whole-function relocation SSA.** Rebuild live GEP/cast address chains from
   each base-allocation relocate. Model those definitions and ordinary scalar
   relocates as stores to temporary promotable allocas, rewrite old uses through
   loads, and call the public `PromoteMemToReg` utility. This constructs
   conditional, loop/backedge, and irreducible-CFG PHIs while preserving raw
   alloca identity for direct memory, lifetime, deopt, StackObjects, and content
   pointer maps.

Future stages remain:

1. **Heap base/derived pointers.** Adapt the scalar GEP/cast path of LLVM's
   `findBasePointer` if a future moving heap collector needs base identity;
   the current rematerialization path is deliberately limited to fixed stack
   allocations.
2. **Additional call shapes only when required by Go.** Do not import generic
   deopt, transition-bundle, invoke/EH edge normalization, or module-wide
   attribute stripping merely because the LLVM pass supports them.

Most useful LLVM implementation helpers are private to
`RewriteStatepointsForGC.cpp`, so GoALLC will reuse their algorithms through
public LLVM IR and utility APIs rather than depending on private symbols. If a
non-trivial implementation block is copied verbatim, it must live in a
separately attributed source file retaining LLVM's Apache-2.0-with-exception
notice; BSD-only Go source files should not silently absorb copied code.

The project-owned LLVM GoObj AsmPrinter path is the boundary between Machine
StackMaps and GoObj. It retains deopt and GC locations in `MCContext` and
resolves StackMaps constant-pool indexes without adding a parallel serializer;
the Go-owned plugin remains responsible for the frontend statepoint rewrite,
not object-format adaptation. LLVM records GoObj statepoint callsites at the CALL
start, matching Go's `PCDATA_StackMapIndex` convention without a command-line
mode. GoObj Go functions use native Go's split-stack policy by default: unless
`go-nosplit` is present, target frame lowering expresses the late-generated
`runtime.morestack` call as an ordinary ABI0 MIR call. Before frame allocation,
formal lowering maps each live type-derived input pointer word onto the existing
fixed home reserved for that ABI input. It still reserves and saves complete ABI
homes for an unused formal so a morestack retry preserves the register
assignment, but does not scan a word that LLVM callers may replace with poison.
Frame lowering records the live homes in a separate zero-byte
`EntryArgsStackMapID` record for every GoObj Go function. This is function
metadata rather than a callsite. The GoObj writer uses it as pair 0: non-empty
`EntryArgs` when present and empty locals, and initializes
`PCDATA_StackMapIndex` to 0. AsmPrinter's PCSP stream has already resolved the
Machine CFG; every transition back to the entry stack depth selects map 0. This
covers the pre-frame morestack path without identifying `runtime.morestack` by
name or manufacturing a statepoint. An ordinary statepoint selects its actual
live map and overrides a same-PC entry-depth transition.
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
slots. Its contents are active only when a matching explicit direct alloca
also occurs in the statepoint's GC operands.

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
non-default-address-space pointers, pointer vectors, and any deopt record that
does not resolve to one unique direct frame object at offset zero.
`PCDATA_ArgLiveIndex` is not implemented. Address-observable allocas emit
`FUNCDATA_StackObjects` and are also represented conservatively in
`LocalsPointerMaps`; direct-only pointer leaves use locals-only records. A
merged alloca address is an ordinary pointer root rather than object contents.
ArgLive is not a replacement for the entry argument bitmap.

TODO: LLVM opaque pointer types currently make the first itab/type word of a
Go interface and pointers to `NotInHeap` types look like ordinary GC pointers.
The type-derived entry map conservatively marks those words for now. A later
change should give non-GC address words a distinct IR representation or a
signature-level GC-shape attribute.

The normal GoALLC build does not require a separate plugin install step.
`make.bash -llvm-dir=...` builds this directory against that payload's CMake
package and atomically installs the module. Build Go through its normal entry
point; `doc/goallc-build.md` documents how to prepare the LLVM payload first:

```sh
cd "$GOROOT/src"
./make.bash \
  -llvm-dir=/path/to/goallc-llvm \
  -llvm-version=23 \
  -llvm-link=dynamic
```

For standalone plugin development, build, test, and install it into the LLVM
payload that contains `llc`:

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
assembly stack loads, scalar-only rewritten `gc-live`/`gc.relocate`, alloca
memory roots with no synthetic statepoint spill, and exact Args/Locals/PCDATA
objview data.
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
