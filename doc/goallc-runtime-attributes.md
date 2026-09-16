# Compiler-known runtime LLVM contracts

The registry in `src/cmd/compile/internal/ssa/llvmfunctions.go` binds contracts
by exact IR name, with independent function properties and precreated LLVM
parameter and result attributes. It does not classify names by prefix at lookup time.

## Audit scope

The current audit covers the 287 distinct function names declared in
`src/cmd/compile/internal/typecheck/_builtin/runtime.go` or referenced by literal
`LookupRuntime`/`LookupRuntimeFunc` calls across `src/cmd/compile/internal`, plus the
function entries in `src/cmd/internal/goobj/mkbuiltin.go` (excluding the TLS variable).
155 have an explicit model; the remaining 132 are listed below.
Also modeled are `Goexit`, `panicmem`, `panicmemAddr`, `throw`, plus the existing
`os.Exit` and testing termination methods. The fourteen generated `mallocgcSmallNoScanSC1..7` and
`mallocgcSmallScanNoHeaderSC1..7` entries also have allocation models.
LLVM intrinsics and the private write-barrier record intrinsic retain their
separate lowering contracts.

## Added contracts and limits

| Implementations | Contracts |
| --- | --- |
| Raw assembly copy, zero, equality, string comparison and Duff devices | GC leaf, `nofree`, `nocallback`, `nounwind`; direct source/destination pointer operands have `captures(none)` with `readonly`/`writeonly`. |
| Fixed-size, float, complex and string equality in `runtime/alg.go` | `nounwind`; two read-only, non-capturing pointers. |
| Concrete memory, string, float and complex hashes in `runtime/alg.go`, including `memhash_varlen` | `nounwind`; one read-only, non-capturing pointer. Audit follows `internal/runtime/maps/memhash_*.go` and `runtime_hash*.go`. |
| Rune decoding/counting, complex division and scalar/soft-float conversions | `nounwind`; aggregate/scalar parameters get no pointer attributes. |
| `rand` and `rand32` | `nounwind`; random state remains mutable and observable. |
| `chanlen`, `chancap` | `nounwind`; read-only, non-capturing channel header pointer, including nil/timer-channel handling. |
| `selectsetpc` | `nounwind`; write-only, non-capturing output pointer. |
| Typed copy/clear, pointer-containing clear, cgo pointer checks, write-barrier entry points | GC leaf and `nounwind`. Audit follows `mbarrier.go`, `mbitmap.go`, `mwbbuf.go`, `cgocheck.go` and assembly entry points. Their slow paths operate on the system stack without a goroutine safe point. |
| Panic helpers, bounds failures, signal panic, `throw`, `block`, Goexit and existing termination APIs | `noreturn`. Recovery resumes an older frame; the helper does not return to its call site. No callback-free or GC-leaf promise follows. |

Function contracts other than memory-effect restrictions apply to both ABIInternal and ABI0 entries. Their NOSPLIT
ABI wrappers preserve these contracts. Parameter attributes only apply to
ABIInternal: ABI0 byval parameters denote argument slots, not the original
pointees, and return homes are additional parameters.

Memory effects describe the logical operation visible to callers. Stack growth,
stack-map processing and pointer relocation are implemented by the Go calling
convention and statepoints. They do not by themselves turn a logical read-only
operation into a writer. These contracts apply to both declarations and
runtime definitions: definition creation uses the same function manager before
adding basic blocks.

Concrete equality, deterministic memory/string hashes, rune scanning and channel
queries have `memory(read)` on ABIInternal entries. Pure numeric/soft-float
helpers and zero-width equality/hash helpers have `memory(none)`. These finite
operations also have `willreturn`. Reads are not restricted to argument memory:
hash seeds, CPU flags, string payloads and closure contexts must remain visible.
The three raw assembly comparisons additionally have `nosync`.

Go helpers remain non-leaf unless independently justified. The statepoint pass
does not use `memory(read)` or `memory(none)` as evidence of GC leaf. Surviving
calls still relocate live pointers, and the rewrite does not copy these memory
restrictions onto the statepoint intrinsic. Optimization tests verify repeated
call elimination and dead-call removal, while a backend regression verifies
that surviving hash/numeric calls still produce statepoints and relocations.

ABI0 wrappers keep unrestricted memory effects because their result homes are
caller-owned memory that must be written. Logical `willreturn` remains valid
for those wrappers.

Other helpers retain unrestricted memory effects: arm64 zeroing updates a
cache, floating-point and complex hashes update random state for NaNs, and bulk
barriers update GC metadata. Barrier destinations are not `writeonly`: old
pointers must be read before overwriting them.

