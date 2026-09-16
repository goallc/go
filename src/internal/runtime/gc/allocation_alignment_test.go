// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc_test

import (
	"internal/runtime/gc"
	"testing"
)

func TestAllocationAlignment(t *testing.T) {
	for _, tc := range []struct {
		size       uint64
		noscan     bool
		word, want uint64
	}{
		{1, true, 8, 1}, {6, true, 8, 2}, {12, true, 8, 4}, {16, true, 8, 16},
		{17, true, 8, 8}, {25, true, 8, 32}, {32, true, 8, 32}, {80, true, 8, 16},
		{128, true, 8, 128}, {136, false, 4, 8}, {136, false, 8, 16},
		{512, false, 8, 512}, {520, false, 8, 8}, {1024, true, 8, 1024},
		{32760, true, 8, 8192}, {32760, false, 8, 8}, {32768, false, 8, 8192},
		{65536, true, 8, 8192},
	} {
		if got := gc.AllocationAlignment(tc.size, tc.noscan, tc.word); got != tc.want {
			t.Errorf("size=%d noscan=%v word=%d: %d want %d", tc.size, tc.noscan, tc.word, got, tc.want)
		}
	}
}
