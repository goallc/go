# In-process LTO validation

Local validation on 2026-10-13 uses the LLVM 23 payload built from source
`1204008cc14b7a995b2dd152b6bc38b3350dfb9d`, matching the clean CI LLVM pin
`267a23e09bf5894dcf2544c8fbff38c49ea6ea92`. The compiler, linker, Go command,
and plugin are rebuilt from this change. LLVM sources are unchanged.

`go build -lto=full` and `go build -lto=thin` execute optimization and GoObj
code generation inside `cmd/link`; neither path uses a standalone LTO tool.

| Check | Result |
| --- | --- |
| Plugin CTest suite, including FullLTO/ThinLTO FileCheck regressions | 178/178 passed |
| Archive preservation, invalid inputs, write failures, option rejection, runtime and cached relink | Passed for both modes |
| Linker LTO version identity test | Passed |
| Existing compiler SSA LLVM tests (`-run TestLLVM`) | Passed |
| macOS arm64 executable and PIE, complete runtime fixture closure | Both modes passed with `GOGC=1` |
| Linux arm64 and amd64 executable, ThinLTO complete closure | Built and ran with `GOGC=1`; arm64 native Docker, amd64 emulated Docker |
| macOS arm64 linker with LLVM/plugin statically linked | Both modes linked PIE and ran with `GOGC=1`; no libLLVM shared dependency |
| Workflow YAML, four LTO matrix combinations, shell syntax | Passed |
| Eight common standard-library short suites | Local full/thin runs in progress |

The runtime fixture covers recursive stack growth with live pointer arrays,
goroutines, repeated GC, reflection, interfaces, panic/recover, stack frames,
ordinary assembly callbacks, and private transitive-package data. Saved IR
checks cross-package `dep.Add` call elimination and preserves `//go:noinline`.
ThinLTO also checks its combined summary index. Archive tests confirm
`__.LLVMBC` without an initial `_go_.o` and reuse of dependency bitcode on a
fresh link action. A mixed no-DWARF main package tests debug-record handling.

Regressions cover ABI padding versus ordinary byte structs, indexed closure
copies (including ABI0 identity), same-named private globals, content-addressed
constants, metadata-only debug/marker references, markers moved by inlining,
and the Go defer callbr intrinsic during ThinLTO summary construction.

CI runs complete short tests for `bytes`, `strings`, `strconv`, `sort`,
`encoding/binary`, `encoding/json`, `hash/crc32`, and `unicode/utf8` in four
jobs: FullLTO/ThinLTO on native Linux amd64/arm64. Tests are serialized and
resource logs are uploaded. No LTO-specific exclusions are applied.

This is not a full standard-library or cgo qualification. Sanitizers, shared
Go linking, plugin/shared-library build modes, and custom optimization
pipelines are rejected as documented. Thin backends are sequential and do
not implement a persistent backend cache. Hosted CI status is reported in
the PR separately from local results; queued or absent checks are not passes.
