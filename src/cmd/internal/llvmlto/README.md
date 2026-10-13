# In-process LLVM LTO

Build with an LLVM-enabled GoALLC toolchain:

```sh
go build -lto=full ./path/to/main
go build -lto=thin ./path/to/main
```

`cmd/go` passes `-enablellvm -llvm-lto` to the compiler and the selected `-lto` mode to
the linker. The compiler writes export data and pre-optimization LLVM
bitcode (`__.LLVMBC`) into each package archive, without generating an
intermediate compiler GoObj. Ordinary Go assembly members remain in the
archive.

Before loading package objects, `cmd/link` calls the existing GoALLC plugin
through a C ABI. On this experimental branch, FullLTO uses
`GoALLCFullLTOSingle`: it combines the bitcode, runs the LLVM O2 LTO pipeline,
and emits exactly one `program.o` in the temporary main archive. Dependency
archives retain export data and assembly/native members. A temporary
`optimized/program.json` manifest records the original packages, import DAG,
fingerprints, cgo directives, linkname references, and symbol-index mapping.
The loader uses these logical package identities for symbol resolution,
initialization order, runtime builtins, linkname checks, pclntab, and DWARF.
Temporary LLVM storage suffixes are removed from GoObj symbol names after
code generation, before the object is loaded.
ThinLTO builds a combined summary index, plans cross-package imports, imports
selected definitions, and runs the O2 ThinLTO pipeline separately for each
package. It preserves package ownership when emitting code. This initial
ThinLTO implementation uses sequential backends; it has no persistent import
or backend cache.
The linker generates GoObj in process using the same
LLVM bindings, code-generation options, and pre-codegen plugin as the
compiler. Its existing object loader, symbol resolver, dead-code analysis,
runtime metadata handling, and final layout consume the resulting objects.
FullLTO has no post-optimization package partitioning; ThinLTO still emits
one object per package. This experiment changes the Go linker loader and
does not claim compatibility with an unmodified linker.

No `llvmlto`, `goallc-lto`, `llvm-as`, `llvm-dis`, or `llc` executable is
required by this path. Both a shared LLVM/plugin build and a linker with
LLVM/plugin statically linked use the same implementation. Ordinary links
do not enter the LTO phase.

Input archives in the build cache are never modified. Temporary replacement
archives preserve export data, original assembly members, headers, padding,
and member order. Cached bitcode packages can be reused for a new LTO link.
The LTO linker version includes LLVM/plugin content identity, and the LTO
compiler flags distinguish the package cache entries. A regular link rejects
a bitcode archive with a diagnostic requesting `-lto=full` or `-lto=thin`.

To inspect the link's bitcode, before/after IR, and generated GoObj archives:

```sh
go build -lto=full -ldflags=-lto-keep ./path/to/main
```

The linker prints `LTO_WORK=<directory>`. This option keeps the directory;
otherwise it is removed when linking exits, including after an error.

The initial implementation supports FullLTO and ThinLTO at O2 for executable and PIE
builds. PIE requests select PIC code generation. Sanitizers, dynamic Go
shared linking, separate compiler `-linkobj` output, `-N`, and custom LLVM
optimization pipelines are rejected. Plugins, shared-library build modes, and aggressive whole-program
internalization are not implemented.
Cgo and the complete standard-library suite have not been qualified.
Single-object FullLTO also needs further FIPS work: the current LLVM backend
has one module-wide default for synthesized read-only constants, so it does
not preserve each originating package's FIPS classification. Do not use this
prototype to qualify a FIPS build. Full DWARF verification also retains the
baseline lexical-range overlap failures; see the validation record.

The backend library and its FileCheck tests are described in
[the plugin documentation](../../llvmplugin/FullLTO.md). End-to-end tests:

```sh
GOALLC_TEST_LTO=1 go test -tags=llvm23,dynamicllvm \
  -gcflags=all=-enablellvm=false cmd/internal/llvmlto -count=1 -v
```

Use the normal toolchain build environment, including any required cgo
linker flag allowlist. `GOALLC_TEST_LTO=1` enables runtime/toolchain probes;
archive unit tests run without it. See [VALIDATION.md](VALIDATION.md) for
the measured validation scope.

CI runs both LTO modes on Linux amd64 and arm64. The standard-library job
runs complete short test suites for `bytes`, `strings`, `strconv`, `sort`,
`encoding/binary`, `encoding/json`, `hash/crc32`, and `unicode/utf8`, with no
LTO-specific test exclusions. To run the same suite locally:

```sh
GOROOT=/path/to/goallc .github/scripts/goallc-lto-stdlib.sh full
GOROOT=/path/to/goallc .github/scripts/goallc-lto-stdlib.sh thin
```
