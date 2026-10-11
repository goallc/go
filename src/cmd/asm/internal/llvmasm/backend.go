// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"fmt"
	"internal/buildcfg"
	"sort"
	"strings"

	"github.com/goallc/go-llvm"
)

// Emit builds an LLVM module in memory. Inline assembly is passed to LLVM's
// integrated assembler; no intermediate assembly file or native Go object is
// used. The returned bytes are owned by Go.
func Emit(ctxt *obj.Link, options Options) ([]byte, error) {
	functions, err := lower(ctxt, options)
	if err != nil {
		return nil, err
	}
	if options.Output != "obj" && options.Output != "ir" && options.Output != "bc" {
		return nil, fmt.Errorf("unknown LLVM assembly output %q (want obj, bc, or ir)", options.Output)
	}
	ctxt.FinalizeConstSyms()
	if options.isGoObj() {
		// Reuse Go's exact content hashes, including its symbol classes and
		// relocation identities. Synthesized constants must not become strong
		// named definitions in every assembly object that happens to use them.
		ctxt.NumberSyms()
	}
	ctxt.PrintAssembly()
	llvm.InitializeAllTargetInfos()
	llvm.InitializeAllTargets()
	llvm.InitializeAllTargetMCs()
	llvm.InitializeAllAsmPrinters()
	llvm.InitializeAllAsmParsers()
	triple, err := options.triple()
	if err != nil {
		return nil, err
	}
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		return nil, err
	}
	relocation := llvm.RelocStatic
	if ctxt.Flag_shared {
		relocation = llvm.RelocPIC
	}
	tm := target.CreateTargetMachine(triple, "generic", "", llvm.CodeGenLevelDefault, relocation, llvm.CodeModelDefault)
	defer tm.Dispose()
	context := llvm.NewContext()
	defer context.Dispose()
	module := context.NewModule(ctxt.Pkgpath)
	defer module.Dispose()
	module.SetTarget(triple)
	if ctxt.Flag_shared {
		// Match Module::setPICLevel so standalone bitcode codegen retains the
		// relocation model selected by the Go assembler's -shared flag.
		const moduleFlagMin = 8
		const bigPIC = 2
		module.AddNamedMetadataOperand("llvm.module.flags", context.MDNode([]llvm.Metadata{
			llvm.ConstInt(context.Int32Type(), moduleFlagMin, false).ConstantAsMetadata(),
			context.MDString("PIC Level"),
			llvm.ConstInt(context.Int32Type(), bigPIC, false).ConstantAsMetadata(),
		}))
	}
	if options.isGoObj() {
		module.SetInlineAsm(".goobj.assembly\n")
		key, setting := buildcfg.GOGOARCH()
		// Tests may cross-emit without changing the process build configuration.
		if options.GOARCH == "amd64" && key != "GOAMD64" {
			key, setting = "GOAMD64", "v1"
		}
		if options.GOARCH == "arm64" && key != "GOARM64" {
			key, setting = "GOARM64", "v8.0"
		}
		var experiments []llvm.Metadata
		for _, exp := range buildcfg.Experiment.Enabled() {
			experiments = append(experiments, context.MDString(exp))
		}
		boolString := func(b bool) string {
			if b {
				return "1"
			}
			return "0"
		}
		var config []llvm.Metadata
		for _, field := range []string{"goallc.goobj", options.GOOS, options.GOARCH, buildcfg.Version, key, setting, "", objabi.PathToPrefix(ctxt.Pkgpath), "0", boolString(ctxt.Flag_shared), boolString(ctxt.Std)} {
			config = append(config, context.MDString(field))
		}
		config = append(config, context.MDNode(experiments), context.MDString("0000000000000000"))
		module.AddNamedMetadataOperand("goobj.config", context.MDNode(config))
		if err := llvm.ConfigureGoObjFromModule(module); err != nil {
			return nil, err
		}
	}
	setFlags := func(v llvm.Value, s *obj.LSym) {
		if !options.isGoObj() {
			return
		}
		flag, flag2 := ctxt.SymbolFlags(s)
		v.SetGlobalMetadata(context.MDKindID("goobj.symbol.flags.exact"), context.MDNode([]llvm.Metadata{
			llvm.ConstInt(context.Int8Type(), uint64(flag), false).ConstantAsMetadata(),
			llvm.ConstInt(context.Int8Type(), uint64(flag2), false).ConstantAsMetadata(),
		}))
	}
	td := tm.CreateTargetData()
	module.SetDataLayout(td.String())
	td.Dispose()
	functionType := llvm.FunctionType(context.VoidType(), nil, false)
	values := make(map[reference]llvm.Value)
	// A symbol used as an address and as a branch must have one LLVM identity.
	text := make(map[*obj.LSym]bool)
	for _, f := range functions {
		text[f.sym] = true
		for _, r := range f.refs {
			if r.text {
				text[r.sym] = true
			}
		}
	}
	canonical := func(r reference) reference {
		r.text = r.text || text[r.sym] || r.sym.ABI() == obj.ABIInternal || r.sym.Type == objabi.STEXT || r.sym.Type == objabi.STEXTFIPS
		return r
	}
	name := func(r reference) string {
		n := r.sym.Name
		if n == "" {
			n = ".goasm.anon"
		}
		if r.text && r.sym.ABI() == obj.ABI0 {
			n += "<ABI0>"
		}
		return n
	}
	var value func(reference) llvm.Value
	value = func(r reference) llvm.Value {
		r = canonical(r)
		if v, ok := values[r]; ok {
			return v
		}
		var v llvm.Value
		if r.text {
			v = llvm.AddFunction(module, name(r), functionType)
			cc := llvm.CallConv(23)
			if r.sym.ABI() == obj.ABIInternal {
				cc = 22
			}
			v.SetFunctionCallConv(cc)
		} else {
			v = llvm.AddGlobal(module, context.Int8Type(), name(r))
			if r.sym.Type == objabi.STLSBSS || r.sym.Name == "runtime.tlsg" {
				v.SetThreadLocal(true)
			}
		}
		values[r] = v
		return v
	}
	// Plan 9 DATA is still represented by finalized bytes and symbolic relocations.
	// It contains no instruction bytes and is lowered to ordinary LLVM globals.
	for _, s := range ctxt.Data {
		value(reference{s, false})
	}
	for _, f := range functions {
		value(reference{f.sym, true})
		for _, r := range f.refs {
			value(r)
		}
	}
	for _, s := range ctxt.Data {
		r := canonical(reference{s, false})
		old := value(r)
		if r.text {
			return nil, fmt.Errorf("assembly symbol %s is both text and data", s.Name)
		}
		relocs := append([]obj.Reloc(nil), s.R...)
		sort.Slice(relocs, func(i, j int) bool { return relocs[i].Off < relocs[j].Off })
		if s.Size < 0 || s.Size > obj.MaxSymSize || int64(len(s.P)) > s.Size {
			return nil, fmt.Errorf("invalid assembly data size for %s", s.Name)
		}
		var fields []llvm.Value
		appendBytes := func(begin, end int) {
			if begin < len(s.P) {
				n := min(end, len(s.P))
				fields = append(fields, context.ConstString(string(s.P[begin:n]), false))
				begin = n
			}
			if begin < end {
				fields = append(fields, llvm.ConstNull(llvm.ArrayType(context.Int8Type(), end-begin)))
			}
		}
		pos := 0
		for _, rel := range relocs {
			if rel.Type != objabi.R_ADDR || rel.Siz != 8 || rel.Sym == nil {
				return nil, fmt.Errorf("unsupported assembly data relocation %s in %s", rel.Type, s.Name)
			}
			off := int(rel.Off)
			if off < pos || int64(off+8) > s.Size {
				return nil, fmt.Errorf("invalid assembly data relocation offset in %s", s.Name)
			}
			if off > pos {
				appendBytes(pos, off)
			}
			addr := value(reference{rel.Sym, false})
			if rel.Add != 0 {
				addr = llvm.ConstGEP(context.Int8Type(), addr, []llvm.Value{llvm.ConstInt(context.Int64Type(), uint64(rel.Add), true)})
			}
			fields = append(fields, addr)
			pos = off + 8
		}
		appendBytes(pos, int(s.Size))
		init := context.ConstStruct(fields, true)
		old.SetName(name(r) + ".declaration")
		g := llvm.AddGlobal(module, init.Type(), name(r))
		old.ReplaceAllUsesWith(g)
		old.EraseFromParentAsGlobal()
		values[r] = g
		g.SetInitializer(init)
		setFlags(g, s)
		if s.ContentAddressable() && options.isGoObj() {
			g.SetGlobalMetadata(context.MDKindID("goobj.content_hash"), context.MDNode([]llvm.Metadata{
				context.MDString(string(obj.ContentHash(ctxt, s))),
			}))
		}
		switch s.Type {
		case objabi.SNOPTRDATA:
			g.SetSection(options.dataSection(".noptrdata"))
		case objabi.SNOPTRBSS:
			g.SetSection(options.dataSection(".noptrbss"))
		case objabi.SBSS:
			g.SetSection(options.dataSection(".bss"))
		case objabi.SRODATA:
			g.SetSection(options.dataSection(".rodata"))
		case objabi.STLSBSS:
			g.SetSection(options.dataSection(".tbss"))
		case objabi.SRODATAFIPS:
			g.SetSection(options.dataSection(".rodata.fips"))
		case objabi.SNOPTRDATAFIPS:
			g.SetSection(options.dataSection(".noptrdata.fips"))
		case objabi.SDATAFIPS:
			g.SetSection(options.dataSection(".data.fips"))
		default:
			g.SetSection(options.dataSection(".data"))
		}
		alignment := int(s.Align)
		if alignment == 0 {
			alignment = 32
			for int64(alignment) > s.Size && alignment > 1 {
				alignment >>= 1
			}
		}
		g.SetAlignment(alignment)
		if s.ContentAddressable() && !options.isGoObj() {
			g.SetLinkage(llvm.PrivateLinkage)
		} else if s.Static() || s.Name == "" {
			g.SetLinkage(llvm.InternalLinkage)
		} else if s.DuplicateOK() {
			g.SetLinkage(llvm.WeakAnyLinkage)
		}
		if s.Local() && g.Linkage() != llvm.PrivateLinkage && g.Linkage() != llvm.InternalLinkage {
			g.SetVisibility(llvm.HiddenVisibility)
		}
		if s.Type == objabi.SRODATA || s.Type == objabi.SRODATAFIPS {
			g.SetGlobalConstant(true)
		}
		if s.Type == objabi.STLSBSS {
			g.SetThreadLocal(true)
		}
	}
	builder := context.NewBuilder()
	defer builder.Dispose()
	for _, f := range functions {
		fn := value(reference{f.sym, true})
		if f.sym.Type == objabi.STEXTFIPS {
			if options.ObjectFormat == "macho" {
				fn.SetSection("__TEXT,__text_fips,regular,pure_instructions")
			} else {
				fn.SetSection(".text.fips")
			}
		}
		if f.sym.Static() {
			fn.SetLinkage(llvm.InternalLinkage)
		} else if f.sym.DuplicateOK() {
			fn.SetLinkage(llvm.WeakAnyLinkage)
		}
		if f.sym.Local() && fn.Linkage() != llvm.InternalLinkage {
			fn.SetVisibility(llvm.HiddenVisibility)
		}
		setFlags(fn, f.sym)
		if f.sym.Align > 0 {
			fn.SetAlignment(int(f.sym.Align))
		}
		if features := assemblyFeatures(options.GOARCH); features != "" {
			fn.AddFunctionAttr(context.CreateStringAttribute("target-features", features))
		}
		fn.AddFunctionAttr(context.CreateEnumAttribute(llvm.AttributeKindID("naked"), 0))
		fn.AddFunctionAttr(context.CreateEnumAttribute(llvm.AttributeKindID("noinline"), 0))
		builder.SetInsertPointAtEnd(context.AddBasicBlock(fn, "entry"))
		var operands []llvm.Value
		var types []llvm.Type
		var constraints []string
		for _, r := range f.refs {
			v := value(r)
			operands = append(operands, v)
			types = append(types, v.Type())
			constraint := "S"
			if options.GOARCH == "amd64" {
				constraint = "Ws"
			}
			constraints = append(constraints, constraint)
		}
		constraints = append(constraints, "~{memory}")
		ty := llvm.FunctionType(context.VoidType(), types, false)
		asm := llvm.InlineAsm(ty, f.body, strings.Join(constraints, ","), true, false, llvm.InlineAsmDialectATT, false)
		builder.CreateCall(ty, asm, operands, "")
		builder.CreateUnreachable()
	}
	if !options.isGoObj() {
		if err := emitNativeMetadata(context, module, ctxt, options, value); err != nil {
			return nil, err
		}
	}
	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		return nil, fmt.Errorf("verify assembly module: %w", err)
	}
	switch options.Output {
	case "ir":
		return []byte(module.String()), nil
	case "bc":
		buf := llvm.WriteBitcodeToMemoryBuffer(module)
		defer buf.Dispose()
		return append([]byte(nil), buf.Bytes()...), nil
	default:
		buf, err := tm.EmitToMemoryBuffer(module, llvm.ObjectFile)
		if err != nil {
			return nil, err
		}
		defer buf.Dispose()
		return append([]byte(nil), buf.Bytes()...), nil
	}
}
