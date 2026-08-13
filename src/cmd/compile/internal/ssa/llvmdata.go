//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/typecheck"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/goallc/go-llvm"
)

// emitGoObjImportMetadata carries the exact package path and linker
// fingerprint already decoded by the Go importer. LLVM must not rediscover
// either value from an importcfg file or a symbol-name prefix.
func emitGoObjImportMetadata() {
	for _, imp := range base.Ctxt.Imports {
		CurrentModule.AddNamedMetadataOperand("goobj.imports", GlobalCtxt.MDNode([]llvm.Metadata{
			GlobalCtxt.MDString(imp.Pkg),
			GlobalCtxt.MDString(objabi.PathToPrefix(imp.Pkg)),
			GlobalCtxt.MDString(hex.EncodeToString(imp.Fingerprint[:])),
		}))
	}
}

func emitGoObjCgoModuleAsm() {
	if len(typecheck.Target.CgoPragmas) == 0 {
		return
	}
	data, err := json.Marshal(typecheck.Target.CgoPragmas)
	if err != nil {
		base.Fatalf("serializing cgo pragmas for LLVM: %v", err)
	}
	// Cgo pragmas are a textual section of the Go object header, so carry
	// them through LLVM as an object-format directive rather than IR metadata.
	CurrentModule.SetInlineAsm(".goobj.cgo " + strconv.Quote(string(data)) + "\n")
}

// attachGoObjSymbolRef attaches the part of an undefined Go symbol's identity
// that cannot be recovered from an LLVM relocation. The optimized relocation
// stream still decides which declarations become GoObj references; the
// attachment only supplies the imported package's symbol index or the builtin
// index when such a relocation survives.
func attachGoObjSymbolRef(value llvm.Value, s *obj.LSym) {
	if value.IsNil() || s == nil {
		base.Fatalf("invalid LLVM value in GoObj symbol reference")
	}

	if !base.Ctxt.Flag_linkshared && !s.IsLinkname() {
		if idx := goobj.BuiltinIdx(s.Name, int(s.ABI())); idx >= 0 {
			value.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.builtin"), GlobalCtxt.MDNode([]llvm.Metadata{
				llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(idx), false).ConstantAsMetadata(),
			}))
			return
		}
	}
	// Linknamed symbols live in GoObj's non-package namespace even when the
	// compiler learned about them through an imported package. Their export
	// symbol index addresses that package's ordinary symbol block and must not
	// be attached to the LLVM declaration as an imported reference.
	if s.PkgIdx == goobj.PkgIdxNone || s.IsLinkname() {
		return
	}
	localPkg := objabi.PathToPrefix(base.Ctxt.Pkgpath)
	if s.Pkg == "" || s.Pkg == `""` || s.Pkg == "_" || s.Pkg == localPkg || !s.Indexed() {
		return
	}
	if s.SymIdx < 0 {
		base.Fatalf("negative indexed Go symbol reference %s", s.Name)
	}
	var flags2 uint64
	if s.UsedInIface() {
		flags2 = goobj.SymFlagUsedInIface
	}
	value.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.import"), GlobalCtxt.MDNode([]llvm.Metadata{
		GlobalCtxt.MDString(s.Pkg),
		llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(s.SymIdx), false).ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int32Type(), flags2, false).ConstantAsMetadata(),
	}))
}

// attachGoObjABISymbolRef is for LLVM-generated runtime calls that do not
// carry an SSA AuxCall. Use the compiler's ABI-aware symbol table so their
// builtin/non-package classification stays identical to the native writer.
func attachGoObjABISymbolRef(value llvm.Value, name string, abi obj.ABI) {
	s := base.Ctxt.LookupABI(name, abi)
	if s == nil || s.Name != name || s.ABI() != abi {
		base.Fatalf("invalid LLVM GoObj symbol model for %s", name)
	}
	attachGoObjSymbolRef(value, s)
}

