// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An unused linknamed variable must not hide the target's linkname restriction.

package main

import "unsafe"

//go:linkname newcoro runtime.newcoro
var newcoro unsafe.Pointer

func main() {}
