// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"internal/buildcfg"
	"slices"
	"strings"

	"github.com/goallc/go-llvm"
)

// llvmCPUProfile is generated registry data, not a mutable per-function state.
// Profiles identify Go predicates; capabilities describe ISA legality.
// A feature floor supplies capabilities without supplying predicates.
type llvmCPUProfile struct {
	name, arch, field, runtimeGuard string
	targetFeatures                  string
	capabilities                    uint64
	predicates                      uint64
}

func llvmCPUProfileByName(name string) *llvmCPUProfile {
	for i := range llvmCPUProfiles {
		if llvmCPUProfiles[i].name == name {
			return &llvmCPUProfiles[i]
		}
	}
	return nil
}

func llvmCPUProfileByRuntimeGuard(arch, symbol string) string {
	if symbol != "" {
		for _, p := range llvmCPUProfiles {
			if p.arch == arch && p.runtimeGuard == symbol {
				return p.name
			}
		}
	}
	return ""
}

func llvmBaselineCPUCapabilities(arch string, amd64Level int, arm64LSE bool) uint64 {
	switch arch {
	case "amd64":
		switch {
		case amd64Level >= 4:
			return llvmCPUBaselineV4
		case amd64Level >= 3:
			return llvmCPUBaselineV3
		case amd64Level >= 2:
			return llvmCPUBaselineV2
		}
	case "arm64":
		if arm64LSE {
			return llvmCPUBaselineLSE
		}
	}
	return 0
}

func llvmCPUProfileCoveredByBaseline(arch, profile string) bool {
	if profile == "" {
		return true
	}
	p := llvmCPUProfileByName(profile)
	caps := llvmBaselineCPUCapabilities(arch, buildcfg.GOAMD64, buildcfg.GOARM64.LSE)
	return p != nil && p.arch == arch && caps&p.capabilities == p.capabilities
}

func llvmFunctionName(f *Func) string {
	name := f.Name
	if f.OwnAux != nil && f.OwnAux.Fn != nil {
		name = f.OwnAux.Fn.Name
	}
	return name
}

func llvmMidwaySIMDFeatureFloor(f *Func) (string, bool) {
	switch name := llvmFunctionName(f); {
	case strings.Contains(name, "@simd512"):
		return goCPUProfileX86AVX512, true
	case strings.Contains(name, "@simd256"):
		return goCPUProfileX86AVX2, true
	case strings.Contains(name, "@simd128"):
		return goCPUProfileX86AVX, true
	case strings.Contains(name, "@simd0"):
		return "", true
	}
	return "", false
}

// llvmSIMDFeatureFloor returns the function-wide target features established
// by Go's fixed-width SIMD parameter/result contract or a Midway variant's
// selected width. The generic Vec256 ABI itself needs AVX, while @simd256 is
// reached only after the portable dispatcher has observed HasAVX2; keep those
// two contracts distinct. This is a precondition, not a new runtime dispatch
// request: the frontend adds the corresponding LLVM target features directly.
func llvmSIMDFeatureFloor(f *Func) string {
	if f.Config.arch != "amd64" {
		return ""
	}
	if floor, midway := llvmMidwaySIMDFeatureFloor(f); midway {
		return floor
	}
	// Block.CPUfeatures also includes local SIMD values, such as a zero
	// vector hoisted out of a guarded region. Those are not caller promises
	// and must not raise the target features of fallback implementations.
	// Preserve the direct SIMD parameter/result contract used by cpufeatures;
	// pointers and ordinary aggregates do not establish that contract.
	var width int64
	if f.Type != nil {
		for _, field := range f.Type.RecvParamsResults() {
			if field.Type.IsSIMD() {
				width = max(width, field.Type.Size())
			}
		}
	}
	switch width {
	case 64:
		return goCPUProfileX86AVX512
	case 16, 32:
		return goCPUProfileX86AVX
	}
	return ""
}

// llvmWideVectorTypeWidth returns the widest fixed SIMD vector nested directly
// in a Go ABI value. Pointers deliberately stop the walk: their referents do
// not cross the call boundary in vector registers.
func llvmWideVectorTypeWidth(t *types.Type) int64 {
	if t == nil {
		return 0
	}
	if t.IsSIMD() {
		return t.Size()
	}
	switch {
	case t.IsArray():
		return llvmWideVectorTypeWidth(t.Elem())
	case t.IsStruct():
		var width int64
		for _, field := range t.Fields() {
			if candidate := llvmWideVectorTypeWidth(field.Type); candidate > width {
				width = candidate
			}
		}
		return width
	}
	return 0
}

