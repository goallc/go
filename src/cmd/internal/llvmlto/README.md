# In-process FullLTO

Build an executable with an LLVM-enabled GoALLC toolchain:

```sh
go build -lto=full ./path/to/main
go build -lto=full -buildmode=pie ./path/to/main
```

`cmd/go` selects `-llvm-lto` compilation and includes the LLVM/plugin identity
in the link cache key. The compiler writes export data and pre-link optimized
bitcode (`__.LLVMBC`) into each package archive after required early IR
lowering, including CPU/FMV preparation, and LLVM's `lto-pre-link<O2>` pipeline.
Assembly and native members
remain ordinary objects. FIPS packages retain ordinary object compilation so
their code and synthesized constants keep their original FIPS classification.

Before loading objects, `cmd/link` calls the Go-owned FullLTO backend and emits
one `program.o`. The ordinary Go linker then performs symbol resolution,
content-addressed deduplication, dead-code removal, runtime metadata generation,
and final executable layout. Both dynamically loaded and statically linked
plugins use the same `GoALLCLTO` entry point. No external LLVM tools are needed.

The temporary `program.json` manifest records package identities, fingerprints,
imports, cgo directives, linkname origins, and the original-to-merged symbol
index mapping. The loader maintains logical compilation units over the single
physical object. Original package-indexed references from ordinary dependency
objects and assembly remain valid. Linkname permissions are checked against the
originating package, including references in functions inlined across packages.

Content-addressed functions stay content-addressed. Distinct IR copies use the
existing Go inline-hash storage spelling; GoObj removes this discriminator
before computing final machine-code hashes and writing names. The linker can
therefore merge identical copies across packages while keeping different code
or relocations distinct. The driver does not rewrite output symbol names or
turn content-addressed functions into package-indexed definitions.

Input archives in the build cache are never modified. Temporary replacement
archives preserve export data, assembly/native members, headers, padding, and
member order. Only the temporary main archive receives the merged compiler
object. Temporary work is removed on success and failure, unless requested:

```sh
go build -lto=full -ldflags=-lto-keep ./path/to/main
```

The linker prints `LTO_WORK=<directory>`, containing input bitcode, input IR
in `optimized/inputs`, optimized IR, the manifest, generated object, and
replacement archives. Ordinary links
reject bitcode archives with a diagnostic requesting `-lto=full`.

This path supports FullLTO at O2 for executable and PIE builds. ThinLTO,
sanitisers, dynamic Go shared linking, separate compiler `-linkobj` output,
`-N`, and custom LLVM optimization pipelines are not supported. Ordinary
non-LTO linking uses the existing path.

## Validation

Archive tests run without an LLVM-enabled linker. Runtime/toolchain tests are
explicitly enabled and cover cross-package optimization, cache reuse, hashed
closures, assembly, mixed native objects, initialization, GC/stack growth,
reflection, panic/traceback, logical DWARF packages, linkname permissions,
PIE, and cgo callbacks:

```sh
GOALLC_TEST_LTO=1 go test -gcflags=all=-enablellvm=false \
  cmd/internal/llvmlto -timeout=30m -count=1 -v
```

CI runs these tests on Linux amd64 and arm64, followed by complete short suites
for bytes, strings, strconv, sort, encoding/binary, encoding/json, hash/crc32,
and unicode/utf8 without LTO-specific exclusions:

```sh
GOROOT=/path/to/goallc .github/scripts/goallc-lto-stdlib.sh
```

Backend FileCheck and GoObj tests are described in
[the plugin documentation](../../llvmplugin/FullLTO.md).
