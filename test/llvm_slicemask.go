// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

func tail(a []byte, i int) []byte {
	return a[i:]
}

func main() {
	storage := [4]byte{1, 2, 3, 4}
	full := storage[:]
	atStart := tail(full, 0)
	atMiddle := tail(full, 2)
	atEnd := tail(full, len(full))

	ok := len(atStart) == 4 && cap(atStart) == 4 && atStart[0] == 1 &&
		len(atMiddle) == 2 && cap(atMiddle) == 2 && atMiddle[0] == 3 &&
		len(atEnd) == 0 && cap(atEnd) == 0
	println(ok)
}
