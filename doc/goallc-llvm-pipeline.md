# Go SSA before LLVM

The LLVM backend uses the normal Go SSA pass order through `cpufeatures`.
LLVM IR is emitted before `rewrite tern` and `lower`, while values still have
generic operations and calls still have logical Go arguments and results.
The LLVM optimization pipeline itself is unchanged.

| Go passes | LLVM path | Reason |
| --- | --- | --- |
| Early generic optimizations through `early fuse` | Run in their original order | Preserve Go-specific facts, inlining results, bounds proofs, and scalar simplification. |
| `expand calls` | Defer until after LLVM emission | Its physical argument/result decomposition would destroy the logical calling convention consumed by LLVM lowering. |
| `decompose builtin`, `softfloat`, `late opt` | Run in their original order | Expose components of slices, strings, and interfaces before later analyses. Soft-float rewriting is inactive on the supported hard-float targets. |
| `branchelim` | Skip for LLVM | Native conditional-select formation uses backend-specific assumptions. LLVM performs its own branch/select optimization; native formation can expose incompatible pointer/integer representations of interface words. |
| `sccp`, `generic deadcode`, `late fuse`, `check bce`, `dse` | Run in their original order | Reuse the existing generic analyses instead of maintaining a parallel LLVM cleanup sequence. |
| `dead auto elim` | Run once at the LLVM boundary, after DSE | Preserve elimination of temporaries whose last read was removed by DSE. The native placement stays unchanged. |
| `memcombine` | Skip for LLVM | Native combination relies on permitted unaligned accesses. LLVM lowering does not yet preserve that alignment contract for widened operations; LLVM performs its own memory combination. |
| `writebarrier`, `cpufeatures` | Run in their original order | LLVM needs the Go GC barrier decisions and CPU-feature facts. |
| `insert resched checks` | Skip for LLVM | These native stack-pointer checks were already outside the old LLVM emission boundary. LLVM retains its existing preemption implementation. |
| `rewrite tern`, target lowering, scheduling, register allocation | Run after LLVM emission | Their target operations are not inputs to LLVM. The native continuation remains necessary for frontend frame allocation and metadata bookkeeping. |

At the emission boundary, direct-interface normalization handles logical
aggregate forms that have not undergone native call expansion. One final CSE
and deadcode cleanup share addresses introduced by the later generic passes.
The early CSE remains necessary for `prove` and nil-check elimination.

After emission, `llvm native calls` runs `expandCalls`, the existing aggregate
cleanup for expanded calls, and deadcode. This second aggregate cleanup handles
new constructors and selectors created by physical ABI expansion; it does not
run another optimization pipeline over the LLVM input. The normal native
lowering then continues so `ssagen` can call `AllocFrame` as before.

`nativeOnly` and `llvmOnly` pass annotations select the paths without changing
the relative order of the native Go passes. New generic Go optimizations added
before the LLVM boundary therefore become available without copying their
scheduling into a separate LLVM pipeline.

## SIMD CPU preconditions

Go CPU features map to LLVM capabilities through the existing `archsimd`
generator. Fixed-width ABI requirements, Midway implementation widths, and
unguarded SIMD operations supply function `target-features`. As in native Go,
an unguarded operation is a caller precondition: the caller must check the CPU
before entering that function or callback. The compiler does not add a check
or fallback on its behalf, and this contract is independent of package names.

Locally guarded operations still use the early LLVM FMV pass. Entry capabilities
do not make Go CPU predicates true; explicit checks and fallback paths retain
their meaning, including feature disabling with `GODEBUG`. The surviving IR
requirements are still verified against each implementation's target features.
Derived algorithm flags such as `maps.UseAeshash` only imply CPU availability;
a capable CPU does not imply that the algorithm has been initialized or enabled.

## An optimization exposed by the shared pipeline

Before builtin decomposition, an interface data word can remain hidden behind
`IData(IMake(...))`. Write-barrier analysis may consequently treat a static
global data pointer as a possible heap pointer. Decomposition exposes the
global address, allowing the existing write-barrier analysis to omit recording
that new value. Recording the overwritten pointer remains necessary.

The codegen regression `llvm_builtin_decompose.go` checks this distinction:
assigning a static string to an interface records one barrier entry rather than
two. Aggregate-memory tests also check the scalar field loads exposed by
decomposition, including their exact offsets and preserved byval ABI.
