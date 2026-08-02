//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"sort"

	"github.com/goallc/go-llvm"
)

// llvmDataLowerer provides two representations for compiler linker data. The
// generic representation is byte runs plus relocation slots. Runtime type
// descriptors additionally carry TypeInfo, so their fixed and variable layout
// can be reconstructed from the same rttype definitions that reflectdata used
// to write the LSym. This makes the important part of the IR inspectable while
// retaining the generic representation for auxiliary linker data.
type llvmDataLowerer struct {
	data             map[*obj.LSym]bool
	roots            map[*obj.LSym]bool
	values           map[*obj.LSym]llvm.Value
	anonymousCount   int
	runtimeTypes     map[*types.Type]llvm.Type
	descriptorTypes  map[*obj.LSym]llvm.Type
	namedRuntimeType map[*types.Type]bool
}

func newLLVMDataLowerer(data map[*obj.LSym]bool) *llvmDataLowerer {
	return &llvmDataLowerer{
		data:             data,
		roots:            make(map[*obj.LSym]bool),
		values:           make(map[*obj.LSym]llvm.Value),
		runtimeTypes:     make(map[*types.Type]llvm.Type),
		descriptorTypes:  make(map[*obj.LSym]llvm.Type),
		namedRuntimeType: make(map[*types.Type]bool),
	}
}

func (l *llvmDataLowerer) dataType(s *obj.LSym) llvm.Type {
	if t := llvmDescriptorGoType(s); t != nil {
		return l.descriptorType(s, t)
	}
	if s.ItabInfo() != nil {
		return l.itabType(s)
	}
	return llvmDataType(s)
}

func (l *llvmDataLowerer) dataInitializer(s *obj.LSym, globals map[*obj.LSym]llvm.Value) llvm.Value {
	if t := llvmDescriptorGoType(s); t != nil {
		return l.descriptorInitializer(s, t, globals)
	}
	if s.ItabInfo() != nil {
		return l.itabInitializer(s, globals)
	}
	return llvmDataInitializer(s, globals, l.data)
}

func llvmDescriptorGoType(s *obj.LSym) *types.Type {
	ti := s.TypeInfo()
	if ti == nil {
		return nil
	}
	t, _ := ti.Type.(*types.Type)
	return t
}

type llvmDescriptorPart struct {
	off        int64
	typ        *types.Type
	arrayElem  *types.Type
	arrayCount int64
}

func (p llvmDescriptorPart) size() int64 {
	if p.typ != nil {
		return p.typ.Size()
	}
	return p.arrayElem.Size() * p.arrayCount
}

func (l *llvmDataLowerer) partType(p llvmDescriptorPart) llvm.Type {
	if p.typ != nil {
		return l.goType(p.typ)
	}
	return llvm.ArrayType(l.goType(p.arrayElem), int(p.arrayCount))
}

func (l *llvmDataLowerer) partValue(s *obj.LSym, p llvmDescriptorPart, globals map[*obj.LSym]llvm.Value) llvm.Value {
	if p.typ != nil {
		return l.goValue(s, p.typ, p.off, globals)
	}
	values := make([]llvm.Value, p.arrayCount)
	for i := range values {
		values[i] = l.goValue(s, p.arrayElem, p.off+int64(i)*p.arrayElem.Size(), globals)
	}
	return llvm.ConstArray(l.goType(p.arrayElem), values)
}

func llvmItabParts(s *obj.LSym) []llvmDescriptorPart {
	size := llvmDataSize(s)
	fixed := rttype.ITab.Size()
	if size < fixed || (size-fixed)%int64(types.PtrSize) != 0 {
		base.Fatalf("invalid runtime itab size %d for %s", size, s.Name)
	}
	parts := []llvmDescriptorPart{{off: 0, typ: rttype.ITab}}
	if tail := (size - fixed) / int64(types.PtrSize); tail != 0 {
		parts = append(parts, llvmDescriptorPart{
			off:        fixed,
			arrayElem:  types.Types[types.TUNSAFEPTR],
			arrayCount: tail,
		})
	}
	return parts
}

func (l *llvmDataLowerer) itabType(s *obj.LSym) llvm.Type {
	if result, ok := l.descriptorTypes[s]; ok {
		return result
	}
	fields := l.descriptorFields(s, llvmItabParts(s), nil)
	fieldTypes := make([]llvm.Type, len(fields))
	for i, field := range fields {
		fieldTypes[i] = field.Type()
	}
	result := GlobalCtxt.StructType(fieldTypes, true)
	l.descriptorTypes[s] = result
	return result
}

