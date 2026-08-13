// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"strings"
	"testing"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/goobj"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"

	"github.com/goallc/go-llvm"
)

type llvmTestTypeName struct {
	sym *types.Sym
}

func (n *llvmTestTypeName) Sym() *types.Sym { return n.sym }
func (*llvmTestTypeName) Pos() src.XPos     { return src.NoXPos }
func (*llvmTestTypeName) Type() *types.Type { return nil }

func TestLLVMABICarrierPreservesNamedAggregateIdentity(t *testing.T) {
	pkg := types.NewPkg("runtime", "runtime")
	namedSlice := types.NewNamed(&llvmTestTypeName{sym: pkg.Lookup("slice")})
	namedSlice.SetUnderlying(types.NewStruct([]*types.Field{
		types.NewField(src.NoXPos, pkg.Lookup("array"), types.Types[types.TUNSAFEPTR]),
		types.NewField(src.NoXPos, pkg.Lookup("len"), types.Types[types.TINT]),
		types.NewField(src.NoXPos, pkg.Lookup("cap"), types.Types[types.TINT]),
	}))
	types.CalcSize(namedSlice)
	builtinSlice := types.NewSlice(types.Types[types.TUINT8])
	types.CalcSize(builtinSlice)

	if getLLVMType(namedSlice) == getLLVMType(builtinSlice) {
		t.Fatal("semantic LLVM types unexpectedly lost named aggregate identity")
	}
	if got, want := getLLVMABIType(namedSlice), getLLVMType(namedSlice); got != want {
		t.Fatalf("named ABI carrier = %v, want semantic type %v", got, want)
	}
	if got, other := getLLVMABIType(namedSlice), getLLVMABIType(builtinSlice); got == other {
		t.Fatalf("named ABI carrier unexpectedly collapsed to builtin carrier %v", got)
	}
}

func TestLLVMABICarrierBridgesPromotedReceiverAtCaller(t *testing.T) {
	module := GlobalCtxt.NewModule("promoted_receiver_carrier")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	pointer := GlobalCtxt.PointerType(0)
	receiver := GlobalCtxt.StructCreateNamed("goallc.test.promoted.receiver")
	receiver.StructSetBody([]llvm.Type{pointer}, false)

	wrap := llvm.AddFunction(module, "wrap", llvm.FunctionType(receiver, []llvm.Type{pointer}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(wrap, "entry"))
	context := &LLVMFuncContext{b: builder}
	builder.CreateRet(context.reshapeLLVMValueToType(wrap.Param(0), receiver, "receiver"))

	unwrap := llvm.AddFunction(module, "unwrap", llvm.FunctionType(pointer, []llvm.Type{receiver}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(unwrap, "entry"))
	builder.CreateRet(context.reshapeLLVMValueToType(unwrap.Param(0), pointer, "receiver"))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected promoted receiver carrier bridge: %v\n%s", err, module.String())
	}
	ir := module.String()
	for _, want := range []string{
		"insertvalue %goallc.test.promoted.receiver undef, ptr %0, 0",
		"extractvalue %goallc.test.promoted.receiver %0, 0",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("promoted receiver bridge does not contain %q\n%s", want, ir)
		}
	}
}

func TestLLVMBuiltinDeclarationKeepsCallSiteSignatures(t *testing.T) {
	oldModule := CurrentModule
	module := GlobalCtxt.NewModule("builtin_call_signatures")
	CurrentModule = module
	t.Cleanup(func() {
		CurrentModule = oldModule
		module.Dispose()
	})

	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(builder.Dispose)
	caller := llvm.AddFunction(module, "caller", llvm.FunctionType(GlobalCtxt.VoidType(), nil, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(caller, "entry"))

	fields := []llvm.Type{GlobalCtxt.PointerType(0), GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type()}
	builtinSlice := llvm.StructType(fields, false)
	runtimeSlice := GlobalCtxt.StructCreateNamed("runtime.slice.call.signature")
	runtimeSlice.StructSetBody(fields, false)
	name, ok := goobj.BuiltinSymbolName("runtime.growslice", int(obj.ABIInternal))
	if !ok {
		t.Fatal("runtime.growslice is absent from the GoObj builtin table")
	}

	newSignature := func(result llvm.Type) llvmFuncSignature {
		return llvmFuncSignature{
			Type:                llvm.FunctionType(result, nil, false),
			ReturnType:          result,
			ResultCount:         1,
			ClosureContextIndex: -1,
		}
	}
	builtinSig := newSignature(builtinSlice)
	fn := getOrInsertLLVMFunction(name, builtinSig, goABIInternalCallConv)
	builtinCall := builder.CreateCall(builtinSig.Type, fn, nil, "builtin.call")
	builtinCall.SetInstructionCallConv(goABIInternalCallConv)

	runtimeSig := newSignature(runtimeSlice)
	fn = getOrInsertLLVMFunction(name, runtimeSig, goABIInternalCallConv)
	runtimeCall := builder.CreateCall(runtimeSig.Type, fn, nil, "runtime.call")
	runtimeCall.SetInstructionCallConv(goABIInternalCallConv)
	builder.CreateRetVoid()

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected builtin calls with distinct signatures: %v\n%s", err, module.String())
	}
}

