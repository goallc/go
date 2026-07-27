// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import "testing"

func TestLLVMDisablesConcurrentBackend(t *testing.T) {
	old := Flag
	defer func() {
		Flag = old
	}()

	Flag = CmdFlags{}
	if !concurrentFlagOk() {
		t.Fatal("default flags unexpectedly disable concurrent compilation")
	}
	Flag.EnableLLVM = true
	if concurrentFlagOk() {
		t.Fatal("-enablellvm must disable concurrent compilation")
	}
}
