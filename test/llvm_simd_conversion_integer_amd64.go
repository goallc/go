//go:build amd64

// run -goexperiment simd -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// Include extrema, alternating bits, and changing low/high lanes.
var patterns = [...]uint64{0, 1, ^uint64(0), 0x80, 0x8000, 0x80000000, 1 << 63, 0x7fffffffffffffff, 0x5555555555555555, 0xaaaaaaaaaaaaaaaa}

//go:noinline
func ExtendToInt16Int8x16(x archsimd.Int8x16) archsimd.Int16x16 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int16x16{}
	}
	return x.ExtendToInt16()
}

func checkExtendToInt16Int8x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX() {
		return
	}
	enabled := archsimd.X86.AVX2()
	for round := 0; round < 64; round++ {
		var x [16]int8
		for i := range x {
			x[i] = int8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendToInt16Int8x16(archsimd.LoadInt8x16Array(&x))
		var got [16]int16
		result.StoreArray(&got)
		for i := range got {
			var want int16
			if enabled && i < len(x) {
				want = int16(x[i])
			}
			if got[i] != want {
				panic("ExtendToInt16Int8x16")
			}
		}
	}
}

//go:noinline
func ExtendToUint16Uint8x32(x archsimd.Uint8x32) archsimd.Uint16x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x32{}
	}
	return x.ExtendToUint16()
}

func checkExtendToUint16Uint8x32() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX512() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [32]uint8
		for i := range x {
			x[i] = uint8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendToUint16Uint8x32(archsimd.LoadUint8x32Array(&x))
		var got [32]uint16
		result.StoreArray(&got)
		for i := range got {
			var want uint16
			if enabled && i < len(x) {
				want = uint16(x[i])
			}
			if got[i] != want {
				panic("ExtendToUint16Uint8x32")
			}
		}
	}
}

//go:noinline
func ExtendToInt32Int16x16(x archsimd.Int16x16) archsimd.Int32x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x16{}
	}
	return x.ExtendToInt32()
}

func checkExtendToInt32Int16x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX512() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [16]int16
		for i := range x {
			x[i] = int16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendToInt32Int16x16(archsimd.LoadInt16x16Array(&x))
		var got [16]int32
		result.StoreArray(&got)
		for i := range got {
			var want int32
			if enabled && i < len(x) {
				want = int32(x[i])
			}
			if got[i] != want {
				panic("ExtendToInt32Int16x16")
			}
		}
	}
}

//go:noinline
func ExtendToUint64Uint32x4(x archsimd.Uint32x4) archsimd.Uint64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Uint64x4{}
	}
	return x.ExtendToUint64()
}

func checkExtendToUint64Uint32x4() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX() {
		return
	}
	enabled := archsimd.X86.AVX2()
	for round := 0; round < 64; round++ {
		var x [4]uint32
		for i := range x {
			x[i] = uint32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendToUint64Uint32x4(archsimd.LoadUint32x4Array(&x))
		var got [4]uint64
		result.StoreArray(&got)
		for i := range got {
			var want uint64
			if enabled && i < len(x) {
				want = uint64(x[i])
			}
			if got[i] != want {
				panic("ExtendToUint64Uint32x4")
			}
		}
	}
}

//go:noinline
func ExtendLo4ToInt64Int8x16(x archsimd.Int8x16) archsimd.Int64x4 {
	if !archsimd.X86.AVX2() {
		return archsimd.Int64x4{}
	}
	return x.ExtendLo4ToInt64()
}

func checkExtendLo4ToInt64Int8x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX() {
		return
	}
	enabled := archsimd.X86.AVX2()
	for round := 0; round < 64; round++ {
		var x [16]int8
		for i := range x {
			x[i] = int8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo4ToInt64Int8x16(archsimd.LoadInt8x16Array(&x))
		var got [4]int64
		result.StoreArray(&got)
		for i := range got {
			var want int64
			if enabled && i < len(x) {
				want = int64(x[i])
			}
			if got[i] != want {
				panic("ExtendLo4ToInt64Int8x16")
			}
		}
	}
}

