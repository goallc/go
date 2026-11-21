// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"encoding/binary"
	"fmt"
	"internal/buildcfg"
	"sort"
	"strings"

	"github.com/goallc/go-llvm"
)

// The v1 format is a little-endian, length-delimited module record. Addresses
// are eight-byte LLVM pointer constants, not embedded symbol names. Native
// relocation, bitcode linking and symbol renaming therefore preserve identity.
// See README.md for the exact field order and the conservative retention rule.
const nativeMetadataMagic = "GoAsm\x00\x01\x00"
const nativePointerSize = 8

type metadataPointer struct {
	offset int
	value  llvm.Value
}

type metadataRecord struct {
	data []byte
	refs []metadataPointer
}

func (r *metadataRecord) u32(v uint32) { r.data = binary.LittleEndian.AppendUint32(r.data, v) }
func (r *metadataRecord) u64(v uint64) { r.data = binary.LittleEndian.AppendUint64(r.data, v) }
func (r *metadataRecord) blob(b []byte) {
	r.u32(uint32(len(b)))
	r.data = append(r.data, b...)
}
func (r *metadataRecord) str(s string) { r.blob([]byte(s)) }
func (r *metadataRecord) align() {
	for len(r.data)%nativePointerSize != 0 {
		r.data = append(r.data, 0)
	}
}
func (r *metadataRecord) ptr(v llvm.Value) {
	r.align()
	r.refs = append(r.refs, metadataPointer{len(r.data), v})
	r.u64(0)
}

func emitNativeMetadata(context llvm.Context, module llvm.Module, ctxt *obj.Link, options Options, value func(reference) llvm.Value) error {
	r := metadataRecord{data: []byte(nativeMetadataMagic)}
	r.u64(0) // Total record length, patched after all symbols have been emitted.
	flags := uint32(goobj.ObjFlagFromAssembly)
	if ctxt.Flag_shared {
		flags |= goobj.ObjFlagShared
	}
	if ctxt.Std {
		flags |= goobj.ObjFlagStd
	}
	r.u32(flags)
	files := ctxt.PosTable.FileTable()
	r.u32(uint32(len(files)))
	symbols := append(append([]*obj.LSym(nil), ctxt.Text...), ctxt.Data...)
	r.u32(uint32(len(symbols)))
	r.u32(0) // Reserved.
	r.str(buildcfg.Version)
	r.str(module.Target())
	r.str(ctxt.Pkgpath)
	r.str(strings.Join(buildcfg.Experiment.Enabled(), ","))
	for _, file := range files {
		r.str(file)
	}
	ptr := func(s *obj.LSym) llvm.Value {
		if s == nil {
			return llvm.ConstNull(llvm.PointerType(context.Int8Type(), 0))
		}
		return value(reference{s, false})
	}
	pcTable := func(s *obj.LSym) {
		if s == nil {
			r.blob(nil)
		} else {
			r.blob(s.P)
		}
	}
	for _, s := range symbols {
		r.align()
		start := len(r.data)
		r.u64(0) // Symbol record length.
		r.ptr(ptr(s))
		r.ptr(ptr(s.Gotype))
		r.u64(uint64(s.Size))
		r.u32(uint32(s.Type))
		r.u32(uint32(s.Align))
		// Attribute numbering is tied to the recorded Go toolchain version.
		// Exclude bookkeeping bits whose values depend on the output writer.
		attrs := s.Attribute & (obj.AttrDuplicateOK | obj.AttrCFunc | obj.AttrNoSplit | obj.AttrLeaf | obj.AttrReflectMethod | obj.AttrLocal | obj.AttrWrapper | obj.AttrNeedCtxt | obj.AttrNoFrame | obj.AttrStatic | obj.AttrABIWrapper | obj.AttrLinkname | obj.AttrLinknameStd | obj.AttrContentAddressable)
		r.u32(uint32(attrs))
		r.u32(uint32(s.ABI()))
		r.str(s.Name)
		r.u32(uint32(len(s.R)))
		for _, rel := range s.R {
			r.u32(uint32(rel.Off))
			r.u32(uint32(rel.Type))
			r.u32(uint32(rel.Siz))
			r.u32(0) // Reserved.
			r.u64(uint64(rel.Add))
			r.ptr(ptr(rel.Sym))
		}
		if fn := s.Func(); fn != nil {
			r.u32(1)
			r.u32(uint32(fn.Args))
			r.u32(uint32(fn.Locals))
			r.u32(uint32(fn.FuncID))
			r.u32(uint32(fn.FuncFlag))
			r.u32(uint32(fn.StartLine))
			pc := &fn.Pcln
			pcTable(pc.Pcsp)
			pcTable(pc.Pcfile)
			pcTable(pc.Pcline)
			pcTable(pc.Pcinline)
			usedFiles := make([]int, 0, len(pc.UsedFiles))
			for file := range pc.UsedFiles {
				usedFiles = append(usedFiles, int(file))
			}
			sort.Ints(usedFiles)
			r.u32(uint32(len(usedFiles)))
			for _, file := range usedFiles {
				r.u32(uint32(file))
			}
			r.u32(uint32(len(pc.Pcdata)))
			for _, table := range pc.Pcdata {
				pcTable(table)
			}
			r.u32(uint32(len(pc.Funcdata)))
			for _, data := range pc.Funcdata {
				r.ptr(ptr(data))
			}
		} else {
			r.u32(0)
		}
		r.align()
		binary.LittleEndian.PutUint64(r.data[start:], uint64(len(r.data)-start))
	}
	r.align()
	binary.LittleEndian.PutUint64(r.data[len(nativeMetadataMagic):], uint64(len(r.data)))
	var fields []llvm.Value
	pos := 0
	for _, ref := range r.refs {
		if ref.offset < pos || ref.offset+nativePointerSize > len(r.data) {
			return fmt.Errorf("invalid native metadata pointer offset")
		}
		if ref.offset > pos {
			fields = append(fields, context.ConstString(string(r.data[pos:ref.offset]), false))
		}
		fields = append(fields, ref.value)
		pos = ref.offset + nativePointerSize
	}
	if pos < len(r.data) {
		fields = append(fields, context.ConstString(string(r.data[pos:]), false))
	}
	initializer := context.ConstStruct(fields, true)
	g := llvm.AddGlobal(module, initializer.Type(), "__go_asm_metadata_v1")
	g.SetInitializer(initializer)
	g.SetGlobalConstant(true)
	g.SetLinkage(llvm.PrivateLinkage)
	g.SetAlignment(nativePointerSize)
	section := ".go.asm"
	if options.ObjectFormat == "macho" {
		section = "__DATA_CONST,__go_asm"
	}
	g.SetSection(section)
	// Until the native Go metadata linker can prune records with their owners,
	// retain the complete module. References from the record also keep private
	// assembly functions/data alive during IR optimization and LTO.
	used := llvm.AddGlobal(module, llvm.ArrayType(g.Type(), 1), "llvm.used")
	used.SetInitializer(llvm.ConstArray(g.Type(), []llvm.Value{g}))
	used.SetLinkage(llvm.AppendingLinkage)
	used.SetSection("llvm.metadata")
	return nil
}
