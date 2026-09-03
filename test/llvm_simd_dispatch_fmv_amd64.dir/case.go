// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

type cpuSupport uint8

const (
	supportAlways cpuSupport = iota
	supportAVX
	supportAVX2
	supportAVX512
)

func (support cpuSupport) available() bool {
	switch support {
	case supportAlways:
		return true
	case supportAVX:
		return archsimd.X86.AVX()
	case supportAVX2:
		return archsimd.X86.AVX2()
	case supportAVX512:
		return archsimd.X86.AVX512()
	}
	panic("unknown CPU support requirement")
}

func (support cpuSupport) String() string {
	switch support {
	case supportAlways:
		return "baseline"
	case supportAVX:
		return "AVX"
	case supportAVX2:
		return "AVX2"
	case supportAVX512:
		return "AVX-512"
	}
	return "unknown CPU support"
}

type dispatchFMVCase struct {
	order      int
	name       string
	godebug    string
	width      int
	emulated   bool
	usedAVX2   bool
	usedAVX512 bool
	support    cpuSupport
}

var dispatchFMVCases []dispatchFMVCase

func registerDispatchFMVCase(test dispatchFMVCase) {
	dispatchFMVCases = append(dispatchFMVCases, test)
}
