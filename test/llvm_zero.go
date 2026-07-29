// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

func zeroByte(dst *[1]byte) {
	*dst = [1]byte{}
}

func zeroBytes(dst *[24]byte) {
	*dst = [24]byte{}
}

func zeroAligned(dst *[3]uint64) {
	*dst = [3]uint64{}
}

func main() {
	one := [1]byte{0xff}
	zeroByte(&one)

	var bytes [24]byte
	bytes[0] = 1
	bytes[11] = 2
	bytes[23] = 3
	zeroBytes(&bytes)

	aligned := [3]uint64{1, 2, 3}
	zeroAligned(&aligned)

	println(one[0], bytes[0], bytes[11], bytes[23], aligned[0], aligned[1], aligned[2])
}
