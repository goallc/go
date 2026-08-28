# GoALLC LLVM pre-codegen plugin

This directory owns the GoALLC-specific LLVM pass pipeline. It deliberately
lives in the Go repository: LLVM only supplies the generic `llc`
`-load-pass-plugin` and pre-codegen callback mechanism.

`GoALLCStatepointPlugin.cpp` adapts that callback to
`goallc::runPreCodeGenPipeline`. GoALLC transformations belong in
`GoALLCPreCodeGen.cpp` or other sources in this directory, not in the LLVM
repository. The core entry point remains separate from the plugin adapter.
The default `cmd/compile -enablellvm` path invokes the same pre-codegen callback
in-process before constructing LLVM code generation. Dynamic LLVM builds load
the module from the selected payload; static LLVM builds call the linked
implementation directly. The retained `llvmtoolexec` path reaches it through
`llc`.

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
After all calls have been rewritten, the pass models each ordinary pointer and
all of its relocates as definitions of a temporary promotable alloca. Loads at
the old uses make the required reaching definition explicit, and LLVM's public
`PromoteMemToReg` utility removes the temporary memory operations and forms the
relocation PHIs. This applies the same SSA construction to straight-line code,
conditional paths, loop backedges, and irreducible control flow.
Raw static `alloca` values are native frame identities, not Go heap pointers.
Pointer-containing allocas are explicit `gc-live` values while their storage
is active so the adjacent alloca ptrmap describes their contents. Direct
loads, stores, lifetime markers, and address derivations retain that identity.
An observable use of an alloca-derived address is rebuilt in its use block.
Direct loads and stores of the raw alloca identity stay on the current
FrameIndex; putting a concrete stack address through whole-function relocation
SSA could spill a cached pre-growth value into an ordinary non-root slot.
PHI/select/freeze and aggregate-leaf forwarding whose every path denotes the
same alloca is normalized to integer offset SSA. Only that offset may cross a
statepoint, and each concrete address is reconstructed as `Base + Offset` in
the consuming block. Different allocas and every other ambiguous provenance
remain ordinary scalar roots.
Typed Go `byval` and `goret` parameters follow the same fixed-frame-address
rule: their raw argument is the storage identity, while SelectionDAG selects
its canonical argument FrameIndex at each use. Their pointer-containing object
layouts independently contribute to `ArgsPointerMaps` only while the contents
are live. Pointer values loaded from any of these homes remain ordinary tracked
roots.

The current SSA value and CFG rewrite support matrix is:

