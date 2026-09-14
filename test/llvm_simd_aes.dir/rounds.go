// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// The scalar/array ABI must remain callable on CPUs without AES or AVX.
//
//go:noinline
func rounds128(input [64]byte, keys [11][16]uint32, decrypt bool) (out [64]byte) {
	if !archsimd.X86.AVXAES() {
		return
	}
	x := archsimd.LoadUint8x16(input[:16])
	for r := 1; r < 10; r++ {
		k := archsimd.LoadUint32x4(keys[r][:4])
		if decrypt {
			x = x.AESDecryptOneRound(k)
		} else {
			x = x.AESEncryptOneRound(k)
		}
	}
	k := archsimd.LoadUint32x4(keys[10][:4])
	if decrypt {
		x = x.AESDecryptLastRound(k)
	} else {
		x = x.AESEncryptLastRound(k)
	}
	x.Store(out[:16])
	return
}

//go:noinline
func rounds256(input [64]byte, keys [11][16]uint32, decrypt bool) (out [64]byte) {
	if !archsimd.X86.VAES() {
		return
	}
	x := archsimd.LoadUint8x32(input[:32])
	for r := 1; r < 10; r++ {
		k := archsimd.LoadUint32x8(keys[r][:8])
		if decrypt {
			x = x.AESDecryptOneRound(k)
		} else {
			x = x.AESEncryptOneRound(k)
		}
	}
	k := archsimd.LoadUint32x8(keys[10][:8])
	if decrypt {
		x = x.AESDecryptLastRound(k)
	} else {
		x = x.AESEncryptLastRound(k)
	}
	x.Store(out[:32])
	return
}

//go:noinline
func rounds512(input [64]byte, keys [11][16]uint32, decrypt bool) (out [64]byte) {
	if !archsimd.X86.AVX512VAES() {
		return
	}
	x := archsimd.LoadUint8x64(input[:])
	for r := 1; r < 10; r++ {
		k := archsimd.LoadUint32x16(keys[r][:])
		if decrypt {
			x = x.AESDecryptOneRound(k)
		} else {
			x = x.AESEncryptOneRound(k)
		}
	}
	k := archsimd.LoadUint32x16(keys[10][:])
	if decrypt {
		x = x.AESDecryptLastRound(k)
	} else {
		x = x.AESEncryptLastRound(k)
	}
	x.Store(out[:])
	return
}

//go:noinline
func keyOps(x [4]uint32, rcon uint8) (assist, mix [4]uint32) {
	if archsimd.X86.AVXAES() {
		v := archsimd.LoadUint32x4(x[:])
		v.AESRoundKeyGenAssist(rcon).Store(assist[:])
		v.AESInvMixColumns().Store(mix[:])
	}
	return
}
