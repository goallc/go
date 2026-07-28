# GoALLC LLVM pre-codegen plugin

This directory owns the GoALLC-specific LLVM pass pipeline. It deliberately
lives in the Go repository: LLVM only supplies the generic `llc`
`-load-pass-plugin` and pre-codegen callback mechanism.

`GoALLCStatepointPlugin.cpp` adapts that callback to
`goallc::runPreCodeGenPipeline`. GoALLC transformations belong in
`GoALLCPreCodeGen.cpp` or other sources in this directory, not in the LLVM
repository. The core entry point is separate from the plugin adapter so a
future in-process `cmd/compile` integration can call the same pipeline without
going through `llc`.

The initial pipeline is intentionally a no-op. Its test proves only that the
out-of-tree module is ABI-compatible with, and invoked by, the selected `llc`.

Build, test, and install it into the LLVM payload that contains `llc`:

```sh
LLVM_PAYLOAD=/path/to/goallc-llvm
PLUGIN_BUILD=/path/to/empty/plugin-build

cmake -S "$GOROOT/src/cmd/llvmplugin" -B "$PLUGIN_BUILD" -G Ninja \
  -DLLVM_DIR="$LLVM_PAYLOAD/lib/cmake/llvm" \
  -DCMAKE_INSTALL_PREFIX="$LLVM_PAYLOAD"
cmake --build "$PLUGIN_BUILD"
ctest --test-dir "$PLUGIN_BUILD" --output-on-failure
cmake --install "$PLUGIN_BUILD"
```

The installed file is `lib/GoALLCStatepoints.dylib` on Darwin or
`lib/GoALLCStatepoints.so` on Linux. Do not build the plugin against a different
LLVM installation and copy it into the payload.
