// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmd/internal/archive"
	"cmd/internal/goobj"
	"cmd/internal/objabi"
	"cmd/internal/sys"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"internal/abi"
	"io"
	"os"
	"strings"

	"golang.org/x/arch/x86/x86asm"
)

// canonicalDocument is intentionally made entirely from structs and slices.
// encoding/json therefore emits fields and records in a stable order, making
// the output suitable for semantic comparisons between compiler backends.
type canonicalDocument struct {
	Format  string            `json:"format"`
	Archive bool              `json:"archive"`
	Members []canonicalMember `json:"members"`
}

type canonicalMember struct {
	Name            string                    `json:"name,omitempty"`
	Kind            string                    `json:"kind"`
	Size            int64                     `json:"size"`
	SHA256          string                    `json:"sha256"`
	ArchiveMetadata *canonicalArchiveMetadata `json:"archive_metadata,omitempty"`
	TextHeader      string                    `json:"text_header,omitempty"`
	GOOS            string                    `json:"goos,omitempty"`
	Arch            string                    `json:"arch,omitempty"`
	GoVersion       string                    `json:"go_version,omitempty"`
	BuildOptions    []string                  `json:"build_options,omitempty"`
	GoObject        *canonicalGoObject        `json:"go_object,omitempty"`

	rawEntry []byte
}

type canonicalArchiveMetadata struct {
	Mtime int64  `json:"mtime"`
	UID   int    `json:"uid"`
	GID   int    `json:"gid"`
	Mode  uint32 `json:"mode"`
}

type canonicalGoObject struct {
	Header      canonicalHeader     `json:"header"`
	Blocks      []canonicalBlock    `json:"blocks"`
	Autolib     []canonicalImport   `json:"autolib"`
	Packages    []string            `json:"packages"`
	Files       []string            `json:"files"`
	Symbols     []canonicalSymbol   `json:"symbols"`
	References  []canonicalSymbol   `json:"references"`
	RefFlags    []canonicalRefFlags `json:"reference_flags"`
	RefNames    []canonicalRefName  `json:"reference_names"`
	Limitations []string            `json:"limitations,omitempty"`

	rawData []byte
}

type canonicalHeader struct {
	Magic       string   `json:"magic"`
	Fingerprint string   `json:"fingerprint"`
	Flags       uint32   `json:"flags"`
	FlagNames   []string `json:"flag_names"`
}

type canonicalBlock struct {
	Name   string `json:"name"`
	Offset uint32 `json:"offset"`
	Size   uint32 `json:"size"`
	SHA256 string `json:"sha256"`
}

type canonicalImport struct {
	Package     string `json:"package"`
	Fingerprint string `json:"fingerprint"`
}

type canonicalSymbol struct {
	Index       uint32             `json:"index"`
	Class       string             `json:"class"`
	ClassIndex  uint32             `json:"class_index"`
	Name        string             `json:"name"`
	ABI         uint16             `json:"abi"`
	Kind        string             `json:"kind"`
	KindValue   uint8              `json:"kind_value"`
	Flags       uint8              `json:"flags"`
	FlagNames   []string           `json:"flag_names"`
	Flags2      uint8              `json:"flags2"`
	Flag2Names  []string           `json:"flag2_names"`
	Size        uint32             `json:"size"`
	Align       uint32             `json:"align"`
	Hash        string             `json:"hash,omitempty"`
	Data        string             `json:"data_hex,omitempty"`
	DataSHA256  string             `json:"data_sha256,omitempty"`
	Relocations []canonicalReloc   `json:"relocations"`
	Aux         []canonicalAux     `json:"aux"`
	Function    *canonicalFunction `json:"function,omitempty"`

	dataOffset uint64
	rawData    []byte
}

type canonicalReference struct {
	PkgIndex uint32 `json:"pkg_index"`
	PkgKind  string `json:"pkg_kind"`
	SymIndex uint32 `json:"sym_index"`
	Package  string `json:"package,omitempty"`
	Name     string `json:"name,omitempty"`
	ABI      *int   `json:"abi,omitempty"`
}

type canonicalReloc struct {
	Offset    int32              `json:"offset"`
	Size      uint8              `json:"size"`
	Type      string             `json:"type"`
	TypeValue uint16             `json:"type_value"`
	Weak      bool               `json:"weak"`
	Addend    int64              `json:"addend"`
	Target    canonicalReference `json:"target"`
}

type canonicalAux struct {
	Index     int                `json:"index"`
	Type      string             `json:"type"`
	TypeValue uint8              `json:"type_value"`
	Target    canonicalReference `json:"target"`
}

type canonicalRefFlags struct {
	Target     canonicalReference `json:"target"`
	Flags      uint8              `json:"flags"`
	FlagNames  []string           `json:"flag_names"`
	Flags2     uint8              `json:"flags2"`
	Flag2Names []string           `json:"flag2_names"`
}

type canonicalRefName struct {
	Target canonicalReference `json:"target"`
	Name   string             `json:"name"`
}

type canonicalFunction struct {
	Info            *canonicalFuncInfo       `json:"info,omitempty"`
	PCQuantum       int                      `json:"pc_quantum"`
	PCTables        []canonicalPCTable       `json:"pc_tables"`
	PCData          []canonicalPCTable       `json:"pcdata"`
	FuncData        []canonicalFuncData      `json:"funcdata"`
	StackMapQueries []canonicalStackMapQuery `json:"stack_map_queries"`
}

type canonicalFuncInfo struct {
	Args      uint32                  `json:"args"`
	Locals    uint32                  `json:"locals"`
	FuncID    uint8                   `json:"func_id"`
	FuncFlags uint8                   `json:"func_flags"`
	FlagNames []string                `json:"flag_names"`
	StartLine int32                   `json:"start_line"`
	Files     []canonicalFuncInfoFile `json:"files"`
	InlTree   []canonicalInlTreeNode  `json:"inline_tree"`
}

type canonicalFuncInfoFile struct {
	Index uint32 `json:"index"`
	Name  string `json:"name,omitempty"`
}

