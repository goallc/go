// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"strings"
	"testing"

	"cmd/compile/internal/base"
	"cmd/internal/obj"
	"cmd/internal/objabi"

	"github.com/goallc/go-llvm"
)

func TestLLVMGenericDataFinalLayout(t *testing.T) {
	oldModule, oldLowerer, oldData := CurrentModule, currentLLVMDataLowerer, base.Ctxt.Data
	oldPtrSize := base.Ctxt.Arch.PtrSize
	module := GlobalCtxt.NewModule("generic_final_layout")
	CurrentModule = module
	base.Ctxt.Arch.PtrSize = 8
	currentLLVMDataLowerer = newLLVMDataLowerer(make(map[*obj.LSym]bool))
	t.Cleanup(func() {
		CurrentModule, currentLLVMDataLowerer, base.Ctxt.Data = oldModule, oldLowerer, oldData
		base.Ctxt.Arch.PtrSize = oldPtrSize
		module.Dispose()
	})

	// SSA can reference compiler data before dumpdata fills in its bytes and
	// relocations. Final lowering must replace that provisional declaration,
	// including its uses and the self references in the completed initializer.
	s := &obj.LSym{Name: "test.final.layout", Type: objabi.SRODATA, Size: 1, P: []byte{'A'}}
	base.Ctxt.Data = []*obj.LSym{s}
	early := llvmGoDataRef(s)
	owner := llvm.AddGlobal(module, GlobalCtxt.PointerType(0), "owner")
	owner.SetInitializer(early)
	s.Size = 24
	s.P = []byte{'A', 'B'}
	s.R = []obj.Reloc{
		{Off: 16, Siz: 8, Type: objabi.R_ADDR, Sym: s, Add: 8},
		{Off: 4, Siz: 4, Type: objabi.R_ADDROFF, Sym: s},
	}
	LowerGoObjData()
	g := module.NamedGlobal(s.Name)
	wantType := GlobalCtxt.StructType([]llvm.Type{
		llvm.ArrayType(GlobalCtxt.Int8Type(), 4), GlobalCtxt.Int32Type(),
		llvm.ArrayType(GlobalCtxt.Int8Type(), 8), GlobalCtxt.PointerType(0),
	}, true)
	if g.GlobalValueType() != wantType || g.Initializer().Type() != wantType {
		t.Fatalf("final data layout has the wrong type:\n%s", module.String())
	}
	if owner.Initializer() != g {
		t.Fatal("early reference was not replaced")
	}
	if s.R[0].Off != 16 || s.R[1].Off != 4 {
		t.Fatal("lowering reordered the source relocations")
	}
	if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
		t.Fatal(err)
	}
	ir := module.String()
	for _, want := range []string{`c"AB\00\00"`, `[8 x i8] zeroinitializer`, `ptrtoint (ptr @test.final.layout to i32)`, `getelementptr (i8, ptr @test.final.layout, i64 8)`} {
		if !strings.Contains(ir, want) {
			t.Errorf("missing %q in final initializer:\n%s", want, ir)
		}
	}
}

func TestLLVMGenericDataByteRuns(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		data []byte
		want string
	}{
		{"empty", 0, nil, `<{ [0 x i8] }> zeroinitializer`},
		{"zero", 32, nil, `<{ [32 x i8] }> zeroinitializer`},
		{"padded", 4, []byte{'A', 'B'}, `[4 x i8] c"AB\00\00"`},
		{"contents_larger_than_size", 1, []byte{'A', 'B'}, `[2 x i8] c"AB"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module := GlobalCtxt.NewModule(tc.name)
			defer module.Dispose()
			s := &obj.LSym{Name: "data", Type: objabi.SRODATA, Size: tc.size, P: tc.data}
			l := newLLVMDataLowerer(map[*obj.LSym]bool{s: true})
			l.lowered[s] = true
			g := llvm.AddGlobal(module, l.dataType(s), s.Name)
			g.SetInitializer(l.dataInitializer(s, map[*obj.LSym]llvm.Value{s: g}))
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(module.String(), tc.want) {
				t.Fatalf("want %q:\n%s", tc.want, module.String())
			}
		})
	}
}