ABIInternal allocation and boxing results have `nonnull`: successful calls
return either allocated storage or a non-null static object, including zerobase.
`convT16`, `convT32` and `convT64` additionally return `dereferenceable(2/4/8)`.
Their static caches preclude a general fresh-allocation/noalias contract.
`mallocgc` has `allocsize(0)`, describing its byte-size parameter without
promising clearing, purity or allocation elision. The nullable type argument is
unchanged. `newobject` and slice allocation sizes depend on type metadata and
cannot be expressed by an `allocsize` parameter index.

`cmpstring` returns `range(-1, 2)`. `chanlen`, `chancap` and `countrunes` return
nonnegative Go ints. Range attributes are precreated at 32 and 64 bits and bound
using the actual IR return width. ABI0 returns through slots and receives none
of these return or allocation-size attributes. Non-leaf allocations still
require statepoints.

### Conditional allocation calls

A direct ABIInternal `mallocgc`, `mallocgcTinySC2`, or modeled size-class
allocation call with a constant nonzero byte size receives
return `noalias`, `allockind("alloc")`, and `"alloc-family"="runtime.mallocgc"`.
If `needzero` is a constant true value, the call instead receives
`allockind("alloc,zeroed")`. Unknown or false zeroing flags do not promise either
zeroed or uninitialized contents. These attributes are precreated and attached
only to the call; the shared function declaration and runtime definition retain
their unconditional contracts. The tiny and fourteen size-class entries share
`nonnull` and `allocsize(0)`, and are registered by exact name.

A `newobject` call whose type argument is an SSA `OpAddr` of a static type symbol
with a known nonzero `TypeInfo.Type.Size()` receives return `noalias`,
`dereferenceable(N)` for that size, and `allockind("alloc,zeroed")`. The
size-dependent LLVM attributes are cached by size and reused across calls. This reuses the type metadata used by Go's fixed-load
rewriting, rather than guessing the allocation size from a pointer result type.
Zero-sized or dynamic type descriptors remain unannotated. The runtime
signature is unchanged; `allocsize` cannot describe a size loaded from type
metadata, so no allocsize attribute is attached to newobject.

LLVM may elide such allocations even though the allocator updates runtime
metadata. Optimizer regressions cover unused allocations, zero-load folding,
and independent memory accesses; surviving calls retain their statepoints.
The executable regression keeps adjacent tiny allocations alive across GC and
checks their independent contents. Sharing a physical tiny block does not make
non-overlapping logical allocations alias.

Zero-byte and dynamic-size calls are currently excluded. An LLVM experiment
folded equality of two marked zero-byte allocation results to false, whereas
these concrete runtime calls both return zerobase. This does not settle Go's
freedom to compare pointers to distinct zero-sized variables; the current
runtime-call IR contract is preserved until that distinction is modeled.
Shared scalar/empty boxing caches and conversions returning input/scratch
storage are not marked as fresh allocations. In an executable LLVM probe,
marking a shared-cache helper as noalias plus allockind changed its address
comparison from true to false after O2. Modeling immutable boxes as independent
logical objects would require a matching representation and address-observation
contract, not just attributes on the existing shared-pointer implementation.