type canonicalInlTreeNode struct {
	Parent   int32              `json:"parent"`
	File     uint32             `json:"file"`
	FileName string             `json:"file_name,omitempty"`
	Line     int32              `json:"line"`
	Func     canonicalReference `json:"func"`
	ParentPC int32              `json:"parent_pc"`
}

type canonicalPCTable struct {
	Index  int                `json:"index"`
	Kind   string             `json:"kind"`
	Symbol canonicalReference `json:"symbol"`
	Raw    string             `json:"raw_hex,omitempty"`
	Ranges []canonicalPCRange `json:"ranges,omitempty"`
	Error  string             `json:"decode_error,omitempty"`
}

type canonicalPCRange struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
	Value int32  `json:"value"`
	File  string `json:"file,omitempty"`
}

// canonicalStackMapQuery normalizes the runtime's stack-map lookup at a call
// return PC. The raw PCDATA ranges remain in PCData; this view captures their
// observable meaning even when two backends encode differently shaped ranges.
type canonicalStackMapQuery struct {
	CallOffset      int32              `json:"call_offset"`
	InstructionSize uint8              `json:"instruction_size"`
	ReturnPC        uint64             `json:"return_pc"`
	LookupPC        uint64             `json:"lookup_pc"`
	StackMapIndex   int32              `json:"stack_map_index"`
	RelocationType  string             `json:"relocation_type"`
	Target          canonicalReference `json:"target"`
	DecodeError     string             `json:"decode_error,omitempty"`
}

type canonicalFuncData struct {
	Index        int                    `json:"index"`
	Kind         string                 `json:"kind"`
	Symbol       canonicalReference     `json:"symbol"`
	Raw          string                 `json:"raw_hex,omitempty"`
	StackMap     *canonicalStackMap     `json:"stack_map,omitempty"`
	StackObjects []canonicalStackObject `json:"stack_objects,omitempty"`
	DecodeError  string                 `json:"decode_error,omitempty"`

	rawData []byte
}

type canonicalStackMap struct {
	Count   int32             `json:"count"`
	NumBits int32             `json:"num_bits"`
	Bitmaps []canonicalBitMap `json:"bitmaps"`
}

type canonicalBitMap struct {
	Index   int    `json:"index"`
	SetBits []int  `json:"set_bits"`
	Bytes   string `json:"bytes_hex"`
}

type canonicalStackObject struct {
	Index     int                 `json:"index"`
	Offset    int32               `json:"offset"`
	Size      int32               `json:"size"`
	PtrBytes  int32               `json:"ptr_bytes"`
	GCDataOff uint32              `json:"gcdata_offset"`
	GCData    *canonicalReference `json:"gcdata,omitempty"`

	gcDataAddend  int64
	gcBits        []int
	gcDataDecoded bool
}

type canonicalParser struct {
	data       []byte
	reader     *goobj.Reader
	header     goobj.Header
	arch       *sys.Arch
	files      []string
	packages   []string
	defs       []canonicalSymbol
	refNames   map[goobj.SymRef]string
	localIndex map[goobj.SymRef]uint32
}

var canonicalBlockNames = [...]string{
	"autolib", "package_index", "file", "symbol_def", "hashed64_def",
	"hashed_def", "nonpackage_def", "nonpackage_ref", "reference_flags",
	"hash64", "hash", "relocation_index", "aux_index", "data_index",
	"relocation", "aux", "data", "reference_name", "end",
}

func parseCanonicalFile(path string) (doc *canonicalDocument, err error) {
	defer func() {
		if p := recover(); p != nil {
			doc = nil
			err = fmt.Errorf("malformed Go object: %v", p)
		}
	}()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	a, err := archive.Parse(f, true)
	if err != nil {
		return nil, err
	}

	var prefix [8]byte
	_, _ = f.ReadAt(prefix[:], 0)
	doc = &canonicalDocument{
		Format:  "goobj-canonical",
		Archive: bytes.Equal(prefix[:], []byte("!<arch>\n")),
	}
	for _, entry := range a.Entries {
		raw, err := readAt(f, entry.Offset, entry.Size)
		if err != nil {
			return nil, fmt.Errorf("member %q: %w", entry.Name, err)
		}
		member := canonicalMember{
			Kind:   entryKind(entry.Type),
			Size:   entry.Size,
			SHA256: hashBytes(raw),
		}
		if doc.Archive {
			member.Name = entry.Name
		}
		if doc.Archive {
			member.ArchiveMetadata = &canonicalArchiveMetadata{
				Mtime: entry.Mtime, UID: entry.Uid, GID: entry.Gid, Mode: uint32(entry.Mode),
			}
		}
		if entry.Type == archive.EntryGoObj {
			member.rawEntry = raw
			member.TextHeader = string(entry.Obj.TextHeader)
			member.Arch = entry.Obj.Arch
			fields := strings.Fields(member.TextHeader)
			if len(fields) >= 5 && fields[0] == "go" && fields[1] == "object" {
				member.GOOS = fields[2]
				member.Arch = fields[3]
				member.GoVersion = fields[4]
				member.BuildOptions = fields[5:]
			}
			objectBytes, err := readAt(f, entry.Obj.Offset, entry.Obj.Size)
			if err != nil {
				return nil, fmt.Errorf("member %q Go object: %w", entry.Name, err)
			}
			member.GoObject, err = parseCanonicalGoObject(objectBytes, entry.Obj.Arch)
			if err != nil {
				return nil, fmt.Errorf("member %q: %w", entry.Name, err)
			}
		}
		doc.Members = append(doc.Members, member)
	}
	return doc, nil
}

func readAt(f *os.File, off, size int64) ([]byte, error) {
	if off < 0 || size < 0 || size > int64(^uint(0)>>1) {
		return nil, errors.New("invalid offset or size")
	}
	b := make([]byte, int(size))
	n, err := f.ReadAt(b, off)
	if err != nil && !(err == io.EOF && n == len(b)) {
		return nil, err
	}
	if n != len(b) {
		return nil, io.ErrUnexpectedEOF
	}
	return b, nil
}

