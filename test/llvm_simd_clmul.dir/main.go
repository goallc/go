// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"simd/archsimd"
)

//go:noinline
func multiply(x, y [2]uint64, selection int) (out [2]uint64, avx bool) {
	// An independent use of AVX must retain its value even when PCLMULQDQ
	// is disabled. Specializing just a composite bit would lose this state.
	avx = archsimd.X86.AVX()
	if !archsimd.X86.AVXPCLMULQDQ() {
		return
	}
	a, b := archsimd.LoadUint64x2(x[:]), archsimd.LoadUint64x2(y[:])
	var product archsimd.Uint64x2
	switch selection {
	case 0:
		product = a.CarrylessMultiplyEven(b)
	case 1:
		product = a.CarrylessMultiplyOddEven(b)
	case 2:
		product = a.CarrylessMultiplyEvenOdd(b)
	case 3:
		product = a.CarrylessMultiplyOdd(b)
	}
	product.Store(out[:])
	return
}

func reference(x, y uint64) (out [2]uint64) {
	for bit := uint(0); bit < 64; bit++ {
		if y>>bit&1 != 0 {
			out[0] ^= x << bit
			if bit != 0 {
				out[1] ^= x >> (64 - bit)
			}
		}
	}
	return
}

func main() {
	if os.Getenv("GOALLC_CLMUL_CHILD") == "" {
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		for _, flags := range []string{"", "cpu.pclmulqdq=off", "cpu.avx=off", "cpu.avx=off,cpu.pclmulqdq=off"} {
			cmd := exec.Command(self)
			cmd.Env = append(os.Environ(), "GOALLC_CLMUL_CHILD=1", "GODEBUG="+flags)
			if output, err := cmd.CombinedOutput(); err != nil {
				panic(fmt.Sprintf("GODEBUG=%s: %v\n%s", flags, err, output))
			} else if len(output) != 0 {
				fmt.Printf("%s", output)
			}
		}
		return
	}
	data := []uint64{0, 1, ^uint64(0), 1 << 63, 0xaaaaaaaaaaaaaaaa, 0x0123456789abcdef}
	enabled, avx := archsimd.X86.AVXPCLMULQDQ(), archsimd.X86.AVX()
	for _, a := range data {
		for _, b := range data {
			x, y := [2]uint64{a, ^a}, [2]uint64{b, ^b}
			for selection := 0; selection < 4; selection++ {
				var want [2]uint64
				if enabled {
					want = reference(x[selection&1], y[selection>>1])
				}
				if got, gotAVX := multiply(x, y, selection); got != want || gotAVX != avx {
					panic(fmt.Sprintf("selection=%d x=%x y=%x got=%x/%v want=%x/%v", selection, x, y, got, gotAVX, want, avx))
				}
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_CLMUL_TRACE") != "" {
		fmt.Printf("avx=%v clmul=%v products=%d\n", avx, enabled, len(data)*len(data)*4)
	}
}
