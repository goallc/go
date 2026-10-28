# Single-object FullLTO backend

`goallc::runLTO` uses LLVM's IR `Linker` to combine compiler bitcode and
`llvm::lto::backend` to run target-aware FullLTO optimization. It writes
`program.bc` and a version 1 `program.json` manifest, plus `program.ll` when
`SaveIR` is enabled. The C entry point `GoALLCLTO` selects `lto<O2>` and
returns either null or an error string that the caller releases with `free`.

The output is optimized IR for later single-object GoObj code generation.
This stage does not connect to cmd/link, produce an executable, or enable
ThinLTO. A post-optimization hook stops the standard LTO backend before
code generation; the caller still owns the module and writes it directly.

## Native LLVM merging

All inputs share one LLVMContext. LLVM selects strong/weak definitions,
renames colliding locals, remaps references and merges named metadata and
module flags. There is no custom symbol-resolution table, second bitcode
parse, module clone, or `goallc.lto.owner/name/packages` metadata protocol.

Go named BSS placeholders are expressed as LLVM `common` definitions while
linking. LLVM selects the largest size and alignment, allows initialized
strong definitions to win, and resolves linknamed placeholders to functions.
The adapter checks the Go-specific minimum storage requirement and permits
a function only for linkname placeholders. After linking, it restores the
selected Go BSS section and ordinary linkage for GoObj emission.

The existing `goobj.gotype` relation is attached to its definition while
linking. LLVM carries the winning definition's type reference; the adapter
restores the normal named-metadata form after optimization. Type references
are retained with `llvm.compiler.used`. No package-owner operand is added
to gotype records, and no custom winner-filtering pass is needed.

## Go identities and output

Package configurations, import DAGs, cgo directives and linkname origins
remain in a C++ side table. Inputs require compatible GoObj configurations,
target triples and data layouts, unique package prefixes, and one main.
CPU-feature preparation runs before combining packages.

During linking, definitions receive unique temporary indices using the
existing `goobj.symbol.index` attachment. LLVM carries the selected index
and the side table maps it back to the original package, index and name,
even when LLVM renames local symbols. Compiler indices are restored before
optimization. Indexed symbols and assembly-visible definitions are retained
with `llvm.compiler.used`; post-optimization checks reject lost identities,
unsupported new definitions, aliases and ifuncs.

Content-addressed functions receive distinct package-qualified identities
before optimization. Go ABI suffixes must remain at the end of storage
names. Bodies with restricted linkname references remain out of cross-package
inlining; the original reference package is preserved for access checks.
FIPS packages, dynamic Go linking and arbitrary module assembly are rejected.

The manifest records the main package, original packages/imports/cgo data,
surviving symbol owners and ABI/index mappings, and linkname access checks.
The original import DAG is not flattened onto main. GoObj serialization,
package ownership and access rules remain Go-specific; LLVM linkage does
not replace those contracts.

## Tests

The test-only `goallc-lto-test` driver links the production static backend.
FileCheck covers optimization, package manifests, local renaming, strong/weak
selection, common size/alignment/type selection in both input orders, ABI0,
closures, BSS errors and linkname function placeholders. Downstream llc and
objview tests check amd64/arm64 statepoints, stack maps, safepoints, retained
symbols and merged variable layout/type references.

```sh
cmake --build <build-dir> --target GoALLCStatepoints goallc-lto-test
ctest --test-dir <build-dir> -R '^full-lto-' --output-on-failure
```