func entryKind(t archive.EntryType) string {
	switch t {
	case archive.EntryPkgDef:
		return "package_definition"
	case archive.EntryGoObj:
		return "go_object"
	case archive.EntryNativeObj:
		return "native_object"
	case archive.EntrySentinelNonObj:
		return "sentinel"
	default:
		return fmt.Sprintf("entry_type_%d", t)
	}
}

func parseCanonicalGoObject(data []byte, archName string) (*canonicalGoObject, error) {
	if err := validateGoObject(data); err != nil {
		return nil, err
	}
	r := goobj.NewReaderFromBytes(data, true)
	if r == nil {
		return nil, errors.New("invalid Go object magic")
	}
	var h goobj.Header
	if err := h.Read(r); err != nil {
		return nil, err
	}
	p := &canonicalParser{
		data: data, reader: r, header: h,
		refNames:   make(map[goobj.SymRef]string),
		localIndex: make(map[goobj.SymRef]uint32),
	}
	for _, a := range sys.Archs {
		if a.Name == archName {
			p.arch = a
			break
		}
	}
	p.files = make([]string, r.NFile())
	for i := range p.files {
		p.files[i] = r.File(i)
	}
	p.packages = r.Pkglist()

	out := &canonicalGoObject{
		Header: canonicalHeader{
			Magic: h.Magic, Fingerprint: hex.EncodeToString(h.Fingerprint[:]),
			Flags: h.Flags, FlagNames: objectFlagNames(h.Flags),
		},
		Packages: p.packages,
		Files:    p.files,
		rawData:  data,
	}
	for i := goobj.BlkAutolib; i <= goobj.BlkEnd; i++ {
		start := h.Offsets[i]
		end := start
		if i < goobj.BlkEnd {
			end = h.Offsets[i+1]
		}
		name := fmt.Sprintf("block_%d", i)
		if i < len(canonicalBlockNames) {
			name = canonicalBlockNames[i]
		}
		out.Blocks = append(out.Blocks, canonicalBlock{
			Name: name, Offset: start, Size: end - start,
			SHA256: hashBytes(data[start:end]),
		})
	}
	for _, lib := range r.Autolib() {
		out.Autolib = append(out.Autolib, canonicalImport{
			Package: lib.Pkg, Fingerprint: hex.EncodeToString(lib.Fingerprint[:]),
		})
	}

	p.collectRefNames()
	p.collectDefinitions()
	p.populateDefinitionDetails()
	out.Symbols = p.defs
	out.References = p.collectReferenceSymbols()
	out.RefFlags = p.collectRefFlags()
	out.RefNames = p.collectCanonicalRefNames()
	if p.arch == nil {
		out.Limitations = append(out.Limitations,
			fmt.Sprintf("unknown architecture %q: PC tables and pointer-sized stack objects are retained as raw data", archName))
	}
	out.Limitations = append(out.Limitations,
		"DWARF, open-coded defer, argument liveness, wrapper, WebAssembly, and SEH payloads are preserved as raw symbol data but are not semantically decoded")
	return out, nil
}

func validateGoObject(data []byte) error {
	const headerSize = len(goobj.Magic) + 8 + 4 + 4*goobj.NBlk
	if len(data) < headerSize {
		return fmt.Errorf("truncated header: %d bytes", len(data))
	}
	if string(data[:len(goobj.Magic)]) != goobj.Magic {
		return errors.New("wrong magic, not a Go object file")
	}
	off := len(goobj.Magic) + 8 + 4
	var offsets [goobj.NBlk]uint32
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[off+i*4:])
		if offsets[i] > uint32(len(data)) {
			return fmt.Errorf("block %d offset %d exceeds object size %d", i, offsets[i], len(data))
		}
		if i == 0 && offsets[i] < uint32(headerSize) {
			return fmt.Errorf("first block overlaps header: %d < %d", offsets[i], headerSize)
		}
		if i > 0 && offsets[i] < offsets[i-1] {
			return fmt.Errorf("block offsets are not monotonic at block %d", i)
		}
	}
	if offsets[goobj.BlkEnd] != uint32(len(data)) {
		return fmt.Errorf("end block offset %d does not match object size %d", offsets[goobj.BlkEnd], len(data))
	}
	blockSize := func(i int) uint32 { return offsets[i+1] - offsets[i] }
	checkMultiple := func(i int, size uint32) error {
		if blockSize(i)%size != 0 {
			return fmt.Errorf("block %d size %d is not a multiple of record size %d", i, blockSize(i), size)
		}
		return nil
	}
	for _, check := range []struct {
		block int
		size  uint32
	}{
		{goobj.BlkAutolib, 16}, {goobj.BlkPkgIdx, 8}, {goobj.BlkFile, 8},
		{goobj.BlkSymdef, goobj.SymSize}, {goobj.BlkHashed64def, goobj.SymSize},
		{goobj.BlkHasheddef, goobj.SymSize}, {goobj.BlkNonpkgdef, goobj.SymSize},
		{goobj.BlkNonpkgref, goobj.SymSize}, {goobj.BlkRefFlags, goobj.RefFlagsSize},
		{goobj.BlkHash64, goobj.Hash64Size}, {goobj.BlkHash, goobj.HashSize},
		{goobj.BlkReloc, goobj.RelocSize}, {goobj.BlkAux, goobj.AuxSize},
		{goobj.BlkRefName, goobj.RefNameSize},
	} {
		if err := checkMultiple(check.block, check.size); err != nil {
			return err
		}
	}
	nDef := blockSize(goobj.BlkSymdef)/goobj.SymSize +
		blockSize(goobj.BlkHashed64def)/goobj.SymSize +
		blockSize(goobj.BlkHasheddef)/goobj.SymSize +
		blockSize(goobj.BlkNonpkgdef)/goobj.SymSize
	for _, block := range []int{goobj.BlkRelocIdx, goobj.BlkAuxIdx, goobj.BlkDataIdx} {
		if blockSize(block) != (nDef+1)*4 {
			return fmt.Errorf("block %d has %d index bytes, want %d", block, blockSize(block), (nDef+1)*4)
		}
	}
	if err := validateIndex(data, offsets, goobj.BlkRelocIdx, nDef,
		blockSize(goobj.BlkReloc)/goobj.RelocSize); err != nil {
		return fmt.Errorf("relocation index: %w", err)
	}
	if err := validateIndex(data, offsets, goobj.BlkAuxIdx, nDef,
		blockSize(goobj.BlkAux)/goobj.AuxSize); err != nil {
		return fmt.Errorf("aux index: %w", err)
	}
	if err := validateIndex(data, offsets, goobj.BlkDataIdx, nDef,
		blockSize(goobj.BlkData)); err != nil {
		return fmt.Errorf("data index: %w", err)
	}

	validateStringRef := func(at uint32) error {
		n := binary.LittleEndian.Uint32(data[at:])
		s := binary.LittleEndian.Uint32(data[at+4:])
		if uint64(s)+uint64(n) > uint64(len(data)) {
			return fmt.Errorf("string reference at %d points outside object", at)
		}
		return nil
	}
	for at := offsets[goobj.BlkAutolib]; at < offsets[goobj.BlkAutolib+1]; at += 16 {
		if err := validateStringRef(at); err != nil {
			return err
		}
	}
	for _, block := range []int{goobj.BlkPkgIdx, goobj.BlkFile} {
		for at := offsets[block]; at < offsets[block+1]; at += 8 {
			if err := validateStringRef(at); err != nil {
				return err
			}
		}
	}
	for _, block := range []int{goobj.BlkSymdef, goobj.BlkHashed64def, goobj.BlkHasheddef, goobj.BlkNonpkgdef, goobj.BlkNonpkgref} {
		for at := offsets[block]; at < offsets[block+1]; at += goobj.SymSize {
			if err := validateStringRef(at); err != nil {
				return err
			}
		}
	}
	for at := offsets[goobj.BlkRefName]; at < offsets[goobj.BlkRefName+1]; at += goobj.RefNameSize {
		if err := validateStringRef(at + 8); err != nil {
			return err
		}
	}
	return nil
}

