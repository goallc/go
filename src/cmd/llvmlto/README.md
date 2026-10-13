# FullLTO package backend

`backend/goallc-lto` combines GoALLC LLVM modules, runs `lto<O2>`, and emits
optimized LLVM IR partitioned by the original Go package. The output is
intended for the existing GoObj code generator and Go linker. This first
layer can be built and checked independently of a Go build driver.

## Build and verify

Use the LLVM 23 payload selected by this Go checkout and the
GoALLCStatepoints plugin built against that payload. A completed dynamic
GoALLC SDK build provides the plugin under `pkg/goallc-llvmplugin`.

```sh
cmake -S src/cmd/llvmlto/backend -B /tmp/goallc-lto -G Ninja \
  -DLLVM_DIR="$LLVM_ROOT/lib/cmake/llvm" \
  -DGOALLC_LTO_PLUGIN="$GOALLC_PASS_PLUGIN" \
  -DBUILD_TESTING=ON
cmake --build /tmp/goallc-lto
ctest --test-dir /tmp/goallc-lto --output-on-failure

/tmp/goallc-lto/goallc-lto --plugin="$GOALLC_PASS_PLUGIN" \
  --output-dir=/tmp/lto-partitions caller.bc callee.bc
```

Input order determines output names (`0.ll`, `1.ll`, ...). Both LLVM text IR
and bitcode are accepted. Inputs must contain the compiler's pre-codegen
GoObj configuration and symbol metadata. The CI build jobs run the backend
FileCheck tests on Linux amd64 and arm64 using the pinned LLVM release.

## Package contract

The backend records each definition's original package and symbol index
before combining modules. After optimization it restores package
configurations and indexed imports. A function inlined into a caller may
reference the original package's private data without moving that data or
changing its Go symbol identity. Imports include the original transitive
dependencies, preserving the package import DAG and fingerprints.

CPU legality processing runs before optimization. Statepoint lowering,
stack maps, PC tables, and GoObj emission belong to the subsequent codegen
stage; final layout and runtime tables remain the Go linker's responsibility.
Ordinary Go assembly objects can be linked alongside the resulting GoObj
partitions. Plan 9 assembly in LLVM IR, native ELF/Mach-O output, and LLD are
not prerequisites.

Definitions are conservatively retained for references from assembly and Go
metadata. Functions referencing package-restricted linkname symbols remain
non-inlineable so the Go linker's authorization checks continue to apply.
Content-addressed constants can be copied into partitions and deduplicated
by the Go linker. Named BSS declarations are resolved before LLVM linking;
unsafe initialized-definition size conflicts are rejected.

This is an experimental FullLTO backend. It does not perform aggressive
whole-program internalization or implement ThinLTO. Package partitioning
currently clones the combined module for each output, which can consume
substantial memory and time. Alias/ifunc inputs and arbitrary module-level
assembly are unsupported. The standalone backend tests do not constitute
end-to-end runtime or complete standard-library qualification.
