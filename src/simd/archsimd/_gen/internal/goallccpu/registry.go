// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package goallccpu shares GoALLC CPU profile data across SIMD generation and
// the compiler, plugin and runtime table emitters.
package goallccpu

// A feature's bit is an observable runtime predicate. Provides describes only
// instruction capabilities, NEVER additional true runtime predicates. Keep bit
// numbers append-only: separately built Go objects and runtimes share this ABI.
type feature struct {
	Name       string
	Bit        uint
	Arch       string
	Field      string
	Provides   []string
	LLVM       []string
	AMD64Level int
}

type profile struct {
	Name         string
	Feature      string
	RuntimeGuard string
	GuardImplies bool // true guarantees the feature; the converse need not hold
	SIMDAliases  []string
}

var features = []feature{
	{Name: "SSE3", Bit: 0, Arch: "amd64", Field: "HasSSE3", AMD64Level: 2},
	{Name: "SSSE3", Bit: 1, Arch: "amd64", Field: "HasSSSE3", AMD64Level: 2},
	{Name: "SSE41", Bit: 2, Arch: "amd64", Field: "HasSSE41", LLVM: []string{"sse4.1"}, AMD64Level: 2},
	{Name: "SSE42", Bit: 3, Arch: "amd64", Field: "HasSSE42", AMD64Level: 2},
	{Name: "AVX", Bit: 4, Arch: "amd64", Field: "HasAVX", LLVM: []string{"avx"}, AMD64Level: 3},
	{Name: "FMA", Bit: 5, Arch: "amd64", Field: "HasFMA", LLVM: []string{"fma"}, AMD64Level: 3},
	{Name: "INITIALIZED", Bit: 6},
	{Name: "POPCNT", Bit: 7, Arch: "amd64", Field: "HasPOPCNT", LLVM: []string{"popcnt"}, AMD64Level: 2},
	{Name: "ARM64LSE", Bit: 8, Arch: "arm64", Field: "HasATOMICS", LLVM: []string{"lse"}},
	{Name: "AVX2", Bit: 9, Arch: "amd64", Field: "HasAVX2", Provides: []string{"AVX"}, LLVM: []string{"avx2"}, AMD64Level: 3},
	{Name: "AVX512", Bit: 10, Arch: "amd64", Field: "HasAVX512", Provides: []string{"AVX2"}, LLVM: []string{"avx512f", "avx512cd", "avx512bw", "avx512dq", "avx512vl"}, AMD64Level: 4},
	{Name: "AVX512BITALG", Bit: 11, Arch: "amd64", Field: "HasAVX512BITALG", Provides: []string{"AVX512"}, LLVM: []string{"avx512bitalg"}},
	{Name: "AVX512VPOPCNTDQ", Bit: 12, Arch: "amd64", Field: "HasAVX512VPOPCNTDQ", Provides: []string{"AVX512"}, LLVM: []string{"avx512vpopcntdq"}},
	{Name: "AVX512VBMI", Bit: 13, Arch: "amd64", Field: "HasAVX512VBMI", Provides: []string{"AVX512"}, LLVM: []string{"avx512vbmi"}},
	{Name: "ARM64PMULL", Bit: 14, Arch: "arm64", Field: "HasPMULL", LLVM: []string{"aes"}},
	{Name: "AVX512VBMI2", Bit: 15, Arch: "amd64", Field: "HasAVX512VBMI2", Provides: []string{"AVX512"}, LLVM: []string{"avx512vbmi2"}},
	{Name: "AES", Bit: 16, Arch: "amd64", Field: "HasAES", LLVM: []string{"aes"}},
	{Name: "PCLMULQDQ", Bit: 17, Arch: "amd64", Field: "HasPCLMULQDQ", LLVM: []string{"pclmul"}},
	{Name: "VAES", Bit: 18, Arch: "amd64", Field: "HasVAES", Provides: []string{"AVX"}, LLVM: []string{"vaes"}},
	{Name: "VPCLMULQDQ", Bit: 19, Arch: "amd64", Field: "HasVPCLMULQDQ", Provides: []string{"AVX"}, LLVM: []string{"vpclmulqdq"}},
	{Name: "AVX512VPCLMULQDQ", Bit: 20, Arch: "amd64", Field: "HasAVX512VPCLMULQDQ", Provides: []string{"AVX512"}, LLVM: []string{"vpclmulqdq"}},
	{Name: "AVX512VAES", Bit: 21, Arch: "amd64", Field: "HasAVX512VAES", Provides: []string{"AVX512"}, LLVM: []string{"vaes"}},
	{Name: "AVX512GFNI", Bit: 22, Arch: "amd64", Field: "HasAVX512GFNI", Provides: []string{"AVX512"}, LLVM: []string{"gfni"}},
	{Name: "SHA", Bit: 23, Arch: "amd64", Field: "HasSHA", LLVM: []string{"sha"}},
}