| Value or control-flow shape | Status | Current contract |
| --- | --- | --- |
| Pointer arguments | AArch64 GoObj qualified; SelectionDAG home reuse also tested on X86 | Values live after a call use caller statepoints; exact stack inputs stay in their fixed ABI homes, while register inputs and transformed values use normal statepoint spills. Call-only arguments are described by the callee's type-derived entry map. |
| Static `alloca` addresses | Supported | The raw alloca is the storage identity and the only extra `gc-live` root. It deliberately has no `gc.relocate`. First-class GEP/cast uses are rebuilt immediately at the use; direct memory uses stay on the FrameIndex and never enter relocation SSA. Pointers loaded from memory remain tracked. |
| Typed `byval`/`goret` addresses | Supported | The raw parameter is a fixed ABI-frame identity and may appear directly in `gc-live`, but deliberately has no `gc.relocate`. First-class uses are rebuilt in each consuming block and SelectionDAG selects the canonical argument FrameIndex before consulting an entry-block export vreg. A separate variable-granularity content interval expands the typed layout into `ArgsPointerMaps`: byval uses backward memory liveness; goret is inactive before definite initialization and stays active through later overwrites until return. |
| `select`, GEP, and pointer casts | Supported | Same-object fixed-frame expressions are represented as one canonical base plus integer offsets. Offset PHIs/selects may cross statepoints, while concrete addresses are rebuilt in each consuming block. Mixed bases, address-space changes, and non-stack pointers remain ordinary scalar roots. |
| Pointer-valued call results | Supported | `gc.result` replaces the ordinary result and later safepoints relocate it. |
| Multiple ordinary calls | Supported | Stable IDs and live sets remain per call; the next statepoint consumes the current relocated SSA value. |
| Ordinary CFG merges | Supported | Call/skip and multiple-safepoint paths merge through scalar-pointer or whole-aggregate PHIs formed by `PromoteMemToReg`. |
| Loops and irreducible CFG | Supported | Relocation definitions are propagated through backedge and multi-entry PHIs without a shape-specific algorithm. |
| Fixed struct/array SSA aggregates | Supported | Whole aggregate SSA values participate in ordinary liveness. At each concrete safepoint, initialized pointer and fixed pointer-vector leaves are resolved from exact `insertvalue` provenance or a call-local `extractvalue`. Same-object fixed-frame leaves carry integer offsets and are reconstructed from the raw alloca/byval/goret base; ordinary leaves use their current relocated SSA values. Nested paths and aggregate PHI/select/freeze forwarding are resolved structurally and fail closed for mixed bases. |
| Fixed-width pointer-vector SSA values | Supported | The vector remains one `gc-live` operand and one same-typed `gc.relocate`; it is not split into lanes. Pointer vectors in allocas remain unsupported. |
| Aggregate arguments and call results | Supported for IR rewriting | The wrapped call keeps its real aggregate ABI type. Only leaves live after the call enter caller `gc-live`; supported fixed formal layouts also contribute pointer words to AArch64 entry maps. |
| Aggregate load results and store operands | Supported | First-class SSA values use whole-value relocation SSA; a post-statepoint store consumes the aggregate with its live pointer leaves updated. Pointer leaves in surviving fixed allocas remain memory roots described by the alloca deopt layout protocol. |
| Pointer-containing `alloca` storage | GoObj qualified for fixed layouts | Go `VarDef` emits `llvm.lifetime.start`; parameter homes and addressed result homes have explicit starts at their initialization sites. The statepoint pass uses starts as backward liveness kills and real address uses as gens, so the last use supplies the implicit end. Active contents contribute callsite `LocalsPointerMaps`; address-observable objects additionally get function-wide `FUNCDATA_StackObjects`. If the optimized producer does not definitely initialize every pointer slot before the next safepoint, the plugin inserts an inline zero initialization at that lifetime start. It preserves existing lifetime markers and emits no new lifetime ends. |
| Scalable vectors | Unsupported | The generic LLVM statepoint rewrite assumes a fixed vector width when constructing relocates; fails closed. |
| General moving-GC base/derived analysis | Unsupported | Base and derived indexes are identical in the current non-moving-heap phase. |
| `invoke`, `callbr`, non-deopt operand bundles, and unsupported parameter attributes | Unsupported | One ordinary deopt bundle is preserved before the alloca suffix. `nest`, `captures`, and `readonly` parameter attributes are preserved; other shapes fail closed. |
| Frontend-qualified `musttail` | Not a caller safepoint | A tail transfer has no caller continuation to relocate. The Go frontend currently emits only direct `void()` transfers; LLVM target lowering validates that initial frame-reuse shape. |

This matrix describes verified IR rewriting, not full runtime qualification.
The Darwin/arm64 Go execution whitelist additionally covers one unconditional
`runtime.GC`, a conditional call/skip merge, two branch-local GC safepoints,
and a nested conditional GC. Sequential safepoints in one activation, loop
backedges, multiple simultaneously live pointers, and pointer-valued call
results have IR and verifier coverage, but are not yet runtime-qualified:
focused execution trials currently expose invalid post-call values or
traceback/stack-map failures in those shapes.

The statepoint pass computes one ordinary SSA liveness relation for scalar
pointers and pointer-containing aggregates. Both kinds are sampled strictly
after the current call: the caller statepoint describes values live across that
call, while values used only as call arguments belong to the callee's
`FUNCDATA_ArgsPointerMaps`. A live aggregate remains one SSA value in this
analysis. At each concrete safepoint, the pass finds its initialized pointer
leaves. Exact `insertvalue` provenance or an already-dominating projection is
reused only when that scalar was independently live at this safepoint (or is
covered by the later fixed-frame/derived analysis), so sharing cannot silently
extend a scalar live range across an unrecorded call. Thus a pointer that is
also independently live remains one root. Every other dynamic leaf gets a
fresh call-local `extractvalue` from the current aggregate definition. A
PHI/select whose incoming aggregates all provide the same exact leaf follows
the same reuse rule.

After the statepoint, the aggregate is updated by inserting each relocated
pointer leaf into the original aggregate value. This is a field update rather
than a full reconstruction: untouched scalar fields, `undef`/`poison` choices,
and padding retain their original semantics. The original aggregate is allowed
to be the syntactic base of the `insertvalue` chain, but every initialized
movable pointer field is overwritten before the updated aggregate has any
ordinary use. X86 and AArch64 pre-RA MIR tests verify that the old pointer
component is not kept as an additional statepoint root or post-call machine
value.

This relocation model has four invariants:

1. `gc-live` contains relocatable scalar pointer values or fixed-width pointer
   vectors, never an aggregate.
2. Every initialized movable pointer leaf of a live aggregate is represented
   by one `gc-live` identity at that safepoint; an independently live direct
   scalar alias is deduplicated by SSA identity.
3. Every such leaf is overwritten with its relocated definition before the
   updated aggregate has an ordinary post-statepoint use. Constant
   `undef`/`poison`/null leaves do not become roots.