func (l *llvmDataLowerer) itabInitializer(s *obj.LSym, globals map[*obj.LSym]llvm.Value) llvm.Value {
	_ = l.itabType(s)
	return GlobalCtxt.ConstStruct(l.descriptorFields(s, llvmItabParts(s), globals), true)
}

// descriptorParts mirrors reflectdata.writeType. It uses the runtime ABI
// structures from rttype for the descriptor prefix and recognizes every
// type-specific variable array. The finalized LSym size determines whether an
// UncommonType and Method array are present. That distinction cannot be made
// from AllMethods: interface methods are IMethod entries, not uncommon methods.
//
// Type descriptors are fully modeled. If reflectdata's layout changes, reject
// the descriptor instead of silently treating a runtime field as opaque bytes.
func descriptorParts(t *types.Type, size int64) []llvmDescriptorPart {
	var rt *types.Type
	var variableElem *types.Type
	var variableCount int64
	switch t.Kind() {
	default:
		rt = rttype.Type
	case types.TARRAY:
		rt = rttype.ArrayType
	case types.TSLICE:
		rt = rttype.SliceType
	case types.TCHAN:
		rt = rttype.ChanType
	case types.TFUNC:
		rt = rttype.FuncType
		variableElem = types.Types[types.TUNSAFEPTR]
		variableCount = int64(t.NumRecvs() + t.NumParams() + t.NumResults())
	case types.TINTER:
		rt = rttype.InterfaceType
		variableElem = rttype.IMethod
		variableCount = int64(len(t.AllMethods()))
	case types.TMAP:
		rt = rttype.MapType
	case types.TPTR:
		rt = rttype.PtrType
	case types.TSTRUCT:
		rt = rttype.StructType
		variableElem = rttype.StructField
		variableCount = int64(t.NumFields())
	}

	parts := []llvmDescriptorPart{{off: 0, typ: rt}}
	off := rt.Size()
	variableSize := int64(0)
	if variableElem != nil {
		variableSize = variableElem.Size() * variableCount
	}
	trailing := size - off - variableSize
	if trailing < 0 {
		base.Fatalf("runtime descriptor %s is too small: have %d, fixed layout requires %d", t, size, off+variableSize)
	}

	// A named type always has UncommonType. For an unnamed type, any bytes
	// between its fixed/variable data and the end are the uncommon header plus
	// concrete method entries.
	if t.Sym() != nil || trailing != 0 {
		if trailing < rttype.UncommonType.Size() {
			base.Fatalf("invalid uncommon runtime descriptor size %d for %s", trailing, t)
		}
		parts = append(parts, llvmDescriptorPart{off: off, typ: rttype.UncommonType})
		off += rttype.UncommonType.Size()
		trailing -= rttype.UncommonType.Size()
	}
	if variableElem != nil {
		parts = append(parts, llvmDescriptorPart{
			off:        off,
			arrayElem:  variableElem,
			arrayCount: variableCount,
		})
		off += variableSize
	}
	if trailing%rttype.Method.Size() != 0 {
		base.Fatalf("invalid runtime method table size %d for %s", trailing, t)
	}
	if n := trailing / rttype.Method.Size(); n != 0 {
		parts = append(parts, llvmDescriptorPart{
			off:        off,
			arrayElem:  rttype.Method,
			arrayCount: n,
		})
		off += rttype.Method.Size() * n
	}
	if off != size {
		base.Fatalf("runtime descriptor layout for %s has size %d, want %d", t, off, size)
	}
	return parts
}

func (l *llvmDataLowerer) descriptorType(s *obj.LSym, t *types.Type) llvm.Type {
	if result, ok := l.descriptorTypes[s]; ok {
		return result
	}
	parts := descriptorParts(t, llvmDataSize(s))
	fields := l.descriptorFields(s, parts, nil)
	types := make([]llvm.Type, len(fields))
	for i, f := range fields {
		types[i] = f.Type()
	}
	result := GlobalCtxt.StructType(types, true)
	l.descriptorTypes[s] = result
	return result
}

func (l *llvmDataLowerer) descriptorInitializer(s *obj.LSym, t *types.Type, globals map[*obj.LSym]llvm.Value) llvm.Value {
	_ = l.descriptorType(s, t)
	return GlobalCtxt.ConstStruct(l.descriptorFields(s, descriptorParts(t, llvmDataSize(s)), globals), true)
}

