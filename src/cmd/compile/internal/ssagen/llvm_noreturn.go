// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import "cmd/compile/internal/types"

// These APIs terminate the current goroutine or process; they cannot return to
// the following statement, even when a deferred function recovers. In
// particular, a feature check followed by t.Skip protects the rest of a test.
// Do not use the inlining heuristic's NeverReturns flag: a function that panics
// can return after recovering in its own defer.
func llvmNoReturnCall(sym *types.Sym) bool {
	switch sym.Pkg.Path {
	case "runtime":
		return sym.Name == "Goexit"
	case "os":
		return sym.Name == "Exit"
	case "testing":
		switch sym.Name {
		case "(*common).Skip", "(*common).Skipf", "(*common).SkipNow",
			"(*common).Fatal", "(*common).Fatalf", "(*common).FailNow":
			return true
		}
	}
	return false
}