4. Scalar pointers, rematerialized derived addresses, and updated aggregates
   all enter the same temporary-slot plus `PromoteMemToReg` repair. The
   resulting pointer or whole-value PHIs handle conditionals, loops, and
   irreducible CFG without shape-specific replacement logic.

Nested fixed structs and arrays are enumerated by leaf index path. A fixed-width
pointer vector is one leaf: LLVM's statepoint verifier and rewrite preserve its
type in `gc-live` and `gc.relocate`, so there is no lane-wise scalarization.
Extracting after an aggregate `freeze` preserves its correlated choice, and
using the original aggregate as the update base preserves `undef` and `poison`
field semantics without manufacturing roots for uninitialized leaves. LLVM
struct padding is not a first-class leaf. LLVM 23 permits an aggregate
`gc.result`, so its pointer leaves become roots at later safepoints and the
field-updated result participates in ordinary relocation SSA.

An aggregate loaded from memory can be tracked as an independent SSA value,
and an aggregate store can consume the field-updated value. For each surviving
fixed pointer-containing alloca, the plugin first inspects the optimized LLVM use
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

For every surviving fixed alloca, statepoint rewriting enumerates pointer
offsets and performs a backward, path-sensitive live-out dataflow over its
optimized address-use graph. A lifetime start is a kill and a real load, store,
call use, comparison, `llvm.fake.use`, or other terminal address use is a gen.
The callsite is sampled before applying that call's use transfer, so an address
used only as a current call argument is live-in but not caller `gc-live`.
Direct GEP/cast chains retain the alloca base. Same-allocation
PHI/select/freeze and aggregate-leaf forwarding are canonicalized to integer
offset SSA and use-block-local `Base + Offset` recipes; mixed-base results
remain independent scalar roots and do not make every possible incoming
alloca live after the merge. This makes the final real use the implicit
lifetime end
without modifying IR. Unclassifiable objects fail closed. The original
lifetime starts remain in IR for the normal code-generation pipeline.
Address-observable objects are different: Go's runtime adjusts every pointer
word in every `StackObjects` record during stack growth, whether the object is
source-live or not. The plugin preserves their function-wide frame identity
with `llvm.stackcoloring.no_merge`, while the existing lifetime interval still
controls when their contents are roots. Any interval whose optimized producer
does not initialize all pointer slots before a safepoint receives an inline
zero initialization immediately after its start. LLVM may hoist a pure
first-class address use before its source `lifetime.start`; content liveness can
then name that storage at an earlier safepoint. For this case, the original
starts first remain liveness kills, then the physical alloca is widened to one
entry lifetime and zeroed there. Address recipes still use the ordinary
per-block `Base + Offset` rematerialization. The record carries one generic
tag, the alloca address, whole-object size and alignment, pointer size, valid
bitmap bit count, and 64-bit bitmap words. The
envelope and every record carry explicit lengths, and a trailing duplicate
envelope length makes the suffix recoverable after ordinary deopt operands.
There is deliberately no contract version in the first grammar.

LowerStatepoint lowers both the deopt alloca address and explicit `gc-live`
alloca through their normal direct FrameIndex paths and preserves the adjacent
constants in Machine StackMaps. For `gc "goallc"`, a static alloca used only
as a deopt layout carrier is not implicitly promoted to a GC root; explicit
`gc-live` is the activity signal. The plugin emits no `gc.relocate` for that
alloca address. Uses after the statepoint rebuild from the original alloca, so
SelectionDAG selects the same FrameIndex without a root spill or reload.
LLVM's GoObj StackMaps bridge retains the deopt prefix and resolves
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

1. **Optimized alloca classification.** Enumerate pointer leaves and classify
   final IR uses as address-observable stack objects or direct-only locals.
2. **Unified SSA liveness and leaf discovery.** Track scalar pointers and
   pointer-containing aggregate SSA values together. For every live aggregate
   at a concrete call, reuse an exact pointer source only when its existing
   live range already covers that call; otherwise create call-local extracts
   for its initialized movable leaves.
3. **Fixed-frame canonicalization, object activity, and statepoint emission.**
   Normalize same-object alloca/byval/goret pointer expressions (including
   nested aggregate leaves and different-offset merges) to one canonical base
   plus integer offset SSA. Compute callsite object activity from frontend
   lifetime starts and terminal address uses, inserting lifetime-local zero
   initialization only when the optimized producer cannot initialize every
   pointer slot before a safepoint. Put the live base and ordinary scalar roots in
   `gc-live`, append object-content ptrmap records to deopt, and emit
   `gc.result`. After splitting statepoint continuations, rebuild each concrete
   fixed address in its consuming block. Emit `gc.relocate` only for ordinary
   roots, never for a fixed-frame identity.
