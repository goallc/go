// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/goobj"
	"cmd/internal/obj"

	"github.com/goallc/go-llvm"
)

// llvmFunctionModel describes audited contracts of compiler-known functions.
// An absent model is conservative. Unconditional properties are attached to
// the function so every direct caller sees the same contract. GC leaf describes
// calls to the function, not how calls inside its body should be compiled.
//
// Future argument-dependent properties belong on the call, never on the shared
// declaration. Parameter properties must use the lowered ABI signature.
type llvmFunctionModel struct {
	noReturn       bool
	gcLeaf         bool
	noFree         bool
	noCallback     bool
	noUnwind       bool
	willReturn     bool
	noSync         bool
	readOnlyMemory bool
	noMemory       bool

	// Attributes for the known ABIInternal signature. Parameter entries use
	// zero-based argument positions; the binding below uses LLVM's 1-based indices.
	parameters [][]llvm.Attribute
	result     []llvm.Attribute
	// Go int result ranges are precreated for each supported integer width.
	resultRange map[int]llvm.Attribute
	allocSize   llvm.Attribute
	// Audited mallocgc-family call contract: (byte size, type, needzero).
	allocation bool
	newObject  bool
	// Typed boxing copies into a new allocation unless the type has zero size.
	copiedObject bool
}

// llvmFunctionManager owns the association between exact LLVM function names,
// LLVM declarations and their semantic attributes. Declarations remain lazy,
// and CurrentModule is the only cache of LLVM values. In particular, no cached
// handle can outlive a module or survive replacement of a provisional signature.
type llvmFunctionManager struct {
	models map[string]llvmFunctionModel
	// Size-dependent attributes belong to GlobalCtxt and are shared by calls.
	dereferenceable map[uint64]llvm.Attribute
}

// Attributes belong to GlobalCtxt and can be reused across modules. Unlike
// function handles, they survive declaration replacement and module disposal.
var (
	llvmNoReturnAttribute       = llvmModelAttribute("noreturn")
	llvmGCLeafAttribute         = GlobalCtxt.CreateStringAttribute(goGCLeafFunctionAttr, "")
	llvmNoFreeAttribute         = llvmModelAttribute("nofree")
	llvmNoCallbackAttribute     = llvmModelAttribute("nocallback")
	llvmNoUnwindAttribute       = llvmModelAttribute("nounwind")
	llvmCapturesNoneAttribute   = llvmModelAttribute("captures")
	llvmReadOnlyAttribute       = llvmModelAttribute("readonly")
	llvmWriteOnlyAttribute      = llvmModelAttribute("writeonly")
	llvmWillReturnAttribute     = llvmModelAttribute("willreturn")
	llvmNoSyncAttribute         = llvmModelAttribute("nosync")
	llvmReadOnlyMemoryAttribute = GlobalCtxt.CreateReadOnlyMemoryAttribute()
	llvmNoMemoryAttribute       = llvmModelAttribute("memory")
	llvmNoAliasAttribute        = llvmModelAttribute("noalias")
	llvmAllocAttribute          = GlobalCtxt.CreateAllocKindAttribute(false)
	llvmZeroAllocAttribute      = GlobalCtxt.CreateAllocKindAttribute(true)
	llvmAllocFamilyAttribute    = GlobalCtxt.CreateStringAttribute("alloc-family", "runtime.mallocgc")
	llvmFunctions               = newLLVMFunctionManager()
)

