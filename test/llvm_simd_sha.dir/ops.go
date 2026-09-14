// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd"

// The array ABI must remain callable without SHA support.
//
//go:noinline
func shaOps(x, y, z [4]uint32, phase uint8) (out [7][4]uint32) {
	if !archsimd.X86.SHA() {
		return
	}
	a := archsimd.LoadUint32x4(x[:])
	b := archsimd.LoadUint32x4(y[:])
	c := archsimd.LoadUint32x4(z[:])
	a.SHA1FourRounds(phase, b).Store(out[0][:])
	a.SHA1NextE(b).Store(out[1][:])
	a.SHA1Message1(b).Store(out[2][:])
	a.SHA1Message2(b).Store(out[3][:])
	a.SHA256TwoRounds(b, c).Store(out[4][:])
	a.SHA256Message1(b).Store(out[5][:])
	a.SHA256Message2(b).Store(out[6][:])
	return
}
