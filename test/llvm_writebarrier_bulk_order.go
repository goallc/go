// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

type payload struct {
	value int
}

type block struct {
	pointers [128]*payload
}

// The scalar record is deferred to LLVM, while the move still uses wbMove.
// The bulk barrier must observe the source after the scalar store.
//
//go:noinline
func copyAfterStore(dst, src *block, p *payload) {
	src.pointers[0] = p
	*dst = *src
}

// A bulk zero followed by a scalar store also exercises the Go zero proof.
//
//go:noinline
func storeAfterClear(dst *block, p *payload) {
	*dst = block{}
	dst.pointers[0] = p
}

var source, destination = new(block), new(block)

func main() {
	for i := 1; i <= 1024; i++ {
		p := &payload{i}
		source.pointers[127] = &payload{-i}
		copyAfterStore(destination, source, p)
		runtime.GC()
		if destination.pointers[0] == nil || destination.pointers[0].value != i {
			panic("scalar store lost before bulk copy")
		}
		if destination.pointers[127] == nil || destination.pointers[127].value != -i {
			panic("bulk copy lost its last pointer")
		}
		storeAfterClear(destination, p)
		runtime.GC()
		if destination.pointers[0] == nil || destination.pointers[0].value != i {
			panic("scalar store lost after bulk clear")
		}
		for _, q := range destination.pointers[1:] {
			if q != nil {
				panic("bulk clear left a pointer")
			}
		}
	}
}
