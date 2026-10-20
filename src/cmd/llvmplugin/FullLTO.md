# Single-object FullLTO backend

`goallc::runLTO` merges compiler bitcode inputs into one LLVM module, optimizes
the module, and writes `program.bc` and `program.json`. With `SaveIR`, it also
writes `program.ll` for inspection. The C entry point `GoALLCLTO` selects the
`lto<O2>` pipeline; a non-null return is an error string owned by the caller
and released with `free`.

This stage produces optimized bitcode, not a machine-code object. The later
linker integration will run final code generation once and consume the
manifest alongside that object's symbols. There is no per-package FullLTO
output. ThinLTO requests are explicitly rejected at this stage.

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
ABI padding, closures, ABI0, and rejected inputs. The plugin CI builds this
executable before running CTest.

After configuring the plugin build with `BUILD_TESTING=ON`, run:

```sh
cmake --build <build-dir> --target GoALLCStatepoints goallc-lto-test
ctest --test-dir <build-dir> -R '^full-lto-' --output-on-failure
```