func TestLLVMGoObjCompilerUsedOnlyKeepsExternalDataRoots(t *testing.T) {
	oldModule := CurrentModule
	oldLowerer := currentLLVMDataLowerer
	oldCompilerUsed := goObjCompilerUsed
	oldCompilerUsedNames := goObjCompilerUsedNames
	oldData := base.Ctxt.Data
	module := GlobalCtxt.NewModule("goobj_external_data_roots")
	CurrentModule = module
	currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
	t.Cleanup(func() {
		base.Ctxt.Data = oldData
		goObjCompilerUsedNames = oldCompilerUsedNames
		goObjCompilerUsed = oldCompilerUsed
		currentLLVMDataLowerer = oldLowerer
		CurrentModule = oldModule
		module.Dispose()
	})

	newLocalData := func(name string, value byte) *obj.LSym {
		s := &obj.LSym{Name: name, Type: objabi.SRODATA, Size: 1, P: []byte{value}}
		s.Set(obj.AttrLocal, true)
		return s
	}
	externalRoot := newLocalData("test.external.root", 1)
	ordinaryLocal := newLocalData("test.ordinary.local", 2)
	base.Ctxt.Data = []*obj.LSym{externalRoot, ordinaryLocal}
	MarkGoObjDataReferencedOutsideLLVM(externalRoot)
	LowerGoObjData()

	var used string
	for _, line := range strings.Split(module.String(), "\n") {
		if strings.HasPrefix(line, "@llvm.compiler.used =") {
			used = line
			break
		}
	}
	if used == "" {
		t.Fatalf("module has no llvm.compiler.used:\n%s", module.String())
	}
	if !strings.Contains(used, "@test.external.root") {
		t.Fatalf("external GoObj root is not compiler-used: %s", used)
	}
	if strings.Contains(used, "@test.ordinary.local") {
		t.Fatalf("ordinary local GoObj data is unnecessarily compiler-used: %s", used)
	}
}

func TestLLVMUntypedABI0FunctionAddressCreatesFunctionDeclaration(t *testing.T) {
	oldModule := CurrentModule
	oldLowerer := currentLLVMDataLowerer
	oldTarget := typecheck.Target
	module := GlobalCtxt.NewModule("abi0_function_address")
	CurrentModule = module
	currentLLVMDataLowerer = nil
	typecheck.Target = new(ir.Package)
	t.Cleanup(func() {
		typecheck.Target = oldTarget
		currentLLVMDataLowerer = oldLowerer
		CurrentModule = oldModule
		module.Dispose()
	})

	typ := llvm.FunctionType(GlobalCtxt.VoidType(), nil, false)
	internal := llvm.AddFunction(module, "runtime.asyncPreempt", typ)
	internal.SetFunctionCallConv(goABIInternalCallConv)

	pkg := types.NewPkg("runtime", "runtime")
	fn := ir.NewFunc(src.NoXPos, src.NoXPos, pkg.Lookup("asyncPreempt"), nil)
	fn.ABI = obj.ABI0
	typecheck.Target.Funcs = append(typecheck.Target.Funcs, fn)
	sym := fn.LinksymABI(fn.ABI)
	if sym.Type != objabi.Sxxx {
		t.Fatalf("test requires an unresolved bodyless LSym, got %v", sym.Type)
	}
	got := llvmGoDataRef(sym)
	if got.IsAFunction().IsNil() || got.Name() != "runtime.asyncPreempt<ABI0>" {
		t.Fatalf("ABI0 function address resolved to %q, want ABI0 function declaration", got.Name())
	}
}