4. **Unified relocation SSA.** Insert relocated leaves into each original
   aggregate base, then rebuild live GEP/cast address chains from each ordinary
   relocatable base. Model updated aggregates, rematerialized addresses, and
   ordinary scalar relocates as stores to temporary promotable allocas, rewrite
   old uses through loads, and call the public `PromoteMemToReg` utility. This
   constructs conditional, loop/backedge, and irreducible-CFG PHIs while
   preserving raw alloca identity for direct memory, lifetime, deopt,
   StackObjects, and content pointer maps.

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

The Go-owned metadata printer is the boundary between Machine StackMaps and
GoObj. It retains deopt and GC locations in LLVM's `MCContext` and resolves
StackMaps constant-pool indexes without adding a parallel serializer; LLVM's
GoObj writer remains responsible for final object-format serialization. LLVM
records GoObj statepoint callsites at the CALL
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
GoObj emits `PCDATA_UnsafePoint` first and the statepoint-derived
`PCDATA_StackMapIndex` second, as required by their Go ABI indexes 0 and 1.
Asynchronous preemption is safe by default for ordinary optimized Go machine
code, including frames, calls, vectors, atomics, and pointer/integer
conversions. The frontend retains `go-async-unsafe` as a fail-closed fallback,
but a Go-owned read-only callback overrides it after final machine lowering.
The callback recognizes the complete write-barrier protocol in optimized IR,
then marks its final machine blocks from the completed flag load through the
raw heap write. It also marks target-inserted stack checks through the
`morestack` call, inline-assembly spans, and the true live range of arm64
R27/REGTMP. Runtime, reflect, and nosplit functions remain whole-function
unsafe; an unrecognized protocol or target also falls back to that state.
LLVM owns only the generic callback and label-to-PC serialization boundary. It
coalesces adjacent marked `MachineInstr` spans into `PCDATA_UnsafePoint` ranges
after final layout, including all instructions emitted by an AsmPrinter pseudo
expansion. No unsafe-point marker is introduced into LLVM IR, so ordinary IR
and machine optimization pipelines are unchanged.
The GoObj writer interprets locations after final layout. An
`Indirect [SP+offset]` location in the current frame contributes a locals
pointer bit; one in the post-prologue caller-owned argument/result area
contributes an args pointer bit. A `Direct SP+offset` stack address contributes
to neither bitmap as an ordinary GC root because the address itself, rather
than the slot contents, is the pointer. Inside a validated alloca deopt record,
the same direct location is instead the frame base whose bitmap selects memory
slots. Its contents are active only when a matching explicit direct alloca
also occurs in the statepoint's GC operands.

Typed `byval` and `goret` parameters describe the address of a complete fixed
Go ABI home, not a movable heap pointer. The statepoint pass keeps that raw
base as the direct GC operand but emits no `gc.relocate`; first-class derived
addresses are rebuilt at their concrete use. SelectionDAG then resolves every
raw base use from the canonical argument FrameIndex before consulting an
entry-block export vreg, so a statepoint cannot leave a cached pre-growth stack
address live into its continuation. This rule is gated by the Go calling
conventions and does not change ordinary ABI lowering. Pointer-containing
fixed arguments also carry the same self-describing deopt layout used for
allocas. Byval inputs use backward memory liveness over the optimized loads,
stores, memory intrinsics, and escaping address uses; a definite overwrite
kills the previous input contents. Goret outputs instead use forward definite
initialization: they contribute no contents before every pointer slot has been
initialized on all incoming paths, then remain active through later overwrites
until return. At the current variable granularity, a non-constant or merged
offset makes a read use the complete pointer layout, while such a write proves
no initialized or overwritten slot. Constant direct accesses retain slot
precision only to recognize complete initialization. An active object
contributes its complete typed bitmap. Results reachable through defer recovery
are conservatively live at every call. GoObj classifies these direct fixed
ranges as argument/result storage and expands active layouts into
`ArgsPointerMaps`; the direct address itself is never a bitmap pointer word.

Ordinary pointer values loaded from stack inputs use the normal statepoint
path. SelectionDAG formal
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
The Go codegen fixtures `test/codegen/llvm_args_pointer_maps.go` and
`test/codegen/llvm_alloca_statepoint.go` check the frontend's typed stack homes
and pointer-containing allocas. The executable `test/abi/llvm_args_results.go`
and `test/llvm_alloca_statepoint_gc.go` fixtures exercise those contracts across
stack growth and `runtime.GC`. Exact rewritten IR, MIR, Args/Locals pointer
maps, and `PCDATA_StackMapIndex` remain fixture-local checks in this plugin's
test suite.

The installed file is `lib/GoALLCStatepoints.dylib` on Darwin or
`lib/GoALLCStatepoints.so` on Linux. Do not build the plugin against a different
LLVM installation and copy it into the payload.
