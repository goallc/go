// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

func init() {
	// runtime must be initialized before this package's task is run.
	runtime.Gosched()
	initOrder = append(initOrder, 1)
}