func TestLLVMNamedAggregateConversionReshapesValue(t *testing.T) {
	module := GlobalCtxt.NewModule("named_aggregate_conversion")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	fields := []llvm.Type{GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type(), GlobalCtxt.Int64Type()}
	pageCache := GlobalCtxt.StructCreateNamed("runtime.pageCache")
	pageCache.StructSetBody(fields, false)
	exportedPageCache := GlobalCtxt.StructCreateNamed("runtime.PageCache")
	exportedPageCache.StructSetBody(fields, false)

	fn := llvm.AddFunction(module, "convert", llvm.FunctionType(exportedPageCache, []llvm.Type{pageCache}, false))
	builder.SetInsertPointAtEnd(llvm.AddBasicBlock(fn, "entry"))
	context := &LLVMFuncContext{b: builder}
	builder.CreateRet(context.reshapeLLVMValueToType(fn.Param(0), exportedPageCache, "pagecache"))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("LLVM verifier rejected named aggregate reshape: %v\n%s", err, module.String())
	}
}

func TestLLVMJumpTableDefaultIsUnreachable(t *testing.T) {
	module := GlobalCtxt.NewModule("jump_table_default")
	builder := GlobalCtxt.NewBuilder()
	t.Cleanup(module.Dispose)
	t.Cleanup(builder.Dispose)

	i64 := GlobalCtxt.Int64Type()
	function := llvm.AddFunction(module, "jump_table_default", llvm.FunctionType(i64, []llvm.Type{i64}, false))
	jumpLLVM := llvm.AddBasicBlock(function, "jump")
	mergeLLVM := llvm.AddBasicBlock(function, "merge")
	otherLLVM := llvm.AddBasicBlock(function, "other")

	jump := &Block{ID: 1, Kind: BlockJumpTable}
	merge := &Block{ID: 2}
	other := &Block{ID: 3}
	control := &Value{ID: 1, Type: types.Types[types.TINT]}
	jump.Controls[0] = control
	jump.Succs = []Edge{{b: merge}, {b: merge}, {b: other}}
	context := &LLVMFuncContext{
		BBs: map[ID]llvm.BasicBlock{
			jump.ID:  jumpLLVM,
			merge.ID: mergeLLVM,
			other.ID: otherLLVM,
		},
		Vs: map[ID]llvm.Value{control.ID: function.Param(0)},
		LF: function,
		b:  builder,
	}
	context.CompileBlock(jump, nil)

	builder.SetInsertPointAtEnd(mergeLLVM)
	phi := builder.CreatePHI(i64, "carried")
	seven := llvm.ConstInt(i64, 7, false)
	phi.AddIncoming([]llvm.Value{seven, seven}, []llvm.BasicBlock{jumpLLVM, jumpLLVM})
	builder.CreateRet(phi)
	builder.SetInsertPointAtEnd(otherLLVM)
	builder.CreateRet(llvm.ConstInt(i64, 9, false))

	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatalf("jump table added a non-SSA default edge: %v\n%s", err, module.String())
	}
	if ir := module.String(); !strings.Contains(ir, "b1.jump.default") || !strings.Contains(ir, "unreachable") {
		t.Fatalf("jump table has no unreachable default block\n%s", ir)
	}
}

func TestLLVMCurrentGRegister(t *testing.T) {
	for _, test := range []struct {
		name     string
		arch     string
		abi      obj.ABI
		register string
		ok       bool
	}{
		{"amd64 ABIInternal", "amd64", obj.ABIInternal, "r14", true},
		{"amd64 ABI0", "amd64", obj.ABI0, "", false},
		{"arm64 ABIInternal", "arm64", obj.ABIInternal, "x28", true},
		{"arm64 ABI0", "arm64", obj.ABI0, "x28", true},
		{"unsupported target", "riscv64", obj.ABIInternal, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			register, ok := llvmCurrentGRegister(test.arch, test.abi)
			if register != test.register || ok != test.ok {
				t.Fatalf("llvmCurrentGRegister(%q, %v) = (%q, %v), want (%q, %v)", test.arch, test.abi, register, ok, test.register, test.ok)
			}
		})
	}
}

