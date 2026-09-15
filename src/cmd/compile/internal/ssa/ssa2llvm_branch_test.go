// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"strings"
	"testing"

	"github.com/goallc/go-llvm"
)

func TestLLVMBranchLikelihood(t *testing.T) {
	for _, test := range []struct {
		name       string
		prediction BranchPrediction
		want       string
	}{
		{"likely", BranchLikely, "i1 true"},
		{"unlikely", BranchUnlikely, "i1 false"},
		{"unknown", BranchUnknown, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			module := GlobalCtxt.NewModule(test.name)
			builder := GlobalCtxt.NewBuilder()
			t.Cleanup(module.Dispose)
			t.Cleanup(builder.Dispose)
			previousModule := CurrentModule
			CurrentModule = module
			defer func() { CurrentModule = previousModule }()
			function := llvm.AddFunction(module, "branch", llvm.FunctionType(GlobalCtxt.VoidType(), []llvm.Type{GlobalCtxt.Int1Type()}, false))
			entryLLVM := llvm.AddBasicBlock(function, "entry")
			trueLLVM := llvm.AddBasicBlock(function, "yes")
			falseLLVM := llvm.AddBasicBlock(function, "no")
			entry := &Block{ID: 1, Kind: BlockIf, Likely: test.prediction}
			yes := &Block{ID: 2, Kind: BlockRet}
			no := &Block{ID: 3, Kind: BlockRet}
			control := &Value{ID: 1, Type: types.Types[types.TBOOL]}
			entry.Controls[0] = control
			entry.Succs = []Edge{{b: yes}, {b: no}}
			context := &LLVMFuncContext{
				BBs: map[ID]llvm.BasicBlock{entry.ID: entryLLVM, yes.ID: trueLLVM, no.ID: falseLLVM},
				Vs:  map[ID]llvm.Value{control.ID: function.Param(0)},
				LF:  function, b: builder,
			}
			context.CompileBlock(entry, nil)
			context.CompileBlock(yes, nil)
			context.CompileBlock(no, nil)
			if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
				t.Fatalf("invalid branch IR: %v\n%s", err, module.String())
			}
			branch := entryLLVM.LastInstruction()
			condition := branch.Operand(0)
			if branch.Successor(0) != trueLLVM || branch.Successor(1) != falseLLVM {
				t.Fatal("branch likelihood changed successor order")
			}
			if test.want == "" {
				if condition != function.Param(0) || strings.Contains(module.String(), "llvm.expect") {
					t.Fatalf("unknown likelihood acquired an expectation\n%s", module.String())
				}
			} else if !strings.Contains(condition.String(), "@llvm.expect.i1(i1 %0, "+test.want+")") {
				t.Fatalf("branch does not use the expected condition: %s", condition.String())
			}
			options := llvm.NewPassBuilderOptions()
			defer options.Dispose()
			options.SetVerifyEach(true)
			if err := module.RunPasses("function(lower-expect)", llvm.TargetMachine{}, options); err != nil {
				t.Fatal(err)
			}
			if branch.Operand(0) != function.Param(0) {
				t.Fatal("lowering retained the expectation call in the branch condition")
			}
			weights := branch.Metadata(GlobalCtxt.MDKindID("prof"))
			if test.want == "" {
				if !weights.IsNil() {
					t.Fatal("unknown likelihood acquired branch weights")
				}
			} else if weights.IsNil() || !strings.Contains(weights.String(), "branch_weights") {
				t.Fatalf("LLVM did not lower the expectation to branch weights\n%s", module.String())
			}
		})
	}
}
