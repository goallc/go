// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goobj

import (
	"strconv"
	"testing"
)

func TestBuiltinSymbolName(t *testing.T) {
	i := BuiltinIdx("runtime.morestack", 0)
	if i < 0 {
		t.Fatal("runtime.morestack ABI0 is absent from builtin table")
	}
	want := "runtime.morestack<builtin." + strconv.Itoa(i) + ">"
	if got, ok := BuiltinSymbolName("runtime.morestack", 0); !ok || got != want {
		t.Fatalf("BuiltinSymbolName(runtime.morestack, ABI0) = (%q, %v), want (%q, true)", got, ok, want)
	}
	if got, ok := BuiltinSymbolName("runtime.notBuiltin", 1); ok || got != "runtime.notBuiltin" {
		t.Fatalf("BuiltinSymbolName(non-builtin) = (%q, %v)", got, ok)
	}
}

func TestLateBuiltins(t *testing.T) {
	want := map[string]int{
		"runtime.gcWriteBarrier1":  1,
		"runtime.gcWriteBarrier2":  1,
		"runtime.gcWriteBarrier3":  1,
		"runtime.gcWriteBarrier4":  1,
		"runtime.gcWriteBarrier5":  1,
		"runtime.gcWriteBarrier6":  1,
		"runtime.gcWriteBarrier7":  1,
		"runtime.gcWriteBarrier8":  1,
		"runtime.morestack":        0,
		"runtime.morestackc":       0,
		"runtime.morestack_noctxt": 0,
	}
	for i := 0; i < NBuiltin(); i++ {
		name, abi := BuiltinName(i)
		wantABI, late := want[name]
		if got := BuiltinIsLate(i); got != late {
			t.Errorf("BuiltinIsLate(%d /* %s */) = %v, want %v", i, name, got, late)
		}
		if late && abi != wantABI {
			t.Errorf("late builtin %s ABI = %d, want %d", name, abi, wantABI)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("late builtins absent from generated table: %v", want)
	}
	if BuiltinIsLate(-1) || BuiltinIsLate(NBuiltin()) {
		t.Fatal("out-of-range builtin index reported as late")
	}
}
