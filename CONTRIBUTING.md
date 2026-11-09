# Contributing to GoALLC

English | [简体中文](CONTRIBUTING.zh-CN.md)

Contributions to GoALLC are welcome through this repository's
[issues](https://github.com/goallc/go/issues) and
[pull requests](https://github.com/goallc/go/pulls). You can help with compiler and
runtime compatibility, performance, tests, documentation, and build tooling.

## Report a problem or suggest a change

Search existing issues first, then choose the bug, performance, or feature form.
English and Chinese reports are welcome.

For a bug, include the GoALLC version (`go version`), OS and architecture, relevant
build flags, a small reproducer, and the expected and actual behavior. If you have
compared with official Go, include its version and result as well.

For performance reports, include the benchmark source, CPU model, toolchain
versions, build flags, repeated measurements, and commands. Keep inputs and
benchmark settings comparable, and include slower cases when reporting a broader
comparison.

## Prepare a pull request

Start from the repository's default branch, `go1.27.master`, and keep each change
focused on one problem. Explain the resulting behavior and link a related issue
when there is one.

Use the [build guide](doc/goallc-build.md) for source builds and the
[architecture overview](GOALLC.md) to locate the relevant implementation.
Go-specific LLVM passes live in [src/cmd/llvmplugin](src/cmd/llvmplugin/README.md);
LLVM target mechanisms live in [goallc/llvm-project](https://github.com/goallc/llvm-project).

Format changed Go files with `gofmt`. For behavior changes, add a focused regression
case and run the relevant tests with the toolchain you changed. In the PR, state
what you tested, on which platform, and any validation still outstanding. For
performance changes, include the comparison method and measurements. Documentation
changes should have their links and examples checked.

Keep the English and Chinese READMEs consistent when changing user-facing project
information. Submit the PR on GitHub against `go1.27.master`.

If the change depends on an open GoALLC LLVM PR, add an uncommented line such as
`LLVM-PR: goallc/llvm-project#123` to the PR body so CI can select its LLVM payload.

## License

Unless otherwise noted, the Go source files use the BSD-style [LICENSE](LICENSE).
Preserve existing copyright and license notices when modifying code.
