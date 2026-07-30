// compile

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is compiled from the same source by the native Go and GoALLC
// alloca-statepoint metadata test.
package p

type pointerLocal struct {
	first  *int
	scalar uintptr
	second *int
	tail   [2]*int
}

var (
	globalFirst  *int
	globalSecond *int
	globalThird  *int
	globalFourth *int
)

//go:noescape
func safepoint()

// mutateLocal is intentionally opaque to both compilers. A real callee may
// update the pointer fields while the call is a safepoint, so GoALLC must use
// the original alloca slots as the relocation homes.
//
//go:noescape
func mutateLocal(value *pointerLocal, branch bool)

// localAcrossSafepoints deliberately takes the address of a pointer-containing
// local. Its pointer leaves must remain rooted in that original stack object
// across both safepoint and mutateLocal. The latter may update the object, so a
// relocated value from a separate spill slot must not be stored over its write.
//
//go:noinline
func localAcrossSafepoints(branch bool, rounds int) uintptr {
	var value pointerLocal
	value.first = globalFirst
	value.scalar = 41
	value.second = globalSecond
	value.tail[0] = globalThird
	value.tail[1] = globalFourth

	mutateLocal(&value, branch)
	safepoint()
	for i := 0; i < rounds; i++ {
		if i&1 == 0 {
			safepoint()
		} else {
			mutateLocal(&value, !branch)
		}
	}
	return value.scalar
}
