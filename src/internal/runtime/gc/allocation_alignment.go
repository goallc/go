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
	header := uint64(0)
	if !noscan && size > ptrSize*ptrSize*8 {
		header = MallocHeaderSize
	}
	slotSize := size + header
	var class uint8
	if slotSize <= SmallSizeMax-8 {
		class = SizeToSizeClass8[(slotSize+SmallSizeDiv-1)/SmallSizeDiv]
	} else {
		class = SizeToSizeClass128[(slotSize-SmallSizeMax+LargeSizeDiv-1)/LargeSizeDiv]
	}
	slotSize = uint64(SizeClassToSize[class])
	alignment := min(slotSize&-slotSize, PageSize)
	if header != 0 {
		alignment = min(alignment, header&-header)
	}
	return alignment
}