func (l *llvmDataLowerer) descriptorFields(s *obj.LSym, parts []llvmDescriptorPart, globals map[*obj.LSym]llvm.Value) []llvm.Value {
	fields := make([]llvm.Value, 0, len(parts)*2+1)
	pos := int64(0)
	for _, part := range parts {
		if part.off < pos || part.off+part.size() > llvmDataSize(s) {
			base.Fatalf("invalid semantic descriptor layout for %s", s.Name)
		}
		fields = append(fields, llvmDataRangeFields(s, pos, part.off, globals, l.data)...)
		fields = append(fields, l.partValue(s, part, globals))
		pos = part.off + part.size()
	}
	fields = append(fields, llvmDataRangeFields(s, pos, llvmDataSize(s), globals, l.data)...)
	if len(fields) == 0 {
		fields = append(fields, llvmDataBytes(nil))
	}
	return fields
}

func (l *llvmDataLowerer) goType(t *types.Type) llvm.Type {
	if result, ok := l.runtimeTypes[t]; ok {
		return result
	}

	switch t.Kind() {
	case types.TBOOL, types.TINT8, types.TUINT8:
		return GlobalCtxt.Int8Type()
	case types.TINT16, types.TUINT16:
		return GlobalCtxt.Int16Type()
	case types.TINT32, types.TUINT32:
		return GlobalCtxt.Int32Type()
	case types.TINT64, types.TUINT64:
		return GlobalCtxt.Int64Type()
	case types.TINT, types.TUINT, types.TUINTPTR:
		return GlobalCtxt.IntType(int(t.Size() * 8))
	case types.TPTR, types.TUNSAFEPTR:
		return GlobalCtxt.PointerType(0)
	case types.TARRAY:
		return llvm.ArrayType(l.goType(t.Elem()), int(t.NumElem()))
	case types.TSLICE:
		return GlobalCtxt.StructType([]llvm.Type{GlobalCtxt.PointerType(0), l.goType(types.Types[types.TINT]), l.goType(types.Types[types.TINT])}, true)
	case types.TSTRING:
		return GlobalCtxt.StructType([]llvm.Type{GlobalCtxt.PointerType(0), l.goType(types.Types[types.TINT])}, true)
	case types.TSTRUCT:
		return l.goStructType(t)
	default:
		base.Fatalf("unsupported runtime descriptor field type %s", t)
		return llvm.Type{}
	}
}

func (l *llvmDataLowerer) goStructType(t *types.Type) llvm.Type {
	if result, ok := l.runtimeTypes[t]; ok {
		return result
	}
	name := llvmRuntimeTypeName(t)
	if name == "" {
		fields := l.goStructElementTypes(t)
		return GlobalCtxt.StructType(fields, true)
	}
	result := CurrentModule.GetTypeByName(name)
	if result.IsNil() {
		result = GlobalCtxt.StructCreateNamed(name)
	}
	l.runtimeTypes[t] = result
	l.namedRuntimeType[t] = true
	if result.StructElementTypesCount() == 0 {
		result.StructSetBody(l.goStructElementTypes(t), true)
	}
	return result
}

func (l *llvmDataLowerer) goStructElementTypes(t *types.Type) []llvm.Type {
	fields := make([]llvm.Type, 0, len(t.Fields())*2+1)
	off := int64(0)
	for _, f := range t.Fields() {
		if f.Offset < off {
			base.Fatalf("overlapping runtime descriptor field %s", f.Sym.Name)
		}
		if f.Offset > off {
			fields = append(fields, llvm.ArrayType(GlobalCtxt.Int8Type(), int(f.Offset-off)))
		}
		if llvmItabFunField(t, f) {
			fields = append(fields, llvm.ArrayType(GlobalCtxt.PointerType(0), int(f.Type.NumElem())))
		} else {
			fields = append(fields, l.goType(f.Type))
		}
		off = f.Offset + f.Type.Size()
	}
	if off < t.Size() {
		fields = append(fields, llvm.ArrayType(GlobalCtxt.Int8Type(), int(t.Size()-off)))
	}
	return fields
}