func TestLLVMFunctionStorageName(t *testing.T) {
	morestack, ok := goobj.BuiltinSymbolName("runtime.morestack", int(obj.ABI0))
	if !ok {
		t.Fatal("runtime.morestack ABI0 is absent from GoObj builtin table")
	}
	for _, test := range []struct {
		name string
		cc   llvm.CallConv
		want string
	}{
		{"runtime.morestack", goABI0CallConv, "runtime.morestack<ABI0>"},
		{"runtime.morestack", goABIInternalCallConv, "runtime.morestack"},
		{morestack, goABI0CallConv, morestack + "<ABI0>"},
	} {
		if got := llvmFunctionStorageName(test.name, test.cc); got != test.want {
			t.Errorf("llvmFunctionStorageName(%q, %d) = %q, want %q", test.name, test.cc, got, test.want)
		}
	}
}

func TestLLVMGoObjReferenceNames(t *testing.T) {
	oldLinkshared := base.Ctxt.Flag_linkshared
	oldLocalDefinitions := llvmGoObjLocalDefinitions
	base.Ctxt.Flag_linkshared = false
	llvmGoObjLocalDefinitions = make(map[llvmGoObjSymbolKey]bool)
	t.Cleanup(func() {
		base.Ctxt.Flag_linkshared = oldLinkshared
		llvmGoObjLocalDefinitions = oldLocalDefinitions
	})

	builtin := base.Ctxt.LookupABI("runtime.panicdivide", obj.ABIInternal)
	wantBuiltin, ok := goobj.BuiltinSymbolName(builtin.Name, int(builtin.ABI()))
	if !ok {
		t.Fatal("runtime.panicdivide is absent from GoObj builtin table")
	}
	if got := llvmGoObjReferenceName(builtin); got != wantBuiltin {
		t.Fatalf("builtin reference name = %q, want %q", got, wantBuiltin)
	}

	// Builtin and linkname are mutually exclusive reference encodings. The
	// builtin table wins when an implementation also carries a linkname bit.
	oldBuiltinLinkname := builtin.IsLinkname()
	builtin.Set(obj.AttrLinkname, true)
	t.Cleanup(func() { builtin.Set(obj.AttrLinkname, oldBuiltinLinkname) })
	if got := llvmGoObjReferenceName(builtin); got != wantBuiltin {
		t.Fatalf("linknamed builtin reference name = %q, want %q", got, wantBuiltin)
	}

	linkname := base.Ctxt.LookupABI("runtime.llvmLinknamePull", obj.ABIInternal)
	oldLinkname := linkname.IsLinkname()
	linkname.Set(obj.AttrLinkname, true)
	t.Cleanup(func() { linkname.Set(obj.AttrLinkname, oldLinkname) })
	if got, want := llvmGoObjReferenceName(linkname), linkname.Name+goobj.LinknameSymbolSuffix; got != want {
		t.Fatalf("linkname pull name = %q, want %q", got, want)
	}
	llvmGoObjLocalDefinitions[llvmGoObjSymbolKeyFor(linkname)] = true
	if got := llvmGoObjReferenceName(linkname); got != linkname.Name {
		t.Fatalf("local linkname definition name = %q, want %q", got, linkname.Name)
	}

	linknameStd := base.Ctxt.LookupABI("runtime.llvmLinknameStdPull", obj.ABIInternal)
	oldLinknameStd := linknameStd.IsLinknameStd()
	linknameStd.Set(obj.AttrLinknameStd, true)
	t.Cleanup(func() { linknameStd.Set(obj.AttrLinknameStd, oldLinknameStd) })
	if got, want := llvmGoObjReferenceName(linknameStd), linknameStd.Name+goobj.LinknameSymbolSuffix; got != want {
		t.Fatalf("linknamestd pull name = %q, want %q", got, want)
	}

	builtin.Set(obj.AttrLinkname, oldBuiltinLinkname)
	base.Ctxt.Flag_linkshared = true
	for _, s := range []*obj.LSym{builtin, linkname, linknameStd} {
		if got := llvmGoObjReferenceName(s); got != s.Name {
			t.Fatalf("linkshared reference name = %q, want %q", got, s.Name)
		}
	}
}