// llvmWideVectorCallWidth reports wide vectors that the Go ABI assigns to
// registers for this call. Stack-only byval arguments and result homes are not
// LLVM register-ABI carriers and therefore need no caller target feature.
func llvmWideVectorCallWidth(aux *AuxCall) int64 {
	var width int64
	for i := int64(0); i < aux.NArgs(); i++ {
		if len(aux.RegsOfArg(i)) == 0 {
			continue
		}
		if candidate := llvmWideVectorTypeWidth(aux.TypeOfArg(i)); candidate > width {
			width = candidate
		}
	}
	for i := int64(0); i < aux.NResults(); i++ {
		if len(aux.RegsOfResult(i)) == 0 {
			continue
		}
		if candidate := llvmWideVectorTypeWidth(aux.TypeOfResult(i)); candidate > width {
			width = candidate
		}
	}
	return width
}

func llvmWideVectorCPUProfile(width int64) string {
	switch {
	case width > 32:
		return goCPUProfileX86AVX512
	case width > 16:
		return goCPUProfileX86AVX
	}
	return ""
}

func llvmCPUProfileSupplies(profile, required string) bool {
	if required == "" {
		return true
	}
	p, r := llvmCPUProfileByName(profile), llvmCPUProfileByName(required)
	return p != nil && r != nil && p.arch == r.arch && p.capabilities&r.capabilities == r.capabilities
}

// llvmCPUFeatureGuard recognizes the enabled successor of an ordinary
// internal/cpu feature test, without changing native CPU analysis policy.
func llvmCPUFeatureGuard(b *Block, arch string) (profile string, enabled int) {
	if b.Kind != BlockIf || b.Controls[0] == nil || b.Controls[1] != nil || len(b.Succs) != 2 {
		return "", -1
	}
	condition := b.Controls[0]
	taken := 0
	if condition.Op == OpNot {
		if len(condition.Args) != 1 {
			return "", -1
		}
		taken = 1
		condition = condition.Args[0]
	}
	profile = llvmCPUFieldProfile(arch, llvmCPUFeatureField(condition, arch))
	if profile == "" {
		return "", -1
	}
	return profile, taken
}

// llvmCPUFeatureGuardProfiles finds effective predicates that protect every
// path to v. Keep the nearest single dominating guard when possible, avoiding
// extra clones for redundant outer conditions. A successor block alone is not
// enough: another incoming edge could bypass the enabled guard edge.
func llvmCPUFeatureGuardProfiles(f *Func, v *Value, required string) []string {
	sdom := f.Sdom()
	for b := sdom.Parent(v.Block); b != nil; b = sdom.Parent(b) {
		profile, taken := llvmCPUFeatureGuard(b, f.Config.arch)
		if taken < 0 || !llvmCPUProfileSupplies(profile, required) {
			continue
		}
		enabled := b.Succs[taken].Block()
		if enabled == f.Entry || !sdom.IsAncestorEq(enabled, v.Block) {
			continue
		}
		dominates := true
		for _, pred := range enabled.Preds {
			if pred.Block() == b && pred.Index() == taken {
				continue
			}
			// Backedges do not provide a new entry to the guarded region.
			if !sdom.IsAncestorEq(enabled, pred.Block()) {
				dominates = false
				break
			}
		}
		if dominates {
			return []string{profile}
		}
	}

	// Cut each backwards path at an enabled edge supplying the requirement.
	// Reaching entry means some path is unguarded. Visited blocks also bound
	// traversal through loops, whose first entry must still cross the cut.
	seen := make(map[*Block]bool)
	profiles := make(map[string]bool)
	work := []*Block{v.Block}
	for len(work) != 0 {
		b := work[len(work)-1]
		work = work[:len(work)-1]
		if b == f.Entry {
			return nil
		}
		if seen[b] {
			continue
		}
		seen[b] = true
		for _, pred := range b.Preds {
			profile, taken := llvmCPUFeatureGuard(pred.Block(), f.Config.arch)
			if taken == pred.Index() && llvmCPUProfileSupplies(profile, required) {
				profiles[profile] = true
				continue
			}
			work = append(work, pred.Block())
		}
	}
	var guards []string
	for profile := range profiles {
		guards = append(guards, profile)
	}
	slices.Sort(guards)
	return guards
}

