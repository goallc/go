// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import "testing"

func TestRuntimeFuncName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"math.archPow", "math.archPow"},
		{"runtime.gopanic.goallc.fmv.baseline", "runtime.gopanic"},
		{"runtime.gopanic.goallc.fmv.lse", "runtime.gopanic"},
		{"math.archPow.goallc.fmv.sse41-fma", "math.archPow"},
		{"math.archPow.goallc.fmv.resolve", "math.archPow"},
		{"pkg.goallc.fmv.", "pkg.goallc.fmv."},
	}
	for _, tt := range tests {
		if got := runtimeFuncName(tt.name); got != tt.want {
			t.Errorf("runtimeFuncName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
