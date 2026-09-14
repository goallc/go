// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"simd/archsimd"
)

func mul(x, y byte) (z byte) {
	for y != 0 {
		if y&1 != 0 {
			z ^= x
		}
		x = x<<1 ^ (x>>7)*0x1b
		y >>= 1
	}
	return
}

func sub(x byte) byte {
	// Inversion in GF(2^8), followed by the AES affine transformation.
	y := byte(1)
	for n := 254; n != 0; n >>= 1 {
		if n&1 != 0 {
			y = mul(y, x)
		}
		x = mul(x, x)
	}
	return y ^ bits.RotateLeft8(y, 1) ^ bits.RotateLeft8(y, 2) ^ bits.RotateLeft8(y, 3) ^ bits.RotateLeft8(y, 4) ^ 0x63
}

func subWord(x uint32) uint32 {
	return uint32(sub(byte(x))) | uint32(sub(byte(x>>8)))<<8 | uint32(sub(byte(x>>16)))<<16 | uint32(sub(byte(x>>24)))<<24
}

func invMix(x uint32) (y uint32) {
	coeff := [4]byte{14, 11, 13, 9}
	for i := 0; i < 4; i++ {
		var b byte
		for j := 0; j < 4; j++ {
			b ^= mul(byte(x>>uint(8*j)), coeff[(j-i+4)%4])
		}
		y |= uint32(b) << uint(8*i)
	}
	return
}

func schedule(key [16]byte) (w [44]uint32) {
	for i := 0; i < 4; i++ {
		w[i] = binary.LittleEndian.Uint32(key[4*i:])
	}
	rcon := byte(1)
	for i := 4; i < len(w); i++ {
		x := w[i-1]
		if i%4 == 0 {
			x = bits.RotateLeft32(subWord(x), -8) ^ uint32(rcon)
			rcon = mul(rcon, 2)
		}
		w[i] = w[i-4] ^ x
	}
	return
}

func main() {
	if os.Getenv("GOALLC_AES_CHILD") == "" {
		self, err := os.Executable()
		if err != nil {
			panic(err)
		}
		for _, flags := range []string{"", "cpu.aes=off", "cpu.avx=off", "cpu.aes=off,cpu.avx=off"} {
			cmd := exec.Command(self)
			cmd.Env = append(os.Environ(), "GOALLC_AES_CHILD=1", "GODEBUG="+flags)
			if output, err := cmd.CombinedOutput(); err != nil {
				panic(fmt.Sprintf("GODEBUG=%s: %v\n%s", flags, err, output))
			} else if len(output) != 0 {
				fmt.Printf("%s", output)
			}
		}
		return
	}
	var plain, cipher [64]byte
	var enc, dec [11][16]uint32
	for block := 0; block < 4; block++ {
		var key [16]byte
		for i := range key {
			key[i] = byte(i*17 + block*73)
			plain[16*block+i] = byte(i*31 + block*67)
		}
		c, err := aes.NewCipher(key[:])
		if err != nil {
			panic(err)
		}
		c.Encrypt(cipher[16*block:], plain[16*block:])
		w := schedule(key)
		for round := 0; round <= 10; round++ {
			for i := 0; i < 4; i++ {
				enc[round][4*block+i] = w[4*round+i]
				x := w[4*(10-round)+i]
				if round != 0 && round != 10 {
					x = invMix(x)
				}
				dec[round][4*block+i] = x
			}
		}
		for _, rcon := range []byte{0, 1, 0x1b, 0xff} {
			x := [4]uint32{w[0], w[1], w[2], w[3]}
			a, m := keyOps(x, rcon)
			var wantA, wantM [4]uint32
			if archsimd.X86.AVXAES() {
				wantA = [4]uint32{subWord(x[1]), bits.RotateLeft32(subWord(x[1]), -8) ^ uint32(rcon), subWord(x[3]), bits.RotateLeft32(subWord(x[3]), -8) ^ uint32(rcon)}
				for i := range x {
					wantM[i] = invMix(x[i])
				}
			}
			if a != wantA || m != wantM {
				panic(fmt.Sprintf("key ops: got %x/%x want %x/%x", a, m, wantA, wantM))
			}
		}
	}
	for _, test := range []struct {
		size    int
		enabled bool
		fn      func([64]byte, [11][16]uint32, bool) [64]byte
	}{
		{16, archsimd.X86.AVXAES(), rounds128},
		{32, archsimd.X86.VAES(), rounds256},
		{64, archsimd.X86.AVX512VAES(), rounds512},
	} {
		for _, decrypt := range []bool{false, true} {
			input, expected, keys := plain, cipher, enc
			if decrypt {
				input, expected, keys = cipher, plain, dec
			}
			for i := 0; i < test.size; i++ {
				input[i] ^= byte(keys[0][i/4] >> uint(8*(i%4)))
			}
			var want [64]byte
			if test.enabled {
				copy(want[:test.size], expected[:test.size])
			}
			if got := test.fn(input, keys, decrypt); got != want {
				panic(fmt.Sprintf("width=%d decrypt=%v got=%x want=%x", test.size*8, decrypt, got, want))
			}
		}
		if os.Getenv("GOALLC_SIMD_AES_TRACE") != "" {
			fmt.Printf("width=%d enabled=%v\n", test.size*8, test.enabled)
		}
	}
}