func TestEmitLateGoObjBuiltinDeclarations(t *testing.T) {
	oldModule := CurrentModule
	oldCompilerUsed := goObjCompilerUsed
	oldCompilerUsedNames := goObjCompilerUsedNames
	oldLinkshared := base.Ctxt.Flag_linkshared
	module := GlobalCtxt.NewModule("late_goobj_builtins")
	CurrentModule = module
	goObjCompilerUsed = nil
	goObjCompilerUsedNames = make(map[string]bool)
	base.Ctxt.Flag_linkshared = false
	t.Cleanup(func() {
		base.Ctxt.Flag_linkshared = oldLinkshared
		goObjCompilerUsedNames = oldCompilerUsedNames
		goObjCompilerUsed = oldCompilerUsed
		CurrentModule = oldModule
		module.Dispose()
	})

	emitLateGoObjBuiltinDeclarations()
	lateCount := 0
	for i := 0; i < goobj.NBuiltin(); i++ {
		if !goobj.BuiltinIsLate(i) {
			continue
		}
		lateCount++
		name, abi := goobj.BuiltinName(i)
		storageName, ok := goobj.BuiltinSymbolName(name, abi)
		if !ok {
			t.Fatalf("late builtin %s has no encoded name", name)
		}
		storageName = llvmFunctionStorageName(storageName, llvmCallConv(obj.ABI(abi)))
		if fn := module.NamedFunction(storageName); fn.IsNil() {
			t.Errorf("late builtin declaration %q is absent", storageName)
		}
	}
	if got := len(goObjCompilerUsed); got != lateCount {
		t.Fatalf("compiler-used late builtin count = %d, want %d", got, lateCount)
	}
	memmove, ok := goobj.BuiltinSymbolName("runtime.memmove", int(obj.ABIInternal))
	if !ok {
		t.Fatal("runtime.memmove is absent from GoObj builtin table")
	}
	if fn := module.NamedFunction(memmove); !fn.IsNil() {
		t.Fatalf("ordinary builtin %q was declared eagerly", memmove)
	}
}

