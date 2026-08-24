// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"fmt"
	"internal/buildcfg"
	"path/filepath"
	"sort"

	"github.com/goallc/go-llvm"
)

// This is the source-semantic graph shared by both Go PCLN and DWARF. LLVM
// resolves its locations and inline stacks against final machine layout; the
// GoObj writer then encodes pcfile/pcline/pcinline and the Go linker's
// per-function DWARF carriers from the same facts.
var (
	llvmDIBuilder        *llvm.DIBuilder
	llvmDICompileUnit    llvm.Metadata
	llvmDIFiles          map[string]llvm.Metadata
	llvmDISubprograms    map[*obj.LSym]llvm.Metadata
	llvmDISubprogramVals map[*obj.LSym]llvm.Value
	llvmDITypes          map[*types.Type]llvm.Metadata
	llvmDITypeBuilding   map[*types.Type]bool
	llvmDIForwardTypes   map[*types.Type]llvm.Metadata
	llvmDIDebugFinalized bool
)

func initLLVMDebugInfo(pkg *types.Pkg) {
	llvmDIBuilder = llvm.NewDIBuilder(CurrentModule)
	llvmDIFiles = make(map[string]llvm.Metadata)
	llvmDISubprograms = make(map[*obj.LSym]llvm.Metadata)
	llvmDISubprogramVals = make(map[*obj.LSym]llvm.Value)
	llvmDITypes = make(map[*types.Type]llvm.Metadata)
	llvmDITypeBuilding = make(map[*types.Type]bool)
	llvmDIForwardTypes = make(map[*types.Type]llvm.Metadata)
	llvmDIDebugFinalized = false

	name := pkg.Path
	if name == "" {
		name = "go-package"
	}
	llvmDICompileUnit = llvmDIBuilder.CreateCompileUnit(llvm.DICompileUnit{
		Language:     llvm.DW_LANG_Go,
		File:         name,
		Producer:     "Go compiler " + buildcfg.Version,
		Optimized:    base.Flag.N == 0,
		EmissionKind: llvm.DwarfEmissionFull,
	})
	dwarfVersion := "dwarf4"
	if buildcfg.Experiment.Dwarf5 {
		dwarfVersion = "dwarf5"
	}
	CurrentModule.AddNamedMetadataOperand("goobj.debug.config",
		GlobalCtxt.MDNode([]llvm.Metadata{
			GlobalCtxt.MDString("pcln-v1"),
			GlobalCtxt.MDString("dwarf-v1"),
			GlobalCtxt.MDString(dwarfVersion),
		}))

	flag := func(name string, value uint64) {
		CurrentModule.AddNamedMetadataOperand("llvm.module.flags", GlobalCtxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(GlobalCtxt.Int32Type(), 2, false).ConstantAsMetadata(),
			GlobalCtxt.MDString(name),
			llvm.ConstInt(GlobalCtxt.Int32Type(), value, false).ConstantAsMetadata(),
		}))
	}
	version := uint64(4)
	if buildcfg.Experiment.Dwarf5 {
		version = 5
	}
	flag("Dwarf Version", version)
	flag("Debug Info Version", 3)
}

func finalizeLLVMDebugInfo() {
	if llvmDIBuilder == nil || llvmDIDebugFinalized {
		return
	}
	syms := make([]*obj.LSym, 0, len(llvmDISubprograms))
	for sym := range llvmDISubprograms {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		if syms[i].Name != syms[j].Name {
			return syms[i].Name < syms[j].Name
		}
		return syms[i].ABI() < syms[j].ABI()
	})
	abstractValues := make(map[string]llvm.Value)
	for _, sym := range syms {
		value, ok := llvmDISubprogramVals[sym]
		if !ok {
			value = llvmDebugSubprogramValue(sym, llvmDISubprograms[sym], abstractValues)
		}
		preserveGoObjMetadataValues(value)
		CurrentModule.AddNamedMetadataOperand("goobj.debug.funcs", GlobalCtxt.MDNode([]llvm.Metadata{
			llvmDISubprograms[sym],
			value.ConstantAsMetadata(),
		}))
	}
	emitGoObjCompilerUsed()
	llvmDIBuilder.Finalize()
	llvmDIBuilder.Destroy()
	llvmDIBuilder = nil
	llvmDIDebugFinalized = true
}

