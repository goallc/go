// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type llvmVarDefResult struct {
	status   *int
	changes  [2]func()
	added    []any
	removed  []any
	assumed  *int
	inFlight []any
}

type llvmVarDefMap map[string]llvmVarDefResult

func (values llvmVarDefMap) zero(name string) llvmVarDefResult {
	if values == nil {
		return llvmVarDefResult{}
	}
	return values[name]
}

//go:noinline
func poisonLLVMVarDefRegister() *int {
	value := 1
	return &value
}

func main() {
	var values llvmVarDefMap
	_ = poisonLLVMVarDefRegister()
	got := (&values).zero("missing")
	if got.status != nil {
		panic("zero aggregate contains a non-nil pointer")
	}
}