// LowerGoObjData lowers compiler-emitted linker data into LLVM globals. The
// Go front end remains responsible for laying out the data: this code only
// preserves the already-finalized LSym bytes, relocations, and GoObj symbol
// attributes in LLVM IR.
//
// Keeping this boundary at LSym is intentional. reflectdata is the source of
// truth for runtime layouts and is also used by the native backend; duplicating
// it here would make LLVM and native type descriptors drift independently.
func LowerGoObjData() {
	data := make(map[*obj.LSym]bool, len(base.Ctxt.Data))
	for _, s := range base.Ctxt.Data {
		data[s] = true
	}

	closure := make(map[*obj.LSym]bool)
	var visit func(*obj.LSym)
	visit = func(s *obj.LSym) {
		if s == nil || closure[s] || !data[s] {
			return
		}
		closure[s] = true
		visit(s.Gotype)
		for _, r := range s.R {
			visit(r.Sym)
		}
	}
	for _, s := range base.Ctxt.Data {
		if llvmDataSymbolKindSupported(s.Type) {
			visit(s)
		}
	}
	if currentLLVMDataLowerer != nil {
		for s := range currentLLVMDataLowerer.roots {
			visit(s)
		}
	}

	if len(closure) == 0 {
		emitGoObjCompilerUsed()
		return
	}

	syms := make([]*obj.LSym, 0, len(closure))
	for s := range closure {
		syms = append(syms, s)
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].Name < syms[j].Name })
	lowerer := currentLLVMDataLowerer
	if lowerer == nil {
		lowerer = newLLVMDataLowerer(data)
		currentLLVMDataLowerer = lowerer
	} else {
		lowerer.data = data
	}

	globals := lowerer.values
	for _, s := range syms {
		lowerer.lowered[s] = true
		t := lowerer.dataType(s)
		g := globals[s]
		if g.IsNil() && s.Name != "" {
			g = CurrentModule.NamedGlobal(s.Name)
		}
		if g.IsNil() {
			g = llvm.AddGlobal(CurrentModule, t, lowerer.globalName(s))
		} else if g.GlobalValueType() != t {
			// Some compiler-generated symbols are referenced by SSA before
			// dumpdata has attached their final TypeInfo and relocation
			// layout. LLVM opaque pointers make those early references
			// independent of the global's pointee type, so replace the
			// provisional declaration once the LSym is finalized.
			name := g.Name()
			replacement := llvm.AddGlobal(CurrentModule, t, name+".goallc.final")
			g.ReplaceAllUsesWith(replacement)
			g.EraseFromParentAsGlobal()
			replacement.SetName(name)
			g = replacement
		}
		g.SetSection(llvmDataSection(s))
		g.SetGlobalConstant(llvmDataIsReadOnly(s))
		setLLVMSymbolLinkage(g, s)
		if s.Align != 0 {
			g.SetAlignment(int(s.Align))
		}
		globals[s] = g
	}

	for _, s := range syms {
		g := globals[s]
		g.SetInitializer(lowerer.dataInitializer(s, globals))
		setGoObjDataFlags(g, s)
		setGoObjOffsetRelocMetadata(g, s)
		setGoObjWeakRelocMetadata(g, s)
		setGoObjKeepMetadata(g, s)
		setGoObjGotypeMetadata(g, s)
		setGoObjMarkerRelocMetadata(g, s)
		if lowerer.externalRoots[s] {
			// This definition is referenced from a different archive member, so
			// its use is invisible to LLVM. Keep it present and distinct through
			// GlobalDCE and ConstantMerge; Go linker reachability still decides
			// whether the GoObj symbol survives in the final binary.
			preserveGoObjMetadataValues(g)
		}
	}
	for s := range lowerer.externalRoots {
		if !lowerer.lowered[s] {
			base.Fatalf("GoObj data referenced outside LLVM was not lowered: %s", s.Name)
		}
	}
	emitGoObjCompilerUsed()
}

// MarkGoObjDataReferencedOutsideLLVM marks compiler data definitions whose
// references live in another archive member and therefore cannot participate
// in LLVM IR reachability. The definitions are kept only through object
// emission; the Go linker remains responsible for final reachability.
func MarkGoObjDataReferencedOutsideLLVM(syms ...*obj.LSym) {
	if currentLLVMDataLowerer == nil {
		base.Fatalf("marking external GoObj data before LLVM module initialization")
	}
	for _, s := range syms {
		if s == nil {
			base.Fatalf("marking nil GoObj data referenced outside LLVM")
		}
		currentLLVMDataLowerer.externalRoots[s] = true
	}
}

