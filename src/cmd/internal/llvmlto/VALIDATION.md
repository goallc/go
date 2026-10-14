# Single-object FullLTO experiment

Local branch `codex/full-lto-single-goobj`, based on `1469e0d858da`,
validated on 2026-10-13. The compiler and LLVM payload are inherited from
the base below; the plugin and cmd/link are rebuilt for this experiment.
No LLVM source or package-cache archive is modified.

FullLTO produces one optimized module and one `program.o` for the complete
45-package runtime fixture closure. Only the temporary main archive contains
that compiler object; the other archives retain export data and assembly/native
members. Logical package identity and original symbol indices are carried by
`program.json` and interpreted by the modified Go linker.

| Check | Result |
| --- | --- |
| Plugin CTest suite, including single-object FileCheck fixture | 179/179 passed |
| Archive, runtime, optimization, and cached-relink integration | FullLTO and ThinLTO passed |
| Linkname compatibility | Six allowed/forbidden fixtures, both with and without DWARF: 12/12 passed |
| Mixed native dependency referencing a merged LLVM leaf | Build and runtime passed |
| macOS arm64 executable and PIE | Runtime fixture passed with `GOGC=1` |
| Linux arm64 / amd64 | Complete runtime closure built and ran; arm64 native Docker, amd64 emulated Docker |
| GoObj and linker loader unit suites | Passed |
| Standard-library short suites during implementation | 8/8 passed on macOS arm64: bytes, strings, strconv, sort, encoding/binary, encoding/json, hash/crc32, unicode/utf8 |

The standard-library run preceded the final symbol-name and DWARF package-name
corrections. Runtime, object-name, and debug-package regressions are rerun after
those corrections. The fixture also checks leaf-before-dependency initialization,
assembly calls in both directions, goroutines, stack growth, repeated GC,
reflection, panic/recover, and stack frames. Object inspection checks that LLVM
storage suffixes do not escape into final GoObj symbol names.

With uncompressed full DWARF, `llvm-dwarfdump --verify` reports 89 overlapping
DIE address ranges. The same fixture built by the base's partitioned FullLTO
reports 89 of the same error category. This is a baseline failure, not a green
DWARF qualification. The new regression checks logical CUs and their declared
Go package names, which must survive single-object emission.

Open experimental limits:

- Synthesized read-only constants still use LLVM's module-wide default kind.
  Per-origin FIPS classification needs a backend contract or a validated
  post-codegen mapping; this branch is not suitable for FIPS qualification.
- Cgo, external linking, the complete standard library, static LLVM linkage,
  and hosted CI have not been requalified for this single-object change.
- No controlled link-time, peak-memory, or binary-size comparison has been run.
  One backend invocation alone is not performance evidence.

Local logs and retained binaries are in `/tmp/go-lto-single-20261013/`.
The base validation below describes the previous partitioned FullLTO path;
its broader checks must not be read as results for this experiment.

## Base implementation validation (before the single-object experiment)


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
| Linux arm64 and amd64 executable, complete closure | Both modes built and ran with `GOGC=1`; arm64 native Docker, amd64 emulated Docker |
| macOS arm64 linker with LLVM/plugin statically linked | Both modes linked PIE and ran with `GOGC=1`; no libLLVM shared dependency |
| Workflow YAML, four LTO matrix combinations, shell syntax | Passed |
| Eight common standard-library short suites on macOS arm64 | All 8/8 passed in each mode, 16/16 total |

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

At publication on 2026-10-13, manual dispatch of the consolidated PR branch
was rejected by GitHub with HTTP 422: `Actions has been disabled for this
repository.` No workflow run was created. The four Linux standard-library
jobs are configured but have not executed; the standard-library results
above are local macOS arm64 results, not hosted Linux qualification.
