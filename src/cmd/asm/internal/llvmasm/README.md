# LLVM assembler backend

This experimental backend translates parsed and preprocessed `obj.Prog`
instructions into target syntax inside naked LLVM functions. LLVM's integrated
assembler and GoObj writer produce the object. The ordinary Go linker still
builds the executable. No instruction bytes from the Go encoder, disassembler,
or external assembler are used in this path.

Build Go with the matching LLVM changes (the `.goobj.asm*` MC directives and
naked Go ABI support). Enable it for selected packages:

```
go test -asmflags='example.com/pkg=-enablellvm' example.com/pkg
go tool asm -enablellvm -p example.com/pkg -o code.o code.s
go tool asm -enablellvm -llvm-output=ir -p example.com/pkg -o code.ll code.s
go tool asm -enablellvm -llvm-output=bc -p example.com/pkg -o code.bc code.s
```

The default assembler remains the Go encoder. `-gensymabis` uses the existing
parser. Only `obj` output is accepted by the Go linker; IR/bitcode output exposes
the carrier for inspection and future LTO integration.

## Ownership

* Go preprocessing owns ABI0/ABIInternal selection, FP/SP addressing, Go frame
  setup/teardown, stack growth checks, wrappers, and inserted FUNCDATA/PCDATA.
* LLVM owns encoding, relaxation, relocations and GoObj serialization. Symbol
  references are inline-asm operands, so LLVM can rename them consistently.
* `.goobj.asmfunc`, `.goobj.asmpc` and `.goobj.asmfuncdata` carry frame and runtime
  metadata using MC symbols. PCSP changes apply after instructions; PCDATA and
  source positions apply at labels. The writer resolves positions after
  relaxation, preserves sparse FUNCDATA slots and emits indirect-call markers.
* LLVM must not infer leafness or synthesize frame/argument maps for these naked
  bodies. In particular, missing assembly stack maps are not replaced by empty
  compiler stack maps.

## Current coverage

Initial targets are amd64 and arm64 on Linux/Darwin. Integer operations,
branches, ordinary memory and symbol references, data definitions, automatic Go
frames and runtime metadata have differential tests. Host tests link a real Go
program and exercise ABI0 calls, signed extension, stack growth, GC and traceback
through an assembly frame. Both architectures have IR optimization/codegen tests.

This is not yet a replacement for every runtime/standard-library assembly file:
SIMD, atomics and some addressing/operand forms still need instruction-family
coverage. Dynamic Go library binding is rejected. DWARF generation for these
assembly bodies is deferred; Go PC/file/line and traceback metadata are emitted.
Native ELF/Mach-O output and whole-program ThinLTO/Full LTO linking are deferred.

## Validation (2026-09-29)

Based on Go `743a300e26` and LLVM `267a23e09bf5`, with the changes in these
worktrees:

* Complete `make.bash` with the matching LLVM payload succeeded.
* `go test cmd/asm/... cmd/internal/obj` passed using the rebuilt toolchain's
  default LLVM compiler. The same focused tests also passed with the native Go
  compiler (`-gcflags=all=-enablellvm=false`).
* `GOALLC_ASM_TEST_AMD64=1` additionally exercised darwin/amd64 execution under
  Rosetta on the arm64 host; both native and LLVM assembler variants passed.
* LLVM `MC/GoObj`: 11/11 passed.
* The broader AArch64/X86 `*go*.ll` selection passed 99/102. The same three
  failures were reproduced after rebuilding the unmodified LLVM base:
  `AArch64/go-statepoint-stack-results.ll`, `AArch64/goobj-abi.ll`, and
  `X86/goobj-stack-growth.ll`. They are not changed by this patch.

Linux has object/codegen coverage in this run, not executable runtime coverage.
The optimization test checks the naked function carrier contract; it does not
qualify a complete ThinLTO or Full LTO Go link.
