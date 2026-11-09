# Go bindings for GoALLC LLVM

This repository contains the LLVM Go bindings used by GoALLC. It intentionally
targets the customized LLVM build maintained by the GoALLC project; a
system-installed LLVM is not supported.

## LLVM payload

The binding reads LLVM headers below `${SRCDIR}/llvm`:

    llvm/include/llvm-c

`llvm` is not committed. The caller must create it as a symlink to an LLVM
payload with this layout. GoALLC's `cmd/dist` manages that symlink from its
`-llvm-dir` option, `GOALLC_LLVM_DIR`, or the default `$GOROOT/llvm`.
Dynamic linking uses the libraries installed by `cmd/dist` in
`$GOROOT/pkg/tool/${GOOS}_${GOARCH}/lib`, with a loader-relative runtime path
for installed tools. Static linking also requires the matching GoALLC pass
plugin in `$GOROOT/pkg/goallc-llvmplugin/lib`. These paths assume the binding
is in `$GOROOT/src/cmd/vendor/github.com/goallc/go-llvm`.

## Build tags

The bindings target LLVM 23 and use dynamic linking by default:

* No version tag is required; `llvm23` remains accepted for existing callers.
* Without `staticllvm`, the binding links the bundled `libLLVM` dynamically.
* `staticllvm` links `${SRCDIR}/libLLVMGoALLC.a`. GoALLC's `cmd/dist`
  assembles and caches this build artifact from the selected payload's normal
  LLVM component archives with `llvm-config` and `llvm-ar`.

For example:

    go test ./...
    go test -tags=staticllvm ./...

For standalone binding development, arrange the headers and libraries above
or provide explicit library search and runtime paths through `CGO_LDFLAGS`.
When using an upstream Go toolchain on Darwin, allow the loader-relative flag
with `CGO_LDFLAGS_ALLOW='^-Wl,-rpath,@loader_path/lib$'`.

The static command requires `libLLVMGoALLC.a` to have been assembled first.
The GoALLC toolchain does this automatically for `-llvm-link=static`.
The aggregate archive must also contain the payload's non-system dependencies,
including static zlib/zstd when enabled. The binding does not search pkg-config
or link third-party shared libraries. The final executable links only host OS
libraries; Linux additionally selects static C++ and GCC runtimes. Use a payload
and `cmd/dist` that implement this dependency closure together.

## Continuous integration

Automatic pull-request checks currently validate the binding source only. Full
dynamic and static tests require the customized GoALLC LLVM payload and are
validated as part of integration work. Once the project publishes prebuilt LLVM
payloads, CI will download those artifacts and run both link modes without
rebuilding LLVM from source.

## License

These bindings originated in LLVM and remain licensed under the Apache License
2.0 with LLVM Exceptions. See `LICENSE.txt`.