// FinalizeGoObjSymbolMetadata carries native GoObj definition classes and
// package-local indices after NumberSyms has assigned them. LowerGoObjData runs
// first so imported-reference metadata retains the same pre-numbering
// classification as the established LLVM path.
func FinalizeGoObjSymbolMetadata() {
	var syms []*obj.LSym
	if currentLLVMDataLowerer != nil {
		syms = make([]*obj.LSym, 0, len(currentLLVMDataLowerer.lowered))
		for s := range currentLLVMDataLowerer.lowered {
			syms = append(syms, s)
		}
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].Name < syms[j].Name })
	for _, s := range syms {
		g := currentLLVMDataLowerer.values[s]
		if g.IsNil() {
			base.Fatalf("missing lowered LLVM global for finalized GoObj symbol %s", s.Name)
		}
		if s.PkgIdx == goobj.PkgIdxSelf {
			setGoObjPackageSymbolIndexMetadata(g, s)
		}
		if s.PkgIdx == goobj.PkgIdxNone {
			setGoObjNonPackageMetadata(g)
		}
		if s.ContentAddressable() {
			setGoObjContentHashMetadata(g, s)
		}
	}
	for _, s := range base.Ctxt.Text {
		fn := CurrentModule.NamedFunction(llvmFunctionStorageName(s.Name, llvmCallConv(s.ABI())))
		if fn.IsNil() {
			continue
		}
		if s.PkgIdx == goobj.PkgIdxSelf {
			setGoObjPackageSymbolIndexMetadata(fn, s)
		}
		if s.PkgIdx == goobj.PkgIdxNone {
			setGoObjNonPackageMetadata(fn)
		}
	}
}

func setGoObjPackageSymbolIndexMetadata(value llvm.Value, s *obj.LSym) {
	if value.IsNil() || s == nil || s.PkgIdx != goobj.PkgIdxSelf || !s.Indexed() || s.SymIdx < 0 {
		base.Fatalf("invalid LLVM GoObj package symbol index")
	}
	// Calls emitted before obj.NumberSyms may initially classify a runtime
	// builtin as an undefined PkgIdxBuiltin reference. When compiling runtime,
	// that same LLVM GlobalValue later becomes this package's definition.
	// Definitions are addressed by their package symbol index, so remove the
	// now-stale undefined-reference attachment before handing the module to the
	// GoObj AsmPrinter.
	value.EraseGlobalMetadata(GlobalCtxt.MDKindID("goobj.builtin"))
	value.EraseGlobalMetadata(GlobalCtxt.MDKindID("goobj.import"))
	value.SetGlobalMetadata(GlobalCtxt.MDKindID(goObjSymbolIndexMD), GlobalCtxt.MDNode([]llvm.Metadata{
		llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(s.SymIdx), false).ConstantAsMetadata(),
	}))
}

func setGoObjNonPackageMetadata(value llvm.Value) {
	value.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.symbol.nonpackage"), GlobalCtxt.MDNode([]llvm.Metadata{
		llvm.ConstInt(GlobalCtxt.Int1Type(), 1, false).ConstantAsMetadata(),
	}))
}

// GoObj content hashes depend on the compiler's native symbol classes,
// package indexes, and relocation identities. Carry the already-canonical
// result into LLVM instead of asking the LLVM object writer to infer it from
// linkage or symbol names.
func setGoObjContentHashMetadata(g llvm.Value, s *obj.LSym) {
	if !s.ContentAddressable() {
		return
	}
	hash := obj.ContentHash(base.Ctxt, s)
	g.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.content_hash"), GlobalCtxt.MDNode([]llvm.Metadata{
		GlobalCtxt.MDString(string(hash)),
	}))
}

// llvmGoDataRef returns the module-global address for a compiler LSym. Local
// data symbols use the same semantic type cache as final data lowering so an
// early OpAddr and the later initializer always agree on the global type.
func llvmGoDataRef(s *obj.LSym) llvm.Value {
	if s == nil {
		base.Fatalf("nil Go data symbol in LLVM lowering")
	}
	// FuncPCABI0 carries an ABI0 LSym through OpAddr, but bodyless assembly
	// functions still have the unresolved Sxxx kind here. Recover the semantic
	// function identity from the front end before choosing an LLVM GlobalValue;
	// ABI alone is insufficient because ordinary data symbols also use ABI0.
	if llvmGoFunctionSymbol(s) {
		data := map[*obj.LSym]bool(nil)
		if currentLLVMDataLowerer != nil {
			data = currentLLVMDataLowerer.data
		}
		return llvmExternalDataRef(s, data)
	}
	if currentLLVMDataLowerer != nil {
		if g := currentLLVMDataLowerer.values[s]; !g.IsNil() {
			attachGoObjSymbolRef(g, s)
			return g
		}
	}
	if s.Name != "" {
		if g := CurrentModule.NamedGlobal(s.Name); !g.IsNil() {
			attachGoObjSymbolRef(g, s)
			return g
		}
	}
	if currentLLVMDataLowerer == nil {
		currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	}
	currentLLVMDataLowerer.roots[s] = true
	local := false
	for _, candidate := range base.Ctxt.Data {
		if candidate == s {
			local = true
			break
		}
	}
	if !local && !llvmDataSymbolKindSupported(s.Type) {
		g := llvm.AddGlobal(CurrentModule, GlobalCtxt.Int8Type(), currentLLVMDataLowerer.globalName(s))
		currentLLVMDataLowerer.values[s] = g
		attachGoObjSymbolRef(g, s)
		return g
	}
	currentLLVMDataLowerer.data[s] = true
	g := llvm.AddGlobal(CurrentModule, currentLLVMDataLowerer.dataType(s), currentLLVMDataLowerer.globalName(s))
	currentLLVMDataLowerer.values[s] = g
	if s.Name == "" {
		// R_USENAMEDMETHOD deliberately uses an anonymous SRODATA symbol whose
		// payload is the method name. LLVM globals need a stable identity, but
		// the GoObj relocation remains a Self reference and the artificial name
		// has no linker semantics.
		g.SetLinkage(llvm.InternalLinkage)
	}
	attachGoObjSymbolRef(g, s)
	return g
}

