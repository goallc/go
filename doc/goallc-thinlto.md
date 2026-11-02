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

`-lto-jobs` controls parallel LLVM optimization (default: `GOMAXPROCS`).
Archive bitcode is passed to LLVM in memory. Each worker imports and optimizes
its module, then emits a GoObj from that same module and context. Object buffers
are written directly into replacement archives. LLVM's process-global GoObj
configuration requires serial code generation; module optimization stays
parallel. Modules are released as their workers finish.

The vendored go-llvm binding owns plugin loading, static dispatch, and C ABI
marshalling. Both modes share the declarations in `SupportBindings.h` and use
LLVM errors and memory buffers, with one disposal API for partial and successful
results. The plugin owns summary writing, importing, optimization, and codegen;
`cmd/internal/llvmlto` only coordinates archives and borrows output buffer views
until rewriting is complete. The compiler writes ThinLTO bitcode directly
through the binding, without depending on the linker archive package.

By default, ThinLTO preparation writes only replacement archives. `-lto-keep`
prints `LTO_WORK` and additionally saves input/optimized bitcode and IR, and
individual Go objects for inspection. The package build cache includes
LLVM/plugin identities; there is currently no persistent ThinLTO backend cache.

## Package identity and imports

Original compiler symbol indices and package fingerprints remain unchanged.
An indexed definition is retained and gets a temporary `goobj.import`
attachment identifying its package and index. Imported references retain that
attachment; local definitions lose it before GoObj emission. Imported bodies
are optimization-only; LLVM's standard ThinLTO post-link pipeline eliminates
their `available_externally` definitions. GoALLC preserves package references
across that elimination (which also clears function metadata), then normalizes
the remaining GoObj metadata.
Each output module must retain exactly its original set of indexed definitions.

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

Native debug metadata follows LLVM importing and inlining. Temporary
`goallc.thin.debug.funcs` records preserve source-subprogram-to-symbol mappings
as names and package references, including linknamed functions. After
optimization, reachable scopes regain exact GoObj debug symbol mappings;
unimported null references are discarded. Package configuration, import DAG,
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
