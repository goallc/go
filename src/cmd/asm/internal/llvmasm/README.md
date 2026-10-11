# LLVM assembler backend

This experimental backend uses Go's Plan 9 parser, preprocessing, instruction
encoder and PC-table generator. Functions and data become LLVM IR definitions.
Each naked function contains native inline assembly with a **fixed layout**:
ordinary instructions retain Go's bytes, and only relocation-bearing instructions
are decoded and printed with symbolic LLVM operands. LLVM MC produces GoObj,
ELF or Mach-O. No Plan 9 callback or parser is needed when materializing bitcode.

```
# GoObj remains the default and works with the ordinary Go linker.
go test -asmflags='example.com/pkg=-enablellvm' example.com/pkg

# Native objects (the assembler must be built with the LLVM bindings).
GOOS=linux GOARCH=amd64 go tool asm -enablellvm -llvm-objfmt=elf \
    -p example.com/pkg -o code.o code_amd64.s
GOOS=darwin GOARCH=arm64 go tool asm -enablellvm -llvm-objfmt=macho \
    -p example.com/pkg -o code.o code_arm64.s

# Save a self-contained carrier for the same target.
GOOS=darwin GOARCH=arm64 go tool asm -enablellvm -llvm-objfmt=macho \
    -llvm-output=bc -p example.com/pkg -o code.bc code_arm64.s
llc -filetype=obj code.bc -o code.o
```

`-llvm-output=ir` writes readable LLVM IR. `-llvm-objfmt` accepts `goobj`, `elf`
(Linux) or `macho` (Darwin), on amd64 and arm64. A carrier is target-specific:
its machine bytes, TLS ABI and relocation expressions cannot be retargeted by
changing the triple. The standalone `llc` must support GoALLC's Go calling
conventions; it does not need an in-process Go assembler or original source.
The default assembler without `-enablellvm` remains unchanged. `-gensymabis`
continues to use the ordinary parser.

## Fixed-layout contract

* Go owns preprocessing, ABI0/ABIInternal, FP/SP addressing, stack checks,
  prologues/epilogues, local branches, alignment padding, literal pools and all
  PC tables. `obj.AssembleText` is shared with the ordinary Go output path.
* Ordinary code is emitted as `.byte` (x86) or `.inst` (AArch64). Explicit data
  and literal pools retain data directives. Optional instructions such as ADX,
  AVX, LSE and SHA512 therefore do not require a second decoder to accept them.
  This is intentional encoding preservation, not a fallback after a decode
  failure. A symbolic instruction that cannot be decoded is rejected.
* LLVM MC decodes only the relevant instruction spans inside Progs that contain
  relocations. A structured MC expression prints the LLVM operand index and
  target relocation modifier; no printed symbol names or numeric immediates
  are rewritten. Literal `$` characters are escaped separately from operands.
* Native syntax differences (ELF low-12/GOT/TLS expressions versus Mach-O
  PAGE/PAGEOFF/GOTPAGE expressions) are centralized in `relocations.go`.
  x86 field-relative addends account for trailing immediates. Near symbolic
  jumps use `{disp32}` so LLVM cannot choose a short jump and relax it later.
* MC assertions check the function entry, every replacement endpoint and the
  complete inline body. All offsets must match Go's original encoding; a size
  change is an error. LLVM function alignment includes every PCALIGN requirement.
  Raw local branches and interior function addresses consequently keep their
  original meaning.
  Native ELF/Mach-O output rejects an explicit AArch64 ADRP without a symbolic
  relocation because moving the function can change its page-relative meaning.
  GoObj preserves Go's original unrelocated encoding, matching the ordinary
  assembler's contract.
* Darwin/amd64's target-less `R_TLS_LE` is the fixed system-reserved Go TLS slot,
  not a native Mach-O TLV relocation. Its displacement is resolved using the
  same `objabi.DarwinAMD64TLSOffset` constant as the Go linker. ELF local-exec
  and initial-exec accesses instead produce normal native TLS relocations.
* Globals use LLVM constants and symbol references. LLVM selects ordinary native
  data/BSS/readonly-with-relocations sections; Go's finer data kinds are retained
  in metadata. FIPS-designated sections retain separate target-specific names.
  Synthesized content-addressable constants keep Go's content hashes in GoObj;
  native objects use private constants to avoid duplicate public definitions.
* The `-shared` PIC mode is also recorded in the standard LLVM module flags,
  so standalone bitcode codegen retains it.

## Go runtime metadata

GoObj output retains its existing `.goobj.asm*` MC adapter. Its PC labels are
aliases of the function start plus fixed offsets, and its indirect-call markers
preserve the native Go offsets, including offsets inside x86 REX prefixes.
The GoObj output path is executable with the current Go linker. It also emits
Go-compatible DWARF function DIEs and line programs from `.goobj.asmdebug` and
`.goobj.asmline` records, preserving wrapper/static identity, declaration files,
statement markers and prologue boundaries. The DWARF version follows Go's
experiment setting. These are GoObj metadata directives; bitcode codegen does
not call back into the Go parser or instruction encoder.

Go symbol flags share `obj.Link.SymbolFlags` with the ordinary writer and are
carried through `!goobj.symbol.flags.exact`: LLVM
internal linkage does not imply Go's independent LOCAL flag. The PCINLINE
auxiliary is retained even when it contains only the absent-inline value -1.
GoObj `-shared`, `-dynlink` and `-linkshared` preserve Go's preprocessing and
relocation semantics. They are checked against ordinary assembler objects.
`-S` (including repeated flags) and `-gensymabis` keep the ordinary assembler's
listing and ABI-discovery behavior.