func llvmGoFunctionSymbol(s *obj.LSym) bool {
	if s.Type == objabi.STEXT || s.Type == objabi.STEXTFIPS || s.ABI() == obj.ABIInternal {
		return true
	}
	// Bodyless assembly declarations are initialized without setupTextLSym, so
	// their LSym remains Sxxx. typecheck.Target.Funcs is the authoritative list
	// of current-package function declarations and includes generated ABI
	// wrappers before LLVM module initialization.
	if typecheck.Target == nil {
		return false
	}
	for _, fn := range typecheck.Target.Funcs {
		if fn == nil || fn.Nname == nil || fn.Sym() == nil || fn.Sym().Name == "_" {
			continue
		}
		if fn.LinksymABI(fn.ABI) == s {
			return true
		}
	}
	return false
}

func (l *llvmDataLowerer) globalName(s *obj.LSym) string {
	if s.Name != "" {
		return s.Name
	}
	l.anonymousCount++
	return fmt.Sprintf(".goallc.anon.%d", l.anonymousCount)
}

func llvmDataSymbolKindSupported(kind objabi.SymKind) bool {
	switch kind {
	case objabi.SRODATA, objabi.SRODATAFIPS, objabi.SNOPTRDATA, objabi.SNOPTRDATAFIPS,
		objabi.SDATA, objabi.SDATAFIPS, objabi.SBSS, objabi.SNOPTRBSS:
		return true
	default:
		return false
	}
}

func llvmDataType(s *obj.LSym) llvm.Type {
	fields := llvmDataFields(s, nil, nil)
	return GlobalCtxt.ConstStruct(fields, true).Type()
}

func llvmDataInitializer(s *obj.LSym, globals map[*obj.LSym]llvm.Value, data map[*obj.LSym]bool) llvm.Value {
	fields := llvmDataFields(s, globals, data)
	return GlobalCtxt.ConstStruct(fields, true)
}

// llvmDataFields models an LSym as a packed aggregate. Byte runs preserve the
// frontend's exact layout while relocation slots retain their LLVM constants.
// Using a packed aggregate is necessary because descriptors contain 32-bit
// offsets adjacent to pointer-sized fields.
func llvmDataFields(s *obj.LSym, globals map[*obj.LSym]llvm.Value, data map[*obj.LSym]bool) []llvm.Value {
	dataSize := int(s.Size)
	if dataSize < len(s.P) {
		dataSize = len(s.P)
	}
	bytes := make([]byte, dataSize)
	copy(bytes, s.P)
	relocs := llvmDataStorageRelocs(s)
	sort.Slice(relocs, func(i, j int) bool { return relocs[i].Off < relocs[j].Off })

	fields := make([]llvm.Value, 0, len(relocs)*2+1)
	pos := 0
	for _, r := range relocs {
		off := int(r.Off)
		end := off + int(r.Siz)
		if off < pos || end > dataSize {
			base.Fatalf("invalid relocation [%d,%d) in Go data symbol %s of size %d", off, end, s.Name, dataSize)
		}
		if off > pos {
			fields = append(fields, llvmDataBytes(bytes[pos:off]))
		}
		if globals == nil {
			fields = append(fields, llvmDataRelocZero(r))
		} else {
			fields = append(fields, llvmDataRelocValue(s, r, globals, data))
		}
		pos = end
	}
	if pos < dataSize {
		fields = append(fields, llvmDataBytes(bytes[pos:]))
	}
	if len(fields) == 0 {
		fields = append(fields, llvmDataBytes(nil))
	}
	return fields
}