func validateIndex(data []byte, offsets [goobj.NBlk]uint32, block int, n, limit uint32) error {
	start := offsets[block]
	var prev uint32
	for i := uint32(0); i <= n; i++ {
		v := binary.LittleEndian.Uint32(data[start+i*4:])
		if i > 0 && v < prev {
			return fmt.Errorf("entries are not monotonic at %d", i)
		}
		if v > limit {
			return fmt.Errorf("entry %d value %d exceeds limit %d", i, v, limit)
		}
		prev = v
	}
	if prev != limit {
		return fmt.Errorf("terminal value %d does not match block item count %d", prev, limit)
	}
	return nil
}

func (p *canonicalParser) collectRefNames() {
	for i := 0; i < p.reader.NRefName(); i++ {
		n := p.reader.RefName(i)
		p.refNames[n.Sym()] = n.Name(p.reader)
	}
}

func (p *canonicalParser) collectDefinitions() {
	index := uint32(0)
	addClass := func(class string, block, count int, hash func(uint32) string) {
		for i := uint32(0); i < uint32(count); i++ {
			s := parseSym(p.data[p.header.Offsets[block]+i*goobj.SymSize:])
			cs := canonicalSymbol{
				Index: index, Class: class, ClassIndex: i, Name: stringAt(p.data, s.nameOff, s.nameLen),
				ABI: s.abi, Kind: objabi.SymKind(s.kind).String(), KindValue: s.kind,
				Flags: s.flags, FlagNames: symFlagNames(s.flags),
				Flags2: s.flags2, Flag2Names: symFlag2Names(s.flags2),
				Size: s.size, Align: s.align,
			}
			if hash != nil {
				cs.Hash = hash(i)
			}
			p.defs = append(p.defs, cs)
			var ref goobj.SymRef
			switch block {
			case goobj.BlkSymdef:
				ref = goobj.SymRef{PkgIdx: goobj.PkgIdxSelf, SymIdx: i}
			case goobj.BlkHashed64def:
				ref = goobj.SymRef{PkgIdx: goobj.PkgIdxHashed64, SymIdx: i}
			case goobj.BlkHasheddef:
				ref = goobj.SymRef{PkgIdx: goobj.PkgIdxHashed, SymIdx: i}
			case goobj.BlkNonpkgdef:
				ref = goobj.SymRef{PkgIdx: goobj.PkgIdxNone, SymIdx: i}
			}
			p.localIndex[ref] = index
			index++
		}
	}
	addClass("package", goobj.BlkSymdef, p.reader.NSym(), nil)
	addClass("hashed64", goobj.BlkHashed64def, p.reader.NHashed64def(),
		func(i uint32) string {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], p.reader.Hash64(i))
			return hex.EncodeToString(b[:])
		})
	addClass("hashed", goobj.BlkHasheddef, p.reader.NHasheddef(),
		func(i uint32) string { return hex.EncodeToString(p.reader.Hash(i)[:]) })
	addClass("nonpackage", goobj.BlkNonpkgdef, p.reader.NNonpkgdef(), nil)
}

type decodedSym struct {
	nameLen, nameOff    uint32
	abi                 uint16
	kind, flags, flags2 uint8
	size, align         uint32
}

func parseSym(b []byte) decodedSym {
	return decodedSym{
		nameLen: binary.LittleEndian.Uint32(b), nameOff: binary.LittleEndian.Uint32(b[4:]),
		abi: binary.LittleEndian.Uint16(b[8:]), kind: b[10], flags: b[11], flags2: b[12],
		size: binary.LittleEndian.Uint32(b[13:]), align: binary.LittleEndian.Uint32(b[17:]),
	}
}