func llvmDebugSubprogramValue(sym *obj.LSym, sp llvm.Metadata, abstractValues map[string]llvm.Value) llvm.Value {
	storageName := llvmFunctionStorageName(sym.Name, llvmCallConv(sym.ABI()))
	value := CurrentModule.NamedFunction(storageName)
	if !value.IsNil() && !value.IsDeclaration() {
		return value
	}

	if !sym.ContentAddressable() {
		return llvmGoDataRef(sym)
	}

	// Imported inline closures can retain the native compiler's call-stack
	// hash even when their body was emitted under the canonical, unhashed name
	// in this object. Reuse that equivalent definition when it exists.
	canonicalName := obj.TrimInlineHash(sym.Name)
	abstractKey := llvmFunctionStorageName(canonicalName, llvmCallConv(sym.ABI()))
	if abstract, ok := abstractValues[abstractKey]; ok {
		return abstract
	}
	if canonicalName != sym.Name {
		canonical := CurrentModule.NamedFunction(
			llvmFunctionStorageName(canonicalName, llvmCallConv(sym.ABI())))
		if !canonical.IsNil() && !canonical.IsDeclaration() {
			return canonical
		}
	}

	// Native GoObj emits a zero-sized STEXT symbol with FuncInfo when a
	// content-addressable closure was completely inlined and has no remaining
	// body. LLVM needs an emitted function symbol for the same linker contract.
	// The unreachable body is never called, but keeps the logical callee
	// and its FuncInfo available to the final pcinline tree.
	if !value.IsNil() {
		if !value.FirstUse().IsNil() {
			return llvmGoDataRef(sym)
		}
		value.EraseFromParentAsFunction()
	}
	value = llvm.AddFunction(CurrentModule, storageName,
		llvm.FunctionType(GlobalCtxt.VoidType(), nil, false))
	value.SetFunctionCallConv(llvmCallConv(sym.ABI()))
	value.SetLinkage(llvm.WeakAnyLinkage)
	value.SetSubprogram(sp)
	b := GlobalCtxt.NewBuilder()
	b.SetInsertPointAtEnd(GlobalCtxt.AddBasicBlock(value, "goobj.abstract"))
	b.CreateUnreachable()
	b.Dispose()
	abstractValues[abstractKey] = value
	return value
}

func llvmSourcePos(xpos src.XPos) src.Pos {
	if xpos == src.NoXPos {
		return src.NoPos
	}
	pos := base.Ctxt.InnermostPos(xpos)
	if !pos.IsKnown() {
		return src.NoPos
	}
	return pos
}

func llvmSourcePath(pos src.Pos) string {
	if !pos.IsKnown() {
		return "llvm-ir"
	}
	if name := pos.AbsFilename(); name != "" {
		return name
	}
	if name := pos.RelFilename(); name != "" {
		return name
	}
	return "llvm-ir"
}

func llvmDIFile(pos src.Pos) llvm.Metadata {
	path := llvmSourcePath(pos)
	if file, ok := llvmDIFiles[path]; ok {
		return file
	}
	dir, name := filepath.Split(path)
	if dir != "" {
		dir = filepath.Clean(dir)
	}
	file := llvmDIBuilder.CreateFile(name, dir)
	llvmDIFiles[path] = file
	return file
}

func llvmDIScopeForPos(scope llvm.Metadata, pos src.Pos, discriminator int) llvm.Metadata {
	return llvmDIBuilder.CreateLexicalBlockFile(scope, llvmDIFile(pos), discriminator)
}

