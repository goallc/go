# SIMD dispatch and FMV executable cases

Each `case_*.go` file describes one runtime-reachable combination of:

- portable SIMD Midway width selection;
- the feature floor attached to that width variant;
- effective AVX2 and AVX-512 feature guards; and
- the FMV feature or fallback implementation selected inside the variant.

`operation.go` contains the combined portable/fixed-width operation, while
`check.go` validates its values and selected branches. `main.go` runs every
case supported by the current CPU.

| Case | Midway variant and floor | Effective guarded paths | Nested FMV needed |
| --- | --- | --- | --- |
| `simd0-all-off` | `@simd0`, no floor | both fallbacks | AVX2 and AVX-512 |
| `simd0-avx2` | `@simd0`, no floor | AVX2, AVX-512 fallback | AVX2 and AVX-512 |
| `simd0-all-features` | `@simd0`, no floor | AVX2 and AVX-512 | AVX2 and AVX-512 |
| `simd128-fallbacks` | `@simd128`, AVX | both fallbacks | AVX2 and AVX-512 |
| `simd128-avx2` | `@simd128`, AVX | AVX2, AVX-512 fallback | AVX2 and AVX-512 |
| `simd128-all-features` | `@simd128`, AVX | AVX2 and AVX-512 | AVX2 and AVX-512 |
| `simd256-floor` | `@simd256`, AVX2 | AVX2, AVX-512 fallback | AVX-512 only |
| `simd256-avx512` | `@simd256`, AVX2 | AVX2 and AVX-512 | AVX-512 only |
| `simd512-floor` | `@simd512`, AVX-512 | AVX2 and AVX-512 | none |

On Linux/amd64, print the matrix with:

```sh
GO111MODULE=off GOALLC_SIMD_FMV_PRINT=1 GOEXPERIMENT=simd \
  ./bin/go run -gcflags=-enablellvm ./test/llvm_simd_dispatch_fmv_amd64.dir
```

Pass a case name to run only that case:

```sh
GO111MODULE=off GOALLC_SIMD_FMV_PRINT=1 GOEXPERIMENT=simd \
  ./bin/go run -gcflags=-enablellvm ./test/llvm_simd_dispatch_fmv_amd64.dir \
  simd256-floor
```

Cases requiring unavailable hardware are skipped when running the whole
directory and rejected with an explanatory error when explicitly selected.
The companion `test/codegen/llvm_simd_dispatch_fmv_amd64.go` fixture checks
all generated Midway variants even when the host cannot execute every width.