func stringAt(data []byte, off, n uint32) string { return string(data[off : off+n]) }

func (p *canonicalParser) populateDefinitionDetails() {
	for i := range p.defs {
		idx := uint32(i)
		data := p.reader.Data(idx)
		p.defs[i].dataOffset = uint64(p.reader.DataOff(idx))
		p.defs[i].rawData = data
		if len(data) != 0 {
			p.defs[i].Data = hex.EncodeToString(data)
			p.defs[i].DataSHA256 = hashBytes(data)
		}
		for j := 0; j < p.reader.NReloc(idx); j++ {
			r := p.reader.Reloc(idx, j)
			rawType := r.Type()
			typ := objabi.RelocType(rawType) &^ objabi.R_WEAK
			p.defs[i].Relocations = append(p.defs[i].Relocations, canonicalReloc{
				Offset: r.Off(), Size: r.Siz(), Type: typ.String(), TypeValue: uint16(typ),
				Weak:   objabi.RelocType(rawType)&objabi.R_WEAK != 0,
				Addend: r.Add(), Target: p.resolve(r.Sym()),
			})
		}
		for j := 0; j < p.reader.NAux(idx); j++ {
			a := p.reader.Aux(idx, j)
			p.defs[i].Aux = append(p.defs[i].Aux, canonicalAux{
				Index: j, Type: auxName(a.Type()), TypeValue: a.Type(), Target: p.resolve(a.Sym()),
			})
		}
		if objabi.SymKind(p.defs[i].KindValue).IsText() {
			p.defs[i].Function = p.decodeFunction(idx)
		}
	}
}

func (p *canonicalParser) collectReferenceSymbols() []canonicalSymbol {
	var out []canonicalSymbol
	start := p.header.Offsets[goobj.BlkNonpkgref]
	for i := uint32(0); i < uint32(p.reader.NNonpkgref()); i++ {
		s := parseSym(p.data[start+i*goobj.SymSize:])
		out = append(out, canonicalSymbol{
			Index: i, Class: "nonpackage_reference", ClassIndex: i,
			Name: stringAt(p.data, s.nameOff, s.nameLen), ABI: s.abi,
			Kind: objabi.SymKind(s.kind).String(), KindValue: s.kind,
			Flags: s.flags, FlagNames: symFlagNames(s.flags),
			Flags2: s.flags2, Flag2Names: symFlag2Names(s.flags2),
			Size: s.size, Align: s.align,
		})
	}
	return out
}

func (p *canonicalParser) collectRefFlags() []canonicalRefFlags {
	out := make([]canonicalRefFlags, 0, p.reader.NRefFlags())
	for i := 0; i < p.reader.NRefFlags(); i++ {
		r := p.reader.RefFlags(i)
		out = append(out, canonicalRefFlags{
			Target: p.resolve(r.Sym()), Flags: r.Flag(), FlagNames: symFlagNames(r.Flag()),
			Flags2: r.Flag2(), Flag2Names: symFlag2Names(r.Flag2()),
		})
	}
	return out
}

func (p *canonicalParser) collectCanonicalRefNames() []canonicalRefName {
	out := make([]canonicalRefName, 0, p.reader.NRefName())
	for i := 0; i < p.reader.NRefName(); i++ {
		n := p.reader.RefName(i)
		out = append(out, canonicalRefName{Target: p.resolve(n.Sym()), Name: n.Name(p.reader)})
	}
	return out
}

func (p *canonicalParser) resolve(ref goobj.SymRef) canonicalReference {
	out := canonicalReference{PkgIndex: ref.PkgIdx, SymIndex: ref.SymIdx}
	switch ref.PkgIdx {
	case goobj.PkgIdxInvalid:
		out.PkgKind = "invalid"
	case goobj.PkgIdxSelf:
		out.PkgKind = "self"
	case goobj.PkgIdxHashed64:
		out.PkgKind = "hashed64"
	case goobj.PkgIdxHashed:
		out.PkgKind = "hashed"
	case goobj.PkgIdxNone:
		out.PkgKind = "none"
	case goobj.PkgIdxBuiltin:
		out.PkgKind = "builtin"
		if int(ref.SymIdx) < goobj.NBuiltin() {
			out.Name, _ = goobj.BuiltinName(int(ref.SymIdx))
			_, abiValue := goobj.BuiltinName(int(ref.SymIdx))
			out.ABI = &abiValue
		}
	default:
		out.PkgKind = "imported"
		if int(ref.PkgIdx) < len(p.packages) {
			out.Package = p.packages[ref.PkgIdx]
		}
	}
	if local, ok := p.localIndex[ref]; ok && int(local) < len(p.defs) {
		out.Name = p.defs[local].Name
		abiValue := int(p.defs[local].ABI)
		out.ABI = &abiValue
	}
	if name, ok := p.refNames[ref]; ok {
		out.Name = name
	}
	return out
}

func (p *canonicalParser) localData(ref goobj.SymRef) ([]byte, uint32, bool) {
	i, ok := p.localIndex[ref]
	if !ok {
		return nil, 0, false
	}
	return p.reader.Data(i), i, true
}

