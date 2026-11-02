# Go bindings for GoALLC LLVM

This repository contains the LLVM Go bindings used by GoALLC. It intentionally
targets the customized LLVM build maintained by the GoALLC project; a
system-installed LLVM is not supported.

## LLVM payload

The binding always reads LLVM headers below `${SRCDIR}/llvm`; dynamic linking
also reads the LLVM shared library there:

    llvm/include/llvm-c
    llvm/lib

`llvm` is not committed. The caller must create it as a symlink to an LLVM
payload with this layout. GoALLC's `cmd/dist` manages that symlink from its
`-llvm-dir` option, whose default is `$GOROOT/llvm`.

## Build tags

The LLVM API version and link mode are independent, mandatory build-tag axes:

* `llvm23` selects the LLVM 23 API. A future LLVM 24 port will add `llvm24`
  without changing how the payload path or link mode is selected.
* `dynamicllvm` links `llvm/lib/libLLVM`; it is the GoALLC default.
* `staticllvm` links `${SRCDIR}/libLLVMGoALLC.a`. GoALLC's `cmd/dist`
  assembles and caches this build artifact from the selected payload's normal
  LLVM component archives with `llvm-config` and `llvm-ar`.

For example:

    go test -tags='llvm23 dynamicllvm' ./...
    go test -tags='llvm23 staticllvm' ./...

The static command requires `libLLVMGoALLC.a` to have been assembled first.
The GoALLC toolchain does this automatically for `-llvm-link=static`.
The aggregate archive must also contain the payload's non-system dependencies,
including static zlib/zstd when enabled. The binding does not search pkg-config
or link third-party shared libraries. The final executable links only host OS
libraries; Linux additionally selects static C++ and GCC runtimes. Use a payload
and `cmd/dist` that implement this dependency closure together.

Do not select multiple version tags or multiple link-mode tags in one build.

## Continuous integration

Automatic pull-request checks currently validate the binding source only. Full
dynamic and static tests require the customized GoALLC LLVM payload and are
validated as part of integration work. Once the project publishes prebuilt LLVM
payloads, CI will download those artifacts and run both link modes without
rebuilding LLVM from source.

## License

These bindings originated in LLVM and remain licensed under the Apache License
2.0 with LLVM Exceptions. See `LICENSE.txt`.
