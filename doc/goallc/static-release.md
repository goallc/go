# Portable static SDK releases

The release SDK embeds LLVM and the GoALLC passes in the compiler. It does not
require a separately installed LLVM, plugin, pkg-config, CMake or C++ runtime.
Cgo programs still require a target C toolchain, as with upstream Go.

The initial runtime contract is:

| Host | Runtime dependencies | Build baseline |
| --- | --- | --- |
| Linux amd64 / arm64 | glibc and OS libraries; no libstdc++, LLVM, zlib or zstd shared library | Ubuntu 22.04, GLIBC <= 2.35; amd64 v1 |
| Darwin arm64 | Apple system libraries including libc++ | macOS deployment target 13.0 |
| Windows amd64 | Windows system and UCRT DLLs | llvm-mingw 20261020 UCRT; amd64 v1 |

“Static” describes the compiler's LLVM/pass/dependency closure. It does not
mean a fully static glibc executable or change user-program linking defaults.
Execution on the minimum supported OS is a separate qualification requirement;
a dependency/version audit is not a substitute for running there.

## Build and package

First build an installed LLVM payload with the matching revision of
`llvm/utils/goallc/build-payload.bash`. That script builds checksum-pinned,
PIC static zlib/zstd, disables optional XML/editline/network dependencies, and
includes their notices. Do not substitute system LLVM or copy build-tree
libraries into an installed payload. The static closure deliberately fails if
a required archive is absent from the payload.

On Darwin, use the same `MACOSX_DEPLOYMENT_TARGET=13.0` for LLVM, the pass and
cgo compilation. On Windows use the same llvm-mingw distribution for LLVM,
the pass and bindings; MSVC libraries are not interchangeable with this ABI.

From the Go tree:

```sh
printf '%s\n' go1.27.0-goallc.1 > VERSION
cd src
./make.bash -llvm-dir=/absolute/payload -llvm-version=23 -llvm-link=static
cd ..
python3 .github/scripts/package-static-sdk.py --root . --llvm /absolute/payload \
  --version go1.27.0-goallc.1 --output /absolute/release
```

Windows uses `src/make.bat` with the same options. The packager uses the normal
`cmd/distpack` layout, adds dependency notices and source provenance, and writes
an archive checksum. Build-only archives, LLVM links, plugin builds and caches
are excluded. Set `VERSION` before bootstrap so the embedded binary version matches the archive;
the packager verifies this. Use an isolated release checkout.

Extract the archive in a different directory containing spaces and remove the
build-time payload/plugin from their original locations. Then run:

```sh
python3 .github/scripts/verify-static-sdk.py '/relocated sdk/go' \
  --windows-target --cgo --report /absolute/release/validation.json
```

The verifier checks all distributed executable dependencies, platform version
requirements, no leaked build inputs, compiler identity, clean-cache build/test,
GC/runtime/reflect/sync tests, native execution, pure-Go cross builds and an
external-link cgo probe. Pure Go operations receive only an OS command PATH.
Cgo receives the caller's C toolchain explicitly. Cross compilation is not
native execution evidence for the target platform.

## Delivery order

1. Land and qualify the LLVM portable-payload changes on all four native hosts.
2. Publish a new immutable payload tag and verify each archive checksum and
   `share/goallc/build-manifest`; do not overwrite an existing release asset.
3. Integrate the common static SDK changes, matching bindings and the independent
   Windows target/host changes. The Windows workflow entry requires those
   functional changes; a matrix entry alone does not establish platform support.
4. Run `Static SDK release candidate` with that LLVM release, its exact revision,
   and the desired SDK version. All four jobs must pass. It uploads candidates
   and evidence, without publishing a public Go release.
5. Qualify minimum OS versions and the supported cgo/buildmode scope before
   publishing those exact candidate bytes. FullLTO is a separate feature gate.

The existing Linux dynamic-development CI remains enabled. The static release
workflow is explicitly dispatched until a qualified four-platform payload and
the Windows prerequisites have landed; the old two-platform pin is not silently
changed or represented as a four-platform release.

## Initial qualification status

Local Darwin arm64 and Ubuntu 22.04 amd64 builds passed static bootstrap,
archive relocation with the original payload/plugin removed, dependency audits,
pure-Go cross builds for Linux amd64/arm64 and Darwin arm64, and external-link
cgo execution. The dynamic plugin's 164 FileCheck tests also passed.

Full SDK qualification remains blocked: both hosts fail `TestTracebackAncestors`,
`TestTracebackElision` and `TestTracebackArgs`. All three also reproduce with the
unmodified Darwin baseline. They remain enabled in the release gate; no test is
skipped to produce a candidate. Linux arm64 native execution, the integrated
Windows SDK, four-host hosted CI and minimum-OS execution still need qualification.
