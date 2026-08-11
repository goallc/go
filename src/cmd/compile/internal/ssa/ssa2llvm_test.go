// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"testing"

	"cmd/internal/obj"
)

func TestLLVMCurrentGRegister(t *testing.T) {
	for _, test := range []struct {
		name     string
		arch     string
		abi      obj.ABI
		register string
		ok       bool
	}{
		{"amd64 ABIInternal", "amd64", obj.ABIInternal, "r14", true},
		{"amd64 ABI0", "amd64", obj.ABI0, "", false},
		{"arm64 ABIInternal", "arm64", obj.ABIInternal, "x28", true},
		{"arm64 ABI0", "arm64", obj.ABI0, "x28", true},
		{"unsupported target", "riscv64", obj.ABIInternal, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			register, ok := llvmCurrentGRegister(test.arch, test.abi)
			if register != test.register || ok != test.ok {
				t.Fatalf("llvmCurrentGRegister(%q, %v) = (%q, %v), want (%q, %v)", test.arch, test.abi, register, ok, test.register, test.ok)
			}
		})
	}
}

func TestLLVMTargetCPU(t *testing.T) {
	for _, test := range []struct {
		arch    string
		goamd64 int
		want    string
	}{
		{"arm64", 1, ""},
		{"amd64", 1, "x86-64"},
		{"amd64", 2, "x86-64-v2"},
		{"amd64", 3, "x86-64-v3"},
		{"amd64", 4, "x86-64-v4"},
	} {
		t.Run(test.arch+"/"+test.want, func(t *testing.T) {
			if got := llvmTargetCPU(test.arch, test.goamd64); got != test.want {
				t.Fatalf("llvmTargetCPU(%q, %d) = %q, want %q", test.arch, test.goamd64, got, test.want)
			}
		})
	}
}
