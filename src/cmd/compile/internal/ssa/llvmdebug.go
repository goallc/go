// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"internal/buildcfg"
	"path/filepath"
	"sort"

	"github.com/goallc/go-llvm"
)

// This deliberately contains only the source locations LLVM needs to build
// Go pcfile, pcline, and pcinline after final machine layout. Source types,
// variables, and their locations belong to the separate target-DWARF path.
var (
	llvmDIBuilder        *llvm.DIBuilder
	llvmDICompileUnit    llvm.Metadata
	llvmDIFiles          map[string]llvm.Metadata
	llvmDISubprograms    map[*obj.LSym]llvm.Metadata
	llvmDISubprogramVals map[*obj.LSym]llvm.Value
	llvmDIDebugFinalized bool
)

func initLLVMDebugInfo(pkg *types.Pkg) {
	llvmDIBuilder = llvm.NewDIBuilder(CurrentModule)
	llvmDIFiles = make(map[string]llvm.Metadata)
	llvmDISubprograms = make(map[*obj.LSym]llvm.Metadata)
	llvmDISubprogramVals = make(map[*obj.LSym]llvm.Value)
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
		EmissionKind: llvm.DwarfEmissionLineTablesOnly,
	})
	CurrentModule.AddNamedMetadataOperand("goobj.debug.config",
		GlobalCtxt.MDNode([]llvm.Metadata{GlobalCtxt.MDString("pcln-v1")}))

	flag := func(name string, value uint64) {
		CurrentModule.AddNamedMetadataOperand("llvm.module.flags", GlobalCtxt.MDNode([]llvm.Metadata{
			llvm.ConstInt(GlobalCtxt.Int32Type(), 2, false).ConstantAsMetadata(),
			GlobalCtxt.MDString(name),
			llvm.ConstInt(GlobalCtxt.Int32Type(), value, false).ConstantAsMetadata(),
		}))
	}
	flag("Dwarf Version", 4)
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

func llvmDebugSubprogram(sym *obj.LSym, pos src.Pos, _ *Func) llvm.Metadata {
	if sym == nil {
		base.Fatalf("invalid LLVM debug subprogram")
	}
	if sp, ok := llvmDISubprograms[sym]; ok {
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
	sp := llvmDIBuilder.CreateFunction(llvmDICompileUnit, llvm.DIFunction{
		Name:         sym.Name,
		LinkageName:  llvmFunctionStorageName(sym.Name, llvmCallConv(sym.ABI())),
		File:         file,
		Line:         line,
		Type:         llvmDIBuilder.CreateSubroutineType(llvm.DISubroutineType{File: file}),
		IsDefinition: true,
		ScopeLine:    line,
		Optimized:    base.Flag.N == 0,
	})
	llvmDISubprograms[sym] = sp
	return sp
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

	// Generic LLVM optimization may combine instructions from different Go
	// inline frames and keep only one of their DILocations. Record one complete
	// frontend location for every inline node independently of the instruction
	// stream. The final machine pass uses this only when an inline edge has
	// otherwise disappeared, so optimization remains unconstrained while Go's
	// pcinline tree still has a real final-layout PC for every source edge.
	if len(chain) != 0 && !lfc.RequiredInlinePos[pos.Base().InliningIndex()] {
		lfc.RequiredInlinePos[pos.Base().InliningIndex()] = true
		CurrentModule.AddNamedMetadataOperand(goObjDebugInlineRequiredMD,
			GlobalCtxt.MDNode([]llvm.Metadata{
				lfc.LF.ConstantAsMetadata(),
				location,
			}))
	}
}
