# Package-preserving FullLTO library

`GoALLCFullLTO` is a C ABI entry point in the existing GoALLC pass plugin.
It combines compiler bitcode, runs FullLTO at O2, and writes one optimized
bitcode partition for each input package. The linker calls this library in
process and performs GoObj code generation before its ordinary loading,
resolution, dead-code elimination, and layout phases.

The library preserves package symbol indices, export fingerprints, import
relationships, assembly-visible definitions, and package-owned metadata.
Cross-package references use indexed GoObj imports. Address-significant
constants retain ownership. Linkname-sensitive bodies stay in their original
package so the Go linker's access checks still apply.

Errors return an allocated string to the caller, which frees it with `free`.
The entry point never launches a subprocess. Unsupported aliases, conflicting
configuration, and optimization-created symbols without ownership are rejected.
The initial pipeline conservatively retains Go definitions; ThinLTO and
aggressive whole-program internalization are not implemented.

With `BUILD_TESTING=ON`, `goallc-lto-test` is a test-only harness linked against
the same library. It is not installed as a toolchain command. Its nine FileCheck
cases live in `testdata/lto` and run with the existing plugin CTest suite.

Local validation: all 172 plugin CTest cases passed with LLVM 23 on macOS arm64,
including the nine FullLTO cases. Go build integration is provided separately.
