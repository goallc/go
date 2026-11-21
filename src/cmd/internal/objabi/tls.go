// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package objabi

// DarwinAMD64TLSOffset is Apple's reserved Go g slot, measured from GS:0.
// It is a fixed platform offset, not a Mach-O thread-local-variable relocation.
// Keep it consistent with runtime/sys_darwin_amd64.s and
// runtime/cgo/gcc_darwin_amd64.c (see issue 23617).
const DarwinAMD64TLSOffset = 0x30