func llvmDataBytes(b []byte) llvm.Value {
	return GlobalCtxt.ConstString(string(b), false)
}

func llvmDataRelocZero(r obj.Reloc) llvm.Value {
	switch r.Type {
	case objabi.R_ADDR, objabi.R_WEAKADDR:
		if r.Siz != uint8(base.Ctxt.Arch.PtrSize) {
			base.Fatalf("unsupported address relocation size %d", r.Siz)
		}
		return llvm.ConstPointerNull(GlobalCtxt.PointerType(0))
	case objabi.R_ADDROFF, objabi.R_WEAKADDROFF, objabi.R_METHODOFF:
		if r.Siz != 4 {
			base.Fatalf("unsupported offset relocation size %d", r.Siz)
		}
		return llvm.ConstInt(GlobalCtxt.Int32Type(), 0, false)
	default:
		base.Fatalf("unsupported Go data relocation %s", r.Type)
		return llvm.Value{}
	}
}

func llvmDataRelocValue(source *obj.LSym, r obj.Reloc, globals map[*obj.LSym]llvm.Value, data map[*obj.LSym]bool) llvm.Value {
	if r.Sym == nil {
		base.Fatalf("nil relocation target in Go data symbol %s", source.Name)
	}
	target, ok := globals[r.Sym]
	if !ok {
		target = llvmExternalDataRef(r.Sym, data)
	}
	addr := target
	if r.Add != 0 {
		if !target.IsAFunction().IsNil() {
			base.Fatalf("non-zero addend %d on function relocation %s -> %s", r.Add, source.Name, r.Sym.Name)
		}
		addr = llvm.ConstGEP(GlobalCtxt.Int8Type(), target, []llvm.Value{
			llvm.ConstInt(GlobalCtxt.Int64Type(), uint64(r.Add), true),
		})
	}

	switch r.Type {
	case objabi.R_ADDR, objabi.R_WEAKADDR:
		if r.Siz != uint8(base.Ctxt.Arch.PtrSize) {
			base.Fatalf("unsupported address relocation size %d", r.Siz)
		}
		return addr
	case objabi.R_ADDROFF, objabi.R_WEAKADDROFF, objabi.R_METHODOFF:
		if r.Siz != 4 {
			base.Fatalf("unsupported offset relocation size %d", r.Siz)
		}
		return llvm.ConstPtrToInt(addr, GlobalCtxt.Int32Type())
	default:
		base.Fatalf("unsupported Go data relocation %s in %s", r.Type, source.Name)
		return llvm.Value{}
	}
}

func llvmExternalDataRef(s *obj.LSym, data map[*obj.LSym]bool) llvm.Value {
	if data[s] {
		base.Fatalf("unlowered Go data symbol %s", s.Name)
	}
	// Runtime references materialized by reflectdata are frequently still Sxxx
	// at this point. Their ABI nevertheless identifies them as functions (for
	// example runtime.memequal64 in an equality closure), so do not rely on
	// STEXT alone here.
	if llvmGoFunctionSymbol(s) {
		storageName := llvmFunctionStorageName(s.Name, llvmCallConv(s.ABI()))
		if f := CurrentModule.NamedFunction(storageName); !f.IsNil() {
			attachGoObjSymbolRef(f, s)
			return f
		}
		f := llvm.AddFunction(CurrentModule, storageName, llvm.FunctionType(GlobalCtxt.VoidType(), nil, false))
		f.SetFunctionCallConv(llvmCallConv(s.ABI()))
		attachGoObjSymbolRef(f, s)
		return f
	}
	if g := CurrentModule.NamedGlobal(s.Name); !g.IsNil() {
		attachGoObjSymbolRef(g, s)
		return g
	}
	g := llvm.AddGlobal(CurrentModule, GlobalCtxt.Int8Type(), s.Name)
	attachGoObjSymbolRef(g, s)
	return g
}

func llvmDataSection(s *obj.LSym) string {
	switch s.Type {
	case objabi.SRODATA, objabi.SRODATAFIPS:
		return ".rodata"
	case objabi.SNOPTRDATA, objabi.SNOPTRDATAFIPS:
		return ".noptrdata"
	case objabi.SDATA, objabi.SDATAFIPS:
		return ".data"
	case objabi.SBSS:
		return ".bss"
	case objabi.SNOPTRBSS:
		return ".noptrbss"
	default:
		base.Fatalf("unsupported Go data symbol kind %s for %s", s.Type, s.Name)
		return ""
	}
}

