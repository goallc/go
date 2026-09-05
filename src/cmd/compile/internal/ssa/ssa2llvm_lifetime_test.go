// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"testing"
)

func TestLLVMLifetimeCyclicBlocks(t *testing.T) {
	c := testConfig(t)
	f := c.Fun("entry",
		Bloc("entry",
			Valu("mem", OpInitMem, types.TypeMem, 0, nil),
			Valu("cond", OpConstBool, c.config.Types.Bool, 1, nil),
			If("cond", "left", "right")),
		Bloc("left", If("cond", "right", "exit")),
		Bloc("right", If("cond", "left", "self")),
		Bloc("self", If("cond", "self", "exit")),
		Bloc("exit", Exit("mem")))
	cyclic := llvmLifetimeCyclicBlocks(f.f)
	for name, b := range f.blocks {
		want := name == "left" || name == "right" || name == "self"
		if cyclic[b.ID] != want {
			t.Errorf("block %s: cyclic = %v, want %v", name, cyclic[b.ID], want)
		}
	}
}

func TestLLVMLifetimeCompleteInitialization(t *testing.T) {
	c := testConfig(t)
	typ := c.config.Types.Int64
	local := c.Temp(typ)
	f := c.Fun("entry",
		Bloc("entry",
			Valu("mem", OpInitMem, types.TypeMem, 0, nil),
			Valu("sp", OpSP, c.config.Types.Uintptr, 0, nil),
			Valu("def", OpVarDef, types.TypeMem, 0, local, "mem"),
			Valu("addr", OpLocalAddr, typ.PtrTo(), 0, local, "sp", "def"),
			Valu("zero", OpZero, types.TypeMem, 8, typ, "addr", "def"),
			Valu("load", OpLoad, typ, 0, nil, "addr", "zero"),
			Exit("zero")))
	addr, zero, load := f.values["addr"], f.values["zero"], f.values["load"]
	if !llvmLifetimeInitializedInBlock(addr, []*Value{addr, zero, load}) {
		t.Fatal("complete initialization was not recognized")
	}
	if llvmLifetimeInitializedInBlock(addr, []*Value{addr, load, zero}) {
		t.Fatal("load before initialization must preserve the old lifetime")
	}
	zero.AuxInt = 4
	if llvmLifetimeInitializedInBlock(addr, []*Value{addr, zero}) {
		t.Fatal("partial initialization must preserve the old lifetime")
	}
	value := addr.Block.NewValue0(addr.Pos, OpConst64, typ)
	store := addr.Block.NewValue3(addr.Pos, OpStore, types.TypeMem, addr, value, f.values["def"])
	if !llvmLifetimeInitializedInBlock(addr, []*Value{addr, value, store}) {
		t.Fatal("complete store initialization was not recognized")
	}
	value.Type = c.config.Types.Int32
	if llvmLifetimeInitializedInBlock(addr, []*Value{addr, value, store}) {
		t.Fatal("partial store must preserve the old lifetime")
	}
}