func llvmDIType(t *types.Type, file llvm.Metadata) llvm.Metadata {
	if t == nil {
		return llvm.Metadata{}
	}
	if typ, ok := llvmDITypes[t]; ok {
		return typ
	}
	types.CalcSize(t)
	// LLVM compilation bypasses the native dwarfgen walk. Request the same Go
	// runtime type symbol so the linker can resolve go:info.<type> references
	// from the emitted per-function carriers.
	_ = reflectdata.TypeLinksym(t)
	name := types.TypeSymName(t)
	size := uint64(t.Size()) * 8
	align := uint32(t.Alignment()) * 8

	if t.IsBoolean() || t.IsInteger() || t.IsFloat() || t.IsComplex() {
		encoding := llvm.DW_ATE_unsigned
		switch {
		case t.IsBoolean():
			encoding = llvm.DW_ATE_boolean
		case t.IsSigned():
			encoding = llvm.DW_ATE_signed
		case t.IsFloat():
			encoding = llvm.DW_ATE_float
		case t.IsComplex():
			encoding = llvm.DW_ATE_complex_float
		}
		typ := llvmDIBuilder.CreateBasicType(llvm.DIBasicType{
			Name: name, SizeInBits: size, Encoding: encoding,
		})
		llvmDITypes[t] = typ
		return typ
	}

	switch t.Kind() {
	case types.TPTR:
		// Do not force the pointee's width here. Backend function compilation
		// runs in parallel with CalcSize disabled, and compiler-generated pointer
		// types may legally refer to a not-yet-sized element. A forward struct is
		// enough for LLVM's source graph; the Go linker resolves the exact rich
		// type through the go:info.<type> carrier relocation.
		var pointee llvm.Metadata
		if elem := t.Elem(); elem.Kind() == types.TSTRUCT {
			if typ, ok := llvmDIForwardTypes[elem]; ok {
				pointee = typ
			} else {
				name := types.TypeSymName(elem)
				pointee = llvmDIBuilder.CreateStructType(llvmDICompileUnit,
					llvm.DIStructType{
						Name: name, File: file, Flags: llvm.FlagFwdDecl,
						UniqueID: name + ".$forward",
					})
				llvmDIForwardTypes[elem] = pointee
			}
		}
		typ := llvmDIBuilder.CreatePointerType(llvm.DIPointerType{
			Pointee: pointee, SizeInBits: size,
			AlignInBits: align, Name: name,
		})
		llvmDITypes[t] = typ
		return typ
	case types.TARRAY:
		typ := llvmDIBuilder.CreateArrayType(llvm.DIArrayType{
			SizeInBits: size, AlignInBits: align,
			ElementType: llvmDIType(t.Elem(), file),
			Subscripts:  []llvm.DISubrange{{Lo: 0, Count: t.NumElem()}},
		})
		llvmDITypes[t] = typ
		return typ
	case types.TSTRUCT:
		// Break only real recursive edges. Ordinary structs keep their complete
		// member graph, while a recursive pointee receives a stable forward DIE.
		if llvmDITypeBuilding[t] {
			if typ, ok := llvmDIForwardTypes[t]; ok {
				return typ
			}
			typ := llvmDIBuilder.CreateStructType(llvmDICompileUnit,
				llvm.DIStructType{
					Name: name, File: file, SizeInBits: size,
					AlignInBits: align, Flags: llvm.FlagFwdDecl,
					UniqueID: name + ".$forward",
				})
			llvmDIForwardTypes[t] = typ
			return typ
		}
		llvmDITypeBuilding[t] = true
		members := make([]llvm.Metadata, 0, t.NumFields())
		for i, field := range t.Fields() {
			fieldName := fmt.Sprintf("field%d", i)
			if field.Sym != nil {
				fieldName = field.Sym.Name
			}
			members = append(members, llvmDIBuilder.CreateMemberType(
				llvmDICompileUnit, llvm.DIMemberType{
					Name: fieldName, File: file,
					SizeInBits:   uint64(field.Type.Size()) * 8,
					AlignInBits:  uint32(field.Type.Alignment()) * 8,
					OffsetInBits: uint64(field.Offset) * 8,
					Type:         llvmDIType(field.Type, file),
				}))
		}
		delete(llvmDITypeBuilding, t)
		typ := llvmDIBuilder.CreateStructType(llvmDICompileUnit,
			llvm.DIStructType{
				Name: name, File: file, SizeInBits: size,
				AlignInBits: align, Elements: members, UniqueID: name,
			})
		llvmDITypes[t] = typ
		return typ
	default:
		// Preserve the exact Go type identity and storage size for runtime
		// representations whose internal fields are not source-level structs.
		typ := llvmDIBuilder.CreateStructType(llvmDICompileUnit,
			llvm.DIStructType{
				Name: name, File: file, SizeInBits: size,
				AlignInBits: align, UniqueID: name,
			})
		llvmDITypes[t] = typ
		return typ
	}
}

func llvmDebugFunctionType(f *Func, file llvm.Metadata) llvm.Metadata {
	params := []llvm.Metadata{{}}
	out := f.OwnAux.ABIInfo().OutParams()
	switch len(out) {
	case 0:
	case 1:
		params[0] = llvmDIType(out[0].Type, file)
	default:
		// A source-level Go result tuple has no single ABI storage layout: any
		// element may be register allocated, in which case Offset is invalid.
		// Keep the function signature semantic and let the typed PPARAMOUT DIEs
		// below describe each result without inventing a stack layout.
		params[0] = llvmDIBuilder.CreateStructType(llvmDICompileUnit,
			llvm.DIStructType{
				Name: f.OwnAux.Fn.Name + ".$results", File: file,
				UniqueID: f.OwnAux.Fn.Name + ".$results",
			})
	}
	for _, param := range f.OwnAux.ABIInfo().InParams() {
		params = append(params, llvmDIType(param.Type, file))
	}
	return llvmDIBuilder.CreateSubroutineType(
		llvm.DISubroutineType{File: file, Parameters: params})
}