func llvmDataIsReadOnly(s *obj.LSym) bool {
	switch s.Type {
	case objabi.SRODATA, objabi.SRODATAFIPS:
		return true
	default:
		return false
	}
}

func setLLVMSymbolLinkage(value llvm.Value, s *obj.LSym) {
	if s.Local() {
		value.SetLinkage(llvm.InternalLinkage)
	} else if s.DuplicateOK() {
		value.SetLinkage(llvm.WeakAnyLinkage)
	}
}

func setGoObjDataFlags(g llvm.Value, s *obj.LSym) {
	var flag, flag2 uint64
	// Local and non-local Dupok symbols use LLVM linkage. LLVM cannot encode
	// both properties at once, so only a Local+Dupok overlap needs a residual
	// metadata bit.
	if s.Local() && s.DuplicateOK() {
		flag |= 1 << 0 // goobj.SymFlagDupok
	}
	if s.MakeTypelink() {
		flag |= 1 << 2 // goobj.SymFlagTypelink
	}
	if s.TypeInfo() != nil {
		// Typed descriptor globals are intentionally literal structs so their
		// exact variable tail remains visible in IR. Carry the GoType bit
		// explicitly rather than relying on a named LLVM wrapper type.
		flag |= 1 << 6 // goobj.SymFlagGoType
	}
	if s.UsedInIface() {
		flag2 |= 1 << 0 // goobj.SymFlagUsedInIface
	}
	if s.ItabInfo() != nil {
		flag2 |= 1 << 1 // goobj.SymFlagItab
	}
	if s.IsLinkname() {
		flag2 |= 1 << 4 // goobj.SymFlagLinkname
	}
	if s.ABIWrapper() {
		flag2 |= 1 << 5 // goobj.SymFlagABIWrapper
	}
	if flag == 0 && flag2 == 0 {
		return
	}
	g.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.symbol.flags"), GlobalCtxt.MDNode([]llvm.Metadata{
		llvm.ConstInt(GlobalCtxt.Int32Type(), flag, false).ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int32Type(), flag2, false).ConstantAsMetadata(),
	}))
}

// Functions carry linker-visible semantic flags in the same GoObj symbol
// record as native compiler output. Local and non-local Dupok symbols use LLVM
// linkage and are recovered by AsmPrinter. LLVM cannot encode both properties
// at once, so only a Local+Dupok overlap needs a residual metadata bit.
func setGoObjFunctionFlags(fn llvm.Value, s *obj.LSym) {
	var flag, flag2 uint64
	if s.Local() && s.DuplicateOK() {
		flag |= goobj.SymFlagDupok
	}
	if s.ReflectMethod() {
		flag |= goobj.SymFlagReflectMethod
	}
	if s.NoSplit() {
		flag |= goobj.SymFlagNoSplit
	}
	if s.IsPkgInit() {
		flag2 |= goobj.SymFlagPkgInit
	}
	if s.IsLinkname() || s.Name == "main.main" {
		flag2 |= goobj.SymFlagLinkname
	}
	if s.IsLinknameStd() {
		flag2 |= goobj.SymFlagLinknameStd
	}
	if s.ABIWrapper() {
		flag2 |= goobj.SymFlagABIWrapper
	}
	if flag == 0 && flag2 == 0 {
		return
	}
	fn.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.symbol.flags"), GlobalCtxt.MDNode([]llvm.Metadata{
		llvm.ConstInt(GlobalCtxt.Int32Type(), flag, false).ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int32Type(), flag2, false).ConstantAsMetadata(),
	}))
}

// FuncIDWrapper is part of recover's semantic stack walk: a deferred-call
// wrapper must not count as an extra frame between gopanic and recover. Carry
// both FuncID and FuncFlag into LLVM so the GoObj writer can reproduce the
// native compiler's FuncInfo record.
func setGoObjFunctionInfo(fn llvm.Value, s *obj.LSym) {
	info := s.Func()
	if info == nil {
		base.Fatalf("missing Go function info for %s", s.Name)
	}
	if info.FuncID == 0 && info.FuncFlag == 0 {
		return
	}
	fn.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.func.info"), GlobalCtxt.MDNode([]llvm.Metadata{
		llvm.ConstInt(GlobalCtxt.Int8Type(), uint64(info.FuncID), false).ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int8Type(), uint64(info.FuncFlag), false).ConstantAsMetadata(),
	}))
}

