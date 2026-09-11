//go:build amd64 || arm64

// run -goexperiment simd -godebug simd=+128 -llvm-package-only

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"simd"
	"simd/archsimd"
)

// The 128-bit Midway floor is AVX, while these broadcasts require AVX2.
// Exercise the existing explicit-guard FMV contract, including its scalar
// fallback, without raising the width variant's entry feature floor.

//go:noinline
func portableBroadcastInt8(x int8, out []int8) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastInt8s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastInt16(x int16, out []int16) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastInt16s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastInt32(x int32, out []int32) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastInt32s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastInt64(x int64, out []int64) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastInt64s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastUint8(x uint8, out []uint8) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastUint8s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastUint16(x uint16, out []uint16) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastUint16s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastUint32(x uint32, out []uint32) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastUint32s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastUint64(x uint64, out []uint64) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastUint64s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastFloat32(x float32, out []float32) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastFloat32s(x).Store(out)
	return true
}

//go:noinline
func portableBroadcastFloat64(x float64, out []float64) bool {
	if runtime.GOARCH == "amd64" && !archsimd.X86.AVX2() {
		for i := range out {
			out[i] = x
		}
		return false
	}
	simd.BroadcastFloat64s(x).Store(out)
	return true
}

type number interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64
}

func raw[T number](x T, bits int) uint64 {
	switch v := any(x).(type) {
	case float32:
		return uint64(math.Float32bits(v))
	case float64:
		return math.Float64bits(v)
	}
	return uint64(x) & (^uint64(0) >> (64 - bits))
}

var broadcastCalls, fallbackCalls int

func check[T number](bits int, inputs []T, broadcast func(T, []T) bool) {
	got := make([]T, simd.VectorBitSize()/bits)
	for _, x := range inputs {
		for i := range got {
			got[i] = T(99)
		}
		usedBroadcast := broadcast(x, got)
		wantBroadcast := runtime.GOARCH != "amd64" || archsimd.X86.AVX2()
		if usedBroadcast != wantBroadcast {
			panic("portable broadcast selected the wrong guarded branch")
		}
		if usedBroadcast {
			broadcastCalls++
		} else {
			fallbackCalls++
		}
		for i, y := range got {
			if raw(y, bits) != raw(x, bits) {
				panic(fmt.Sprintf("broadcast bits=%d lane=%d got=%x want=%x", bits, i, raw(y, bits), raw(x, bits)))
			}
		}
	}
}

func main() {
	check(8, []int8{0, 1, -1, -1 << 7, 1<<7 - 1}, portableBroadcastInt8)
	check(16, []int16{0, 1, -1, -1 << 15, 1<<15 - 1}, portableBroadcastInt16)
	check(32, []int32{0, 1, -1, -1 << 31, 1<<31 - 1}, portableBroadcastInt32)
	check(64, []int64{0, 1, -1, -1 << 63, 1<<63 - 1}, portableBroadcastInt64)
	check(8, []uint8{0, 1, ^uint8(0), 1 << 7}, portableBroadcastUint8)
	check(16, []uint16{0, 1, ^uint16(0), 1 << 15}, portableBroadcastUint16)
	check(32, []uint32{0, 1, ^uint32(0), 1 << 31}, portableBroadcastUint32)
	check(64, []uint64{0, 1, ^uint64(0), 1 << 63}, portableBroadcastUint64)
	check(32, []float32{0, math.Float32frombits(0x80000000), math.Float32frombits(0x7fc00123), math.Float32frombits(0x7f800001), math.SmallestNonzeroFloat32, -1.5}, portableBroadcastFloat32)
	check(64, []float64{0, math.Float64frombits(0x8000000000000000), math.Float64frombits(0x7ff8000000000123), math.Float64frombits(0x7ff0000000000001), math.SmallestNonzeroFloat64, -1.5}, portableBroadcastFloat64)
	if os.Getenv("GOALLC_SIMD_SHUFFLE_TRACE") == "1" {
		fmt.Printf("portable broadcast: types=10 bits=%d emulated=%t broadcast=%d fallback=%d\n", simd.VectorBitSize(), simd.Emulated(), broadcastCalls, fallbackCalls)
	}
}
