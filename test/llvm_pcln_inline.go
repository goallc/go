// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"runtime/debug"
	"strings"
)

func llvmPCLNInner() {
	llvmPCLNCapture()
}

func llvmPCLNMiddle() {
	llvmPCLNInner()
}

func llvmPCLNOuter() {
	func() {
		llvmPCLNMiddle()
	}()
}

//go:noinline
func llvmPCLNCapture() {
	panic("pcln-inline")
}

func main() {
	defer func() {
		if value := recover(); value != "pcln-inline" {
			panic("unexpected recovered value")
		}
		trace := string(debug.Stack())
		frames := []string{
			"main.llvmPCLNCapture()",
			"main.llvmPCLNInner(...)",
			"main.llvmPCLNMiddle(...)",
			"main.llvmPCLNOuter.func1",
			"main.llvmPCLNOuter(...)",
			"main.main()",
		}
		last := -1
		for _, frame := range frames {
			index := strings.Index(trace, frame)
			if index < 0 || index <= last {
				panic("missing or out-of-order inline frame " + frame + ":\n" + trace)
			}
			last = index
		}
		if strings.Count(trace, "llvm_pcln_inline.go:") < len(frames) {
			panic("inline traceback lost source locations:\n" + trace)
		}
	}()
	llvmPCLNOuter()
}