// LLVM can express the address relationship but not GoObj's 32-bit section
// offsets. Record the object-format-specific relocation type explicitly;
// weakness remains orthogonal in !goobj.weak_relocs.
func setGoObjOffsetRelocMetadata(g llvm.Value, s *obj.LSym) {
	entries := make([]llvm.Metadata, 0)
	for _, r := range s.R {
		var typ objabi.RelocType
		switch r.Type {
		case objabi.R_ADDROFF, objabi.R_METHODOFF:
			typ = r.Type
		case objabi.R_WEAKADDROFF:
			typ = objabi.R_ADDROFF
		default:
			continue
		}
		entries = append(entries, GlobalCtxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(r.Off), false).ConstantAsMetadata(),
			llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(typ), false).ConstantAsMetadata(),
		}))
	}
	if len(entries) != 0 {
		g.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.relocs"), GlobalCtxt.MDNode(entries))
	}
}

func setGoObjWeakRelocMetadata(g llvm.Value, s *obj.LSym) {
	entries := make([]llvm.Metadata, 0)
	for _, r := range s.R {
		switch r.Type {
		case objabi.R_WEAKADDR, objabi.R_WEAKADDROFF:
			entries = append(entries,
				llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(r.Off), false).ConstantAsMetadata())
		case objabi.R_ADDR, objabi.R_ADDROFF, objabi.R_METHODOFF:
			// LLVM constants carry the offset, size, target, and addend.
			// Offset relocation types are recorded separately in
			// !goobj.relocs.
		case objabi.R_KEEP, objabi.R_USEIFACE, objabi.R_USEIFACEMETHOD,
			objabi.R_USENAMEDMETHOD, objabi.R_INITORDER:
			continue
		default:
			base.Fatalf("unsupported Go data relocation %s in %s", r.Type, s.Name)
		}
	}
	if len(entries) != 0 {
		g.SetGlobalMetadata(GlobalCtxt.MDKindID("goobj.weak_relocs"), GlobalCtxt.MDNode(entries))
	}
}

// GoObj marker relocations describe linker relationships and occupy no storage.
// Keep their source and target as ordinary LLVM values, with the relocation
// kind and addend in a module relationship table shared by functions and data.
func setGoObjMarkerRelocMetadata(source llvm.Value, s *obj.LSym) {
	for _, r := range s.R {
		switch r.Type {
		case objabi.R_USEIFACE, objabi.R_USEIFACEMETHOD, objabi.R_USENAMEDMETHOD:
		case objabi.R_INITORDER:
			if r.Off != 0 || r.Siz != 0 || r.Add != 0 {
				base.Fatalf("invalid R_INITORDER relocation in %s", s.Name)
			}
		default:
			continue
		}
		if r.Sym == nil {
			base.Fatalf("nil GoObj marker target in %s", s.Name)
		}
		target := llvmGoDataRef(r.Sym)
		addGoObjMarkerReloc(source, target, r.Type, r.Add)
	}
}

// Function marker relocations participate in linker reachability but carry no
// storage. Represent them as inlineable side-effect markers until the LLVM
// optimization pipeline has finished. The statepoint plugin then anchors each
// cloned marker to its final containing function and removes the intrinsic.
func emitGoObjFunctionMarkerRelocs(b llvm.Builder, s *obj.LSym) {
	var sideeffect llvm.Value
	for _, r := range s.R {
		switch r.Type {
		case objabi.R_USEIFACE, objabi.R_USEIFACEMETHOD, objabi.R_USENAMEDMETHOD:
		case objabi.R_INITORDER:
			if r.Off != 0 || r.Siz != 0 || r.Add != 0 {
				base.Fatalf("invalid R_INITORDER relocation in %s", s.Name)
			}
		default:
			continue
		}
		if r.Sym == nil {
			base.Fatalf("nil GoObj marker target in %s", s.Name)
		}
		if sideeffect.IsNil() {
			sideeffect = getLLVMIntrinsicDeclaration("llvm.sideeffect")
		}
		target := llvmGoDataRef(r.Sym)
		preserveGoObjMetadataValues(target)
		marker := b.CreateCall(sideeffect.GlobalValueType(), sideeffect, nil, "")
		marker.SetMetadata(GlobalCtxt.MDKindID(goObjMarkerRelocMD), GlobalCtxt.MDNode([]llvm.Metadata{
			target.ConstantAsMetadata(),
			llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(uint16(r.Type)), false).ConstantAsMetadata(),
			llvm.ConstInt(GlobalCtxt.Int64Type(), uint64(r.Add), true).ConstantAsMetadata(),
		}))
	}
}