func (p *canonicalParser) decodeFunction(index uint32) *canonicalFunction {
	f := &canonicalFunction{}
	if p.arch != nil {
		f.PCQuantum = p.arch.MinLC
	}
	pcdataIndex, funcdataIndex := 0, 0
	for j := 0; j < p.reader.NAux(index); j++ {
		a := p.reader.Aux(index, j)
		ref := a.Sym()
		data, _, local := p.localData(ref)
		switch a.Type() {
		case goobj.AuxFuncInfo:
			if local {
				f.Info = p.decodeFuncInfo(data)
			}
		case goobj.AuxPcsp, goobj.AuxPcfile, goobj.AuxPcline, goobj.AuxPcinline:
			t := p.decodePCTable(auxName(a.Type()), -1, ref, data, local)
			f.PCTables = append(f.PCTables, t)
		case goobj.AuxPcdata:
			t := p.decodePCTable(pcdataName(pcdataIndex), pcdataIndex, ref, data, local)
			f.PCData = append(f.PCData, t)
			pcdataIndex++
		case goobj.AuxFuncdata:
			fd := p.decodeFuncData(funcdataIndex, ref, data, local)
			f.FuncData = append(f.FuncData, fd)
			funcdataIndex++
		}
	}
	var stackMapRanges []canonicalPCRange
	if table := findCanonicalPCTable(f.PCData, "stack_map_index"); table != nil {
		stackMapRanges = table.Ranges
	}
	for j := 0; j < p.reader.NReloc(index); j++ {
		reloc := p.reader.Reloc(index, j)
		typ := objabi.RelocType(reloc.Type()) &^ objabi.R_WEAK
		if !typ.IsDirectCall() && typ != objabi.R_CALLIND {
			continue
		}
		if reloc.Off() < 0 {
			continue
		}
		query := canonicalStackMapQuery{
			CallOffset: reloc.Off(), RelocationType: typ.String(),
			Target: p.resolve(reloc.Sym()), StackMapIndex: -1,
		}
		size, err := p.callInstructionSize(index, reloc, typ)
		if err != nil {
			query.DecodeError = err.Error()
			f.StackMapQueries = append(f.StackMapQueries, query)
			continue
		}
		query.InstructionSize = size
		returnPC := uint64(reloc.Off()) + uint64(size)
		lookupPC := returnPC - 1
		query.ReturnPC = returnPC
		query.LookupPC = lookupPC
		query.StackMapIndex = lookupPCValue(stackMapRanges, lookupPC)
		f.StackMapQueries = append(f.StackMapQueries, query)
	}
	return f
}

func (p *canonicalParser) callInstructionSize(index uint32, reloc *goobj.Reloc, typ objabi.RelocType) (uint8, error) {
	if reloc.Siz() != 0 {
		return reloc.Siz(), nil
	}
	if typ != objabi.R_CALLIND {
		return 0, fmt.Errorf("call relocation %s has no size", typ)
	}
	if p.arch == nil {
		return 0, errors.New("unknown architecture")
	}
	off := int(reloc.Off())
	code := p.reader.Data(index)
	if off < 0 || off >= len(code) {
		return 0, fmt.Errorf("call offset %d is outside %d-byte function", off, len(code))
	}
	switch p.arch.Name {
	case "386", "amd64":
		mode := 32
		if p.arch.Name == "amd64" {
			mode = 64
		}
		inst, err := x86asm.Decode(code[off:], mode)
		if err != nil {
			return 0, fmt.Errorf("decode indirect call: %w", err)
		}
		if inst.Len <= 0 || inst.Len > 255 {
			return 0, fmt.Errorf("decoded indirect call has invalid size %d", inst.Len)
		}
		return uint8(inst.Len), nil
	default:
		if p.arch.MinLC <= 0 || p.arch.MinLC > 255 {
			return 0, fmt.Errorf("architecture has invalid PC quantum %d", p.arch.MinLC)
		}
		return uint8(p.arch.MinLC), nil
	}
}

func findCanonicalPCTable(tables []canonicalPCTable, kind string) *canonicalPCTable {
	for i := range tables {
		if tables[i].Kind == kind {
			return &tables[i]
		}
	}
	return nil
}

func lookupPCValue(ranges []canonicalPCRange, pc uint64) int32 {
	for _, r := range ranges {
		if r.Start <= pc && pc < r.End {
			return r.Value
		}
	}
	return -1
}

func (p *canonicalParser) decodeFuncInfo(data []byte) *canonicalFuncInfo {
	if len(data) < 20 {
		return nil
	}
	var fi goobj.FuncInfo
	numFile := binary.LittleEndian.Uint32(data[16:])
	fileOff := uint32(20)
	numInlOff := uint64(fileOff) + uint64(numFile)*4
	if numInlOff+4 > uint64(len(data)) {
		return nil
	}
	numInlTree := binary.LittleEndian.Uint32(data[numInlOff:])
	inlTreeOff := uint32(numInlOff + 4)
	need := uint64(inlTreeOff) + uint64(numInlTree)*24
	if need > uint64(len(data)) {
		return nil
	}
	out := &canonicalFuncInfo{
		Args: fi.ReadArgs(data), Locals: fi.ReadLocals(data),
		FuncID: uint8(fi.ReadFuncID(data)), FuncFlags: uint8(fi.ReadFuncFlag(data)),
		FlagNames: funcFlagNames(uint8(fi.ReadFuncFlag(data))), StartLine: fi.ReadStartLine(data),
	}
	for i := uint32(0); i < numFile; i++ {
		index := uint32(fi.ReadFile(data, fileOff, i))
		item := canonicalFuncInfoFile{Index: index}
		if int(index) < len(p.files) {
			item.Name = p.files[index]
		}
		out.Files = append(out.Files, item)
	}
	for i := uint32(0); i < numInlTree; i++ {
		n := fi.ReadInlTree(data, inlTreeOff, i)
		item := canonicalInlTreeNode{
			Parent: n.Parent, File: uint32(n.File), Line: n.Line,
			Func: p.resolve(n.Func), ParentPC: n.ParentPC,
		}
		if int(n.File) < len(p.files) {
			item.FileName = p.files[n.File]
		}
		out.InlTree = append(out.InlTree, item)
	}
	return out
}

func (p *canonicalParser) decodePCTable(kind string, index int, ref goobj.SymRef, data []byte, local bool) canonicalPCTable {
	out := canonicalPCTable{Index: index, Kind: kind, Symbol: p.resolve(ref)}
	if ref.IsZero() {
		return out
	}
	if !local {
		out.Error = "table symbol is not defined in this object"
		return out
	}
	out.Raw = hex.EncodeToString(data)
	if p.arch == nil {
		out.Error = "unknown architecture"
		return out
	}
	ranges, err := decodePCRanges(data, p.arch.MinLC)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if kind == "pcfile" {
		for i := range ranges {
			if ranges[i].Value >= 0 && int(ranges[i].Value) < len(p.files) {
				ranges[i].File = p.files[ranges[i].Value]
			}
		}
	}
	out.Ranges = ranges
	return out
}

