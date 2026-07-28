// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Objview inspects Go object files and package archives.
//
// Usage:
//
//	go tool objview [-json] object-or-archive
//
// The default output is a diagnostic hexadecimal/text dump. -json prints a
// deterministic structured representation for machine comparison.
package main

import (
	"cmd/internal/archive"
	"cmd/internal/disasm"
	"cmd/internal/goobj"
	"cmd/internal/objabi"
	"cmd/internal/objfile"
	"cmd/internal/telemetry/counter"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"internal/abi"
	"internal/goarch"
	"log"
	"math"
	"os"
	"strings"
	"unsafe"
)

var jsonOutput = flag.Bool("json", false, "print canonical, machine-readable JSON")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go tool objview [-json] binary\n\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("objview: ")
	counter.Open()

	flag.Usage = usage
	flag.Parse()
	counter.Inc("objview/invocations")
	counter.CountFlags("objview/flag:", *flag.CommandLine)
	if flag.NArg() != 1 {
		usage()
	}
	if *jsonOutput {
		doc, err := parseCanonicalFile(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doc); err != nil {
			log.Fatal(err)
		}
		return
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	a, err := archive.Parse(f, false)
	if err != nil {
		log.Fatal(err)
	}

	objf, err := objfile.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer objf.Close()

	dis, err := disasm.DisasmForFile(objf)

	for _, e := range a.Entries {
		switch e.Type {
		case archive.EntryPkgDef, archive.EntrySentinelNonObj:
			continue
		case archive.EntryGoObj:
			o := e.Obj
			b := make([]byte, o.Size)
			_, err := f.ReadAt(b, o.Offset)
			if err != nil {
				log.Fatal(err)
			}
			r := goobj.NewReaderFromBytes(b, false)
			//var arch *sys.Arch
			//for _, a := range sys.Archs {
			//	if a.Name == e.Obj.Arch {
			//		arch = a
			//		break
			//	}
			//}
			dumpGoObj(r, b, dis)

		}
	}
}

var blk2str = map[int]string{
	goobj.BlkAutolib:     "Autolib",
	goobj.BlkPkgIdx:      "PkgIdx",
	goobj.BlkFile:        "File",
	goobj.BlkSymdef:      "Symdef",
	goobj.BlkHashed64def: "Hashed64def",
	goobj.BlkHasheddef:   "Hasheddef",
	goobj.BlkNonpkgdef:   "Nonpkgdef",
	goobj.BlkNonpkgref:   "Nonpkgref",
	goobj.BlkRefFlags:    "RefFlags",
	goobj.BlkHash64:      "Hash64",
	goobj.BlkHash:        "Hash",
	goobj.BlkRelocIdx:    "RelocIdx",
	goobj.BlkAuxIdx:      "AuxIdx",
	goobj.BlkDataIdx:     "DataIdx",
	goobj.BlkReloc:       "Reloc",
	goobj.BlkAux:         "Aux",
	goobj.BlkData:        "Data",
	goobj.BlkRefName:     "RefName",
	goobj.BlkEnd:         "End",
}

func dumpGoObj(r *goobj.Reader, b []byte, dis *disasm.Disasm) {
	var h goobj.Header
	err := h.Read(r)
	if err != nil {
		log.Fatal(err)
	}

	offset := 0
	headerLength := len(h.Magic) + len(h.Fingerprint) + int(unsafe.Sizeof(h.Flags)) + int(unsafe.Sizeof(h.Offsets))
	// fmt.Sprintf("%-10v %-7v %-7v %-7v %-7v\n", "flags", "symkind", "size", "rawdata"), fmt.Sprintf("%+v", r.Flags(),r.)

	content := fmt.Sprintf("Magic: %v, Fingerprint: %v, Flags: %v\n", h.Magic, h.Fingerprint, h.Flags)
	for i, off := range h.Offsets {
		content += fmt.Sprintf(blk2str[i]+": %v\n", off)
	}
	printHexdumpWithText(b[offset:headerLength], content, offset, "Header", "", "", "")
	offset += headerLength

	for blk := goobj.BlkAutolib; blk < goobj.BlkEnd; blk++ {
		header, content := extractBlock(blk, r, &h, b, dis)
		printHexdumpWithText(b[h.Offsets[blk]:h.Offsets[blk+1]], content, offset, blk2str[blk], "", header, "")
		offset += int(h.Offsets[blk+1] - h.Offsets[blk])
		fmt.Println()
	}
}