//go:noinline
func ExtendLo8ToUint64Uint8x16(x archsimd.Uint8x16) archsimd.Uint64x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint64x8{}
	}
	return x.ExtendLo8ToUint64()
}

func checkExtendLo8ToUint64Uint8x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX512() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [16]uint8
		for i := range x {
			x[i] = uint8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo8ToUint64Uint8x16(archsimd.LoadUint8x16Array(&x))
		var got [8]uint64
		result.StoreArray(&got)
		for i := range got {
			var want uint64
			if enabled && i < len(x) {
				want = uint64(x[i])
			}
			if got[i] != want {
				panic("ExtendLo8ToUint64Uint8x16")
			}
		}
	}
}

//go:noinline
func TruncToInt8Int16x32(x archsimd.Int16x32) archsimd.Int8x32 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int8x32{}
	}
	return x.TruncToInt8()
}

func checkTruncToInt8Int16x32() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX512() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [32]int16
		for i := range x {
			x[i] = int16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToInt8Int16x32(archsimd.LoadInt16x32Array(&x))
		var got [32]int8
		result.StoreArray(&got)
		for i := range got {
			var want int8
			if enabled && i < len(x) {
				want = int8(x[i])
			}
			if got[i] != want {
				panic("TruncToInt8Int16x32")
			}
		}
	}
}

//go:noinline
func TruncToUint16Uint32x8(x archsimd.Uint32x8) archsimd.Uint16x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.TruncToUint16()
}

func checkTruncToUint16Uint32x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [8]uint32
		for i := range x {
			x[i] = uint32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToUint16Uint32x8(archsimd.LoadUint32x8Array(&x))
		var got [8]uint16
		result.StoreArray(&got)
		for i := range got {
			var want uint16
			if enabled && i < len(x) {
				want = uint16(x[i])
			}
			if got[i] != want {
				panic("TruncToUint16Uint32x8")
			}
		}
	}
}

//go:noinline
func TruncToInt32Int64x8(x archsimd.Int64x8) archsimd.Int32x8 {
	if !archsimd.X86.AVX512() {
		return archsimd.Int32x8{}
	}
	return x.TruncToInt32()
}

func checkTruncToInt32Int64x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX512() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [8]int64
		for i := range x {
			x[i] = int64(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToInt32Int64x8(archsimd.LoadInt64x8Array(&x))
		var got [8]int32
		result.StoreArray(&got)
		for i := range got {
			var want int32
			if enabled && i < len(x) {
				want = int32(x[i])
			}
			if got[i] != want {
				panic("TruncToInt32Int64x8")
			}
		}
	}
}

//go:noinline
func TruncToUint8Uint64x2(x archsimd.Uint64x2) archsimd.Uint8x16 {
	if !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.TruncToUint8()
}

func checkTruncToUint8Uint64x2() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if !archsimd.X86.AVX() {
		return
	}
	enabled := archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [2]uint64
		for i := range x {
			x[i] = uint64(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToUint8Uint64x2(archsimd.LoadUint64x2Array(&x))
		var got [16]uint8
		result.StoreArray(&got)
		for i := range got {
			var want uint8
			if enabled && i < len(x) {
				want = uint8(x[i])
			}
			if got[i] != want {
				panic("TruncToUint8Uint64x2")
			}
		}
	}
}

func main() {
	checkExtendToInt16Int8x16()
	checkExtendToUint16Uint8x32()
	checkExtendToInt32Int16x16()
	checkExtendToUint64Uint32x4()
	checkExtendLo4ToInt64Int8x16()
	checkExtendLo8ToUint64Uint8x16()
	checkTruncToInt8Int16x32()
	checkTruncToUint16Uint32x8()
	checkTruncToInt32Int64x8()
	checkTruncToUint8Uint64x2()
}
