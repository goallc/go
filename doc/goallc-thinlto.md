# ThinLTO

`go build -lto=thin` and `go test -lto=thin` compile packages with LLVM's
ThinLTO pre-link pipeline and write slim archives containing export data and
summary-bearing `__.LLVMBC` members. The linker combines summaries, imports
eligible function bodies, and optimizes modules in parallel. It then emits one
ordinary Go object per package and uses the existing Go linker for the final
executable. There is no merged whole-program IR module or custom loader.

The compiler uses a single `-llvm-lto=thin|full` option. Omitting it selects
ordinary object compilation; an explicit value must be `thin` or `full`.
`thin` writes ThinLTO bitcode and summaries, while `full` preserves the
existing FullLTO pre-link bitcode path. The build/link
driver in this change supports `-lto=thin`.

```sh
go build -lto=thin ./cmd/app
go test -lto=thin -short ./...
go build -lto=thin -ldflags='-lto-jobs=4 -lto-keep' ./cmd/app
```

`-lto-jobs` controls parallel LLVM backends (default: `GOMAXPROCS`).
Archive bitcode is passed to LLVM in memory. The Go-owned plugin normalizes
symbol identities and supplies resolutions to `lto::LTO`, which constructs the
combined index, plans imports, and schedules the standard in-process backends.
Each backend optimizes and emits one GoObj from its module and context. GoObj
configuration belongs to the individual `MCContext`, so code generation can
run concurrently. Object buffers feed replacement archives directly.

The plugin owns its C ABI in `GoALLCThinLTOC.h` and the Go-specific input/output
adaptation. Its mutable post-optimization callback restores package metadata
and prepares Go markers before codegen initializes AsmPrinter. LLVM owns the
LTO algorithms and codegen pipeline. `cmd/internal/llvmbackend` adapts Go calls
to the plugin through dynamic loading or static dispatch and owns LLVM error
and buffer lifetimes. `cmd/internal/llvmlto` coordinates archives and borrows
output byte slices until rewriting is complete. Plugin-specific operations do
not extend the vendored LLVM binding.

By default, ThinLTO preparation writes only replacement archives. `-lto-keep`
prints `LTO_WORK` and additionally saves input/optimized bitcode and IR, and
individual Go objects for inspection. The package build cache includes
LLVM/plugin identities; there is currently no persistent ThinLTO backend cache.

## Package identity and imports

Original compiler symbol indices and package fingerprints remain unchanged.
An indexed definition is retained with its package and index. Functions carry
this reference identity in string attributes, which survive LLVM deleting an
imported body; globals use `goobj.import` metadata. Local definitions lose the
reference identity before GoObj emission. LLVM's standard ThinLTO post-link
pipeline eliminates imported `available_externally` definitions. Each output
module must retain exactly its original set of indexed definitions.

LLVM normally selects one prevailing definition for each name. Before LTO,
the adapter gives duplicate Go definitions collision-checked temporary LLVM
names and rebuilds their summaries. Functions involving these unresolved
identities are not imported. After optimization, each package restores its
original names before object emission. This preserves package slots and
content-addressed references while leaving BSS replacement and duplicate
selection to the Go linker, including when native objects participate.

The current resolution policy leaves native symbol resolution authoritative,
including duplicate BSS and content-addressed symbols. Retaining original
package slots and the import restrictions below are conservative choices for
this GoObj representation, not requirements of ThinLTO itself. Duplicate
definitions, content-addressed bodies, references to unexportable local data,
and functions with Go reachability-marker
references are not imported. Such functions still receive ordinary per-module
optimization and code generation. In particular, marker references stored only
in metadata must not be lost through LLVM's summary-based importer. Global
initializers are not imported, so LLVM cannot replace native BSS resolution with
an imported initializer.

Native debug metadata follows LLVM importing and inlining. `DISubprogram`
annotations preserve exact source-to-symbol identities, including linknamed
functions, without rebuilding a separate mapping table after optimization.
Package configuration, import DAG,
CPU configuration, generated read-only data classification, and cgo assembly
directives remain owned by each original module. FIPS packages participate in
ThinLTO while retaining their FIPS symbol sections; their classification of
generated static data must not propagate into importing packages.
LLVM already keeps ordinary module assembly, including `.goobj.cgo`,
in its owning module during importing. GoALLC does not copy it back after
optimization. The GoObj parser rejects `.symver`, so LLVM's ELF symbol-version
import exception does not apply.

Archive rewriting preserves export data, assembly, cgo objects, member order,
and member headers. Ordinary archives can participate in the same link. For
example, `-gcflags='example.com/app/pkg=-enablellvm=false'`
compiles one package with the native Go backend. Archive layout and slim-member
validation are collected before optimization and reused when writing objects.

## Supported configurations

The initial implementation supports executable and PIE builds on Linux
amd64/arm64 and macOS arm64, including cgo and external linking. Shared Go
linking, sanitizers, aliases/ifuncs, and incompatible compiler optimization
flags are rejected. LLVM must support summary generation for Go defer `callbr`
intrinsics. FullLTO linking is not enabled by this change.

Regression coverage includes cross-package imports and symbol indices,
reachability-marker boundaries, imported debug symbols and PCLN on both
architectures, archive preservation, runtime/GC/stack growth, reflection,
closures, defer/recover, linkname permissions, BSS, mixed archives, PIE, and cgo.
Set `GOALLC_TEST_LTO=1` to run the executable integration tests in this package.