func llvmDebugSubprogram(sym *obj.LSym, pos src.Pos, f *Func) llvm.Metadata {
	if sym == nil {
		base.Fatalf("invalid LLVM debug subprogram")
	}
	if sp, ok := llvmDISubprograms[sym]; ok {
		if f != nil {
			sp.ReplaceSubprogramType(llvmDebugFunctionType(f, llvmDIFile(pos)))
		}
		return sp
	}
	file := llvmDIFile(pos)
	line := 0
	if pos.IsKnown() {
		line = int(pos.RelLine())
	}
	if info := sym.Func(); info != nil && info.StartLine > 0 {
		line = int(info.StartLine)
	}
	if line == 0 {
		line = 1
	}
	typ := llvmDIBuilder.CreateSubroutineType(llvm.DISubroutineType{File: file})
	if f != nil {
		typ = llvmDebugFunctionType(f, file)
	}
	sp := llvmDIBuilder.CreateFunction(llvmDICompileUnit, llvm.DIFunction{
		Name:         sym.Name,
		LinkageName:  llvmFunctionStorageName(sym.Name, llvmCallConv(sym.ABI())),
		File:         file,
		Line:         line,
		Type:         typ,
		IsDefinition: true,
		ScopeLine:    line,
		Optimized:    base.Flag.N == 0,
	})
	llvmDISubprograms[sym] = sp
	return sp
}

func (lfc *LLVMFuncContext) emitDebugVariables() {
	fn := lfc.F.Frontend().Func()
	if fn == nil {
		return
	}
	argNos := make(map[int]int)
	expr := llvmDIBuilder.CreateExpression(nil)
	for _, name := range fn.Dcl {
		if name == nil || name.Sym() == nil || name.Type() == nil ||
			name.Sym().Name == "_" || name.Type().IsUntyped() ||
			name.Type().Kind() == types.TSSA || ir.IsAutoTmp(name) ||
			!IsVarWantedForDebug(name) {
			continue
		}
		if name.Class != ir.PPARAM && name.Class != ir.PPARAMOUT &&
			name.Class != ir.PAUTO {
			continue
		}
		pos := llvmSourcePos(name.Pos())
		if !pos.IsKnown() {
			pos = llvmSourcePos(lfc.F.Entry.Pos)
		}
		line := 1
		if pos.IsKnown() && pos.RelLine() != 0 {
			line = int(pos.RelLine())
		}
		file := llvmDIFile(pos)
		diType := llvmDIType(name.Type(), file)
		scope := lfc.DISubprogram
		inlIndex := -1
		if name.InlFormal() || name.InlLocal() {
			fullPos := base.Ctxt.PosTable.Pos(name.Pos())
			// Compiler-generated declarations such as range-over-func yield
			// parameters can retain the inlined-variable flag while carrying an
			// autogenerated position with no inline index. Match native DWARF:
			// keep those variables in the physical function scope rather than
			// inventing an inline origin or rejecting otherwise valid code.
			if fullPos.Base() != nil && fullPos.Base().InliningIndex() >= 0 {
				inlIndex = fullPos.Base().InliningIndex()
				callee := base.Ctxt.InlTree.InlinedFunction(inlIndex)
				scope = llvmDebugSubprogram(callee, pos, nil)
			}
		}
		isReturn := name.Class == ir.PPARAMOUT
		isParam := name.Class == ir.PPARAM || isReturn || name.InlFormal()
		var diVar llvm.Metadata
		if isParam {
			argNos[inlIndex]++
			diVar = llvmDIBuilder.CreateParameterVariable(scope,
				llvm.DIParameterVariable{
					Name: name.Sym().Name, File: file, Line: line,
					Type: diType, AlwaysPreserve: true,
					ArgNo: argNos[inlIndex],
				})
		} else {
			diVar = llvmDIBuilder.CreateAutoVariable(scope,
				llvm.DIAutoVariable{
					Name: name.Sym().Name, File: file, Line: line,
					Type: diType, AlwaysPreserve: true,
				})
		}
		flags := uint64(0)
		if isReturn {
			flags = 1
		}
		CurrentModule.AddNamedMetadataOperand("goobj.debug.vars",
			GlobalCtxt.MDNode([]llvm.Metadata{
				diVar,
				GlobalCtxt.MDString(types.TypeSymName(name.Type())),
				llvm.ConstInt(GlobalCtxt.Int32Type(), flags, false).ConstantAsMetadata(),
			}))

		// Declare only a real canonical memory home. SSA-only, split,
		// heap-promoted, captured, and inlined variables remain explicitly
		// unavailable until a final-machine location can be proven exact.
		if slot, ok := lfc.Locals[llvmLocalKeyForName(name)]; ok && inlIndex < 0 {
			llvmDIBuilder.InsertDeclareAtEnd(slot.Value, diVar, expr,
				llvm.DebugLoc{Line: uint(line), Scope: lfc.DISubprogram},
				lfc.Prologue)
		}
	}

	if !lfc.ClosureContext.IsNil() {
		pos := llvmSourcePos(lfc.F.Entry.Pos)
		file := llvmDIFile(pos)
		argNos[-1]++
		goType := types.Types[types.TUNSAFEPTR]
		context := llvmDIBuilder.CreateParameterVariable(lfc.DISubprogram,
			llvm.DIParameterVariable{
				Name: ".closureptr", File: file, Line: int(pos.RelLine()),
				Type: llvmDIType(goType, file), AlwaysPreserve: true,
				Flags: llvm.FlagArtificial, ArgNo: argNos[-1],
			})
		CurrentModule.AddNamedMetadataOperand("goobj.debug.vars",
			GlobalCtxt.MDNode([]llvm.Metadata{
				context,
				GlobalCtxt.MDString(types.TypeSymName(goType)),
				llvm.ConstInt(GlobalCtxt.Int32Type(), 0, false).ConstantAsMetadata(),
			}))
	}
}

