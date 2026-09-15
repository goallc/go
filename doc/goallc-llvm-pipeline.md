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
generator. Fixed-width ABI requirements and Midway implementation widths supply
function `target-features`; they do not make runtime CPU predicates true.

Locally hardware-guarded operations use the early LLVM FMV pass. Explicit checks
and fallback paths retain their meaning, including feature disabling with
`GODEBUG`. If a SIMD operation or wide-register call needs features beyond the
function's contract and has no recognized hardware guard, its requirement
automatically requests FMV instead of raising the original function's feature
floor. This also applies to unguarded operations in the entry block. The same
Go CPU profiles and runtime resolver select the whole-function versions.

Automatic requests are collected from live instructions after LLVM's noreturn
cleanup. Compound requirements stay together instead of requesting every
individual feature combination; independent hardware observations still get
their own versions. Overlapping requests with the same runtime predicate share
one version. After specialization, a version whose simplified body and ABI match
the baseline reuses that baseline implementation, ignoring only the extra
`target-features` during comparison and retaining its dispatch predicate.

Supported versions retain the original SIMD instructions and register ABI,
without outlining or aggregate-memory argument/result carriers. In versions
without the required features, a source-local anchor becomes `unreachable`.
The anchor precedes the operation but follows operand evaluation. Unsupported
instructions and their now-unreachable continuation are removed before normal
LLVM optimization.
Surviving requirements are checked against each version's target features.

Ordinary algorithm flags such as `maps.UseAeshash` are not interpreted by the
compiler. The program must ensure that an unsupported SIMD operation is not
executed, including when its protection is outside the function or callback.
Violating that precondition is undefined behavior, not a recoverable panic or
a guaranteed trap. LLVM may
simplify control flow using this precondition. Automatic FMV does not invent
a software algorithm or interpret an ordinary business flag as a CPU predicate.
The resolver still uses the effective Go CPU snapshot after `GODEBUG` overrides;
before CPU initialization it uses baseline without caching the selection.

Known terminating Go APIs, including `testing.T.Skip`, receive LLVM's standard
`noreturn` attribute. LLVM removes their unreachable continuations before CPU
requirement checks; this does not rewrite Go SSA control flow.

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