func llvmCallAux(v *Value) *AuxCall {
	switch v.Op {
	case OpStaticCall, OpStaticLECall, OpTailLECall,
		OpClosureCall, OpClosureLECall,
		OpInterCall, OpInterLECall, OpTailLECallInter:
		return auxToCall(v.Aux)
	}
	return nil
}

type llvmCPURequirementKind uint8

const (
	llvmCPUGenerated llvmCPURequirementKind = iota
	llvmCPUWideCall
)

// Entry capability assumptions are not runtime predicates.
type llvmCPUFeaturePlan struct {
	floor        string
	requirements map[ID]string
	guards       map[ID]string
	profiles     []string
}

func llvmCPURequirement(v *Value, arch string) (string, llvmCPURequirementKind) {
	if info, ok := goALLCSIMDInfo(v.Op); ok {
		return info.archInfo(arch).cpuProfile, llvmCPUGenerated
	}
	if arch == "amd64" {
		if helper, ok := llvmSIMDHelperInfo(v); ok {
			return helper.profile, llvmCPUGenerated
		}
		if aux := llvmCallAux(v); aux != nil {
			return llvmWideVectorCPUProfile(llvmWideVectorCallWidth(aux)), llvmCPUWideCall
		}
	}
	return "", llvmCPUGenerated
}

// llvmPlanCPUFeatures is the one planning boundary for generated SIMD,
// Go feature checks and wide register-ABI calls. Emission
// consumes the completed plan; it does not discover or mutate feature policy.
func llvmPlanCPUFeatures(f *Func) *llvmCPUFeaturePlan {
	plan := &llvmCPUFeaturePlan{
		floor:        llvmSIMDFeatureFloor(f),
		requirements: make(map[ID]string),
		guards:       make(map[ID]string),
	}
	type guardKey struct {
		block    *Block
		required string
	}
	// Operations in one block share the same protection. In particular, do
	// not repeat a CFG walk for each vector operation in a large block.
	guardCache := make(map[guardKey][]string)
	findGuards := func(v *Value, required string) []string {
		key := guardKey{v.Block, required}
		if guards, ok := guardCache[key]; ok {
			return guards
		}
		guards := llvmCPUFeatureGuardProfiles(f, v, required)
		guardCache[key] = guards
		return guards
	}
	type requirement struct {
		value   *Value
		profile string
		kind    llvmCPURequirementKind
		guards  []string
	}
	type guardValue struct {
		id        ID
		profile   string
		selective bool
	}
	var guards []guardValue
	var pending []requirement
	selected := make(map[string]bool)
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			// Go already supplies hardware/fallback feature checks for
			// scalar and atomic intrinsics. Collect all of them before
			// marking source loads, independently of block storage order.
			if name, selective := llvmCPUFeatureGuardValue(v, f.Config.arch); name != "" {
				guards = append(guards, guardValue{v.ID, name, selective})
				if !selective && !llvmCPUProfileCoveredByBaseline(f.Config.arch, name) {
					selected[name] = true
				}
			}
			profile, kind := llvmCPURequirement(v, f.Config.arch)
			if profile == "" || llvmCPUProfileCoveredByBaseline(f.Config.arch, profile) {
				continue
			}
			r := requirement{value: v, profile: profile, kind: kind}
			if kind == llvmCPUWideCall {
				if llvmCPUProfileSupplies(plan.floor, profile) {
					continue
				}
				r.guards = findGuards(v, profile)
			}
			pending = append(pending, r)
		}
	}
	// Preserve native fixed-width calling semantics: an unguarded wide call
	// raises the function floor, whereas a guarded call requests FMV. Finish
	// this step before handling any operation, independent of block order.
	for _, r := range pending {
		if r.kind == llvmCPUWideCall && len(r.guards) == 0 && !llvmCPUProfileSupplies(plan.floor, r.profile) {
			plan.floor = r.profile
		}
	}
	for _, r := range pending {
		if llvmCPUProfileSupplies(plan.floor, r.profile) {
			continue
		}
		guards := r.guards
		if r.kind == llvmCPUGenerated {
			guards = findGuards(r.value, r.profile)
		}
		if len(guards) == 0 {
			// An unguarded SIMD operation requests its Go feature so the baseline
			// verifier continues to reject the surviving requirement.
			guards = []string{r.profile}
		}
		plan.requirements[r.value.ID] = r.profile
		for _, guard := range guards {
			// A virtual Go feature is a conjunction, not a new runtime bit.
			// Request its atoms independently so partial feature combinations
			// preserve other observations of those same Go booleans.
			p := llvmCPUProfileByName(guard)
			for _, atom := range llvmCPUProfiles {
				if atom.field != "" && atom.arch == p.arch && atom.predicates&p.predicates == atom.predicates {
					selected[atom.name] = true
				}
			}
		}
	}
	for _, guard := range guards {
		if !guard.selective || selected[guard.profile] {
			plan.guards[guard.id] = guard.profile
		}
	}
	for _, p := range llvmCPUProfiles {
		if selected[p.name] {
			plan.profiles = append(plan.profiles, p.name)
		}
	}
	return plan
}