// Preserve the existing FMV subset order, suffixes and target-feature order.
// In particular, FMA's existing capability contract is not widened to AVX in
// this refactor. Native instruction aliases below preserve the old generator
// mapping; implementing a new lowering still requires its own ISA audit.
var profiles = []profile{
	{Name: "x86.sse41", Feature: "SSE41", RuntimeGuard: "runtime.x86HasSSE41"},
	{Name: "x86.avx", Feature: "AVX", SIMDAliases: []string{"AVX"}},
	{Name: "x86.avx2", Feature: "AVX2", SIMDAliases: []string{"AVX2", "AVXVNNI"}},
	{Name: "x86.avx512", Feature: "AVX512", SIMDAliases: []string{"AVX512", "AVX512F", "AVX512CD", "AVX512BW", "AVX512DQ", "AVX512VL", "AVX512VNNI"}},
	{Name: "x86.avx512bitalg", Feature: "AVX512BITALG", SIMDAliases: []string{"AVX512BITALG"}},
	{Name: "x86.avx512vpopcntdq", Feature: "AVX512VPOPCNTDQ", SIMDAliases: []string{"AVX512VPOPCNTDQ"}},
	{Name: "x86.fma", Feature: "FMA", RuntimeGuard: "runtime.x86HasFMA", SIMDAliases: []string{"FMA"}},
	{Name: "x86.popcnt", Feature: "POPCNT", RuntimeGuard: "runtime.x86HasPOPCNT"},
	{Name: "arm64.lse", Feature: "ARM64LSE", RuntimeGuard: "runtime.arm64HasATOMICS"},
	{Name: "x86.avx512vbmi", Feature: "AVX512VBMI", SIMDAliases: []string{"AVX512VBMI"}},
	{Name: "arm64.pmull", Feature: "ARM64PMULL", SIMDAliases: []string{"PMULL"}},
	{Name: "x86.avx512vbmi2", Feature: "AVX512VBMI2", SIMDAliases: []string{"AVX512VBMI2"}},
	{Name: "x86.aes", Feature: "AES"},
	{Name: "x86.pclmulqdq", Feature: "PCLMULQDQ"},
	{Name: "x86.vaes", Feature: "VAES", SIMDAliases: []string{"VAES"}},
	// Virtual features expand using the upstream definitions. They have no
	// runtime bit or field; FMV specializes their constituent predicates.
	{Name: "x86.avxaes", Feature: "AVXAES", RuntimeGuard: "internal/runtime/maps.UseAeshash", GuardImplies: true, SIMDAliases: []string{"AVXAES"}},
	{Name: "x86.avxpclmulqdq", Feature: "AVXPCLMULQDQ", SIMDAliases: []string{"AVXPCLMULQDQ"}},
	{Name: "x86.vpclmulqdq", Feature: "VPCLMULQDQ", SIMDAliases: []string{"VPCLMULQDQ"}},
	{Name: "x86.avx512vpclmulqdq", Feature: "AVX512VPCLMULQDQ", SIMDAliases: []string{"AVX512VPCLMULQDQ"}},
	{Name: "x86.avx512vaes", Feature: "AVX512VAES", SIMDAliases: []string{"AVX512VAES"}},
	{Name: "x86.avx512gfni", Feature: "AVX512GFNI", SIMDAliases: []string{"AVX512GFNI"}},
	{Name: "x86.sha", Feature: "SHA", SIMDAliases: []string{"SHA"}},
}

var unprofiledSIMDAliases = []string{""}
