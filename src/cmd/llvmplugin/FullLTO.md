# Single-object FullLTO backend

`goallc::runLTO` uses LLVM's resolution-based `llvm::lto::LTO` API to merge
compiler bitcode inputs and optimize one module. It writes `program.bc` and
`program.json`. With `SaveIR`, it also writes `program.ll` for inspection. The C entry point `GoALLCLTO` selects the
`lto<O2>` pipeline; a non-null return is an error string owned by the caller
and released with `free`.

This stage produces optimized bitcode, not a machine-code object. The later
linker integration will run final code generation once and consume the
manifest alongside that object's symbols. There is no per-package FullLTO
output. ThinLTO requests are explicitly rejected at this stage.

## LLVM LTO integration

Go-specific input preparation runs before `lto::InputFile::create`. Prepared
inputs are serialized to in-memory bitcode buffers, which stay alive through
`LTO::add` and `LTO::run`. LLVM owns module combination, internalization and
the target-aware optimization pipeline; this backend no longer calls the IR
linker or constructs the whole-program optimization pass manager itself.

The caller supplies `SymbolResolution` records: strong definitions take
precedence over weak definitions, duplicate strong definitions are errors,
and visible Go symbols remain available to ordinary objects outside LTO.
This remains deliberately conservative until the Go linker supplies more
precise reachability information. Go named BSS placeholders are normalized
before resolution because they do not use LLVM common-linkage semantics.

LLVM rereads the inputs into its own context. Temporary named metadata
transports package settings across that boundary and is removed from the
output. `PreOptModuleHook` records retained identities. `PostOptModuleHook`
checks them and restores the GoObj output metadata on a clone of the optimized
module, then returns false to stop before machine-code generation. The clone
respects the hook's const-module contract. A native output callback reports
an error if code generation is unexpectedly reached.

The on-disk bitcode remains the interface to the later GoObj code-generation
stage. It is not required by LLVM's LTO API; future integration can replace
this interface with direct object output after arranging the Go metadata and
pre-codegen callbacks at the appropriate stages. This PR does not implement
that direct-output path or enable ThinLTO.

## Package and symbol identity

Each input must have a unique package prefix and a compatible GoObj
configuration, target triple, and data layout. Exactly one input must be the
main package. CPU feature preparation runs before modules are combined, while
the original package metadata is still available.

Definitions retain package ownership through optimization. Indexed Go
symbols are kept even when their only users may be ordinary assembly or
packages outside LTO. Content-addressed functions receive distinct indexed
identities before cross-package optimization. Optimizer-created aliases,
ifuncs, duplicate indices, and unowned definitions other than eligible
constants are rejected.

The version 1 manifest contains:

- `main`: the index of the original main package in `packages`.
- `packages`: each package's prefix, debug name, standard-library flag,
  fingerprint, imports, and cgo directives.
- `symbols`: each surviving symbol's owning package, name, ABI, and, where
  present, its original and combined-object symbol indices. Renamed symbols
  also retain their original name.
- `linknames`: references whose access checks must use the originating
  package, including references inside bodies that were optimized away.

The original import DAG lives in the manifest. The combined module does not
put every package's imports on main, which would create artificial cycles.
Bodies with restricted linkname references are kept out of cross-package
inlining. Named BSS declarations are resolved against definitions with size
checks before LLVM module linking.

FIPS packages and modules carrying `goobj.static_rodata_type` must remain
ordinary compiler objects. Dynamic Go linking, input aliases and ifuncs,
and module assembly other than cgo directive records are also rejected.

## Tests

The test-only `goallc-lto-test` executable calls the production backend and
links its static library. It is not installed into the toolchain. FileCheck
fixtures cover cross-package optimization, manifests and import DAGs, BSS,
ABI padding, closures, ABI0, strong/weak resolution in both input orders,
and rejected inputs. Additional fixtures take optimized bitcode through
`llc` on amd64 and arm64, checking late statepoint callbacks and using
`objview` to validate stack maps, safepoints and assembly-visible symbols.
The plugin CI builds this executable before running CTest.

After configuring the plugin build with `BUILD_TESTING=ON`, run:

```sh
cmake --build <build-dir> --target GoALLCStatepoints goallc-lto-test
ctest --test-dir <build-dir> -R '^full-lto-' --output-on-failure
```
