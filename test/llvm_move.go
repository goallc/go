// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "unsafe"

func moveForwardOverlap(a *[8]byte) {
	src := (*[4]byte)(unsafe.Pointer(a))
	dst := (*[4]byte)(unsafe.Add(unsafe.Pointer(a), 1))
	*dst = *src
}

func moveBackwardOverlap(a *[8]byte) {
	dst := (*[4]byte)(unsafe.Pointer(a))
	src := (*[4]byte)(unsafe.Add(unsafe.Pointer(a), 1))
	*dst = *src
}

func moveAligned(dst, src *[3]uint64) {
	*dst = *src
}

func moveLarge(dst *[128]byte, src [128]byte) {
	*dst = src
}

func main() {
	forward := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	moveForwardOverlap(&forward)

	backward := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	moveBackwardOverlap(&backward)

	src := [3]uint64{11, 22, 33}
	var dst [3]uint64
	moveAligned(&dst, &src)

	var largeSrc [128]byte
	largeSrc[0] = 0x5a
	largeSrc[63] = 0x65
	largeSrc[127] = 0xa5
	var largeDst [128]byte
	moveLarge(&largeDst, largeSrc)

	ok := forward[0] == 1 && forward[1] == 1 && forward[2] == 2 &&
		forward[3] == 3 && forward[4] == 4 && forward[5] == 6 &&
		forward[6] == 7 && forward[7] == 8 &&
		backward[0] == 2 && backward[1] == 3 && backward[2] == 4 &&
		backward[3] == 5 && backward[4] == 5 && backward[5] == 6 &&
		backward[6] == 7 && backward[7] == 8 &&
		dst[0] == 11 && dst[1] == 22 && dst[2] == 33 &&
		largeDst[0] == 0x5a && largeDst[63] == 0x65 && largeDst[127] == 0xa5
	println(ok)
}