func (l *llvmDataLowerer) goValue(s *obj.LSym, t *types.Type, off int64, globals map[*obj.LSym]llvm.Value) llvm.Value {
	// Composite values can start with a pointer relocation (for example the
	// data pointer of a slice). Recurse into those values before considering a
	// relocation at their base offset.
	if t.Kind() != types.TSTRUCT && t.Kind() != types.TARRAY && t.Kind() != types.TSLICE && t.Kind() != types.TSTRING {
		if r, ok := llvmDataRelocAt(s, off); ok {
			var value llvm.Value
			if globals == nil {
				value = llvmDataRelocZero(r)
			} else {
				value = llvmDataRelocValue(s, r, globals, l.data)
			}
			return llvmCoerceDataReloc(value, l.goType(t), s, off)
		}
	}

	switch t.Kind() {
	case types.TSTRUCT:
		values := make([]llvm.Value, 0, len(t.Fields())*2+1)
		pos := off
		for _, f := range t.Fields() {
			values = append(values, llvmDataRangeFields(s, pos, off+f.Offset, globals, l.data)...)
			if llvmItabFunField(t, f) {
				fun := make([]llvm.Value, f.Type.NumElem())
				for i := range fun {
					fun[i] = l.goValue(s, types.Types[types.TUNSAFEPTR], off+f.Offset+int64(i*types.PtrSize), globals)
				}
				values = append(values, llvm.ConstArray(GlobalCtxt.PointerType(0), fun))
			} else {
				values = append(values, l.goValue(s, f.Type, off+f.Offset, globals))
			}
			pos = off + f.Offset + f.Type.Size()
		}
		values = append(values, llvmDataRangeFields(s, pos, off+t.Size(), globals, l.data)...)
		typ := l.goType(t)
		if l.namedRuntimeType[t] {
			llvmCheckStructInitializer(typ, values, s.Name, off)
			return llvm.ConstNamedStruct(typ, values)
		}
		return GlobalCtxt.ConstStruct(values, true)
	case types.TARRAY:
		values := make([]llvm.Value, t.NumElem())
		for i := range values {
			values[i] = l.goValue(s, t.Elem(), off+int64(i)*t.Elem().Size(), globals)
		}
		return llvm.ConstArray(l.goType(t.Elem()), values)
	case types.TSLICE:
		ptr := l.goValue(s, types.Types[types.TUNSAFEPTR], off, globals)
		len := l.goValue(s, types.Types[types.TINT], off+int64(types.PtrSize), globals)
		cap := l.goValue(s, types.Types[types.TINT], off+2*int64(types.PtrSize), globals)
		return GlobalCtxt.ConstStruct([]llvm.Value{ptr, len, cap}, true)
	case types.TSTRING:
		ptr := l.goValue(s, types.Types[types.TUNSAFEPTR], off, globals)
		len := l.goValue(s, types.Types[types.TINT], off+int64(types.PtrSize), globals)
		return GlobalCtxt.ConstStruct([]llvm.Value{ptr, len}, true)
	case types.TPTR, types.TUNSAFEPTR:
		if !llvmDataAllZero(s, off, t.Size()) {
			base.Fatalf("non-relocated pointer data in Go type descriptor %s at %d", s.Name, off)
		}
		return llvm.ConstPointerNull(GlobalCtxt.PointerType(0))
	case types.TBOOL, types.TINT8, types.TUINT8, types.TINT16, types.TUINT16,
		types.TINT32, types.TUINT32, types.TINT64, types.TUINT64, types.TINT,
		types.TUINT, types.TUINTPTR:
		return llvm.ConstInt(l.goType(t), llvmDataUint(s, off, t.Size()), false)
	default:
		base.Fatalf("unsupported runtime descriptor constant type %s", t)
		return llvm.Value{}
	}
}

func llvmItabFunField(t *types.Type, f *types.Field) bool {
	return t == rttype.ITab && f.Sym != nil && f.Sym.Name == "Fun"
}

func llvmCoerceDataReloc(value llvm.Value, want llvm.Type, s *obj.LSym, off int64) llvm.Value {
	if value.Type() == want {
		return value
	}
	if value.Type().TypeKind() == llvm.PointerTypeKind && want.TypeKind() == llvm.IntegerTypeKind {
		return llvm.ConstPtrToInt(value, want)
	}
	base.Fatalf("relocation in %s at %d has LLVM type kind %d, want %d", s.Name, off, value.Type().TypeKind(), want.TypeKind())
	return llvm.Value{}
}

func llvmCheckStructInitializer(typ llvm.Type, values []llvm.Value, symbol string, off int64) {
	expected := typ.StructElementTypes()
	if len(expected) != len(values) {
		base.Fatalf("LLVM runtime layout mismatch for %s at %d: have %d fields, want %d", symbol, off, len(values), len(expected))
	}
	for i, want := range expected {
		if got := values[i].Type(); got != want {
			base.Fatalf("LLVM runtime layout mismatch for %s at %d field %d: got type kind %d, want %d", symbol, off, i, got.TypeKind(), want.TypeKind())
		}
	}
}

