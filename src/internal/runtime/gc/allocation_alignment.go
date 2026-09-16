// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

// AllocationAlignment returns the normal allocator's guaranteed alignment of
// the user pointer. ptrSize is explicit because the compiler may cross-compile.
// Tiny suballocations and inline malloc headers do not retain slot alignment.
// The sbrk debug allocator must provide at least this same guarantee.
func AllocationAlignment(size uint64, noscan bool, ptrSize uint64) uint64 {
	if size == 0 {
		return ptrSize
	}
	if noscan && size < TinySize {
		return min(size&-size, 8)
	}
	if size > MaxSmallSize-MallocHeaderSize {
		return PageSize
	}
	if !noscan && size > ptrSize*ptrSize*8 {
		// Slots are at least 8-byte aligned; the 8-byte header preserves
		// that guarantee but prevents any stronger user-pointer alignment.
		return MallocHeaderSize
	}
	if size <= SmallSizeMax-8 {
		return uint64(SizeClassToAlignment[SizeToSizeClass8[(size+SmallSizeDiv-1)/SmallSizeDiv]])
	}
	return uint64(SizeClassToAlignment[SizeToSizeClass128[(size-SmallSizeMax+LargeSizeDiv-1)/LargeSizeDiv]])
}
