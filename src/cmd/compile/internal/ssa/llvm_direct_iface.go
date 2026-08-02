// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
)

// llvmDirectIfacePass exposes the single pointer word of direct-interface
// aggregates before LLVM emission. The normal Go pipeline performs these
// rewrites later, after LLVM has already consumed the generic SSA.
func llvmDirectIfacePass(f *Func) {
	if !base.Flag.EnableLLVM {
		return
	}

	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op != OpIMake || len(v.Args) != 2 {
				continue
			}
			// Optimization may fold the itab word of a nil interface to a
			// uintptr-typed ConstNil. LLVM represents that physical interface
			// word as a pointer, so keep its Go SSA representation pointer-shaped
			// before aggregate construction.
			itab := v.Args[0]
			if itab.Op == OpConstNil && !itab.Type.IsPtrShaped() {
				v.SetArg(0, b.NewValue0(v.Pos.WithNotStmt(), OpConstNil, f.Config.Types.BytePtr))
			}
			data := v.Args[1]
			if data.Type.IsPtrShaped() {
				continue
			}
			if !types.IsDirectIface(data.Type) {
				continue
			}

			// Reuse the existing generic aggregate-load rule. It exposes a
			// StructMake or ArrayMake1 whose pointer-y operand can be used
			// directly without manufacturing the OpCopy expected by the full
			// generic rewrite driver.
			if data.Op == OpLoad {
				if !rewriteValuegeneric_OpLoad(data) {
					v.Fatalf("direct interface load %s was not normalized by the generic rewrite", data.LongString())
				}
			}
			for data.Op == OpStructMake || data.Op == OpArrayMake1 {
				var next *Value
				for _, arg := range data.Args {
					if arg.Type.Size() != 0 {
						next = arg
						break
					}
				}
				if next == nil {
					v.Fatalf("direct interface constructor %s has no non-zero operand", data.LongString())
				}
				data = next
			}
			if !data.Type.IsPtrShaped() {
				// Formal arguments and aggregate phis have not yet reached the
				// normal decomposition passes. Express their unwrapping with
				// ordinary Go SSA selectors instead of LLVM-specific extracts.
				data = llvmDirectIfacePointer(v, data)
			}
			v.SetArg(1, data)
		}
	}

	// Reuse the existing generic folds for values extracted from a direct
	// interface. These selectors otherwise reach early LLVM emission with an
	// aggregate Go type whose physical IData representation is one pointer.
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if len(v.Args) != 1 || v.Args[0].Op != OpIData {
				continue
			}
			switch v.Op {
			case OpStructSelect:
				rewriteValuegeneric_OpStructSelect(v)
			case OpArraySelect:
				rewriteValuegeneric_OpArraySelect(v)
			}
		}
	}
}

func llvmDirectIfacePointer(use, data *Value) *Value {
	for !data.Type.IsPtrShaped() {
		t := data.Type
		switch t.Kind() {
		case types.TSTRUCT:
			field := -1
			for i := 0; i < t.NumFields(); i++ {
				if t.FieldType(i).Size() == 0 {
					continue
				}
				if field >= 0 {
					use.Fatalf("direct interface struct %v has multiple non-zero fields", t)
				}
				field = i
			}
			if field < 0 {
				use.Fatalf("direct interface struct %v has no non-zero field", t)
			}
			data = use.Block.NewValue1I(use.Pos.WithNotStmt(), OpStructSelect, t.FieldType(field), int64(field), data)
		case types.TARRAY:
			if t.NumElem() != 1 {
				use.Fatalf("direct interface array %v has %d elements", t, t.NumElem())
			}
			data = use.Block.NewValue1I(use.Pos.WithNotStmt(), OpArraySelect, t.Elem(), 0, data)
		default:
			use.Fatalf("direct interface data %v has no pointer-shaped SSA representation", t)
		}
	}
	return data
}