// R_KEEP is a zero-width linker reachability edge, not a storage relocation.
// Keep it separate from !goobj.weak_relocs so llc can synthesize the GoObj record
// without inventing bytes or a target-address expression in the global.
func setGoObjKeepMetadata(g llvm.Value, s *obj.LSym) {
	for _, r := range s.R {
		if r.Type != objabi.R_KEEP {
			continue
		}
		if r.Sym == nil {
			base.Fatalf("nil R_KEEP target in %s", s.Name)
		}
		target := llvmGoDataRef(r.Sym)
		preserveGoObjMetadataValues(g, target)
		CurrentModule.AddNamedMetadataOperand("goobj.keep", GlobalCtxt.MDNode([]llvm.Metadata{
			g.ConstantAsMetadata(),
			target.ConstantAsMetadata(),
		}))
	}
}

func setGoObjGotypeMetadata(g llvm.Value, s *obj.LSym) {
	if s.Gotype == nil {
		return
	}
	target := CurrentModule.NamedGlobal(s.Gotype.Name)
	if target.IsNil() {
		target = llvmGoDataRef(s.Gotype)
	}
	preserveGoObjMetadataValues(g, target)
	CurrentModule.AddNamedMetadataOperand("goobj.gotype", GlobalCtxt.MDNode([]llvm.Metadata{
		g.ConstantAsMetadata(),
		target.ConstantAsMetadata(),
	}))
}

func addGoObjMarkerReloc(source, target llvm.Value, typ objabi.RelocType, addend int64) {
	preserveGoObjMetadataValues(source, target)
	CurrentModule.AddNamedMetadataOperand("goobj.marker_relocs", goObjMarkerRelocMetadata(source, target, typ, addend))
}

func goObjMarkerRelocMetadata(source, target llvm.Value, typ objabi.RelocType, addend int64) llvm.Metadata {
	return GlobalCtxt.MDNode([]llvm.Metadata{
		source.ConstantAsMetadata(),
		target.ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int32Type(), uint64(uint16(typ)), false).ConstantAsMetadata(),
		llvm.ConstInt(GlobalCtxt.Int64Type(), uint64(addend), true).ConstantAsMetadata(),
	})
}

// Named metadata references participate in RAUW, but GlobalDCE is allowed to
// delete values that are referenced only from metadata. llvm.compiler.used is
// LLVM's native way to state that those values also have object-format
// semantics. It emits no storage; Go linker reachability remains controlled by
// the R_KEEP and marker records produced from the relationship tables.
func preserveGoObjMetadataValues(values ...llvm.Value) {
	for _, v := range values {
		if v.IsNil() || v.Name() == "" {
			base.Fatalf("invalid LLVM value in GoObj metadata relationship")
		}
		if goObjCompilerUsedNames[v.Name()] {
			continue
		}
		goObjCompilerUsedNames[v.Name()] = true
		goObjCompilerUsed = append(goObjCompilerUsed, v)
	}
}

func emitGoObjCompilerUsed() {
	if len(goObjCompilerUsed) == 0 {
		return
	}
	sort.Slice(goObjCompilerUsed, func(i, j int) bool {
		return goObjCompilerUsed[i].Name() < goObjCompilerUsed[j].Name()
	})
	values := append([]llvm.Value(nil), goObjCompilerUsed...)
	if old := CurrentModule.NamedGlobal("llvm.compiler.used"); !old.IsNil() {
		init := old.Initializer()
		for i := 0; i < init.OperandsCount(); i++ {
			values = append(values, init.Operand(i))
		}
		old.EraseFromParentAsGlobal()
	}
	init := llvm.ConstArray(GlobalCtxt.PointerType(0), values)
	used := llvm.AddGlobal(CurrentModule, init.Type(), "llvm.compiler.used")
	used.SetLinkage(llvm.AppendingLinkage)
	used.SetSection("llvm.metadata")
	used.SetInitializer(init)
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
}

func llvmDataStorageRelocs(s *obj.LSym) []obj.Reloc {
	relocs := make([]obj.Reloc, 0, len(s.R))
	for _, r := range s.R {
		switch r.Type {
		case objabi.R_KEEP, objabi.R_USEIFACE, objabi.R_USEIFACEMETHOD,
			objabi.R_USENAMEDMETHOD, objabi.R_INITORDER:
			continue
		}
		relocs = append(relocs, r)
	}
	return relocs
}
