//go:build amd64 || arm64

// run -goexperiment simd

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime"
	"simd/archsimd"
)

// Include extrema, alternating bits, and changing low/high lanes.
var patterns = [...]uint64{0, 1, ^uint64(0), 0x80, 0x8000, 0x80000000, 1 << 63, 0x7fffffffffffffff, 0x5555555555555555, 0xaaaaaaaaaaaaaaaa}

//go:noinline
func ExtendLo8ToInt16Int8x16(x archsimd.Int8x16) archsimd.Int16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Int16x8{}
	}
	return x.ExtendLo8ToInt16()
}

func checkExtendLo8ToInt16Int8x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [16]int8
		for i := range x {
			x[i] = int8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo8ToInt16Int8x16(archsimd.LoadInt8x16Array(&x))
		var got [8]int16
		result.StoreArray(&got)
		for i := range got {
			var want int16
			if enabled && i < len(x) {
				want = int16(x[i])
			}
			if got[i] != want {
				panic("ExtendLo8ToInt16Int8x16")
			}
		}
	}
}

//go:noinline
func ExtendLo8ToUint16Uint8x16(x archsimd.Uint8x16) archsimd.Uint16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Uint16x8{}
	}
	return x.ExtendLo8ToUint16()
}

func checkExtendLo8ToUint16Uint8x16() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [16]uint8
		for i := range x {
			x[i] = uint8(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo8ToUint16Uint8x16(archsimd.LoadUint8x16Array(&x))
		var got [8]uint16
		result.StoreArray(&got)
		for i := range got {
			var want uint16
			if enabled && i < len(x) {
				want = uint16(x[i])
			}
			if got[i] != want {
				panic("ExtendLo8ToUint16Uint8x16")
			}
		}
	}
}

//go:noinline
func ExtendLo4ToInt32Int16x8(x archsimd.Int16x8) archsimd.Int32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Int32x4{}
	}
	return x.ExtendLo4ToInt32()
}

func checkExtendLo4ToInt32Int16x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [8]int16
		for i := range x {
			x[i] = int16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo4ToInt32Int16x8(archsimd.LoadInt16x8Array(&x))
		var got [4]int32
		result.StoreArray(&got)
		for i := range got {
			var want int32
			if enabled && i < len(x) {
				want = int32(x[i])
			}
			if got[i] != want {
				panic("ExtendLo4ToInt32Int16x8")
			}
		}
	}
}

//go:noinline
func ExtendLo4ToUint32Uint16x8(x archsimd.Uint16x8) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Uint32x4{}
	}
	return x.ExtendLo4ToUint32()
}

func checkExtendLo4ToUint32Uint16x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [8]uint16
		for i := range x {
			x[i] = uint16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo4ToUint32Uint16x8(archsimd.LoadUint16x8Array(&x))
		var got [4]uint32
		result.StoreArray(&got)
		for i := range got {
			var want uint32
			if enabled && i < len(x) {
				want = uint32(x[i])
			}
			if got[i] != want {
				panic("ExtendLo4ToUint32Uint16x8")
			}
		}
	}
}

//go:noinline
func ExtendLo2ToInt64Int32x4(x archsimd.Int32x4) archsimd.Int64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Int64x2{}
	}
	return x.ExtendLo2ToInt64()
}

func checkExtendLo2ToInt64Int32x4() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [4]int32
		for i := range x {
			x[i] = int32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo2ToInt64Int32x4(archsimd.LoadInt32x4Array(&x))
		var got [2]int64
		result.StoreArray(&got)
		for i := range got {
			var want int64
			if enabled && i < len(x) {
				want = int64(x[i])
			}
			if got[i] != want {
				panic("ExtendLo2ToInt64Int32x4")
			}
		}
	}
}

//go:noinline
func ExtendLo2ToUint64Uint32x4(x archsimd.Uint32x4) archsimd.Uint64x2 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return archsimd.Uint64x2{}
	}
	return x.ExtendLo2ToUint64()
}

func checkExtendLo2ToUint64Uint32x4() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX()
	for round := 0; round < 64; round++ {
		var x [4]uint32
		for i := range x {
			x[i] = uint32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := ExtendLo2ToUint64Uint32x4(archsimd.LoadUint32x4Array(&x))
		var got [2]uint64
		result.StoreArray(&got)
		for i := range got {
			var want uint64
			if enabled && i < len(x) {
				want = uint64(x[i])
			}
			if got[i] != want {
				panic("ExtendLo2ToUint64Uint32x4")
			}
		}
	}
}

//go:noinline
func TruncToInt8Int16x8(x archsimd.Int16x8) archsimd.Int8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int8x16{}
	}
	return x.TruncToInt8()
}