func decodePCRanges(data []byte, minLC int) ([]canonicalPCRange, error) {
	var out []canonicalPCRange
	if len(data) == 0 {
		return out, nil
	}
	var pc uint64
	value := int32(-1)
	first := true
	for len(data) != 0 {
		uvdelta, n, err := readUvarint32(data)
		if err != nil {
			return nil, err
		}
		data = data[n:]
		if uvdelta == 0 && !first {
			if len(data) != 0 {
				return nil, errors.New("trailing bytes after PC table terminator")
			}
			return out, nil
		}
		if uvdelta&1 != 0 {
			value += int32(^(uvdelta >> 1))
		} else {
			value += int32(uvdelta >> 1)
		}
		pcdelta, n, err := readUvarint32(data)
		if err != nil {
			return nil, err
		}
		data = data[n:]
		next := pc + uint64(pcdelta)*uint64(minLC)
		if next < pc {
			return nil, errors.New("PC table offset overflow")
		}
		out = append(out, canonicalPCRange{Start: pc, End: next, Value: value})
		pc = next
		first = false
	}
	return nil, errors.New("PC table has no terminator")
}

func readUvarint32(data []byte) (uint32, int, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		if i >= len(data) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := data[i]
		if i == 4 && b > 15 {
			return 0, 0, errors.New("PC table varint overflows uint32")
		}
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, errors.New("PC table varint is too long")
}

func (p *canonicalParser) decodeFuncData(index int, ref goobj.SymRef, data []byte, local bool) canonicalFuncData {
	out := canonicalFuncData{Index: index, Kind: funcdataName(index), Symbol: p.resolve(ref), rawData: data}
	if ref.IsZero() {
		return out
	}
	if !local {
		out.DecodeError = "funcdata symbol is not defined in this object"
		return out
	}
	out.Raw = hex.EncodeToString(data)
	switch index {
	case abi.FUNCDATA_ArgsPointerMaps, abi.FUNCDATA_LocalsPointerMaps:
		sm, err := decodeStackMap(data)
		if err != nil {
			out.DecodeError = err.Error()
		} else {
			out.StackMap = sm
		}
	case abi.FUNCDATA_StackObjects:
		if p.arch == nil {
			out.DecodeError = "unknown architecture"
			break
		}
		objects, err := p.decodeStackObjects(ref, data)
		if err != nil {
			out.DecodeError = err.Error()
		} else {
			out.StackObjects = objects
		}
	}
	return out
}

func decodeStackMap(data []byte) (*canonicalStackMap, error) {
	if len(data) < 8 {
		return nil, errors.New("stack map is shorter than its header")
	}
	n := int32(binary.LittleEndian.Uint32(data))
	nbit := int32(binary.LittleEndian.Uint32(data[4:]))
	if n < 0 || nbit < 0 {
		return nil, errors.New("stack map has negative dimensions")
	}
	bytesPerMap := (uint64(nbit) + 7) / 8
	need := uint64(8) + uint64(n)*bytesPerMap
	if need != uint64(len(data)) {
		return nil, fmt.Errorf("stack map size %d does not match dimensions %d x %d", len(data), n, nbit)
	}
	out := &canonicalStackMap{Count: n, NumBits: nbit}
	at := uint64(8)
	for i := 0; i < int(n); i++ {
		b := data[at : at+bytesPerMap]
		bm := canonicalBitMap{Index: i, Bytes: hex.EncodeToString(b)}
		for bit := 0; bit < int(nbit); bit++ {
			if b[bit/8]&(1<<uint(bit&7)) != 0 {
				bm.SetBits = append(bm.SetBits, bit)
			}
		}
		out.Bitmaps = append(out.Bitmaps, bm)
		at += bytesPerMap
	}
	return out, nil
}

