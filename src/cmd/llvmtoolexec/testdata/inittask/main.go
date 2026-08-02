// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var initOrder []int

func main() {
	if len(initOrder) != 2 || initOrder[0] != 1 || initOrder[1] != 2 {
		panic("package initialization order is incorrect")
	}
}
