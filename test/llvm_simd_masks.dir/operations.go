// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

type maskBox struct {
	mask archsimd.Mask32x4
	tag  uint64
}

//go:noinline
func maskResult(bits uint8) maskBox {
	if archsimd.X86.AVX512() {
		return maskBox{archsimd.Mask32x4FromBits(bits), 0x12345678}
	}
	return maskBox{}
}

//go:noinline
func zeros(x []byte) (bool, bool) {
	if archsimd.X86.AVX() {
		a, b := archsimd.LoadUint8x16(x), archsimd.LoadUint8x32(x)
		// Clearing hardware upper bits must preserve live Go SIMD values.
		archsimd.ClearAVXUpperBits()
		return a.IsZero(), b.IsZero()
	}
	return false, false
}

//go:noinline
func int8x16(x, y []int8, bits uint64, out [3][]int8) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask8x16FromBits(uint16(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>48) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt8x16(x), archsimd.LoadInt8x16(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int8x32(x, y []int8, bits uint64, out [3][]int8) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask8x32FromBits(uint32(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>32) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt8x32(x), archsimd.LoadInt8x32(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int8x64(x, y []int8, bits uint64, out [3][]int8) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask8x64FromBits(uint64(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>0) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt8x64(x), archsimd.LoadInt8x64(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int16x8(x, y []int16, bits uint64, out [3][]int16) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask16x8FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>56) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt16x8(x), archsimd.LoadInt16x8(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int16x16(x, y []int16, bits uint64, out [3][]int16) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask16x16FromBits(uint16(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>48) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt16x16(x), archsimd.LoadInt16x16(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int16x32(x, y []int16, bits uint64, out [3][]int16) {
	if archsimd.X86.AVX512VBMI2() {
		mask := archsimd.Mask16x32FromBits(uint32(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>32) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt16x32(x), archsimd.LoadInt16x32(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int32x4(x, y []int32, bits uint64, out [3][]int32) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x4FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>60) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt32x4(x), archsimd.LoadInt32x4(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int32x8(x, y []int32, bits uint64, out [3][]int32) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x8FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>56) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt32x8(x), archsimd.LoadInt32x8(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int32x16(x, y []int32, bits uint64, out [3][]int32) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x16FromBits(uint16(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>48) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt32x16(x), archsimd.LoadInt32x16(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float32x4(x, y []float32, bits uint64, out [3][]float32) {
	if archsimd.X86.AVX() {
		checkNaN(x[:4], uint64(archsimd.LoadFloat32x4(x).IsNaN().ToBits()))
	}
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x4FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>60) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat32x4(x), archsimd.LoadFloat32x4(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float32x8(x, y []float32, bits uint64, out [3][]float32) {
	if archsimd.X86.AVX() {
		checkNaN(x[:8], uint64(archsimd.LoadFloat32x8(x).IsNaN().ToBits()))
	}
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x8FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>56) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat32x8(x), archsimd.LoadFloat32x8(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float32x16(x, y []float32, bits uint64, out [3][]float32) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask32x16FromBits(uint16(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>48) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat32x16(x), archsimd.LoadFloat32x16(y)
		checkNaN(x[:16], uint64(v.IsNaN().ToBits()))
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int64x2(x, y []int64, bits uint64, out [3][]int64) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x2FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>62) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt64x2(x), archsimd.LoadInt64x2(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int64x4(x, y []int64, bits uint64, out [3][]int64) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x4FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>60) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt64x4(x), archsimd.LoadInt64x4(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func int64x8(x, y []int64, bits uint64, out [3][]int64) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x8FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>56) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadInt64x8(x), archsimd.LoadInt64x8(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float64x2(x, y []float64, bits uint64, out [3][]float64) {
	if archsimd.X86.AVX() {
		checkNaN(x[:2], uint64(archsimd.LoadFloat64x2(x).IsNaN().ToBits()))
	}
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x2FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>62) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat64x2(x), archsimd.LoadFloat64x2(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float64x4(x, y []float64, bits uint64, out [3][]float64) {
	if archsimd.X86.AVX() {
		checkNaN(x[:4], uint64(archsimd.LoadFloat64x4(x).IsNaN().ToBits()))
	}
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x4FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>60) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat64x4(x), archsimd.LoadFloat64x4(y)
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}

//go:noinline
func float64x8(x, y []float64, bits uint64, out [3][]float64) {
	if archsimd.X86.AVX512() {
		mask := archsimd.Mask64x8FromBits(uint8(bits))
		if uint64(mask.ToBits()) != bits&(^uint64(0)>>56) {
			panic("mask bitmap round trip")
		}
		v, other := archsimd.LoadFloat64x8(x), archsimd.LoadFloat64x8(y)
		checkNaN(x[:8], uint64(v.IsNaN().ToBits()))
		v.Compress(mask).Store(out[0])
		v.Expand(mask).Store(out[1])
		v.IfElse(mask, other).Store(out[2])
	}
}