func (p *canonicalParser) decodeStackObjects(ref goobj.SymRef, data []byte) ([]canonicalStackObject, error) {
	ptrSize := p.arch.PtrSize
	if len(data) < ptrSize {
		return nil, errors.New("stack object data is shorter than its count")
	}
	var count uint64
	if ptrSize == 4 {
		count = uint64(binary.LittleEndian.Uint32(data))
	} else if ptrSize == 8 {
		count = binary.LittleEndian.Uint64(data)
	} else {
		return nil, fmt.Errorf("unsupported pointer size %d", ptrSize)
	}
	if count > (^uint64(0)-uint64(ptrSize))/16 {
		return nil, errors.New("stack object count overflows its encoded size")
	}
	need := uint64(ptrSize) + count*16
	if need != uint64(len(data)) {
		return nil, fmt.Errorf("stack object size %d does not match count %d", len(data), count)
	}
	_, symIndex, _ := p.localData(ref)
	out := make([]canonicalStackObject, 0, count)
	for i := uint64(0); i < count; i++ {
		at := ptrSize + int(i)*16
		item := canonicalStackObject{
			Index: int(i), Offset: int32(binary.LittleEndian.Uint32(data[at:])),
			Size:      int32(binary.LittleEndian.Uint32(data[at+4:])),
			PtrBytes:  int32(binary.LittleEndian.Uint32(data[at+8:])),
			GCDataOff: binary.LittleEndian.Uint32(data[at+12:]),
		}
		if item.Size < 0 || item.PtrBytes < 0 || item.PtrBytes > item.Size {
			return nil, fmt.Errorf("stack object %d has invalid size %d and pointer bytes %d", i, item.Size, item.PtrBytes)
		}
		if item.PtrBytes%int32(ptrSize) != 0 {
			return nil, fmt.Errorf("stack object %d pointer bytes %d are not pointer-aligned", i, item.PtrBytes)
		}
		for j := 0; j < p.reader.NReloc(symIndex); j++ {
			reloc := p.reader.Reloc(symIndex, j)
			if reloc.Off() == int32(at+12) {
				target := p.resolve(reloc.Sym())
				item.GCData = &target
				item.gcDataAddend = reloc.Add()
				gcdata, _, local := p.localData(reloc.Sym())
				if local {
					if reloc.Add() < 0 || uint64(reloc.Add()) > uint64(len(gcdata)) {
						return nil, fmt.Errorf("stack object %d GC bitmap addend %d is outside %d-byte symbol", i, reloc.Add(), len(gcdata))
					}
					words := (uint64(item.PtrBytes) + uint64(ptrSize) - 1) / uint64(ptrSize)
					bytesNeeded := (words + 7) / 8
					start := uint64(reloc.Add())
					if start+bytesNeeded > uint64(len(gcdata)) {
						return nil, fmt.Errorf("stack object %d GC bitmap needs %d bytes at addend %d in %d-byte symbol", i, bytesNeeded, reloc.Add(), len(gcdata))
					}
					bitmap := gcdata[start : start+bytesNeeded]
					item.gcDataDecoded = true
					for bit := uint64(0); bit < words; bit++ {
						if bitmap[bit/8]&(1<<uint(bit&7)) != 0 {
							item.gcBits = append(item.gcBits, int(bit))
						}
					}
				}
				break
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func objectFlagNames(v uint32) []string {
	var out []string
	for _, item := range []struct {
		bit  uint32
		name string
	}{
		{goobj.ObjFlagShared, "shared"}, {goobj.ObjFlagFromAssembly, "from_assembly"},
		{goobj.ObjFlagUnlinkable, "unlinkable"}, {goobj.ObjFlagStd, "standard_library"},
	} {
		if v&item.bit != 0 {
			out = append(out, item.name)
		}
	}
	return out
}

func symFlagNames(v uint8) []string {
	var out []string
	for _, item := range []struct {
		bit  uint8
		name string
	}{
		{goobj.SymFlagDupok, "dupok"}, {goobj.SymFlagLocal, "local"},
		{goobj.SymFlagTypelink, "typelink"}, {goobj.SymFlagLeaf, "leaf"},
		{goobj.SymFlagNoSplit, "nosplit"}, {goobj.SymFlagReflectMethod, "reflect_method"},
		{goobj.SymFlagGoType, "go_type"},
	} {
		if v&item.bit != 0 {
			out = append(out, item.name)
		}
	}
	return out
}

func symFlag2Names(v uint8) []string {
	var out []string
	for _, item := range []struct {
		bit  uint8
		name string
	}{
		{goobj.SymFlagUsedInIface, "used_in_interface"}, {goobj.SymFlagItab, "itab"},
		{goobj.SymFlagDict, "dictionary"}, {goobj.SymFlagPkgInit, "package_init"},
		{goobj.SymFlagLinkname, "linkname"}, {goobj.SymFlagLinknameStd, "standard_linkname"},
		{goobj.SymFlagABIWrapper, "abi_wrapper"}, {goobj.SymFlagWasmExport, "wasm_export"},
	} {
		if v&item.bit != 0 {
			out = append(out, item.name)
		}
	}
	return out
}

func funcFlagNames(v uint8) []string {
	var out []string
	for _, item := range []struct {
		bit  uint8
		name string
	}{
		{uint8(abi.FuncFlagTopFrame), "top_frame"},
		{uint8(abi.FuncFlagSPWrite), "sp_write"},
		{uint8(abi.FuncFlagAsm), "assembly"},
	} {
		if v&item.bit != 0 {
			out = append(out, item.name)
		}
	}
	return out
}

func auxName(v uint8) string {
	switch v {
	case goobj.AuxGotype:
		return "gotype"
	case goobj.AuxFuncInfo:
		return "funcinfo"
	case goobj.AuxFuncdata:
		return "funcdata"
	case goobj.AuxDwarfInfo:
		return "dwarf_info"
	case goobj.AuxDwarfLoc:
		return "dwarf_location"
	case goobj.AuxDwarfRanges:
		return "dwarf_ranges"
	case goobj.AuxDwarfLines:
		return "dwarf_lines"
	case goobj.AuxPcsp:
		return "pcsp"
	case goobj.AuxPcfile:
		return "pcfile"
	case goobj.AuxPcline:
		return "pcline"
	case goobj.AuxPcinline:
		return "pcinline"
	case goobj.AuxPcdata:
		return "pcdata"
	case goobj.AuxWasmImport:
		return "wasm_import"
	case goobj.AuxWasmType:
		return "wasm_type"
	case goobj.AuxSehUnwindInfo:
		return "seh_unwind_info"
	default:
		return fmt.Sprintf("aux_%d", v)
	}
}

func pcdataName(index int) string {
	switch index {
	case abi.PCDATA_UnsafePoint:
		return "unsafe_point"
	case abi.PCDATA_StackMapIndex:
		return "stack_map_index"
	case abi.PCDATA_InlTreeIndex:
		return "inline_tree_index"
	case abi.PCDATA_ArgLiveIndex:
		return "argument_live_index"
	case abi.PCDATA_PanicBounds:
		return "panic_bounds"
	default:
		return fmt.Sprintf("pcdata_%d", index)
	}
}

func funcdataName(index int) string {
	switch index {
	case abi.FUNCDATA_ArgsPointerMaps:
		return "args_pointer_maps"
	case abi.FUNCDATA_LocalsPointerMaps:
		return "locals_pointer_maps"
	case abi.FUNCDATA_StackObjects:
		return "stack_objects"
	case abi.FUNCDATA_InlTree:
		return "inline_tree"
	case abi.FUNCDATA_OpenCodedDeferInfo:
		return "open_coded_defer"
	case abi.FUNCDATA_ArgInfo:
		return "argument_info"
	case abi.FUNCDATA_ArgLiveInfo:
		return "argument_live_info"
	case abi.FUNCDATA_WrapInfo:
		return "wrapper_info"
	default:
		return fmt.Sprintf("funcdata_%d", index)
	}
}
