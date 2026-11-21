# LLVM assembler backend

## Deferred Plan 9 carrier prototype

`-enablellvm -llvm-plan9` selects a source-carrying path. Function definitions,
declarations and DATA/GLOBL objects become LLVM IR. Each naked function contains
one inline-asm block with preprocessed Plan 9 instructions, local labels,
alignment and explicit inline data. TEXT, PCDATA and FUNCDATA are not in that
block. A versioned `go-plan9-context` attribute carries the original frame/ABI
context, source positions and PC directive events. Symbol dependencies are real
LLVM operands; their original names are only lookup keys for Go's parser.

The frontend parses and preprocesses once to discover ABI facts, implicit calls
and generated globals, but does not encode function instructions for IR/bitcode
output. At AsmPrinter time the matching LLVM extension calls the Go assembler,
which reconstructs TEXT context programmatically, parses the body and performs
native preprocessing/encoding. The callback returns bytes, native Go relocations
and final PC events. LLVM emits bytes directly and its GoObj writer consumes
those relocation records without MC instruction decoding or target asm parsing.
The Plan 9 path has no x/arch or LLVM disassembler dependency.

With the new assembler binary:

```
asm -enablellvm -llvm-plan9 -p example.com/pkg -o code.o code.s
asm -enablellvm -llvm-plan9 -llvm-output=ir -p example.com/pkg -o code.ll code.s
asm -enablellvm -llvm-plan9 -llvm-output=bc -p example.com/pkg -o code.bc code.s
opt -passes='lto-pre-link<O2>,lto<O2>' code.bc -o optimized.bc
asm -enablellvm -llvm-input -o optimized.o optimized.bc
```

`-llvm-input` accepts text IR or bitcode; original assembly/include files are not
needed. The matching Go driver registers `LLVMGoSetPlan9Assembler` for the
codegen OS thread. Standalone `llc` has no Go assembler registered and rejects
these carriers with a specific diagnostic. Native ELF/Mach-O output and loading
the callback in standalone LTO workers are not implemented by this prototype.
The opt pipeline test is not an end-to-end FullLTO/ThinLTO link.

The serialized carrier restores target settings and experiment configuration and
requires the originating Go version. Current targets are Linux/Darwin amd64/arm64,
with GoObj output and the ordinary Go linker. The Go-specific ABI carrier retains
the existing opaque function signature contract; TEXT argument byte counts are
not treated as typed LLVM parameter lists.

Tests cover byte/relocation and PCSP/PCDATA parity, explicit IR symbol references,
bitcode reload, symbol renaming, LTO optimization followed by materialization,
ADX/EVEX/LSE/SHA512 instructions, and host execution with stack growth and GC.
`GOALLC_ASM_TEST_AMD64=1` includes execution under Rosetta on an arm64 Mac.

## Encoded-stream backend

This experimental backend reuses Go's parser, preprocessing and native instruction
encoder. It passes the resulting bytes and relocations to LLVM's MCDisassembler,
restores symbolic operands and local labels, and uses MCInstPrinter to produce
one inline-asm block per naked LLVM function. LLVM's integrated assembler and
GoObj writer produce the object; the ordinary Go linker builds the executable.
There is no hand-written Plan 9 instruction translation table, external
assembler/disassembler process, or intermediate native object file.

Build Go with the matching LLVM changes (the `.goobj.asm*` MC directives, naked
Go ABI support, assembly relocation merging and layout assertions). Enable it
for selected packages:

```
go test -asmflags='example.com/pkg=-enablellvm' example.com/pkg
go tool asm -enablellvm -p example.com/pkg -o code.o code.s
go tool asm -enablellvm -llvm-output=ir -p example.com/pkg -o code.ll code.s
go tool asm -enablellvm -llvm-output=bc -p example.com/pkg -o code.bc code.s
```

The default assembler remains the Go encoder and Go object writer. `-gensymabis`
uses the existing parser. Only `obj` output is accepted by the Go linker;
IR/bitcode output exposes the naked inline-asm carrier for future LTO integration.

## Ownership

* Go owns instruction selection, pseudo-instruction expansion, ABI0/ABIInternal,
  FP/SP addressing, frames, stack checks, literal pools and unsafe-point marking.
  The bridge reads the final instruction list after native encoding. AArch64,
  like x86, records each Prog's encoded extent in `Isize`.
* LLVM MC owns instruction decoding and target syntax. The C++ bridge uses
  MCSymbolizer to restore symbolic operands; Go supplies only relocation-family
  mappings. x86 PC-relative addends account for immediates after a displacement;
  AArch64 composite relocations supply page and low-12 expressions.
* External symbols are inline-asm operands and local labels use LLVM's unique
  inline-asm expansion ID. Numeric PC-relative targets become labels before
  printing, so relaxation cannot leave an old branch displacement behind.
* Explicit BYTE/WORD data, literal pools, trailing padding and standalone x86
  prefixes retain their original bytes. PCALIGN/PCALIGNMAX remain alignment
  directives. An undecodable instruction or unmatched relocation is an error,
  never an implicit raw-byte fallback.
* `.goobj.asmfunc`, `.goobj.asmpc` and `.goobj.asmfuncdata` carry frame and runtime
  metadata using MC symbols. SP changes apply after instructions; PCDATA and
  source positions apply at labels. GoObj resolves positions after relaxation,
  preserves sparse FUNCDATA and emits indirect-call markers. Assembly-owned
  AArch64 relocation pairs remain composite for Go's dynamic-import handling.
* References to a local assembly function plus an interior offset are allowed
  only at recorded boundaries, with a final MC layout assertion (PC event -4).
  If re-encoding changes that offset, object emission fails. Known external
  function interior references cannot be validated and are rejected.
* Naked bodies retain the frontend's frame/leaf facts. Missing assembly stack
  maps are not replaced by empty compiler-generated maps.

## Coverage and limits

Initial targets are amd64 and arm64 on Linux/Darwin. Coverage comes from Go's
encoder and LLVM's decoder/printer, including SIMD and atomics, rather than a
second instruction table. Optional AArch64 ISA features are enabled only on the
naked assembly carriers and decoder; callers still own CPU dispatch, just as
with native Go assembly. This does not change the compiler's baseline ISA.

Dynamic Go library binding is rejected. Assembly DWARF is deferred; Go runtime
PC/file/line and traceback metadata are emitted. GoObj assembly directives still
need a separate native-object metadata path before whole-program ThinLTO/Full
LTO linking can work. The IR optimization test verifies the carrier contract,
not a complete LTO link. Raw instruction/data directives remain the author's
responsibility; numeric PC dependencies inside raw bytes cannot be recovered.

## Validation

The regression suite compares native and LLVM machine code, relocations and
runtime metadata for both architectures. It covers immediate/sign extension,
SIMD, cryptography, atomics, literal pools, x86 prefixes, PC-relative references
with trailing immediates, long/local branches, interior address assertions,
ABIInternal pointers in DATA, signed frame metadata, and raw data directives.
Host tests link real Go callers and exercise ABI0 wrappers, stack growth, GC and
traceback. `GOALLC_ASM_TEST_AMD64=1` additionally enables amd64 execution under
Rosetta on an arm64 macOS host.

Validation results for the exact PR commits are recorded in the PR descriptions.
Linux has local object/codegen coverage, not executable runtime coverage.
