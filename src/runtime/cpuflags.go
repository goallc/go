// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/cpu"
	"unsafe"
)

// Offsets into internal/cpu records for use in assembly.
const (
	offsetX86HasAVX    = unsafe.Offsetof(cpu.X86.HasAVX)
	offsetX86HasAVX2   = unsafe.Offsetof(cpu.X86.HasAVX2)
	offsetX86HasAVX512 = unsafe.Offsetof(cpu.X86.HasAVX512) // F+CD+BW+DQ+VL
	offsetX86HasERMS   = unsafe.Offsetof(cpu.X86.HasERMS)
	offsetX86HasRDTSCP = unsafe.Offsetof(cpu.X86.HasRDTSCP)

	offsetARMHasIDIVA = unsafe.Offsetof(cpu.ARM.HasIDIVA)

	offsetMIPS64XHasMSA = unsafe.Offsetof(cpu.MIPS64X.HasMSA)

	offsetLOONG64HasLSX  = unsafe.Offsetof(cpu.Loong64.HasLSX)
	offsetLOONG64HasLASX = unsafe.Offsetof(cpu.Loong64.HasLASX)
)

var (
	// goallcCPUFeatures is the immutable CPU-feature snapshot consumed by
	// GoALLC-generated function multiversion dispatchers. Keep the bit numbers
	// in sync with src/cmd/llvmplugin/GoALLCCPUFeatures.def.
	//
	// cpuinit assigns the initialized bit together with the effective feature
	// bits after internal/cpu has applied GODEBUG overrides. Runtime startup is
	// single-threaded here, and the snapshot is immutable afterwards. A resolver
	// that runs before cpuinit must use its baseline implementation and must not
	// cache that choice.
	goallcCPUFeatures uint64

	// Set in runtime.cpuinit.
	// TODO: deprecate these; use internal/cpu directly.
	x86HasAVX    bool
	x86HasFMA    bool
	x86HasPOPCNT bool
	x86HasSSE41  bool

	armHasVFPv4 bool

	arm64HasATOMICS bool

	loong64HasLAMCAS     bool
	loong64HasLAM_BH     bool
	loong64HasDBAR_HINTS bool
	loong64HasLSX        bool

	riscv64HasZbb bool
)

const (
	goallcCPUFeatureSSE3 uint64 = 1 << iota
	goallcCPUFeatureSSSE3
	goallcCPUFeatureSSE41
	goallcCPUFeatureSSE42
	goallcCPUFeatureAVX
	goallcCPUFeatureFMA
	goallcCPUFeaturesInitialized
	goallcCPUFeaturePOPCNT
	goallcCPUFeatureARM64LSE
	goallcCPUFeatureAVX2
	goallcCPUFeatureAVX512
	goallcCPUFeatureAVX512BITALG
	goallcCPUFeatureAVX512VPOPCNTDQ
)
