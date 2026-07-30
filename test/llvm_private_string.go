// run

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

//go:noinline
func privateStringFirst(s string) byte {
	return s[0]
}

func main() {
	if got := privateStringFirst("private=%d"); got != 'p' {
		panic("private string mismatch")
	}
}
