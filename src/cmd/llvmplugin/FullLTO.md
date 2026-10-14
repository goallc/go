# LLVM LTO library and single-object FullLTO experiment

`GoALLCLTO` and `GoALLCFullLTOSingle` are C ABI entry points in the existing GoALLC pass plugin.
The linker calls it in process, selects FullLTO or ThinLTO, then performs
GoObj code generation before ordinary object loading, symbol resolution,
dead-code elimination, and layout.

This branch's FullLTO combines compiler bitcode, runs `lto<O2>`, assigns a
single dense output symbol-index space, and writes `program.bc` plus a
logical-package manifest. Code generation emits one GoObj, loaded through
main's temporary archive. Package identity remains in the manifest and
linker's logical compilation units; it is not inferred from the physical
object. Assembly/native members remain in their original package archives.
The old partitioned `GoALLCLTO` FullLTO entry remains available for library
tests, while cmd/link selects `GoALLCFullLTOSingle`.
ThinLTO constructs and combines LLVM summary indexes,
uses LLVM's cost-based cross-module import planner and function importer,
and runs `thinlto<O2>` separately on each package plus its imported bodies.
Thin backends currently run sequentially without a persistent backend cache.
`-lto-keep` retains the combined ThinLTO index as `optimized/thin.index.bc`
and the before/after package IR for inspection.

Both paths preserve package symbol identity, export fingerprints, imports,
assembly-visible definitions, and package metadata. Cross-package references
use indexed GoObj imports. Single-object FullLTO maps original package
indices to the dense output space, including references from non-LTO objects.
Address-significant constants retain ownership.
Content-addressed function copies receive distinct package indices so newly
exposed cross-package references can resolve before their machine-code hashes
exist. Their temporary LLVM names are restored after code generation, while
FMV suffixes remain recognizable to the backend. ABI padding types remain
distinct from ordinary byte structs.
Linkname-sensitive bodies remain uninlined so Go linker access checks apply.

Errors return an allocated string which the caller frees with `free()`.
No subprocess is launched. Unsupported aliases, conflicting configuration,
and optimization-created definitions without ownership are rejected. Go
symbols are conservatively retained because ordinary assembly is outside
LLVM's reachability graph.

With `BUILD_TESTING=ON`, `goallc-lto-test` is a test-only harness linked with
the same library. It is not installed as a toolchain command. FileCheck cases
in `testdata/lto` run with the existing plugin CTest suite. Runtime, archive,
and build-cache integration tests live in `cmd/internal/llvmlto`; see its
[README](../internal/llvmlto/README.md) and
[validation record](../internal/llvmlto/VALIDATION.md).