func llvmCPUFeatureField(v *Value, arch string) string {
	switch arch {
	case "amd64":
		return cpuFeatureField(v, "internal/cpu.X86")
	case "arm64":
		return cpuFeatureField(v, "internal/cpu.ARM64")
	}
	return ""
}

func llvmCPUFieldProfile(arch, field string) string {
	if field == "" {
		return ""
	}
	for _, p := range llvmCPUProfiles {
		if p.arch == arch && p.field == field {
			return p.name
		}
	}
	return ""
}

// User archsimd field loads
// are marked only for selected predicates; compiler-generated scalar/atomic
// guard loads retain their unconditional marker behavior.
func llvmCPUFeatureGuardValue(v *Value, arch string) (profile string, selective bool) {
	if v.Op == OpHasCPUFeature && arch == "amd64" {
		if sym, ok := v.Aux.(*obj.LSym); ok {
			return llvmCPUProfileByRuntimeGuard(arch, sym.Name), false
		}
	}
	if v.Op == OpLoad {
		if profile := llvmCPUFieldProfile(arch, llvmCPUFeatureField(v, arch)); profile != "" {
			return profile, true
		}
		if arch == "arm64" && len(v.Args) != 0 && v.Args[0].Op == OpAddr {
			if sym, ok := v.Args[0].Aux.(*obj.LSym); ok {
				return llvmCPUProfileByRuntimeGuard(arch, sym.Name), false
			}
		}
	}
	return "", false
}

func (lfc *LLVMFuncContext) requireCPUFeature(v *Value, instruction llvm.Value) {
	if profile := lfc.CPUFeatures.requirements[v.ID]; profile != "" {
		instruction.SetMetadata(GlobalCtxt.MDKindID(goCPURequiresMD), GlobalCtxt.MDNode([]llvm.Metadata{GlobalCtxt.MDString(profile)}))
	}
}

// A SIMD operation may fold to a constant, argument, or a producer
// outside its source guard. Anchor the preplanned requirement at the source
// position instead of attaching it to that result. The early FMV pass keeps
// the anchor through specialization and removes it after checking legality.
func (lfc *LLVMFuncContext) requireSIMDCPUFeature(v *Value) {
	if lfc.CPUFeatures.requirements[v.ID] == "" {
		return
	}
	fn := getLLVMIntrinsicDeclaration("llvm.sideeffect")
	anchor := lfc.b.CreateCall(fn.GlobalValueType(), fn, nil, "")
	anchor.SetMetadata(GlobalCtxt.MDKindID(goCPURequireAnchorMD), GlobalCtxt.MDNode(nil))
	lfc.requireCPUFeature(v, anchor)
}

func (lfc *LLVMFuncContext) markCPUFeatureGuard(v *Value, load llvm.Value) {
	if profile := lfc.CPUFeatures.guards[v.ID]; profile != "" {
		load.SetMetadata(GlobalCtxt.MDKindID(goCPUGuardMD), GlobalCtxt.MDNode([]llvm.Metadata{GlobalCtxt.MDString(profile)}))
	}
}

func (lfc *LLVMFuncContext) finishCPUFeatures() {
	if profiles := lfc.CPUFeatures.profiles; len(profiles) != 0 {
		// Keep the canonical imported GoObj LSym identity alive for the early
		// plugin, which materializes the load before normal optimization.
		llvmGoDataRef(ir.Syms.GoALLCCPUFeatures)
		lfc.LF.AddTargetDependentFunctionAttr(goCPUMultiversionAttr, strings.Join(profiles, ","))
	}
}