The LLVM definitions of these attributes are in the
[LLVM language reference](https://llvm.org/docs/LangRef.html#function-attributes).
`nounwind` does not mean that a function has no runtime side effects or that
asynchronous faults cannot occur.

### Container and boolean results

`makechan` and `makechan64` return non-null channel headers even for zero
capacity. `makemap`, `makemap64` and `makemap_small` return non-null map headers;
`makemap` may reuse the caller's header, so it is not a noalias allocator.
`mapaccess1` and its fast32/fast64/faststr variants return a slot or the shared
`zeroVal` on a miss, never null. `mapassign` and its five fast variants return
existing or newly allocated slots. All receive return `nonnull`, without
restrictions on memory, synchronization, callbacks or termination.
The `return nil` after `fatal` in fast map access is unreachable.
`mapaccess1_fat` is excluded: its miss result is a caller-provided zero pointer.
`makechan`, `makechan64`, and `makemap_small` additionally have return
`noalias`: each creates a nonzero fresh header, including unbuffered or
zero-element channels. This does not promise allocation-only effects or
elidability. `makemap`/`makemap64` can reuse a supplied header and remain
unrestricted for aliasing.
The mandatory `assertE2I` similarly returns a non-null itab or panics;
`assertE2I2` and `typeAssert` can legitimately return null.

Scalar bools in register arguments/results now use LLVM `i1`, including tuple
result components. They need no per-function range model. Storage, aggregate
fields, ABI0 slots and register-exhaustion stack slots retain Go's byte layout.
The existing Go backend lowers the register carrier to the native Go ABI.
Dynamic equality can still panic and channel operations can synchronize or
block; their scalar type does not imply memory or termination attributes.

### Remaining opportunities requiring caller facts

The census includes literal lookups throughout the compiler, algorithm helper
selection in reflectdata, builtin declarations and GoObj extras. The fourteen
size-class names selected by formatted lookups are explicitly modeled above.
Existing modeled functions were also reviewed for stronger contracts, not just
previously unmodeled names.

Further extents for slice allocation or boxing need the element type, a known
length, or target layout at the call. No positive extent applies uniformly to
zero-sized allocations. `slicecopy` and `typedslicecopy` cannot unconditionally
promise a nonnegative result based on their bodies: negative raw length
arguments can yield a negative result. Such facts need caller preconditions.
Pointer parameter `nonnull`/`dereferenceable` cannot be inferred merely from a
load if doing so would remove a Go nil panic. Cgo failure diagnostics observe
pointer addresses, so successful-path behavior alone does not prove
`captures(none)`. Map hash/equality dispatch, sanitizer hooks and panic defers
also prevent a blanket memory or callback contract for the remaining helpers.

## Entries left without a new model

Each row lists exact runtime names (with the `runtime.` prefix omitted).
These exclusions preserve conservative behavior where the audited fast path
does not establish a contract for the full implementation and build modes.
They are not a claim that no more precise contract could ever be proved.

### Liveness

`KeepAlive` has a dedicated compiler liveness contract and a source fallback that references printing. Do not substitute memory-effect or capture assumptions for liveness.

`KeepAlive`.

### Coverage registration

`addCovMeta` forwards to `internal/coverage/rtcov.AddMeta`, which retains the metadata pointer and updates global registration structures.

`addCovMeta`.

### Instrumentation

Race, MSan, ASan and fuzzing hooks call external runtimes and can have callbacks and address-observation effects. No common leaf or capture contract is assumed across build modes.

`asanread`, `asanregisterglobals`, `asanwrite`, `libfuzzerHookEqualFold`, `libfuzzerHookStrCmp`, `libfuzzerTraceCmp1`, `libfuzzerTraceCmp2`, `libfuzzerTraceCmp4`, `libfuzzerTraceCmp8`, `libfuzzerTraceConstCmp1`, `libfuzzerTraceConstCmp2`, `libfuzzerTraceConstCmp4`, `libfuzzerTraceConstCmp8`, `msanmove`, `msanread`, `msanwrite`, `racefuncenter`, `racefuncexit`, `raceread`, `racereadrange`, `racewrite`, `racewriterange`.

### Interface caches

`iface.go` performs cache updates, allocations and possible assertion panics. Metadata can be retained in itabs/caches.

`assertE2I2`, `interfaceSwitch`, `typeAssert`.

### Channels and select

`chan.go` and `select.go` can park, synchronize with other goroutines, retain element addresses in sudogs, invoke timer machinery or panic. Nonblocking variants share these paths.

`chanrecv1`, `chanrecv2`, `chansend1`, `closechan`, `selectgo`, `selectnbrecv`, `selectnbsend`.

### Pointer validation

The checks inspect pointer addresses, runtime allocation metadata and failure paths. Unsafe slice/string checks can panic and run user defers. Do not infer raw-pointer capture or leaf contracts from the successful path.

`checkptrAlignment`, `checkptrArithmetic`, `unsafeslicecheckptr`, `unsafestringcheckptr`.

### Allocation and conversion

These helpers allocate, may run GC/instrumentation/error paths, retain type/data pointers, or return aliases of scratch/input storage. `slicecopy` and `typedslicecopy` also call sanitizer hooks. A contract common to all supported build modes is not inferred from the fast path.

`concatbyte2`, `concatbyte3`, `concatbyte4`, `concatbyte5`, `concatbytes`, `concatstring2`, `concatstring3`, `concatstring4`, `concatstring5`, `concatstrings`, `growslice`, `growsliceBuf`, `growsliceBufNoAlias`, `growsliceNoAlias`, `intstring`, `moveSlice`, `moveSliceNoCap`, `moveSliceNoCapNoScan`, `moveSliceNoScan`, `slicebytetostring`, `slicebytetostringtmp`, `slicecopy`, `slicerunetostring`, `stringtoslicebyte`, `stringtoslicerune`, `typedslicecopy`.

### Scheduling and defer

These entries schedule execution, register callbacks, or observe/change panic/defer state. `gorecover` also inspects physical frames. Preserve their existing dedicated lowering contracts.

`deferproc`, `deferprocStack`, `deferprocat`, `deferrangefunc`, `deferreturn`, `gorecover`, `goschedguarded`, `newproc`.

### Dynamic equality and hashing

`alg.go` dispatches through type metadata and can panic for uncomparable or unhashable dynamic types. The panic path invokes user defers; the simple scalar-helper contracts do not cover it.

`efaceeq`, `ifaceeq`, `interequal`, `interhash`, `nilinterequal`, `nilinterhash`.

### Integer division

Zero divisors reach panic handling and user defers. Unlike software floating point, these are not unconditionally arithmetic-only helpers.

`int64div`, `int64mod`, `uint64div`, `uint64mod`.

### Maps

Map operations can allocate, synchronize, retain keys/types, call generated hash/equality functions, or panic. Type metadata and caches do not have a uniform read-only/non-capturing contract.

`mapIterNext`, `mapIterStart`, `mapaccess1_fat`, `mapaccess2`, `mapaccess2_fast32`, `mapaccess2_fast64`, `mapaccess2_faststr`, `mapaccess2_fat`, `mapclear`, `mapdelete`, `mapdelete_fast32`, `mapdelete_fast64`, `mapdelete_faststr`.

### Stack and indirect-call trampolines

The morestack entries switch and resume stacks through backend-specific control flow. Retpolines transfer to arbitrary indirect targets. They do not have ordinary GC-leaf or no-callback contracts; morestackc is separately modeled as noreturn because its Go body unconditionally throws.

`morestack`, `morestack_noctxt`, `retpolineAX`, `retpolineBP`, `retpolineBX`, `retpolineCX`, `retpolineDI`, `retpolineDX`, `retpolineR10`, `retpolineR11`, `retpolineR12`, `retpolineR13`, `retpolineR14`, `retpolineR15`, `retpolineR8`, `retpolineR9`, `retpolineSI`.

### Printing

`print.go` reaches `write` and the externally replaceable `overrideWrite` callback in `time_nofake.go`. Pointer printing also observes pointer addresses. Even small print helpers cannot be modeled as raw no-callback functions.

`printbool`, `printcomplex128`, `printcomplex64`, `printeface`, `printfloat32`, `printfloat64`, `printhex`, `printiface`, `printint`, `printlock`, `printnl`, `printpointer`, `printquoted`, `printslice`, `printsp`, `printstring`, `printuint`, `printuintptr`, `printunlock`.

### Legacy declaration

`throwinit` remains in the compiler builtin declarations, but has no runtime implementation in this tree; do not assign an implementation-derived contract.

`throwinit`.

## Alias audit follow-up

A `convT` or `convTnoptr` call with a static type descriptor of known positive
size receives return `noalias`. This reuses the newobject type-symbol proof.
Zero-sized types and dynamic descriptors retain the existing contract. The
attribute describes only the new box storage: copied pointer fields still
refer to their original pointees. Neither helper gets `allockind`, `memory`,
or new capture promises; copying, instrumentation and GC effects remain.
Optimizer tests prove forwarding across a store through the fresh result and
retain an unmodeled control and the effectful call.

Other candidates require additional call-site proofs:

- `makeslice`/`makeslice64`: positive element size and capacity. `makeslicecopy`
  additionally needs a non-overflowing byte count; its fast path trusts the
  source length. Zero-sized elements and zero capacity can use zerobase.
- `makemap`/`makemap64`: a null supplied-header argument would prove fresh
  header storage, whereas a non-null argument may be returned directly.
- `convT16`/`convT32`/`convT64`: arguments outside the staticuint64s cache.
  The cache boundary is a runtime implementation detail that needs a shared
  contract before binding a numeric threshold in compiler code.
- `convTstring`/`convTslice`: exclude their shared empty/nil boxes; freshness
  would apply only to the box, never the string/slice backing storage.
- String/slice conversions and concatenation may return the caller's scratch
  buffer, input storage, a one-byte string cache, or empty storage. Several
  helpers return aggregates, for which a pointer return attribute cannot
  directly annotate only the data field.

No blanket pointer-parameter `noalias` is added: memmove and typed copies may
legitimately overlap, and read-only comparison operands may be identical.
Map lookup/assignment and assertion results may be shared. Parameter capture
restrictions must also account for instrumentation and failure diagnostics.
