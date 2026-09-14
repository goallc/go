// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"simd/archsimd"
)

func sigma0(x uint32) uint32 {
	return bits.RotateLeft32(x, -7) ^ bits.RotateLeft32(x, -18) ^ x>>3
}

func sigma1(x uint32) uint32 {
	return bits.RotateLeft32(x, -17) ^ bits.RotateLeft32(x, -19) ^ x>>10
}

func reference(x, y, z [4]uint32, phase uint8) (out [7][4]uint32) {
	// SHA-1 packs a,b,c,d and message words from high to low lanes.
	a, b, c, d, e := x[3], x[2], x[1], x[0], uint32(0)
	k := [4]uint32{0x5a827999, 0x6ed9eba1, 0x8f1bbcdc, 0xca62c1d6}[phase]
	for i := 0; i < 4; i++ {
		f := b ^ c ^ d
		if phase == 0 {
			f = b&c | ^b&d
		} else if phase == 2 {
			f = b&c | b&d | c&d
		}
		// The original e is already added to the first message word.
		a, b, c, d, e = bits.RotateLeft32(a, 5)+f+e+k+y[3-i], a, bits.RotateLeft32(b, 30), c, d
	}
	out[0] = [4]uint32{d, c, b, a}
	out[1] = y
	out[1][3] += bits.RotateLeft32(x[3], 30)
	out[2] = [4]uint32{x[0] ^ y[2], x[1] ^ y[3], x[2] ^ x[0], x[3] ^ x[1]}
	out[3][3] = bits.RotateLeft32(x[3]^y[2], 1)
	out[3][2] = bits.RotateLeft32(x[2]^y[1], 1)
	out[3][1] = bits.RotateLeft32(x[1]^y[0], 1)
	out[3][0] = bits.RotateLeft32(x[0]^out[3][3], 1)

	// SHA-256 packs h,g,d,c in x and f,e,b,a in y.
	a, b, c, d, e = y[3], y[2], x[3], x[2], y[1]
	f, g, h := y[0], x[1], x[0]
	for i := 0; i < 2; i++ {
		s0 := bits.RotateLeft32(a, -2) ^ bits.RotateLeft32(a, -13) ^ bits.RotateLeft32(a, -22)
		s1 := bits.RotateLeft32(e, -6) ^ bits.RotateLeft32(e, -11) ^ bits.RotateLeft32(e, -25)
		t1 := h + s1 + (e&f ^ ^e&g) + z[i]
		t2 := s0 + (a&b ^ a&c ^ b&c)
		a, b, c, d, e, f, g, h = t1+t2, a, b, c, d+t1, e, f, g
	}
	out[4] = [4]uint32{f, e, b, a}
	out[5] = [4]uint32{x[0] + sigma0(x[1]), x[1] + sigma0(x[2]), x[2] + sigma0(x[3]), x[3] + sigma0(y[0])}
	out[6][0] = x[0] + sigma1(y[2])
	out[6][1] = x[1] + sigma1(y[3])
	out[6][2] = x[2] + sigma1(out[6][0])
	out[6][3] = x[3] + sigma1(out[6][1])
	return
}

func main() {
	if os.Getenv("GOALLC_SHA_CHILD") == "" {
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		for _, flags := range []string{"", "cpu.sha=off", "cpu.avx=off"} {
			cmd := exec.Command(self)
			cmd.Env = append(os.Environ(), "GOALLC_SHA_CHILD=1", "GODEBUG="+flags)
			if output, err := cmd.CombinedOutput(); err != nil {
				panic(fmt.Sprintf("GODEBUG=%s: %v\n%s", flags, err, output))
			} else if len(output) != 0 {
				fmt.Printf("%s", output)
			}
		}
		return
	}
	enabled := archsimd.X86.SHA()
	state := uint32(0x31415927)
	for trial := 0; trial < 32; trial++ {
		var input [3][4]uint32
		for i := range input {
			for j := range input[i] {
				state ^= state << 13
				state ^= state >> 17
				state ^= state << 5
				input[i][j] = state
				if trial < 2 {
					input[i][j] = 0 - uint32(trial)
				}
			}
		}
		for phase := uint8(0); phase < 4; phase++ {
			var want [7][4]uint32
			if enabled {
				want = reference(input[0], input[1], input[2], phase)
			}
			if got := shaOps(input[0], input[1], input[2], phase); got != want {
				panic(fmt.Sprintf("trial=%d phase=%d got=%x want=%x", trial, phase, got, want))
			}
		}
	}
	if os.Getenv("GOALLC_SIMD_SHA_TRACE") != "" {
		fmt.Printf("SHA=%v\n", enabled)
	}
}
