// asmcheck

// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package codegen

//go:noinline
func privateStringFirst(s string) byte {
	return s[0]
}

//go:noinline
func PrivateString() byte {
	// LLVM: @[[FORMAT:[.]str[^ ]*]] = private unnamed_addr constant [10 x i8] c"private=%d"
	// LLVM: call goabiinternal i8 @codegen.privateStringFirst({ ptr, i64 } { ptr @[[FORMAT]], i64 10 })
	return privateStringFirst("private=%d")
}