func TestLLVMAMD64MapPackedByteLowering(t *testing.T) {
	oldTypes := type2lTypes
	oldModule := CurrentModule
	type2lTypes = map[*types.Type]llvm.Type{
		types.Types[types.TUINT8]:  GlobalCtxt.Int8Type(),
		types.Types[types.TUINT64]: GlobalCtxt.Int64Type(),
		types.TypeInt128:           llvmAMD64ByteVectorType(),
	}
	defer func() {
		type2lTypes = oldTypes
		CurrentModule = oldModule
	}()

	newContext := func(t *testing.T, name string, parameterCount int) (llvm.Module, *LLVMFuncContext, []*Value) {
		t.Helper()
		module := GlobalCtxt.NewModule(name)
		CurrentModule = module
		parameters := make([]llvm.Type, parameterCount)
		for i := range parameters {
			parameters[i] = GlobalCtxt.Int64Type()
		}
		function := llvm.AddFunction(module, name, llvm.FunctionType(GlobalCtxt.Int64Type(), parameters, false))
		block := llvm.AddBasicBlock(function, "entry")
		builder := GlobalCtxt.NewBuilder()
		t.Cleanup(module.Dispose)
		t.Cleanup(builder.Dispose)
		builder.SetInsertPointAtEnd(block)
		context := &LLVMFuncContext{
			Vs: make(map[ID]llvm.Value),
			b:  builder,
		}
		arguments := make([]*Value, parameterCount)
		for i := range arguments {
			arguments[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: types.Types[types.TUINT64]}
			context.Vs[arguments[i].ID] = function.Param(i)
		}
		return module, context, arguments
	}

	t.Run("GOAMD64v1", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v1", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		unpacked := &Value{ID: 5, Op: OpAMD64PUNPCKLBW, Type: types.TypeInt128, Args: []*Value{h2, h2}}
		broadcast := &Value{ID: 6, Op: OpAMD64PSHUFLW, Type: types.TypeInt128, AuxInt: 0, Args: []*Value{unpacked}}
		equal := &Value{ID: 7, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		mask := &Value{ID: 8, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT8], Args: []*Value{equal}}
		result := &Value{ID: 9, Op: OpZeroExt8to64, Type: types.Types[types.TUINT64], Args: []*Value{mask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v1 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"bitcast i64 %0 to <8 x i8>",
			"shufflevector <8 x i8>",
			"shufflevector <16 x i8>",
			"bitcast <16 x i8>",
			"icmp eq <16 x i8>",
			"sext <16 x i1>",
			"call i32 @llvm.x86.sse2.pmovmskb.128(<16 x i8>",
			"trunc i32",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v1 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v2", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v2", 2)
		group := &Value{ID: 3, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[0]}}
		h2 := &Value{ID: 4, Op: OpAMD64MOVQi2f, Type: types.TypeInt128, Args: []*Value{args[1]}}
		broadcast := &Value{ID: 5, Op: OpAMD64PSHUFBbroadcast, Type: types.TypeInt128, Args: []*Value{h2}}
		equal := &Value{ID: 6, Op: OpAMD64PCMPEQB, Type: types.TypeInt128, Args: []*Value{broadcast, group}}
		equalMask := &Value{ID: 7, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{equal}}
		signed := &Value{ID: 8, Op: OpAMD64PSIGNB, Type: types.TypeInt128, Args: []*Value{group, group}}
		signMask := &Value{ID: 9, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{signed}}
		result := &Value{ID: 10, Op: OpOr64, Type: types.Types[types.TUINT64], Args: []*Value{equalMask, signMask}}
		context.b.CreateRet(context.GenLV(result))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v2 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		for _, want := range []string{
			"insertelement <16 x i8>",
			"icmp sgt <16 x i8>",
			"icmp slt <16 x i8>",
			"select <16 x i1>",
		} {
			if !strings.Contains(ir, want) {
				t.Errorf("v2 LLVM IR does not contain %q\n%s", want, ir)
			}
		}
	})

	t.Run("GOAMD64v4", func(t *testing.T) {
		module, context, args := newContext(t, "maps_v4", 1)
		broadcast := &Value{ID: 2, Op: OpAMD64VPBROADCASTB, Type: types.TypeInt128, Args: []*Value{args[0]}}
		mask := &Value{ID: 3, Op: OpAMD64PMOVMSKB, Type: types.Types[types.TUINT64], Args: []*Value{broadcast}}
		context.b.CreateRet(context.GenLV(mask))
		if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
			t.Fatalf("LLVM verifier rejected v4 packed-byte lowering: %v\n%s", err, module.String())
		}
		ir := module.String()
		if !strings.Contains(ir, "trunc i64 %0 to i8") || !strings.Contains(ir, "shufflevector <16 x i8>") {
			t.Errorf("v4 LLVM IR does not broadcast the low input byte\n%s", ir)
		}
	})
}

func TestLLVMTargetCPU(t *testing.T) {
	for _, test := range []struct {
		arch    string
		goamd64 int
		want    string
	}{
		{"arm64", 1, ""},
		{"amd64", 1, "x86-64"},
		{"amd64", 2, "x86-64-v2"},
		{"amd64", 3, "x86-64-v3"},
		{"amd64", 4, "x86-64-v4"},
	} {
		t.Run(test.arch+"/"+test.want, func(t *testing.T) {
			if got := llvmTargetCPU(test.arch, test.goamd64); got != test.want {
				t.Fatalf("llvmTargetCPU(%q, %d) = %q, want %q", test.arch, test.goamd64, got, test.want)
			}
		})
	}
}

func TestLLVMRuntimeGorecoverUsesLinkSymbolName(t *testing.T) {
	recoverFn := &Func{
		Name:   "gorecover",
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "runtime.gorecover"}},
	}
	if !llvmIsRuntimeGorecover(recoverFn) {
		t.Fatal("gorecover definition was not recognized from its qualified link symbol")
	}

	unrelated := &Func{
		Name:   "gorecover",
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "other.gorecover"}},
	}
	if llvmIsRuntimeGorecover(unrelated) {
		t.Fatal("unqualified SSA function name incorrectly identified another package's gorecover")
	}

	caller := &Func{
		OwnAux: &AuxCall{Fn: &obj.LSym{Name: "runtime.preprintpanics.func1"}},
		Blocks: []*Block{{
			Values: []*Value{{Aux: &AuxCall{Fn: &obj.LSym{Name: "runtime.gorecover"}}}},
		}},
	}
	if llvmIsRuntimeGorecover(caller) {
		t.Fatal("direct gorecover caller was incorrectly identified as gorecover itself")
	}
}
