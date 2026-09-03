// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

const wordMask = 63

type bitSet struct {
	length uint
	set    []uint64
}

func bitCapacity() uint {
	return ^uint(0)
}

// Keep this nontrivial constant expression. Folding it used to leave an
// unused memory value beside the slice-field store chain in safeSet.
func wordsNeeded(i uint) int {
	if i > bitCapacity()-wordMask {
		return int(bitCapacity() >> 6)
	}
	return int((i + wordMask) >> 6)
}

//go:noinline
func (b *bitSet) safeSet() []uint64 {
	if b.set == nil {
		b.set = make([]uint64, wordsNeeded(0))
	}
	return b.set
}

func main() {
	var b bitSet
	set := b.safeSet()
	if set == nil || b.set == nil || len(set) != 0 {
		panic("safeSet did not install an empty non-nil slice")
	}
}