func checkTruncToInt8Int16x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [8]int16
		for i := range x {
			x[i] = int16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToInt8Int16x8(archsimd.LoadInt16x8Array(&x))
		var got [16]int8
		result.StoreArray(&got)
		for i := range got {
			var want int8
			if enabled && i < len(x) {
				want = int8(x[i])
			}
			if got[i] != want {
				panic("TruncToInt8Int16x8")
			}
		}
	}
}

//go:noinline
func TruncToUint8Uint16x8(x archsimd.Uint16x8) archsimd.Uint8x16 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint8x16{}
	}
	return x.TruncToUint8()
}

func checkTruncToUint8Uint16x8() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [8]uint16
		for i := range x {
			x[i] = uint16(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToUint8Uint16x8(archsimd.LoadUint16x8Array(&x))
		var got [16]uint8
		result.StoreArray(&got)
		for i := range got {
			var want uint8
			if enabled && i < len(x) {
				want = uint8(x[i])
			}
			if got[i] != want {
				panic("TruncToUint8Uint16x8")
			}
		}
	}
}

//go:noinline
func TruncToInt16Int32x4(x archsimd.Int32x4) archsimd.Int16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int16x8{}
	}
	return x.TruncToInt16()
}

func checkTruncToInt16Int32x4() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [4]int32
		for i := range x {
			x[i] = int32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToInt16Int32x4(archsimd.LoadInt32x4Array(&x))
		var got [8]int16
		result.StoreArray(&got)
		for i := range got {
			var want int16
			if enabled && i < len(x) {
				want = int16(x[i])
			}
			if got[i] != want {
				panic("TruncToInt16Int32x4")
			}
		}
	}
}

//go:noinline
func TruncToUint16Uint32x4(x archsimd.Uint32x4) archsimd.Uint16x8 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint16x8{}
	}
	return x.TruncToUint16()
}

func checkTruncToUint16Uint32x4() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [4]uint32
		for i := range x {
			x[i] = uint32(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToUint16Uint32x4(archsimd.LoadUint32x4Array(&x))
		var got [8]uint16
		result.StoreArray(&got)
		for i := range got {
			var want uint16
			if enabled && i < len(x) {
				want = uint16(x[i])
			}
			if got[i] != want {
				panic("TruncToUint16Uint32x4")
			}
		}
	}
}

//go:noinline
func TruncToInt32Int64x2(x archsimd.Int64x2) archsimd.Int32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Int32x4{}
	}
	return x.TruncToInt32()
}

func checkTruncToInt32Int64x2() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [2]int64
		for i := range x {
			x[i] = int64(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToInt32Int64x2(archsimd.LoadInt64x2Array(&x))
		var got [4]int32
		result.StoreArray(&got)
		for i := range got {
			var want int32
			if enabled && i < len(x) {
				want = int32(x[i])
			}
			if got[i] != want {
				panic("TruncToInt32Int64x2")
			}
		}
	}
}

//go:noinline
func TruncToUint32Uint64x2(x archsimd.Uint64x2) archsimd.Uint32x4 {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX512() {
		return archsimd.Uint32x4{}
	}
	return x.TruncToUint32()
}

func checkTruncToUint32Uint64x2() {
	// Call feature-disabled variants when the fixed-width ABI is supported.
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX() {
		return
	}
	enabled := runtime.GOARCH != "amd64" || archsimd.X86.AVX512()
	for round := 0; round < 64; round++ {
		var x [2]uint64
		for i := range x {
			x[i] = uint64(patterns[(round+i)%len(patterns)] + uint64(round/len(patterns)))
		}
		result := TruncToUint32Uint64x2(archsimd.LoadUint64x2Array(&x))
		var got [4]uint32
		result.StoreArray(&got)
		for i := range got {
			var want uint32
			if enabled && i < len(x) {
				want = uint32(x[i])
			}
			if got[i] != want {
				panic("TruncToUint32Uint64x2")
			}
		}
	}
}

func main() {
	checkExtendLo8ToInt16Int8x16()
	checkExtendLo8ToUint16Uint8x16()
	checkExtendLo4ToInt32Int16x8()
	checkExtendLo4ToUint32Uint16x8()
	checkExtendLo2ToInt64Int32x4()
	checkExtendLo2ToUint64Uint32x4()
	checkTruncToInt8Int16x8()
	checkTruncToUint8Uint16x8()
	checkTruncToInt16Int32x4()
	checkTruncToUint16Uint32x4()
	checkTruncToInt32Int64x2()
	checkTruncToUint32Uint64x2()
}
