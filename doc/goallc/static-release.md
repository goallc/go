# Portable static SDK releases

The release SDK embeds LLVM, GoALLC passes, and their non-system dependencies in
its compiler. Pure Go builds require no separate LLVM, plugin, pkg-config, CMake,
or C++ toolchain. Cgo programs still require a target C toolchain, as with Go.

| Host | Runtime dependencies | Build baseline |
| --- | --- | --- |
| Linux amd64 / arm64 | glibc and OS libraries; no shared libstdc++, LLVM, zlib or zstd | Ubuntu 22.04, GLIBC <= 2.35; amd64 v1 |
| Darwin arm64 | Apple system libraries including libc++ | macOS deployment target 13.0 |
| Windows amd64 | Windows system and UCRT DLLs | llvm-mingw 20261020 UCRT; amd64 v1 |

Static describes the compiler's LLVM/pass/dependency closure. It does not mean
a fully static glibc executable or change user-program linking defaults.
Execution on the minimum supported OS remains a separate qualification
requirement; a dependency/version audit does not substitute for running there.

## Build and package

Use the immutable four-host LLVM release pinned in `.github/workflows/goallc.yml`.
Verify its archive checksum and `share/goallc/build-manifest`, including the
source revision, clean tree, Release build, and `bundled-static` dependency mode.
The static closure fails if a required archive is absent from the payload.

On Darwin, use `MACOSX_DEPLOYMENT_TARGET=13.0` for LLVM, the pass and cgo
compilation. On Windows use the same llvm-mingw UCRT distribution for LLVM,
the pass and bindings; MSVC libraries are not interchangeable with this ABI.

From an isolated Go checkout:

```sh
printf '%s\n' go1.27.3-goallc.1 > VERSION
cd src
./make.bash -llvm-dir=/absolute/payload -llvm-version=23 -llvm-link=static
cd ..
mkdir -p lib/goallc/licenses
cp -R /absolute/payload/share/licenses/. lib/goallc/licenses/
{
  printf 'go_revision=%s\nlink_mode=static\n' "$(git rev-parse HEAD)"
  cat /absolute/payload/share/goallc/build-manifest
} > lib/goallc/build-manifest
./bin/go run cmd/distpack
```

Windows uses `src/make.bat` with the same build options and includes the
llvm-mingw license in `lib/goallc/licenses`. Set `VERSION` before bootstrap so
the embedded version matches the archive. `cmd/distpack` writes the standard
platform archive to `pkg/distpack`, excluding LLVM headers and archives, plugin
build trees, and caches. The workflow also records an archive SHA-256 checksum.

## CI and publishing

The GoALLC workflow builds static SDKs on all four native hosts and runs the
existing `dist test` stdlib/compiler/tools/runtime groups. Tests use
`-llvm-link=static`, including nested compiler builds, and the same existing
`test/llvm_known_failures.json` policy as Linux. No platform-specific exclusions
are introduced to qualify this release. Unix shared-plugin FileCheck tests run
in a separate CMake build; Windows does not support dynamically loaded plugins.

Each build also extracts its standard release archive to a path containing
spaces, hides the original LLVM and plugin directories, audits all distributed
binary dependencies, runs fmt/reflect/sync with a fresh cache and system-only
PATH, cross-builds the existing hello-world fixture for all four targets, and
executes the existing cgo callback test with external linking. LLVM/compiler
headers and archives needed by compiler tests travel in CI-only artifacts,
separate from the release archive.

PR and ordinary branch runs upload candidates only. To publish, dispatch the
GoALLC workflow on an existing `goX.Y.Z-goallc.N` tag with `publish=true`.
The version must match that checked-out tag. The publish job requires all four
builds and all sixteen test jobs to pass, checks all four archive digests, and
creates a new release without replacing existing assets. The workflow's default
is `publish=false`. A dispatch on a branch cannot publish.

A passing CI run establishes the tested native-host and buildmode coverage;
minimum-OS execution and general FullLTO support remain separate gates. Existing
known failures, including excluded traceback tests, remain visible in each test
job's summary and uploaded manifest.