func newLLVMFunctionManager() llvmFunctionManager {
	// The raw assembly helpers in runtime/memmove_*.s, memclr_*.s and
	// internal/bytealg/equal_*.s neither capture argument pointers, free memory,
	// call back into Go, nor unwind through LLVM EH. Their implementations may
	// read CPU flags, and arm64 memclr updates its cached ZVA block size, so
	// parameter access modes do not imply an argmem-only function effect.
	readPointer := []llvm.Attribute{llvmCapturesNoneAttribute, llvmReadOnlyAttribute}
	writePointer := []llvm.Attribute{llvmCapturesNoneAttribute, llvmWriteOnlyAttribute}
	nonnull := llvmModelAttribute("nonnull")
	nonnullResult := []llvm.Attribute{nonnull}
	freshResult := []llvm.Attribute{nonnull, llvmNoAliasAttribute}
	boxedResult := func(bytes uint64) []llvm.Attribute {
		return []llvm.Attribute{nonnull, GlobalCtxt.CreateEnumAttribute(llvm.AttributeKindID("dereferenceable"), bytes)}
	}
	comparisonRange := make(map[int]llvm.Attribute)
	lengthRange := make(map[int]llvm.Attribute)
	for _, bits := range []uint{32, 64} {
		comparisonRange[int(bits)] = GlobalCtxt.CreateConstantRangeAttribute(llvm.AttributeKindID("range"), bits, []uint64{^uint64(0)}, []uint64{2})
		lengthRange[int(bits)] = GlobalCtxt.CreateConstantRangeAttribute(llvm.AttributeKindID("range"), bits, []uint64{0}, []uint64{uint64(1) << (bits - 1)})
	}
	mallocModel := llvmFunctionModel{
		result: nonnullResult, allocSize: GlobalCtxt.CreateAllocSizeAttribute(0), allocation: true,
	}
	models := map[string]llvmFunctionModel{
		// These raw helpers already have a GC-leaf call contract in both the SSA
		// static-call path and the dedicated memory-operation lowering paths.
		"runtime.memmove": {
			gcLeaf: true, noFree: true, noCallback: true, noUnwind: true,
			parameters: [][]llvm.Attribute{writePointer, readPointer},
		},
		"runtime.memequal": {
			gcLeaf: true, noFree: true, noCallback: true, noUnwind: true,
			willReturn: true, noSync: true, readOnlyMemory: true,
			parameters: [][]llvm.Attribute{readPointer, readPointer},
		},
		"runtime.memclrNoHeapPointers": {
			gcLeaf: true, noFree: true, noCallback: true, noUnwind: true,
			parameters: [][]llvm.Attribute{writePointer},
		},
		// The variable-length equality entry reads its size from the closure
		// context and tail-calls the same assembly comparison body.
		"runtime.memequal_varlen": {
			gcLeaf: true, noFree: true, noCallback: true, noUnwind: true,
			willReturn: true, noSync: true, readOnlyMemory: true,
			parameters: [][]llvm.Attribute{readPointer, readPointer},
		},
		// String arguments are aggregate values in LLVM IR, not pointer
		// parameters. The assembly only compares bytes (and may read CPU flags).
		"runtime.cmpstring": {resultRange: comparisonRange, gcLeaf: true, noFree: true, noCallback: true, noUnwind: true, willReturn: true, noSync: true, readOnlyMemory: true},
		"runtime.wbMove":    {gcLeaf: true, noUnwind: true},
		"runtime.wbZero":    {gcLeaf: true, noUnwind: true},

		// Successful allocation returns a non-null pointer, including zerobase
		// for zero bytes. These attributes imply neither GC leaf nor purity.
		"runtime.mallocgc":                     mallocModel,
		"runtime.mallocgcTinySC2":              mallocModel,
		"runtime.mallocgcSmallNoScanSC1":       mallocModel,
		"runtime.mallocgcSmallNoScanSC2":       mallocModel,
		"runtime.mallocgcSmallNoScanSC3":       mallocModel,
		"runtime.mallocgcSmallNoScanSC4":       mallocModel,
		"runtime.mallocgcSmallNoScanSC5":       mallocModel,
		"runtime.mallocgcSmallNoScanSC6":       mallocModel,
		"runtime.mallocgcSmallNoScanSC7":       mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC1": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC2": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC3": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC4": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC5": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC6": mallocModel,
		"runtime.mallocgcSmallScanNoHeaderSC7": mallocModel,

		"runtime.newobject":     {result: nonnullResult, newObject: true},
		"runtime.makeslice":     {result: nonnullResult},
		"runtime.makeslice64":   {result: nonnullResult},
		"runtime.makeslicecopy": {result: nonnullResult},
		"runtime.convT":         {result: nonnullResult, copiedObject: true},
		"runtime.convTnoptr":    {result: nonnullResult, copiedObject: true},
		// Small values can use staticuint64s: readable, but not fresh/noalias.
		"runtime.convT16":     {result: boxedResult(2)},
		"runtime.convT32":     {result: boxedResult(4)},
		"runtime.convT64":     {result: boxedResult(8)},
		"runtime.convTstring": {result: nonnullResult},
		"runtime.convTslice":  {result: nonnullResult},

		// Normal returns are non-null, including zero-capacity channels and
		// missing map keys (zeroVal). Map constructors may reuse the caller
		// header; access/assignment returns shared slots, not fresh allocations.
		// Channel headers and makemap_small always allocate nonzero fresh objects.
		"runtime.makechan":            {result: freshResult},
		"runtime.makechan64":          {result: freshResult},
		"runtime.makemap":             {result: nonnullResult},
		"runtime.makemap64":           {result: nonnullResult},
		"runtime.makemap_small":       {result: freshResult},
		"runtime.mapaccess1":          {result: nonnullResult},
		"runtime.mapaccess1_fast32":   {result: nonnullResult},
		"runtime.mapaccess1_fast64":   {result: nonnullResult},
		"runtime.mapaccess1_faststr":  {result: nonnullResult},
		"runtime.mapassign":           {result: nonnullResult},
		"runtime.mapassign_fast32":    {result: nonnullResult},
		"runtime.mapassign_fast32ptr": {result: nonnullResult},
		"runtime.mapassign_fast64":    {result: nonnullResult},
		"runtime.mapassign_fast64ptr": {result: nonnullResult},
		"runtime.mapassign_faststr":   {result: nonnullResult},

		// The mandatory assertion either returns a cached/new itab or panics.
		"runtime.assertE2I": {result: nonnullResult},

		// These contracts describe logical memory effects. Stack relocation is
		// handled by statepoints; these Go helpers are deliberately not GC leaf.
		"runtime.memequal0":   {noUnwind: true, willReturn: true, noMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.memequal8":   {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.memequal16":  {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.memequal32":  {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.memequal64":  {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.memequal128": {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.f32equal":    {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.f64equal":    {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.c64equal":    {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.c128equal":   {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},
		"runtime.strequal":    {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer, readPointer}},

		// Hashing also reads global seeds; floating-point hashes can update
		// the per-M random state for NaNs, so those hashes remain unrestricted.
		"runtime.memhash":        {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash0":       {noUnwind: true, willReturn: true, noMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash8":       {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash16":      {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash32":      {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash64":      {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash128":     {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.memhash_varlen": {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.strhash":        {noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.f32hash":        {noUnwind: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.f64hash":        {noUnwind: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.c64hash":        {noUnwind: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.c128hash":       {noUnwind: true, parameters: [][]llvm.Attribute{readPointer}},

		// Aggregate and scalar operands need no pointer parameter attributes.
		"runtime.decoderune":      {noUnwind: true, willReturn: true, readOnlyMemory: true},
		"runtime.countrunes":      {resultRange: lengthRange, noUnwind: true, willReturn: true, readOnlyMemory: true},
		"runtime.complex128div":   {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.float64toint64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.float64touint64": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.float64touint32": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.int64tofloat64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.int64tofloat32":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.uint64tofloat64": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.uint64tofloat32": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.uint32tofloat64": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.rand":            {noUnwind: true},
		"runtime.rand32":          {noUnwind: true},
		"runtime.selectsetpc":     {noUnwind: true, parameters: [][]llvm.Attribute{writePointer}},

		// Software floating-point entry points use scalar bit-pattern operands.
		"runtime.f32to64":     {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f32toint32":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f32toint64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f32touint64": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f64to32":     {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f64toint32":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f64toint64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.f64touint64": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fadd32":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fadd64":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fdiv32":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fdiv64":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.feq32":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.feq64":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fge32":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fge64":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fgt32":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fgt64":       {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fint32to32":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fint32to64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fint64to32":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fint64to64":  {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fmul32":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fmul64":      {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fuint64to32": {noUnwind: true, willReturn: true, noMemory: true},
		"runtime.fuint64to64": {noUnwind: true, willReturn: true, noMemory: true},

		// Channel queries only inspect the header, including timer-channel
		// handling. They neither retain the channel nor perform send/receive.
		"runtime.chanlen": {resultRange: lengthRange, noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},
		"runtime.chancap": {resultRange: lengthRange, noUnwind: true, willReturn: true, readOnlyMemory: true, parameters: [][]llvm.Attribute{readPointer}},

		// The bulk barriers and cgo pointer checks run without safe points,
		// including their system-stack slow paths. Barrier destinations must
		// not be writeonly: the barrier reads their old pointer values first.
		"runtime.typedmemmove":      {gcLeaf: true, noUnwind: true},
		"runtime.typedmemclr":       {gcLeaf: true, noUnwind: true},
		"runtime.memclrHasPointers": {gcLeaf: true, noUnwind: true},
		"runtime.cgoCheckMemmove":   {gcLeaf: true, noUnwind: true},
		"runtime.cgoCheckPtrWrite":  {gcLeaf: true, noUnwind: true},

		// Assembly-only entry points have implicit register operands; they get
		// function contracts only, never ordinary parameter contracts.
		"runtime.duffcopy":        {gcLeaf: true, noFree: true, noCallback: true, noUnwind: true},
		"runtime.duffzero":        {gcLeaf: true, noFree: true, noCallback: true, noUnwind: true},
		"runtime.gcWriteBarrier1": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier2": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier3": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier4": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier5": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier6": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier7": {gcLeaf: true, noUnwind: true},
		"runtime.gcWriteBarrier8": {gcLeaf: true, noUnwind: true},

		// These entries never return to their call site. Recovery resumes an
		// older frame; it does not make the panic helper return. They may run
		// user defers, so no leaf, nofree or nocallback contract is implied.
		"runtime.gopanic":                 {noReturn: true},
		"runtime.panicBounds":             {noReturn: true},
		"runtime.panicExtend":             {noReturn: true},
		"runtime.sigpanic":                {noReturn: true},
		"runtime.panicmem":                {noReturn: true},
		"runtime.panicmemAddr":            {noReturn: true},
		"runtime.panicoverflow":           {noReturn: true},
		"runtime.panicdivide":             {noReturn: true},
		"runtime.panicshift":              {noReturn: true},
		"runtime.panicmakeslicelen":       {noReturn: true},
		"runtime.panicmakeslicecap":       {noReturn: true},
		"runtime.panicwrap":               {noReturn: true},
		"runtime.panicdottypeE":           {noReturn: true},
		"runtime.panicdottypeI":           {noReturn: true},
		"runtime.panicnildottype":         {noReturn: true},
		"runtime.panicrangestate":         {noReturn: true},
		"runtime.panicunsafeslicelen":     {noReturn: true},
		"runtime.panicunsafeslicenilptr":  {noReturn: true},
		"runtime.panicunsafestringlen":    {noReturn: true},
		"runtime.panicunsafestringnilptr": {noReturn: true},
		"runtime.panicSimdImm":            {noReturn: true},
		"runtime.block":                   {noReturn: true},
		"runtime.morestackc":              {noReturn: true},
		"runtime.throw":                   {noReturn: true},
		"runtime.goPanicIndex":            {noReturn: true},
		"runtime.goPanicIndexU":           {noReturn: true},
		"runtime.goPanicSliceAlen":        {noReturn: true},
		"runtime.goPanicSliceAlenU":       {noReturn: true},
		"runtime.goPanicSliceAcap":        {noReturn: true},
		"runtime.goPanicSliceAcapU":       {noReturn: true},
		"runtime.goPanicSliceB":           {noReturn: true},
		"runtime.goPanicSliceBU":          {noReturn: true},
		"runtime.goPanicSlice3Alen":       {noReturn: true},
		"runtime.goPanicSlice3AlenU":      {noReturn: true},
		"runtime.goPanicSlice3Acap":       {noReturn: true},
		"runtime.goPanicSlice3AcapU":      {noReturn: true},
		"runtime.goPanicSlice3B":          {noReturn: true},
		"runtime.goPanicSlice3BU":         {noReturn: true},
		"runtime.goPanicSlice3C":          {noReturn: true},
		"runtime.goPanicSlice3CU":         {noReturn: true},
		"runtime.goPanicSliceConvert":     {noReturn: true},

		// These APIs terminate the goroutine or process. Do not use the inliner's
		// NeverReturns heuristic: a callee can recover its own panic and return.
		"runtime.Goexit":            {noReturn: true},
		"os.Exit":                   {noReturn: true},
		"testing.(*common).Skip":    {noReturn: true},
		"testing.(*common).Skipf":   {noReturn: true},
		"testing.(*common).SkipNow": {noReturn: true},
		"testing.(*common).Fatal":   {noReturn: true},
		"testing.(*common).Fatalf":  {noReturn: true},
		"testing.(*common).FailNow": {noReturn: true},
	}
	m := llvmFunctionManager{models: make(map[string]llvmFunctionModel)}
	// Register the exact spellings emitted by the existing GoObj naming layer.
	// Builtin indices come from its table rather than being hard-coded here.
	// Plain names are used for definitions and linkshared references. Lookup
	// never strips suffixes or translates an IR name back into a Go symbol.
	for name, model := range models {
		for _, abi := range []obj.ABI{obj.ABIInternal, obj.ABI0} {
			cc := llvmCallConv(abi)
			m.models[llvmFunctionStorageName(name, cc)] = model
			m.models[llvmFunctionStorageName(name+goobj.LinknameSymbolSuffix, cc)] = model
			if builtin, ok := goobj.BuiltinSymbolName(name, int(abi)); ok {
				m.models[llvmFunctionStorageName(builtin, cc)] = model
			}
		}
	}
	return m
}

// name is the final IR function name, including any GoObj or ABI suffixes.
func (m *llvmFunctionManager) getOrInsert(name string, sig llvmFuncSignature, cc llvm.CallConv) llvm.Value {
	fn := CurrentModule.NamedFunction(name)
	if fn.IsNil() {
		fn = llvm.AddFunction(CurrentModule, name+".goallc.final", sig.Type)
		if placeholder := CurrentModule.NamedGlobal(name); !placeholder.IsNil() {
			// An OpAddr may have needed the code address before this function
			// reached the compile queue. Opaque pointers let the provisional
			// global be replaced by the correctly typed function definition.
			placeholder.ReplaceAllUsesWith(fn)
			placeholder.EraseFromParentAsGlobal()
		}
		fn.SetName(name)
	} else if got := fn.GlobalValueType(); got != sig.Type {
		if fn.BasicBlocksCount() != 0 {
			base.Fatalf("conflicting LLVM function type for definition %s", name)
		}
		// Compiler data can refer to an ABI function before AuxCall exposes
		// its exact signature. Replace that provisional declaration now.
		replacement := llvm.AddFunction(CurrentModule, name+".goallc.final", sig.Type)
		fn.ReplaceAllUsesWith(replacement)
		fn.EraseFromParentAsFunction()
		replacement.SetName(name)
		fn = replacement
	}
	configureLLVMFunction(fn, sig, cc)
	model := m.models[fn.Name()]
	if model.noReturn {
		fn.AddFunctionAttr(llvmNoReturnAttribute)
	}
	// These contracts also hold for the NOSPLIT ABI wrappers.
	if model.gcLeaf {
		fn.AddFunctionAttr(llvmGCLeafAttribute)
	}
	if model.noFree {
		fn.AddFunctionAttr(llvmNoFreeAttribute)
	}
	if model.noCallback {
		fn.AddFunctionAttr(llvmNoCallbackAttribute)
	}
	if model.noUnwind {
		fn.AddFunctionAttr(llvmNoUnwindAttribute)
	}
	if model.willReturn {
		fn.AddFunctionAttr(llvmWillReturnAttribute)
	}
	if model.noSync {
		fn.AddFunctionAttr(llvmNoSyncAttribute)
	}
	// ABI0 returns through caller-owned slots, so its wrapper writes memory
	// even when the underlying comparison is read-only.
	if cc == goABIInternalCallConv {
		if model.readOnlyMemory {
			fn.AddFunctionAttr(llvmReadOnlyMemoryAttribute)
		}
		if model.noMemory {
			fn.AddFunctionAttr(llvmNoMemoryAttribute)
		}
	}
	// ABI0 pointer parameters denote byval argument slots, not the original
	// pointees. Only ABIInternal uses the parameter contracts below.
	if cc == goABIInternalCallConv {
		for _, attribute := range model.result {
			fn.AddAttributeAtIndex(0, attribute)
		}
		if model.resultRange != nil {
			fn.AddAttributeAtIndex(0, model.resultRange[sig.Type.ReturnType().IntTypeWidth()])
		}
		if model.allocSize.C != nil {
			fn.AddFunctionAttr(model.allocSize)
		}
		for i, parameter := range model.parameters {
			for _, attribute := range parameter {
				fn.AddAttributeAtIndex(i+1, attribute)
			}
		}
	}
	return fn
}

func llvmModelAttribute(name string) llvm.Attribute {
	kind := llvm.AttributeKindID(name)
	if kind == 0 {
		base.Fatalf("unknown LLVM runtime model attribute %q", name)
	}
	// captures is an integer-valued attribute; zero is captures(none).
	return GlobalCtxt.CreateEnumAttribute(kind, 0)
}

// bindCall adds argument-dependent contracts without strengthening a shared
// declaration or definition. A zero-byte mallocgc returns the shared zerobase;
// preserve that identity until zero-sized allocation provenance is modeled.
func (m *llvmFunctionManager) bindCall(call, fn llvm.Value, args []llvm.Value, cc llvm.CallConv, source *Value) {
	if cc != goABIInternalCallConv {
		return
	}
	model := m.models[fn.Name()]
	kind := llvmAllocAttribute
	switch {
	case model.allocation:
		size := args[0]
		if size.IsAConstantInt().IsNil() || size.ZExtValue() == 0 {
			return
		}
		if zero := args[2]; !zero.IsAConstantInt().IsNil() && zero.ZExtValue() != 0 {
			kind = llvmZeroAllocAttribute
		}
	case model.newObject || model.copiedObject:
		// Use the same static type-symbol information as Go's fixed-load
		// rewriting. A pointer result type alone does not prove allocation size.
		if source == nil || len(source.Args) == 0 || source.Args[0].Op != OpAddr {
			return
		}
		sym, ok := source.Args[0].Aux.(*obj.LSym)
		if !ok || sym.TypeInfo() == nil {
			return
		}
		typ := sym.TypeInfo().Type.(*types.Type)
		if typ.Size() <= 0 {
			return
		}
		if model.copiedObject {
			// Only the box is fresh; pointer fields still alias their original
			// pointees. Copying and instrumentation effects remain unrestricted.
			call.AddCallSiteAttribute(0, llvmNoAliasAttribute)
			return
		}
		if m.dereferenceable == nil {
			m.dereferenceable = make(map[uint64]llvm.Attribute)
		}
		size := uint64(typ.Size())
		attribute, ok := m.dereferenceable[size]
		if !ok {
			attribute = GlobalCtxt.CreateEnumAttribute(llvm.AttributeKindID("dereferenceable"), size)
			m.dereferenceable[size] = attribute
		}
		call.AddCallSiteAttribute(0, attribute)
		kind = llvmZeroAllocAttribute
	default:
		return
	}
	call.AddCallSiteAttribute(0, llvmNoAliasAttribute)
	call.AddCallSiteAttribute(llvmAttributeFunctionIndex, kind)
	call.AddCallSiteAttribute(llvmAttributeFunctionIndex, llvmAllocFamilyAttribute)
}
