# In-process FullLTO validation

Local validation on 2026-10-13 used the LLVM 23 payload built from source
tree `1204008cc14b7a995b2dd152b6bc38b3350dfb9d`, matching the CI LLVM pin
`267a23e09bf5894dcf2544c8fbff38c49ea6ea92`. The Go toolchain was rebuilt
from this PR, including bootstrap, compiler, plugin, and linker. LLVM/LLD
sources were not modified for this integration.

The production command is `go build -lto=full`; all optimization and GoObj
code generation execute inside `cmd/link` through library calls. The old
`cmd/llvmlto` driver and standalone backend executable have been removed.

| Check | Result |
| --- | --- |
| Fresh dynamic LLVM toolchain bootstrap | Passed |
| Plugin CTest suite, including nine FullLTO FileCheck cases | 172/172 passed |
| Archive preservation, malformed inputs, write failures, option rejection, runtime and cache integration | Six Go tests passed |
| Linker LTO version identity test | Passed |
| Existing compiler SSA LLVM tests (`-run TestLLVM`) | Passed |
| macOS arm64, default PIE, complete 45-package closure | Built, linked, and ran with `GOGC=1` |
| macOS arm64, explicit executable, complete closure | Baseline comparison and cached relink passed with `GOGC=1` |
| Linux arm64, executable, complete 47-package closure | Built, linked, and ran with `GOGC=1` in native Docker |
| Linux amd64, executable, complete 47-package closure | Built, linked, and ran with `GOGC=1` in emulated Docker |
| Linux arm64 and amd64, PIE, complete closures | Built, linked, and ran with `GOGC=1` in native/emulated Docker |
| Ordinary link given a bitcode archive | Rejected with the required `-lto=full` diagnostic |
| macOS arm64 linker with LLVM/plugin statically linked | Built, performed FullLTO, linked PIE, and ran with `GOGC=1` |

The static linker has no `libLLVM` shared-library dependency. The compiler
for that probe came from the dynamic SDK; this is a static-linker/C ABI
check, not a separate full static SDK bootstrap.

The runtime fixture covers recursive stack growth with live pointer arrays,
four goroutines, repeated GC, reflection, interfaces, panic/recover, stack
frames, ordinary Go assembly calling back into Go, and references to private
transitive-package data. Each three-platform IR inspection found the
cross-package `dep.Add` call before LTO and no such call afterwards. The
`//go:noinline` call remains. Archive inspection confirms that the compiler
writes `__.LLVMBC` without `_go_.o`; a fresh link reuses cached dependency
bitcode while still running FullLTO.

This is not a full standard-library qualification. Cgo, sanitizers, shared
Go linking, and ThinLTO are not qualified. Unsupported option combinations
are rejected where documented in README.md.

Hosted CI status is separate from these local results. Earlier dispatches
were rejected by GitHub with `Actions has been disabled for this repository`;
see the PR descriptions for the latest dispatch status. No empty or queued
check set is treated as a passing CI run.
