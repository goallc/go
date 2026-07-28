# objview

`objview` inspects Go object files and package archives. Its original output is
a side-by-side hexadecimal and textual diagnostic dump. `-json` emits a
deterministic structured representation intended for compiler-backend
comparisons:

```sh
go tool objview -json package.a
go tool objview -json _go_.o
```

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

The representation deliberately preserves raw symbol data alongside semantic
decodings. DWARF payloads, open-coded defer data, argument-liveness payloads,
wrapper metadata, WebAssembly metadata, and SEH unwind payloads are not yet
semantically decoded; they remain losslessly available through symbol data,
relocations, and auxiliary references. These boundaries are also reported in
the top-level Go object `limitations` field.
