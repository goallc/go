# objview

`objview` inspects Go object files and package archives. It has three output
formats:

```sh
go tool objview -format=raw package.a
go tool objview -format=text package.a
go tool objview -format=json package.a
```

`text`, the default, is a function-oriented disassembly for people. It uses
the standard Go architecture decoders and shows source positions, function
offsets, object PCs, instruction encodings, decoded instructions, `pcsp`, and
every `PCDATA` stream. Selected stack-map indices are joined to
`ArgsPointerMaps` and `LocalsPointerMaps`; entry, ordinary-call, and
stack-growth safe points are distinguished. `StackObjects` and argument
liveness are decoded separately rather than conflated with pointer maps.
Decoded bitmaps use a fixed-width `0`/`1` string with bit index 0 at the left,
matching Go GC bitmap names such as `runtime.gcbits.01`; `-` denotes a
zero-width bitmap.

`raw` is a deterministic, side-by-side view of the Go object encoding. The
left column contains the exact hexadecimal bytes of each header and block; the
right column explains symbols, hashes, indices, relocations, auxiliary
references, data records, and decoded function metadata using the same strict
parser as JSON and text.

`json` is the deterministic structured representation intended for
compiler-backend comparisons. The existing `-json` flag remains an alias for
`-format=json`.

The JSON representation includes:

- every archive member, with its kind, size, and content digest;
- the Go object header and every block boundary;
- imports, package and file tables;
- all symbol classes, symbol attributes, hashes, data, relocations, auxiliary
  records, reference flags, and reference names;
- resolved local, hashed, builtin, non-package, and imported symbol references;
- `FuncInfo`, file tables, and inline trees;
- decoded `pcsp`, `pcfile`, `pcline`, `pcinline`, and indexed `PCDATA` ranges;
- the architecture PC quantum and normalized stack-map queries at each call
  relocation (`return_pc`, runtime lookup PC `return_pc-1`, and the selected
  stack-map index), while retaining the complete raw stack-map-index ranges;
- pointer stack maps and stack object records, including GC bitmap
  relocations.

The parser validates the binary block layout and all index and string
boundaries before calling the unsafe zero-copy accessors in
`cmd/internal/goobj`. Malformed objects therefore produce an error instead of
an out-of-bounds panic.

The JSON representation deliberately preserves raw symbol data alongside
semantic decodings. DWARF payloads, open-coded defer data,
argument-liveness payloads, wrapper metadata, WebAssembly metadata, and SEH
unwind payloads are not yet semantically decoded in JSON; they remain
losslessly available through symbol data, relocations, and auxiliary
references. Text mode additionally interprets argument liveness. These
boundaries are also reported in the top-level Go object `limitations` field.

Text mode is strict and buffers a complete result before writing it. An
unsupported architecture, unknown instruction, malformed PC table, invalid
stack map, or unmatched call relocation therefore fails with member,
function, PC, and metadata-table context as applicable instead of producing
partial disassembly.
