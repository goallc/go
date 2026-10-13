# GoObj FullLTO

`llvmlto` is an experimental, opt-in `go build -toolexec` driver. It stores
selected packages' LLVM bitcode in their archives, optimizes the combined
IR, replaces each package's GoObj, and invokes the unchanged Go linker.

```text
Go source -> package bitcode -> FullLTO -> per-package GoObj
Go assembly --------------------------> ordinary GoObj
                                             |
                                      original Go linker
```

## Use with go build

Build the backend as described below, then build the driver:

```sh
go build -gcflags=all=-enablellvm=false -o /tmp/llvmlto cmd/llvmlto
go build -buildmode=exe \
  -toolexec="/tmp/llvmlto -backend=/tmp/goallc-lto/goallc-lto \
  -llc=$LLVM_ROOT/bin/llc -plugin=$GOALLC_PASS_PLUGIN \
  -packages=example.com/project/..." ./cmd/app
```

`-packages` accepts comma-separated exact import paths and `path/...`
patterns; its default is `all`, including runtime and standard-library
dependencies. Unselected packages follow the ordinary compiler path.
The LLVM payload must provide `llvm-as`, `llvm-dis`, and `llc`.

Compilation emits both an ordinary Go object and real pre-codegen bitcode,
stored as `__.LLVMBC`. The link action writes temporary replacement archives,
preserving export data and assembly members, and redirects its importcfg
to those archives. Cached input archives are never rewritten. Tool and
LLVM library contents, the package selection, and the underlying tool
version participate in the build cache identity.

Use `-keep` on the driver to print `LTO_WORK` and preserve the input bitcode,
text IR, optimized partitions, GoObj files, replacement archives, and
importcfg for inspection.

Only `-buildmode=exe` with the normal O2 pipeline is supported. Specify it
explicitly on macOS, where the default build mode may be PIE. The driver
rejects PIE/shared/plugin modes, `-N`, custom LLVM pipelines, sanitizer
instrumentation, and other configuration it cannot preserve through the
second code-generation step. Cgo is not part of the current qualification.

`backend/goallc-lto` combines GoALLC LLVM modules, runs `lto<O2>`, and emits
optimized LLVM IR partitioned by the original Go package. The output is
intended for the existing GoObj code generator and Go linker. This layer
can also be built and checked independently of the Go build driver.

## Build and verify the backend

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

This is an experimental FullLTO implementation. It does not perform aggressive
whole-program internalization or implement ThinLTO. Package partitioning
currently clones the combined module for each output, which can consume
substantial memory and time. Alias/ifunc inputs and arbitrary module-level
assembly are unsupported.

## Runtime validation

```sh
GOALLC_LTO_BACKEND=/tmp/goallc-lto/goallc-lto \
GOALLC_LLC="$LLVM_ROOT/bin/llc" \
GOALLC_PASS_PLUGIN="$GOALLC_PASS_PLUGIN" \
  go test -gcflags=all=-enablellvm=false cmd/llvmlto -count=1 -v
```

The integration test explicitly compares ordinary LLVM compilation with
FullLTO. It disables Go frontend inlining in the caller and checks the
saved IR for LLVM's removal of a cross-package call. Runtime checks cover
GC, stack growth, goroutines, reflection/interfaces, panic/recover,
traceback, ordinary assembly calling Go, and cached-bitcode relinking.
Unit tests check archive preservation, write failures, unsupported options,
and cache invalidation. CI runs these tests and a whole-dependency-closure
FullLTO fixture on Linux amd64 and arm64.

The verified local scope and remaining limits are in [VALIDATION.md](VALIDATION.md).