Native output has no `.goobj.*` directives. It carries one private, packed LLVM
constant in `.go.asm` (ELF) or `__DATA_CONST,__go_asm` (Mach-O). Machine relocations
are produced by LLVM MC; original Go relocation records, including target-less
semantic records, are also preserved in this metadata. Address fields are LLVM
symbol references, so bitcode linking and renaming update them automatically.
Symbol names in the record are the original Go names for identity/diagnostics.

The v1 native metadata format is little-endian and currently requires eight-byte
symbol-reference slots. A slot is the native symbol value (a TLS symbol can have
a TLS-relative value), not necessarily a process address. All integer fields
are unsigned bit patterns; signed Go fields retain their two's-complement bits.
A `blob` is a u32 byte count followed by those bytes, and strings are UTF-8 blobs.
A `ref` first pads the stream to eight-byte alignment, then contains an eight-byte
LLVM pointer constant, or zero with no relocation for an absent reference.

The module record is:

1. Eight bytes `GoAsm\0\1\0`, u64 total byte length, u32 GoObj module flags,
   u32 file count, u32 symbol count, u32 reserved zero.
2. Four strings: Go toolchain version, target triple, package path, enabled
   experiments joined with commas; then the file-count filename strings in
   Go's compilation-unit file order.
3. Symbol records, each starting and ending at eight-byte alignment. Their
   lengths include trailing padding. The module ends at eight-byte alignment.

Each symbol record is:

1. u64 record byte length, ref symbol, ref optional Go type descriptor, u64 size;
   u32 Go symbol kind, u32 requested alignment, u32 semantic `obj.Attribute`
   bits, u32 Go ABI; original Go name string.
2. u32 original Go relocation count. Each entry contains u32 offset, u32 type,
   u32 width, u32 reserved zero, u64 addend, ref target. These are descriptive
   Go records, not a second set of native relocations to apply to linked code.
3. u32 function-payload-present. For a function: u32 args, locals, FuncID,
   FuncFlag and StartLine; four blobs for PCSP, PCFILE, PCLINE and PCINLINE;
   u32 used-file count followed by those u32 indices; u32 PCDATA count followed
   by its blobs; u32 FUNCDATA count followed by its refs, including absent slots.

Go enum/attribute numbering is tied to the recorded toolchain version. A future
consumer must validate this version and the record format before interpreting
those fields. `llvm.used` conservatively retains the module record and its
referenced functions/data. Native metadata pruning and reconciliation of weak
or duplicate definitions require the future Go-aware native linker integration.

## Validation and limits

The tests compare machine bytes (excluding only native relocation bits),
function sizes, native relocation kinds, original Go relocations, PC tables and
sparse FUNCDATA. They cover ELF/Mach-O on both architectures, symbol renaming,
bitcode materialization at O0, LTO optimization passes, TLS, optional ISA,
interior addresses, alignment and rejected layout changes. GoObj comparisons
also check complete symbol flags, relocation identities, sparse auxiliaries,
PCINLINE, DWARF 4/5 bytes and relocations, and shared/dynamic-link options.
Linked Go executables are opened with `debug/dwarf` to verify assembly function
DIEs and their source-line mappings. Native host-link
probes execute no-argument leaf functions and pointer-bearing constants with
Clang. GoObj host tests exercise real Go ABI wrappers, stack growth, GC and
traceback; `GOALLC_ASM_TEST_AMD64=1` enables the amd64 lanes under Rosetta.

Producing a native object is not yet a complete native Go linking pipeline.
The current Go linker does not read this native metadata format, and ordinary
LLD/system linkers do not build Go's final pclntab/moduledata from it. Native
Go runtime metadata ingestion, linker pruning/ICF/relaxation policy, native Go
unwind/debug information and end-to-end FullLTO/ThinLTO links remain separate
work. Do not enable transformations that change a carrier function's internal
layout. Native ELF/Mach-O output still rejects Go dynamic-library binding
(`-dynlink`/`-linkshared`); that restriction does not apply to GoObj output.

## GoObj parity qualification (2026-10-10)

The qualified scope is Linux/Darwin on amd64/arm64. The Go compiler is held in
native mode so the changed assembler can be compared independently.

* Go assembler/object/disassembly suites passed. Object comparisons cover
  normal, shared, dynlink and linkshared modes on all four targets. DWARF 4/5
  and linked source-line lookup passed on both Darwin architectures.
* Darwin arm64 `go test std`: 261 packages passed, 119 had no test files; `net`
  failed four connection/DNS tests. All four failures reproduced with the
  ordinary assembler. The full runtime suite passed.
* Darwin amd64 under Rosetta: full runtime runs with both assemblers failed
  only `TestMmapErrorSign` (errno 22 instead of 12). No skip was added.
* Darwin arm64/amd64: executable, PIE, c-archive, c-shared and plugin probes
  built and executed with both assemblers (20 combinations).
* Linux amd64: cross-built runtime tests were executed on Ubuntu. Thirteen
  stack-growth, traceback, GC and memory-stat tests passed with both assemblers.
  This is execution coverage, not a full Linux standard-library qualification.

Other operating systems/architectures and end-to-end native-object Go linking
remain outside this qualification. Passing LTO optimization/materialization
checks does not establish complete FullLTO/ThinLTO linking.
