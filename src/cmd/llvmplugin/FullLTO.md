# Single-object FullLTO backend

`goallc::runLTO` uses LLVM's IR `Linker` to combine compiler bitcode and
`llvm::lto::backend` to run target-aware FullLTO optimization. It writes
`program.bc` and a version 1 `program.json` manifest. When `SaveIR` is enabled,
it also writes `program.ll` and snapshots each parsed input in `inputs/`,
before adapting its Go metadata. The C entry point `GoALLCLTO` selects `lto<O2>` and
returns either null or an error string that the caller releases with `free`.

The backend returns optimized IR to cmd/link's in-process GoObj code generator.
A post-optimization hook stops the standard LLVM LTO backend before native
code generation; the caller still owns the module and writes it directly.
The complete build/link flow is documented in
[cmd/internal/llvmlto](../internal/llvmlto/README.md).

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

The frontend already attaches `goobj.gotype`, keep edges and linker markers
to their source definitions. LLVM preserves the selected definition's
attachments and remaps their references; LTO performs no table-to-attachment
or attachment-to-table conversion. The frontend's `llvm.compiler.used` roots
retain values that are referenced only by Go metadata.

The shared `goobj.abi` contract uses LLVM's `Error` module flag, so LLVM rejects
incompatible inputs without a separate field-by-field comparison. Native
compile units, source modules, retained variables and debug records merge
directly. LTO neither reconstructs a debug-variable table nor selects a single
package's DWARF configuration for the whole program. Declared package names
stay in the native debug graph; the linker reads them from GoObj rather than
from a second manifest copy.

## Go identities and output

Package configurations, import DAGs, cgo directives and linkname origins
remain in a C++ side table. Inputs require the current five-field package configuration and `goobj.abi`
module flag, compatible target triples and data layouts, unique package
prefixes, and one main.
The compiler completes functional early IR lowering and `lto-pre-link<O2>`
before serializing each input. LTO rejects CPU configuration without the
existing completion marker, then discards the consumed configuration.
Function attributes carry the prepared target policy into whole-program
optimization and code generation.

During linking, definitions receive unique temporary indices using the
existing `goobj.symbol.index` attachment. LLVM carries the selected index
and the side table maps it back to the original package, index and name,
even when LLVM renames local symbols. Compiler indices are restored before
optimization. Indexed symbols and assembly-visible definitions are retained
with `llvm.compiler.used`; post-optimization checks reject lost identities,
unsupported new definitions, aliases and ifuncs.

Content-addressed functions retain their native hash classification. Distinct
copies use Go's existing #base64# inline-hash storage discriminator, which
GoObj strips before writing names and hashing final machine code. Go ABI
suffixes remain at the end. Identical code and relocations can deduplicate
across packages in the ordinary linker; different bodies remain distinct.
Their logical debug owner is the source package when it participates in LTO,
or main for compiler helpers without such an owner. No output name patching
or artificial package indices are needed for these copies.

Linkname origins are validated by the linker's manifest consumer, so LTO does
not add noinline restrictions to bodies containing linkname references.
FIPS bitcode inputs, dynamic Go linking and arbitrary module assembly are
rejected. The build driver leaves FIPS packages as ordinary objects.

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
symbols and merged variable layout/type references. Native debug graph tests
cover declared package names, retained result annotations and LLVM inlining
in both input orders; strong/weak tests check that only the winning
definition supplies gotype, keep and marker relations.

```sh
cmake --build <build-dir> --target GoALLCStatepoints goallc-lto-test
ctest --test-dir <build-dir> -R '^full-lto-' --output-on-failure
```