func llvmDataRangeFields(s *obj.LSym, start, end int64, globals map[*obj.LSym]llvm.Value, data map[*obj.LSym]bool) []llvm.Value {
	if start == end {
		return nil
	}
	relocs := make([]obj.Reloc, 0)
	for _, r := range s.R {
		if r.Type == objabi.R_KEEP {
			continue
		}
		if int64(r.Off) >= start && int64(r.Off) < end {
			relocs = append(relocs, r)
		}
	}
	sort.Slice(relocs, func(i, j int) bool { return relocs[i].Off < relocs[j].Off })
	fields := make([]llvm.Value, 0, len(relocs)*2+1)
	pos := start
	for _, r := range relocs {
		off, rEnd := int64(r.Off), int64(r.Off)+int64(r.Siz)
		if off < pos || rEnd > end {
			base.Fatalf("relocation crosses semantic descriptor boundary in %s", s.Name)
		}
		if off > pos {
			fields = append(fields, llvmDataBytes(llvmDataBytesAt(s, pos, off-pos)))
		}
		if globals == nil {
			fields = append(fields, llvmDataRelocZero(r))
		} else {
			fields = append(fields, llvmDataRelocValue(s, r, globals, data))
		}
		pos = rEnd
	}
	if pos < end {
		fields = append(fields, llvmDataBytes(llvmDataBytesAt(s, pos, end-pos)))
	}
	return fields
}

func llvmDataRelocAt(s *obj.LSym, off int64) (obj.Reloc, bool) {
	for _, r := range s.R {
		if r.Type != objabi.R_KEEP && int64(r.Off) == off {
			return r, true
		}
	}
	return obj.Reloc{}, false
}

func llvmDataSize(s *obj.LSym) int64 {
	if s.Size < int64(len(s.P)) {
		return int64(len(s.P))
	}
	return s.Size
}

func llvmDataBytesAt(s *obj.LSym, off, size int64) []byte {
	if off < 0 || size < 0 || off+size > llvmDataSize(s) {
		base.Fatalf("invalid byte range [%d,%d) in %s", off, off+size, s.Name)
	}
	data := make([]byte, size)
	if off < int64(len(s.P)) {
		copy(data, s.P[off:min(off+size, int64(len(s.P)))])
	}
	return data
}

func llvmDataAllZero(s *obj.LSym, off, size int64) bool {
	for _, b := range llvmDataBytesAt(s, off, size) {
		if b != 0 {
			return false
		}
	}
	return true
}

func llvmDataUint(s *obj.LSym, off, size int64) uint64 {
	b := llvmDataBytesAt(s, off, size)
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		return uint64(base.Ctxt.Arch.ByteOrder.Uint16(b))
	case 4:
		return uint64(base.Ctxt.Arch.ByteOrder.Uint32(b))
	case 8:
		return base.Ctxt.Arch.ByteOrder.Uint64(b)
	default:
		base.Fatalf("unsupported scalar size %d in Go type descriptor", size)
		return 0
	}
}

func llvmRuntimeTypeName(t *types.Type) string {
	// Embedded fields are independently translated by rttype.FromReflect, so
	// pointer identity alone does not recognize the Type field nested inside
	// ArrayType, StructType, and friends. Match its compiler layout as well.
	if t == rttype.Type || llvmSameStructLayout(t, rttype.Type) {
		return "go.runtime.Type"
	}
	switch t {
	case rttype.ArrayType:
		return "go.runtime.ArrayType"
	case rttype.ChanType:
		return "go.runtime.ChanType"
	case rttype.FuncType:
		return "go.runtime.FuncType"
	case rttype.InterfaceType:
		return "go.runtime.InterfaceType"
	case rttype.MapType:
		return "go.runtime.MapType"
	case rttype.PtrType:
		return "go.runtime.PtrType"
	case rttype.SliceType:
		return "go.runtime.SliceType"
	case rttype.StructType:
		return "go.runtime.StructType"
	case rttype.IMethod:
		return "go.runtime.Imethod"
	case rttype.Method:
		return "go.runtime.Method"
	case rttype.StructField:
		return "go.runtime.StructField"
	case rttype.UncommonType:
		return "go.runtime.UncommonType"
	case rttype.ITab:
		return "go.runtime.ITab"
	default:
		return ""
	}
}

func llvmSameStructLayout(a, b *types.Type) bool {
	if a.Kind() != types.TSTRUCT || b.Kind() != types.TSTRUCT || a.Size() != b.Size() {
		return false
	}
	af, bf := a.Fields(), b.Fields()
	if len(af) != len(bf) {
		return false
	}
	for i := range af {
		if af[i].Sym.Name != bf[i].Sym.Name || af[i].Offset != bf[i].Offset || af[i].Type.Kind() != bf[i].Type.Kind() || af[i].Type.Size() != bf[i].Type.Size() {
			return false
		}
	}
	return true
}
