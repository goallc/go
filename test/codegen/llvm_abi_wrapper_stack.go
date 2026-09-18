// asmcheck

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

import _ "unsafe"

// ABI0 wrappers must retain the stack policy of restricted Go targets.
// LLVM-DAG: define weak goabi0 void @"codegen.wrapperNoSplit<ABI0>"({{.*}}) #[[NOSPLIT:[0-9]+]]
// LLVM-DAG: attributes #[[NOSPLIT]] = { {{.*}}"go-nosplit"{{.*}} }

//go:linkname wrapperNoSplit
//go:nosplit
func wrapperNoSplit(x int) int {
	return x + 1
}