type StringRef struct {
	length uint32
	offset uint32
}

type Sym struct {
	Name  StringRef
	ABI   uint16
	Type  uint8
	Flag  uint8
	Flag2 uint8
	Siz   uint32
	Align uint32
}

type Reloc struct {
	sym    goobj.SymRef
	offset int32
	size   uint8
	typ    objabi.RelocType
	add    int64
}

type RefFlags struct {
	Sym   goobj.SymRef
	Flag  uint8
	Flag2 uint8
}

type StackMap struct {
	n        int32   // number of bitmaps
	nbit     int32   // number of bits in each bitmap
	bytedata [1]byte // bitmaps, each starting on a byte boundary
}

func readUint32(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

var symIdx = 0

func extractBlock(block int, r *goobj.Reader, h *goobj.Header, data []byte, dis *disasm.Disasm) (string, string) {
	switch block {
	case goobj.BlkAutolib:
		var libs string
		header := fmt.Sprintf("%-50v %-20v %v\n", "name", "fingerprint", "rawdata")
		start := int(h.Offsets[goobj.BlkAutolib])
		for i, lib := range r.Autolib() {
			rawdata := StringRef{
				length: readUint32(data[start+i*16:]),
				offset: readUint32(data[start+4+i*16:]),
			}
			libs += fmt.Sprintf("%-50v %-20x %v\n", lib.Pkg, binary.LittleEndian.Uint64(lib.Fingerprint[:]), rawdata)
		}
		return header, libs
	case goobj.BlkPkgIdx:
		var pkgs string
		header := fmt.Sprintf("%-50v %v\n", "name", "rawdata")
		start := int(h.Offsets[goobj.BlkPkgIdx])
		for i := 0; i < r.NPkg(); i++ {
			rawdata := StringRef{
				length: readUint32(data[start+i*8:]),
				offset: readUint32(data[start+4+i*8:]),
			}
			pkgs += fmt.Sprintf("%-50v %v\n", r.Pkg(i), rawdata)
		}
		return header, pkgs
	case goobj.BlkFile:
		var files string
		header := fmt.Sprintf("%-50v %v\n", "name", "rawdata")
		start := int(h.Offsets[goobj.BlkFile])
		for i := 0; i < r.NFile(); i++ {
			rawdata := StringRef{
				length: readUint32(data[start+i*8:]),
				offset: readUint32(data[start+4+i*8:]),
			}
			files += fmt.Sprintf("%-50v %v\n", r.File(i), rawdata)
		}
		return header, files
	case goobj.BlkHashed64def:
		var symdefs string
		start := int(h.Offsets[goobj.BlkHashed64def])
		header := fmt.Sprintf("%-10v %-40v %-20v %-15v %-8v %v\n", "index", "name", "value", "symkind", "size", "rawdata")
		for i := 0; i < r.NHashed64def(); i++ {
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			symdata := Sym{
				Name: StringRef{
					length: readUint32(data[start+i*goobj.SymSize:]),
					offset: readUint32(data[start+i*goobj.SymSize+4:]),
				},
				ABI:   sym.ABI(),
				Type:  sym.Type(),
				Flag:  sym.Flag(),
				Flag2: sym.Flag2(),
				Siz:   sym.Siz(),
				Align: sym.Align(),
			}
			symdefs += fmt.Sprintf("%-10v %-40v %-20x %-15v %-8v %v\n", symIdx, sym.Name(r), r.Hash64(uint32(i)), objabi.SymKind(sym.Type()), sym.Siz(), symdata)
			symIdx++
		}
		return header, fmt.Sprintf("%+v", symdefs)
	case goobj.BlkHasheddef:
		var symdefs string
		start := int(h.Offsets[goobj.BlkHasheddef])
		header := fmt.Sprintf("%-10v %-50v %-35v %-15v %-8v %v\n", "index", "name", "value", "symkind", "size", "rawdata")
		for i := 0; i < r.NHasheddef(); i++ {
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			symdata := Sym{
				Name: StringRef{
					length: readUint32(data[start+i*goobj.SymSize:]),
					offset: readUint32(data[start+i*goobj.SymSize+4:]),
				},
				ABI:   sym.ABI(),
				Type:  sym.Type(),
				Flag:  sym.Flag(),
				Flag2: sym.Flag2(),
				Siz:   sym.Siz(),
				Align: sym.Align(),
			}
			symdefs += fmt.Sprintf("%-10v %-50v %-35x %-15v %-8v %v\n", symIdx, sym.Name(r), *r.Hash(uint32(i)), objabi.SymKind(sym.Type()), sym.Siz(), symdata)
			symIdx++
		}
		return header, fmt.Sprintf("%+v", symdefs)
	case goobj.BlkNonpkgdef, goobj.BlkNonpkgref, goobj.BlkSymdef:
		var symrefs string
		start := int(h.Offsets[block])
		header := fmt.Sprintf("%-10v %-50v %-15v %-8v %v\n", "index", "name", "symkind", "size", "rawdata")
		var num int
		switch block {
		case goobj.BlkNonpkgref:
			num = r.NNonpkgref()
		case goobj.BlkNonpkgdef:
			num = r.NNonpkgdef()
		case goobj.BlkSymdef:
			num = r.NSym()
		}
		for i := 0; i < num; i++ {
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			symdata := Sym{
				Name: StringRef{
					length: readUint32(data[start+i*goobj.SymSize:]),
					offset: readUint32(data[start+i*goobj.SymSize+4:]),
				},
				ABI:   sym.ABI(),
				Type:  sym.Type(),
				Flag:  sym.Flag(),
				Flag2: sym.Flag2(),
				Siz:   sym.Siz(),
				Align: sym.Align(),
			}
			symrefs += fmt.Sprintf("%-10v %-50v %-15v %-8v %v\n", symIdx, sym.Name(r), objabi.SymKind(sym.Type()), sym.Siz(), symdata)
			symIdx++
		}
		return header, fmt.Sprintf("%+v", symrefs)
	case goobj.BlkRefFlags:
		var symrefs string
		start := int(h.Offsets[block])
		header := fmt.Sprintf("%-50v %-15v %-8v %v\n", "sym", "flag", "flag2", "rawdata")
		for i, n := 0, r.NRefFlags(); i < n; i++ {
			rf := (*goobj.RefFlags)(unsafe.Pointer(&data[start+i*goobj.RefFlagsSize]))
			rfdata := RefFlags{
				Sym: goobj.SymRef{
					PkgIdx: readUint32(data[start+i*goobj.RefFlagsSize:]),
					SymIdx: readUint32(data[start+i*goobj.RefFlagsSize+4:]),
				},
				Flag:  rf.Flag(),
				Flag2: rf.Flag2(),
			}
			symrefs += fmt.Sprintf("%-30v %-10v %10v %v\n", rf.Sym(), rf.Flag(), rf.Flag2(), rfdata)
		}
		return header, fmt.Sprintf("%+v", symrefs)
	case goobj.BlkHash64:
		var hash64s string
		header := fmt.Sprintf("%v\n", "value")
		for i := 0; i < r.NHashed64def(); i++ {
			hash64s += fmt.Sprintf("%-20x\n", r.Hash64(uint32(i)))
		}
		return header, hash64s
	case goobj.BlkHash:
		var hashs string
		header := fmt.Sprintf("%v\n", "value")
		for i := 0; i < r.NHasheddef(); i++ {
			hashs += fmt.Sprintf("%-32x\n", *r.Hash(uint32(i)))
		}
		return header, hashs
	case goobj.BlkRelocIdx, goobj.BlkAuxIdx, goobj.BlkDataIdx:
		var header string
		if block == goobj.BlkRelocIdx {
			header = fmt.Sprintf("%-10v %-10v %-10v | %-10v %-10v %-10v | %-10v %-10v %-10v\n", "symidx", "relocNum", "relocStart", "symidx", "relocNum", "relocStart", "symidx", "relocNum", "relocStart")
		} else if block == goobj.BlkAuxIdx {
			header = fmt.Sprintf("%-10v %-10v %-10v | %-10v %-10v %-10v | %-10v %-10v %-10v\n", "symidx", "auxNum", "auxStart", "symidx", "auxNum", "auxStart", "symidx", "auxNum", "auxStart")
		} else {
			header = fmt.Sprintf("%-10v %-10v %-10v | %-10v %-10v %-10v | %-10v %-10v %-10v\n", "symidx", "size", "offset", "symidx", "size", "offset", "symidx", "size", "offset")
		}
		ndef := r.NSym() + r.NHashed64def() + r.NHasheddef() + r.NNonpkgdef()
		start := int(h.Offsets[block])
		var content string
		preRelocIdx := readUint32(data[start:])
		for i := 1; i < ndef; i++ {
			relocStart := readUint32(data[start+i*4:])
			content += fmt.Sprintf("%-10v %-10v %-10v | ", i-1, relocStart-preRelocIdx, preRelocIdx)
			preRelocIdx = relocStart
			if i%3 == 0 {
				content += "\n"
			}
		}
		return header, content
	case goobj.BlkReloc:
		var relocs string
		start := uint32(h.Offsets[block])
		header := fmt.Sprintf("%-10v %-10v %-10v %-15v %-10v %v\n", "index", "offset", "size", "typ", "add", "symbol")
		num := (h.Offsets[block+1] - h.Offsets[block]) / goobj.RelocSize
		for i := uint32(0); i < num; i++ {
			reloc := (*goobj.Reloc)(unsafe.Pointer(&data[start+i*goobj.RelocSize]))
			symdata := Reloc{
				offset: reloc.Off(),
				size:   reloc.Siz(),
				typ:    objabi.RelocType(reloc.Type()) &^ objabi.R_WEAK,
				add:    reloc.Add(),
				sym:    reloc.Sym(),
			}
			relocs += fmt.Sprintf("%-10v %-10v %-10v %-15v %-10v %v\n", i, symdata.offset, symdata.size, symdata.typ, symdata.add, symdata.sym)
		}
		return header, fmt.Sprintf("%+v", relocs)
	case goobj.BlkAux:
		var auxs string
		start := uint32(h.Offsets[block])
		header := fmt.Sprintf("%-15v %-15v %v\n", "type", "pkg", "sym")
		num := (h.Offsets[block+1] - h.Offsets[block]) / goobj.AuxSize
		funcdataCount := 0
		for i := uint32(0); i < num; i++ {
			aux := (*goobj.Aux)(unsafe.Pointer(&data[start+i*goobj.AuxSize]))
			symref := aux.Sym()
			var sym *goobj.Sym
			switch symref.PkgIdx {
			case goobj.PkgIdxInvalid:
			case goobj.PkgIdxNone:
				start := uint32(h.Offsets[goobj.BlkNonpkgdef])
				sym = (*goobj.Sym)(unsafe.Pointer(&data[start+aux.Sym().SymIdx*goobj.SymSize]))
				sym2aux[sym] = aux.Type()
			case goobj.PkgIdxHashed64:
				start := uint32(h.Offsets[goobj.BlkHashed64def])
				sym = (*goobj.Sym)(unsafe.Pointer(&data[start+aux.Sym().SymIdx*goobj.SymSize]))

				sym2aux[sym] = aux.Type()
			case goobj.PkgIdxHashed:
				start := uint32(h.Offsets[goobj.BlkHasheddef])
				sym = (*goobj.Sym)(unsafe.Pointer(&data[start+aux.Sym().SymIdx*goobj.SymSize]))
				sym2aux[sym] = aux.Type()
			case goobj.PkgIdxBuiltin:
			case goobj.PkgIdxSelf:
				start := uint32(h.Offsets[goobj.BlkSymdef])
				sym = (*goobj.Sym)(unsafe.Pointer(&data[start+aux.Sym().SymIdx*goobj.SymSize]))

				sym2aux[sym] = aux.Type()
			default:
			}
			if _, ok := pkg2str[aux.Sym().PkgIdx]; !ok {
				auxs += fmt.Sprintf("%-15v %-15v %v\n", aux2str[aux.Type()], aux.Sym().PkgIdx, aux.Sym().SymIdx)
			} else {
				auxs += fmt.Sprintf("%-15v %-15v %v\n", aux2str[aux.Type()], pkg2str[aux.Sym().PkgIdx], aux.Sym().SymIdx)
			}
			if aux.Type() == goobj.AuxFuncdata {
				sym2funcdata[sym] = funcdataCount
				funcdataCount++
			} else {
				funcdataCount = 0
			}
		}
		return header, fmt.Sprintf("%+v", auxs)
	case goobj.BlkData:
		var content string
		offset := h.Offsets[block]

		for i := uint32(0); i < uint32(r.NSym()); i++ {
			start := uint32(h.Offsets[goobj.BlkSymdef])
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			content = extractSymData(r, i, sym, content, dis, offset)
			offset += sym.Siz()
		}
		for i := uint32(0); i < uint32(r.NHashed64def()); i++ {
			start := uint32(h.Offsets[goobj.BlkHashed64def])
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			content = extractSymData(r, i+uint32(r.NSym()), sym, content, dis, offset)
			offset += sym.Siz()
		}
		for i := uint32(0); i < uint32(r.NHasheddef()); i++ {
			start := uint32(h.Offsets[goobj.BlkHasheddef])
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			content = extractSymData(r, i+uint32(r.NSym()+r.NHashed64def()), sym, content, dis, offset)
			offset += sym.Siz()
		}
		for i := uint32(0); i < uint32(r.NNonpkgdef()); i++ {
			start := uint32(h.Offsets[goobj.BlkNonpkgdef])
			sym := (*goobj.Sym)(unsafe.Pointer(&data[start+i*goobj.SymSize]))
			content = extractSymData(r, i+uint32(r.NSym()+r.NHashed64def()+r.NHasheddef()), sym, content, dis, offset)
			offset += sym.Siz()
		}
		return "", content
	case goobj.BlkRefName:
	case goobj.BlkEnd:
	case goobj.NBlk:
	}
	return "", ""
}

func extractSymData(r *goobj.Reader, i uint32, sym *goobj.Sym, content string, dis *disasm.Disasm, offset uint32) string {
	switch objabi.SymKind(sym.Type()) {
	case objabi.SDATA, objabi.SRODATA, objabi.SNOPTRDATA:
		b := r.Data(i)
		content = tryAux(sym, content, i, b)
	case objabi.STEXT:
		text, err := disassembleText(dis, uint64(offset), uint64(offset)+uint64(sym.Siz()))
		if err != nil {
			content += fmt.Sprintf("%v disassembly unavailable: %v; code=%x\n", i, err, r.Data(i))
		} else {
			content += fmt.Sprintf("%v %v\n", i, text)
		}
	case objabi.Sxxx:
	case objabi.SBSS:
	case objabi.SNOPTRBSS:
	case objabi.STEXTFIPS:
	case objabi.SRODATAFIPS:
	case objabi.SNOPTRDATAFIPS:
	case objabi.SDATAFIPS:
	case objabi.STLSBSS:
	case objabi.SDWARFCUINFO:
	case objabi.SDWARFCONST:
	case objabi.SDWARFFCN:
	case objabi.SDWARFABSFCN:
	case objabi.SDWARFTYPE:
	case objabi.SDWARFVAR:
	case objabi.SDWARFRANGE:
	case objabi.SDWARFLOC:
	case objabi.SDWARFLINES:
	case objabi.SDWARFADDR:
	case objabi.SLIBFUZZER_8BIT_COUNTER:
	case objabi.SCOVERAGE_COUNTER:
	case objabi.SCOVERAGE_AUXVAR:
	case objabi.SSEHUNWINDINFO:
	}
	return content
}

func disassembleText(dis *disasm.Disasm, start, end uint64) (text string, err error) {
	if dis == nil {
		return "", fmt.Errorf("disassembler is unavailable")
	}
	defer func() {
		if p := recover(); p != nil {
			text = ""
			err = fmt.Errorf("%v", p)
		}
	}()
	var out strings.Builder
	dis.Print(&out, nil, start, end, false, false)
	return out.String(), nil
}

type stackObjectRecord struct {
	// offset in frame
	// if negative, offset from varp
	// if non-negative, offset from argp
	off       int32
	size      int32
	ptrBytes  int32
	gcdataoff uint32 // offset to gcdata from moduledata.rodata
}

type InlTreeNode struct {
	Parent   int32
	File     uint32
	Line     int32
	Func     goobj.SymRef
	ParentPC int32
}

func tryAux(sym *goobj.Sym, content string, i uint32, b []byte) string {
	if typ, ok := sym2aux[sym]; ok {
		switch typ {
		case goobj.AuxGotype:
			content += fmt.Sprintf("%v %v\n", i, "Gotype")
		case goobj.AuxFuncInfo:
			fi := FuncInfo{}
			fi.Args = readUint32(b)
			fi.Locals = readUint32(b[4:])
			fi.FuncID = abi.FuncID(b[8])
			fi.FuncFlag = abi.FuncFlag(b[9])
			fi.StartLine = readUint32(b[12:])

			const numfileOff = 16
			fi.NumFile = readUint32(b[numfileOff:])
			fi.FileOff = numfileOff + 4

			numinltreeOff := fi.FileOff + 4*fi.NumFile
			fi.NumInlTree = binary.LittleEndian.Uint32(b[numinltreeOff:])
			fi.InlTreeOff = numinltreeOff + 4
			content += fmt.Sprintf("%v Funcinfo: %+v\n", i, fi)
			b = b[fi.InlTreeOff:]
			for inlIdx := range fi.NumInlTree {
				itn := InlTreeNode{}
				itn.Parent = int32(readUint32(b))
				itn.File = uint32(readUint32(b[4:]))
				itn.Line = int32(readUint32(b[8:]))
				itn.Func = goobj.SymRef{
					PkgIdx: readUint32(b[12:]),
					SymIdx: readUint32(b[16:]),
				}
				itn.ParentPC = int32(readUint32(b[20:]))
				content += fmt.Sprintf("inlTree: %v %+v\n", inlIdx, itn)
			}

		case goobj.AuxFuncdata:
			switch sym2funcdata[sym] {
			case abi.FUNCDATA_ArgsPointerMaps:
				sm := (*StackMap)(unsafe.Pointer(unsafe.SliceData(b)))
				content += fmt.Sprintf("%v Funcdata args pointer maps: %+v\n", i, *sm)
			case abi.FUNCDATA_LocalsPointerMaps:
				sm := (*StackMap)(unsafe.Pointer(unsafe.SliceData(b)))
				content += fmt.Sprintf("%v Funcdata locals pointer maps: %+v\n", i, *sm)
			case abi.FUNCDATA_StackObjects:
				p := unsafe.Pointer(unsafe.SliceData(b))
				n := *(*uintptr)(p)
				p = unsafe.Add(p, goarch.PtrSize)
				r0 := (*stackObjectRecord)(p)
				content += fmt.Sprintf("%v Funcdata stack objects: %v %+v\n", i, n, *r0)
			case abi.FUNCDATA_InlTree:
				//p := unsafe.Pointer(unsafe.SliceData(b))
				//ic := unsafe.Slice((*inlinedCall)(p), uintptr(len(b))/unsafe.Sizeof(inlinedCall{}))
				//content += fmt.Sprintf("%v Funcdata inline call: %+v\n", i, ic)
			case abi.FUNCDATA_OpenCodedDeferInfo:
			case abi.FUNCDATA_ArgInfo:
			case abi.FUNCDATA_ArgLiveInfo:
			case abi.FUNCDATA_WrapInfo:
			}

		case goobj.AuxDwarfInfo:
		case goobj.AuxDwarfLoc:
		case goobj.AuxDwarfRanges:
		case goobj.AuxDwarfLines:
		case goobj.AuxPcsp:
		case goobj.AuxPcfile:
		case goobj.AuxPcline:
		case goobj.AuxPcinline:
		case goobj.AuxPcdata:
		case goobj.AuxWasmImport:
		case goobj.AuxWasmType:
		case goobj.AuxSehUnwindInfo:
		}
	}
	return content
}

type FuncInfo struct {
	Args       uint32
	Locals     uint32
	FuncID     abi.FuncID
	FuncFlag   abi.FuncFlag
	StartLine  uint32
	NumFile    uint32
	FileOff    uint32
	NumInlTree uint32
	InlTreeOff uint32
}

var aux2str = map[uint8]string{
	goobj.AuxGotype:        "Gotype",
	goobj.AuxFuncInfo:      "FuncInfo",
	goobj.AuxFuncdata:      "Funcdata",
	goobj.AuxDwarfInfo:     "DwarfInfo",
	goobj.AuxDwarfLoc:      "DwarfLoc",
	goobj.AuxDwarfRanges:   "DwarfRanges",
	goobj.AuxDwarfLines:    "DwarfLines",
	goobj.AuxPcsp:          "Pcsp",
	goobj.AuxPcfile:        "Pcfile",
	goobj.AuxPcline:        "Pcline",
	goobj.AuxPcinline:      "Pcinline",
	goobj.AuxPcdata:        "Pcdata",
	goobj.AuxWasmImport:    "WasmImport",
	goobj.AuxWasmType:      "WasmType",
	goobj.AuxSehUnwindInfo: "SehUnwindInfo",
}

var pkg2str = map[uint32]string{
	goobj.PkgIdxNone:     "None",
	goobj.PkgIdxHashed64: "Hashed64",
	goobj.PkgIdxHashed:   "Hashed",
	goobj.PkgIdxBuiltin:  "Builtin",
	goobj.PkgIdxSelf:     "Self",
	goobj.PkgIdxInvalid:  "Invalid ",
}

var sym2aux = map[*goobj.Sym]uint8{}
var sym2funcdata = map[*goobj.Sym]int{}

func printHexdumpWithText(data []byte, text string, startOffset int, binTitle, binHeader, txtTitle, txtHeader string) {
	const bytesPerLine = 32
	const leftGroup = 16
	const groupGap = 2 // 两组之间额外空格数，模拟 hexdump -C
	const maxTextCols = 200

	// 计算显示的 offset 基点（向下取整到 32 的倍数）
	displayBase := startOffset - (startOffset % bytesPerLine)

	// 计算需要显示的行数（保证覆盖所有数据）
	totalSlots := int((startOffset - displayBase) + (len(data)))
	hexLines := int(math.Ceil(float64(totalSlots) / float64(bytesPerLine)))
	if hexLines == 0 {
		hexLines = 1
	}

	// 准备文本行：先按 \n 分段，然后对每段做按 rune 的自动折行（宽度 maxTextCols）
	textLines := wrapTextRespectingNewlines(text, maxTextCols)

	// 计算固定列宽（字节区域）
	// leftGroup 个字节字符串宽 = leftGroup*3 -1 = 16*3 -1 = 47 (每个字节 "xx" + " "，最后无 trailing space)
	leftGroupWidth := leftGroup*3 - 1
	hexWidth := leftGroupWidth + groupGap + leftGroupWidth // 总 hex 列宽（32 字节占位）
	offsetWidth := 8 + 2                                   // bytes count for padding

	// 先打印标题 / 表头（如果有）
	if binTitle != "" || txtTitle != "" {
		printTwoColumnLine(binTitle, txtTitle, offsetWidth, hexWidth)
	}
	if binHeader != "" || txtHeader != "" {
		printTwoColumnLine(binHeader, txtHeader, offsetWidth, hexWidth)
	}

	// 为每一行构造并打印
	maxLines := hexLines
	if len(textLines) > maxLines {
		maxLines = len(textLines)
	}

	// 把 data 放到基于 displayBase 的槽位上，方便索引
	// dataAbsStart .. dataAbsEnd-1 是有数据的区间
	dataAbsStart := startOffset
	dataAbsEnd := startOffset + (len(data))

	for i := 0; i < maxLines; i++ {
		lineBase := displayBase + (i * bytesPerLine)

		// 如果该行属于 hexLines 范围，打印 offset，否则空出 offset 区域
		if i < hexLines {
			fmt.Printf("%08x  ", lineBase)
		} else {
			// 打印等宽空白以占位 offset 区域
			fmt.Print(strings.Repeat(" ", offsetWidth))
		}

		// hex 部分：32 个槽位，若该槽位有数据则显示 xx，否则显示两空格占位
		var hexParts []string
		for j := 0; j < bytesPerLine; j++ {
			abs := lineBase + (j)
			if abs >= dataAbsStart && abs < dataAbsEnd {
				b := data[abs-dataAbsStart]
				hexParts = append(hexParts, hex.EncodeToString([]byte{b}))
			} else {
				hexParts = append(hexParts, "  ")
			}
		}

		// 拼接为两组，每组中间的 byte 用空格分隔；组间用 groupGap 个空格
		left := strings.Join(hexParts[:leftGroup], " ")
		right := strings.Join(hexParts[leftGroup:], " ")
		// left 计算为固定宽 leftGroupWidth；如果右端为空（全部 "  "），仍保留占位
		hexLine := fmt.Sprintf("%-*s%s%s", leftGroupWidth, left, strings.Repeat(" ", groupGap), right)

		fmt.Print(hexLine)

		// 然后打印分隔和文本列 (文本列左侧有 " | " )
		fmt.Print("  |  ")

		// 文本列对应行：如果有则打印，否则保持空
		if i < len(textLines) {
			fmt.Print(textLines[i])
		}

		fmt.Println()
	}
}

// printTwoColumnLine 在两列中分别打印 left 和 right，并把 left 列填充到 offsetWidth + hexWidth 的宽度
func printTwoColumnLine(left, right string, offsetWidth, hexWidth int) {
	leftAreaWidth := offsetWidth + hexWidth // +2 for the "  |  " gap printed later approx
	// 注意：这里以字节长度做填充（对于包含宽字符如中文的情况，终端宽度可能会和 rune 宽度不同）
	if left == "" && right == "" {
		return
	}
	if left == "" {
		fmt.Printf("%-*s  |  %s\n", leftAreaWidth, "", right)
		return
	}
	fmt.Printf("%-*s  |  %s\n", leftAreaWidth, left, right)
}

// wrapTextRespectingNewlines 先按 \n 分段，再对每段按 rune 做 maxCols 折行，返回最终的行切片
func wrapTextRespectingNewlines(text string, maxCols int) []string {
	if text == "" {
		return nil
	}
	var out []string
	paras := strings.Split(text, "\n")
	for _, p := range paras {
		// trim not necessary — 保留用户原始段落空格
		runes := []rune(p)
		if len(runes) == 0 {
			// 如果用户写了连续的 \n，我们也需要在输出中保留空行
			out = append(out, "")
			continue
		}
		for i := 0; i < len(runes); i += maxCols {
			end := i + maxCols
			if end > len(runes) {
				end = len(runes)
			}
			out = append(out, string(runes[i:end]))
		}
	}
	return out
}