func (lfc *LLVMFuncContext) setDebugLocation(xpos src.XPos) {
	pos := llvmSourcePos(xpos)
	if !pos.IsKnown() || pos.RelLine() == 0 {
		lfc.b.ClearCurrentDebugLocation()
		return
	}

	scope := lfc.DISubprogram
	var inlinedAt llvm.Metadata
	inlIndex := pos.Base().InliningIndex()
	var chain []int
	for inlIndex >= 0 {
		chain = append(chain, inlIndex)
		inlIndex = base.Ctxt.InlTree.Parent(inlIndex)
	}
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}

	for i, index := range chain {
		callPos := llvmSourcePos(base.Ctxt.InlTree.CallPos(index))
		if !callPos.IsKnown() || callPos.RelLine() == 0 {
			base.Fatalf("inline callsite %d has no LLVM debug position", index)
		}
		callScope := llvmDIScopeForPos(scope, callPos, index+1)
		inlinedAt = GlobalCtxt.CreateDebugLocation(
			callPos.RelLine(), callPos.RelCol(), callScope, inlinedAt)

		calleePos := pos
		if i+1 < len(chain) {
			calleePos = llvmSourcePos(base.Ctxt.InlTree.CallPos(chain[i+1]))
		}
		callee := base.Ctxt.InlTree.InlinedFunction(index)
		scope = llvmDebugSubprogram(callee, calleePos, nil)
	}

	locationScope := llvmDIScopeForPos(scope, pos, 0)
	location := GlobalCtxt.CreateDebugLocation(
		pos.RelLine(), pos.RelCol(), locationScope, inlinedAt)
	lfc.b.SetCurrentDebugLocationMetadata(location)

	// LLVM may combine instructions from distinct frontend inline frames and
	// retain only one DILocation. Keep one representative location per frontend
	// inline node so the final machine pass can restore a node only when its
	// complete edge has otherwise disappeared. This metadata does not constrain
	// optimization or choose a machine-code insertion point.
	if len(chain) != 0 && !lfc.RequiredInlinePos[pos.Base().InliningIndex()] {
		lfc.RequiredInlinePos[pos.Base().InliningIndex()] = true
		CurrentModule.AddNamedMetadataOperand(goObjDebugInlineRequiredMD,
			GlobalCtxt.MDNode([]llvm.Metadata{
				lfc.LF.ConstantAsMetadata(),
				location,
			}))
	}
}
