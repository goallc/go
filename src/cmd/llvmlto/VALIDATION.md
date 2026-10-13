# FullLTO validation

Validated locally on 2026-10-13. These are focused qualification results,
not a claim that the complete standard-library test suite passes with LTO.

## Source and toolchain

The changes are split into the standalone package backend and the Go build
driver. Both start from Go `743a300e2689`; no compiler, assembler, Go linker,
LLVM, or LLD source changes are required.

The local LLVM build used clean revision `435b946ca771`. Its source tree
`1204008cc14b7a995b2dd152b6bc38b3350dfb9d` is identical to the CI-pinned
release revision `267a23e09bf5894dcf2544c8fbff38c49ea6ea92`. A fresh Go SDK
and GoALLCStatepoints plugin were built against that isolated LLVM payload.
The SDK bootstrap used the native Go backend; the test controls and LTO
inputs explicitly enabled LLVM. The open Plan 9 IR and native-object
experiments were not included.

## Checks

| Check | Coverage | Result |
|---|---|---|
| CTest/FileCheck | Cross-package optimization, package indices and imports, address identity, assembly-visible definition retention, BSS and invalid-input diagnostics | 9/9 passed |
| Go tests | Archive preservation/errors, supported options, tool/library cache identity, ordinary LLVM versus FullLTO runtime, cached dependency archives | 7/7 passed, including two consecutive integration runs |
| macOS arm64 | All 45 packages in the fixture dependency closure | Build/link/run passed |
| Linux arm64 | All 47 packages in the fixture dependency closure | Cross-build/link and native arm64 container execution passed |
| Linux amd64 | All 47 packages in the fixture dependency closure | Cross-build/link and amd64 container execution under emulation passed |

All three complete dependency closures include runtime and the fixture's
standard-library dependencies. Each executable ran with `GOGC=1`, reporting
`LTO RUNTIME OK`. Saved pre-LTO IR contained one `dep.Add` call from
`main.main`; the optimized caller contained none. Ordinary Go assembly
remained an archive member and its callback into Go executed successfully.

The fixture exercises live heap pointers during collection, recursive
stack growth with stack pointer arrays, four goroutines, interface calls,
reflective method lookup, panic/recover, frame enumeration, noinline calls,
and references through a transitive dependency to mutable private data.

The original prototype fails the added backend metadata fixture: it drops
an unreferenced indexed static and duplicates an address-significant local
constant. The backend now preserves their identities before IR linking.

CI runs the backend, driver integration, and complete fixture dependency
closure on native Linux amd64 and arm64 runners. CI results must be checked
separately for each PR head; local cross-target runs do not replace them.

## Limits

The supported mode is an ordinary executable with the fixed O2 pipeline.
PIE/shared/plugin modes, sanitizer instrumentation, cgo qualification,
ThinLTO, and full standard-library qualification remain outside this change.
Unsupported driver configurations produce errors rather than silently
dropping the requested semantics.

No performance claim is made. The prototype compiles ordinary GoObj before
the LTO link and clones the combined IR for every output package. Retention
is conservative; final reachability is left to the original Go linker.
