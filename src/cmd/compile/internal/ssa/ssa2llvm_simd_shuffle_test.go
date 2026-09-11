// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package ssa

import (
	"cmd/compile/internal/types"
	"fmt"
	"github.com/goallc/go-llvm"
	"strings"
	"testing"
)

func TestLLVMGeneratedSIMDStaticShuffles(t *testing.T) {
	oldTypes, oldModule := type2lTypes, CurrentModule
	type2lTypes = make(map[*types.Type]llvm.Type)
	defer func() { type2lTypes, CurrentModule = oldTypes, oldModule }()
	tests := []struct {
		op                                         Op
		kind                                       string
		bits, sourceLanes, otherLanes, resultLanes int
		arches, mask                               string
	}{
		{OpGetHiFloat32x8, "float", 32, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetHiFloat32x16, "float", 32, 16, 0, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetHiFloat64x4, "float", 64, 4, 0, 2, "amd64", "<i32 2, i32 3>"},
		{OpGetHiFloat64x8, "float", 64, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetHiInt8x32, "int", 8, 32, 0, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetHiInt8x64, "int", 8, 64, 0, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>"},
		{OpGetHiInt16x16, "int", 16, 16, 0, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetHiInt16x32, "int", 16, 32, 0, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetHiInt32x8, "int", 32, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetHiInt32x16, "int", 32, 16, 0, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetHiInt64x4, "int", 64, 4, 0, 2, "amd64", "<i32 2, i32 3>"},
		{OpGetHiInt64x8, "int", 64, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetHiUint8x32, "uint", 8, 32, 0, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetHiUint8x64, "uint", 8, 64, 0, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>"},
		{OpGetHiUint16x16, "uint", 16, 16, 0, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetHiUint16x32, "uint", 16, 32, 0, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetHiUint32x8, "uint", 32, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetHiUint32x16, "uint", 32, 16, 0, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetHiUint64x4, "uint", 64, 4, 0, 2, "amd64", "<i32 2, i32 3>"},
		{OpGetHiUint64x8, "uint", 64, 8, 0, 4, "amd64", "<i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoFloat32x8, "float", 32, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpGetLoFloat32x16, "float", 32, 16, 0, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoFloat64x4, "float", 64, 4, 0, 2, "amd64", "<i32 0, i32 1>"},
		{OpGetLoFloat64x8, "float", 64, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpGetLoInt8x32, "int", 8, 32, 0, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetLoInt8x64, "int", 8, 64, 0, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetLoInt16x16, "int", 16, 16, 0, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoInt16x32, "int", 16, 32, 0, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetLoInt32x8, "int", 32, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpGetLoInt32x16, "int", 32, 16, 0, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoInt64x4, "int", 64, 4, 0, 2, "amd64", "<i32 0, i32 1>"},
		{OpGetLoInt64x8, "int", 64, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpGetLoUint8x32, "uint", 8, 32, 0, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetLoUint8x64, "uint", 8, 64, 0, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpGetLoUint16x16, "uint", 16, 16, 0, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoUint16x32, "uint", 16, 32, 0, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpGetLoUint32x8, "uint", 32, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpGetLoUint32x16, "uint", 32, 16, 0, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7>"},
		{OpGetLoUint64x4, "uint", 64, 4, 0, 2, "amd64", "<i32 0, i32 1>"},
		{OpGetLoUint64x8, "uint", 64, 8, 0, 4, "amd64", "<i32 0, i32 1, i32 2, i32 3>"},
		{OpInterleaveHiInt16x8, "int", 16, 8, 8, 8, "amd64,arm64", "<i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>"},
		{OpInterleaveHiInt32x4, "int", 32, 4, 4, 4, "amd64,arm64", "<i32 2, i32 6, i32 3, i32 7>"},
		{OpInterleaveHiInt64x2, "int", 64, 2, 2, 2, "amd64,arm64", "<i32 1, i32 3>"},
		{OpInterleaveHiUint16x8, "uint", 16, 8, 8, 8, "amd64,arm64", "<i32 4, i32 12, i32 5, i32 13, i32 6, i32 14, i32 7, i32 15>"},
		{OpInterleaveHiUint32x4, "uint", 32, 4, 4, 4, "amd64,arm64", "<i32 2, i32 6, i32 3, i32 7>"},
		{OpInterleaveHiUint64x2, "uint", 64, 2, 2, 2, "amd64,arm64", "<i32 1, i32 3>"},
		{OpInterleaveHiGroupedInt16x16, "int", 16, 16, 16, 16, "amd64", "<i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveHiGroupedInt16x32, "int", 16, 32, 32, 32, "amd64", "<i32 4, i32 36, i32 5, i32 37, i32 6, i32 38, i32 7, i32 39, i32 12, i32 44, i32 13, i32 45, i32 14, i32 46, i32 15, i32 47, i32 20, i32 52, i32 21, i32 53, i32 22, i32 54, i32 23, i32 55, i32 28, i32 60, i32 29, i32 61, i32 30, i32 62, i32 31, i32 63>"},
		{OpInterleaveHiGroupedInt32x8, "int", 32, 8, 8, 8, "amd64", "<i32 2, i32 10, i32 3, i32 11, i32 6, i32 14, i32 7, i32 15>"},
		{OpInterleaveHiGroupedInt32x16, "int", 32, 16, 16, 16, "amd64", "<i32 2, i32 18, i32 3, i32 19, i32 6, i32 22, i32 7, i32 23, i32 10, i32 26, i32 11, i32 27, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveHiGroupedInt64x4, "int", 64, 4, 4, 4, "amd64", "<i32 1, i32 5, i32 3, i32 7>"},
		{OpInterleaveHiGroupedInt64x8, "int", 64, 8, 8, 8, "amd64", "<i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>"},
		{OpInterleaveHiGroupedUint16x16, "uint", 16, 16, 16, 16, "amd64", "<i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveHiGroupedUint16x32, "uint", 16, 32, 32, 32, "amd64", "<i32 4, i32 36, i32 5, i32 37, i32 6, i32 38, i32 7, i32 39, i32 12, i32 44, i32 13, i32 45, i32 14, i32 46, i32 15, i32 47, i32 20, i32 52, i32 21, i32 53, i32 22, i32 54, i32 23, i32 55, i32 28, i32 60, i32 29, i32 61, i32 30, i32 62, i32 31, i32 63>"},
		{OpInterleaveHiGroupedUint32x8, "uint", 32, 8, 8, 8, "amd64", "<i32 2, i32 10, i32 3, i32 11, i32 6, i32 14, i32 7, i32 15>"},
		{OpInterleaveHiGroupedUint32x16, "uint", 32, 16, 16, 16, "amd64", "<i32 2, i32 18, i32 3, i32 19, i32 6, i32 22, i32 7, i32 23, i32 10, i32 26, i32 11, i32 27, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveHiGroupedUint64x4, "uint", 64, 4, 4, 4, "amd64", "<i32 1, i32 5, i32 3, i32 7>"},
		{OpInterleaveHiGroupedUint64x8, "uint", 64, 8, 8, 8, "amd64", "<i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>"},
		{OpInterleaveLoInt16x8, "int", 16, 8, 8, 8, "amd64,arm64", "<i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>"},
		{OpInterleaveLoInt32x4, "int", 32, 4, 4, 4, "amd64,arm64", "<i32 0, i32 4, i32 1, i32 5>"},
		{OpInterleaveLoInt64x2, "int", 64, 2, 2, 2, "amd64,arm64", "<i32 0, i32 2>"},
		{OpInterleaveLoUint16x8, "uint", 16, 8, 8, 8, "amd64,arm64", "<i32 0, i32 8, i32 1, i32 9, i32 2, i32 10, i32 3, i32 11>"},
		{OpInterleaveLoUint32x4, "uint", 32, 4, 4, 4, "amd64,arm64", "<i32 0, i32 4, i32 1, i32 5>"},
		{OpInterleaveLoUint64x2, "uint", 64, 2, 2, 2, "amd64,arm64", "<i32 0, i32 2>"},
		{OpInterleaveLoGroupedInt16x16, "int", 16, 16, 16, 16, "amd64", "<i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27>"},
		{OpInterleaveLoGroupedInt16x32, "int", 16, 32, 32, 32, "amd64", "<i32 0, i32 32, i32 1, i32 33, i32 2, i32 34, i32 3, i32 35, i32 8, i32 40, i32 9, i32 41, i32 10, i32 42, i32 11, i32 43, i32 16, i32 48, i32 17, i32 49, i32 18, i32 50, i32 19, i32 51, i32 24, i32 56, i32 25, i32 57, i32 26, i32 58, i32 27, i32 59>"},
		{OpInterleaveLoGroupedInt32x8, "int", 32, 8, 8, 8, "amd64", "<i32 0, i32 8, i32 1, i32 9, i32 4, i32 12, i32 5, i32 13>"},
		{OpInterleaveLoGroupedInt32x16, "int", 32, 16, 16, 16, "amd64", "<i32 0, i32 16, i32 1, i32 17, i32 4, i32 20, i32 5, i32 21, i32 8, i32 24, i32 9, i32 25, i32 12, i32 28, i32 13, i32 29>"},
		{OpInterleaveLoGroupedInt64x4, "int", 64, 4, 4, 4, "amd64", "<i32 0, i32 4, i32 2, i32 6>"},
		{OpInterleaveLoGroupedInt64x8, "int", 64, 8, 8, 8, "amd64", "<i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>"},
		{OpInterleaveLoGroupedUint16x16, "uint", 16, 16, 16, 16, "amd64", "<i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27>"},
		{OpInterleaveLoGroupedUint16x32, "uint", 16, 32, 32, 32, "amd64", "<i32 0, i32 32, i32 1, i32 33, i32 2, i32 34, i32 3, i32 35, i32 8, i32 40, i32 9, i32 41, i32 10, i32 42, i32 11, i32 43, i32 16, i32 48, i32 17, i32 49, i32 18, i32 50, i32 19, i32 51, i32 24, i32 56, i32 25, i32 57, i32 26, i32 58, i32 27, i32 59>"},
		{OpInterleaveLoGroupedUint32x8, "uint", 32, 8, 8, 8, "amd64", "<i32 0, i32 8, i32 1, i32 9, i32 4, i32 12, i32 5, i32 13>"},
		{OpInterleaveLoGroupedUint32x16, "uint", 32, 16, 16, 16, "amd64", "<i32 0, i32 16, i32 1, i32 17, i32 4, i32 20, i32 5, i32 21, i32 8, i32 24, i32 9, i32 25, i32 12, i32 28, i32 13, i32 29>"},
		{OpInterleaveLoGroupedUint64x4, "uint", 64, 4, 4, 4, "amd64", "<i32 0, i32 4, i32 2, i32 6>"},
		{OpInterleaveLoGroupedUint64x8, "uint", 64, 8, 8, 8, "amd64", "<i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>"},
		{OpSetHiFloat32x8, "float", 32, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetHiFloat32x16, "float", 32, 16, 8, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>"},
		{OpSetHiFloat64x4, "float", 64, 4, 2, 4, "amd64", "<i32 0, i32 1, i32 4, i32 5>"},
		{OpSetHiFloat64x8, "float", 64, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetHiInt8x32, "int", 8, 32, 16, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>"},
		{OpSetHiInt8x64, "int", 8, 64, 32, 64, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95>"},
		{OpSetHiInt16x16, "int", 16, 16, 8, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>"},
		{OpSetHiInt16x32, "int", 16, 32, 16, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>"},
		{OpSetHiInt32x8, "int", 32, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetHiInt32x16, "int", 32, 16, 8, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>"},
		{OpSetHiInt64x4, "int", 64, 4, 2, 4, "amd64", "<i32 0, i32 1, i32 4, i32 5>"},
		{OpSetHiInt64x8, "int", 64, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetHiUint8x32, "uint", 8, 32, 16, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>"},
		{OpSetHiUint8x64, "uint", 8, 64, 32, 64, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31, i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95>"},
		{OpSetHiUint16x16, "uint", 16, 16, 8, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>"},
		{OpSetHiUint16x32, "uint", 16, 32, 16, 32, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47>"},
		{OpSetHiUint32x8, "uint", 32, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetHiUint32x16, "uint", 32, 16, 8, 16, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 4, i32 5, i32 6, i32 7, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23>"},
		{OpSetHiUint64x4, "uint", 64, 4, 2, 4, "amd64", "<i32 0, i32 1, i32 4, i32 5>"},
		{OpSetHiUint64x8, "uint", 64, 8, 4, 8, "amd64", "<i32 0, i32 1, i32 2, i32 3, i32 8, i32 9, i32 10, i32 11>"},
		{OpSetLoFloat32x8, "float", 32, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpSetLoFloat32x16, "float", 32, 16, 8, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpSetLoFloat64x4, "float", 64, 4, 2, 4, "amd64", "<i32 4, i32 5, i32 2, i32 3>"},
		{OpSetLoFloat64x8, "float", 64, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpSetLoInt8x32, "int", 8, 32, 16, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpSetLoInt8x64, "int", 8, 64, 32, 64, "amd64", "<i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>"},
		{OpSetLoInt16x16, "int", 16, 16, 8, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpSetLoInt16x32, "int", 16, 32, 16, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpSetLoInt32x8, "int", 32, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpSetLoInt32x16, "int", 32, 16, 8, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpSetLoInt64x4, "int", 64, 4, 2, 4, "amd64", "<i32 4, i32 5, i32 2, i32 3>"},
		{OpSetLoInt64x8, "int", 64, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpSetLoUint8x32, "uint", 8, 32, 16, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpSetLoUint8x64, "uint", 8, 64, 32, 64, "amd64", "<i32 64, i32 65, i32 66, i32 67, i32 68, i32 69, i32 70, i32 71, i32 72, i32 73, i32 74, i32 75, i32 76, i32 77, i32 78, i32 79, i32 80, i32 81, i32 82, i32 83, i32 84, i32 85, i32 86, i32 87, i32 88, i32 89, i32 90, i32 91, i32 92, i32 93, i32 94, i32 95, i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 48, i32 49, i32 50, i32 51, i32 52, i32 53, i32 54, i32 55, i32 56, i32 57, i32 58, i32 59, i32 60, i32 61, i32 62, i32 63>"},
		{OpSetLoUint16x16, "uint", 16, 16, 8, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpSetLoUint16x32, "uint", 16, 32, 16, 32, "amd64", "<i32 32, i32 33, i32 34, i32 35, i32 36, i32 37, i32 38, i32 39, i32 40, i32 41, i32 42, i32 43, i32 44, i32 45, i32 46, i32 47, i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 24, i32 25, i32 26, i32 27, i32 28, i32 29, i32 30, i32 31>"},
		{OpSetLoUint32x8, "uint", 32, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpSetLoUint32x16, "uint", 32, 16, 8, 16, "amd64", "<i32 16, i32 17, i32 18, i32 19, i32 20, i32 21, i32 22, i32 23, i32 8, i32 9, i32 10, i32 11, i32 12, i32 13, i32 14, i32 15>"},
		{OpSetLoUint64x4, "uint", 64, 4, 2, 4, "amd64", "<i32 4, i32 5, i32 2, i32 3>"},
		{OpSetLoUint64x8, "uint", 64, 8, 4, 8, "amd64", "<i32 8, i32 9, i32 10, i32 11, i32 4, i32 5, i32 6, i32 7>"},
		{OpConcatEvenInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14, i32 16, i32 18, i32 20, i32 22, i32 24, i32 26, i32 28, i32 30>"},
		{OpConcatEvenInt16x8, "int", 16, 8, 8, 8, "arm64", "<i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14>"},
		{OpConcatEvenInt32x4, "int", 32, 4, 4, 4, "arm64", "<i32 0, i32 2, i32 4, i32 6>"},
		{OpConcatEvenInt64x2, "int", 64, 2, 2, 2, "arm64", "<i32 0, i32 2>"},
		{OpConcatEvenUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14, i32 16, i32 18, i32 20, i32 22, i32 24, i32 26, i32 28, i32 30>"},
		{OpConcatEvenUint16x8, "uint", 16, 8, 8, 8, "arm64", "<i32 0, i32 2, i32 4, i32 6, i32 8, i32 10, i32 12, i32 14>"},
		{OpConcatEvenUint32x4, "uint", 32, 4, 4, 4, "arm64", "<i32 0, i32 2, i32 4, i32 6>"},
		{OpConcatEvenUint64x2, "uint", 64, 2, 2, 2, "arm64", "<i32 0, i32 2>"},
		{OpConcatOddInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15, i32 17, i32 19, i32 21, i32 23, i32 25, i32 27, i32 29, i32 31>"},
		{OpConcatOddInt16x8, "int", 16, 8, 8, 8, "arm64", "<i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15>"},
		{OpConcatOddInt32x4, "int", 32, 4, 4, 4, "arm64", "<i32 1, i32 3, i32 5, i32 7>"},
		{OpConcatOddInt64x2, "int", 64, 2, 2, 2, "arm64", "<i32 1, i32 3>"},
		{OpConcatOddUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15, i32 17, i32 19, i32 21, i32 23, i32 25, i32 27, i32 29, i32 31>"},
		{OpConcatOddUint16x8, "uint", 16, 8, 8, 8, "arm64", "<i32 1, i32 3, i32 5, i32 7, i32 9, i32 11, i32 13, i32 15>"},
		{OpConcatOddUint32x4, "uint", 32, 4, 4, 4, "arm64", "<i32 1, i32 3, i32 5, i32 7>"},
		{OpConcatOddUint64x2, "uint", 64, 2, 2, 2, "arm64", "<i32 1, i32 3>"},
		{OpInterleaveEvenInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 0, i32 16, i32 2, i32 18, i32 4, i32 20, i32 6, i32 22, i32 8, i32 24, i32 10, i32 26, i32 12, i32 28, i32 14, i32 30>"},
		{OpInterleaveEvenInt16x8, "int", 16, 8, 8, 8, "arm64", "<i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>"},
		{OpInterleaveEvenInt32x4, "int", 32, 4, 4, 4, "arm64", "<i32 0, i32 4, i32 2, i32 6>"},
		{OpInterleaveEvenInt64x2, "int", 64, 2, 2, 2, "arm64", "<i32 0, i32 2>"},
		{OpInterleaveEvenUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 0, i32 16, i32 2, i32 18, i32 4, i32 20, i32 6, i32 22, i32 8, i32 24, i32 10, i32 26, i32 12, i32 28, i32 14, i32 30>"},
		{OpInterleaveEvenUint16x8, "uint", 16, 8, 8, 8, "arm64", "<i32 0, i32 8, i32 2, i32 10, i32 4, i32 12, i32 6, i32 14>"},
		{OpInterleaveEvenUint32x4, "uint", 32, 4, 4, 4, "arm64", "<i32 0, i32 4, i32 2, i32 6>"},
		{OpInterleaveEvenUint64x2, "uint", 64, 2, 2, 2, "arm64", "<i32 0, i32 2>"},
		{OpInterleaveHiInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveHiUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 8, i32 24, i32 9, i32 25, i32 10, i32 26, i32 11, i32 27, i32 12, i32 28, i32 13, i32 29, i32 14, i32 30, i32 15, i32 31>"},
		{OpInterleaveLoInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23>"},
		{OpInterleaveLoUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 0, i32 16, i32 1, i32 17, i32 2, i32 18, i32 3, i32 19, i32 4, i32 20, i32 5, i32 21, i32 6, i32 22, i32 7, i32 23>"},
		{OpInterleaveOddInt8x16, "int", 8, 16, 16, 16, "arm64", "<i32 1, i32 17, i32 3, i32 19, i32 5, i32 21, i32 7, i32 23, i32 9, i32 25, i32 11, i32 27, i32 13, i32 29, i32 15, i32 31>"},
		{OpInterleaveOddInt16x8, "int", 16, 8, 8, 8, "arm64", "<i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>"},
		{OpInterleaveOddInt32x4, "int", 32, 4, 4, 4, "arm64", "<i32 1, i32 5, i32 3, i32 7>"},
		{OpInterleaveOddInt64x2, "int", 64, 2, 2, 2, "arm64", "<i32 1, i32 3>"},
		{OpInterleaveOddUint8x16, "uint", 8, 16, 16, 16, "arm64", "<i32 1, i32 17, i32 3, i32 19, i32 5, i32 21, i32 7, i32 23, i32 9, i32 25, i32 11, i32 27, i32 13, i32 29, i32 15, i32 31>"},
		{OpInterleaveOddUint16x8, "uint", 16, 8, 8, 8, "arm64", "<i32 1, i32 9, i32 3, i32 11, i32 5, i32 13, i32 7, i32 15>"},
		{OpInterleaveOddUint32x4, "uint", 32, 4, 4, 4, "arm64", "<i32 1, i32 5, i32 3, i32 7>"},
		{OpInterleaveOddUint64x2, "uint", 64, 2, 2, 2, "arm64", "<i32 1, i32 3>"},
		{Opbroadcast1To2Float64x2, "float", 64, 2, 0, 2, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To2Int64x2, "int", 64, 2, 0, 2, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To2Uint64x2, "uint", 64, 2, 0, 2, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To4Float32x4, "float", 32, 4, 0, 4, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To4Float64x2, "float", 64, 2, 0, 4, "amd64", "zeroinitializer"},
		{Opbroadcast1To4Int32x4, "int", 32, 4, 0, 4, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To4Int64x2, "int", 64, 2, 0, 4, "amd64", "zeroinitializer"},
		{Opbroadcast1To4Uint32x4, "uint", 32, 4, 0, 4, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To4Uint64x2, "uint", 64, 2, 0, 4, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Float32x4, "float", 32, 4, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Float64x2, "float", 64, 2, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Int16x8, "int", 16, 8, 0, 8, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To8Int32x4, "int", 32, 4, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Int64x2, "int", 64, 2, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Uint16x8, "uint", 16, 8, 0, 8, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To8Uint32x4, "uint", 32, 4, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To8Uint64x2, "uint", 64, 2, 0, 8, "amd64", "zeroinitializer"},
		{Opbroadcast1To16Float32x4, "float", 32, 4, 0, 16, "amd64", "zeroinitializer"},
		{Opbroadcast1To16Int8x16, "int", 8, 16, 0, 16, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To16Int16x8, "int", 16, 8, 0, 16, "amd64", "zeroinitializer"},
		{Opbroadcast1To16Int32x4, "int", 32, 4, 0, 16, "amd64", "zeroinitializer"},
		{Opbroadcast1To16Uint8x16, "uint", 8, 16, 0, 16, "amd64,arm64", "zeroinitializer"},
		{Opbroadcast1To16Uint16x8, "uint", 16, 8, 0, 16, "amd64", "zeroinitializer"},
		{Opbroadcast1To16Uint32x4, "uint", 32, 4, 0, 16, "amd64", "zeroinitializer"},
		{Opbroadcast1To32Int8x16, "int", 8, 16, 0, 32, "amd64", "zeroinitializer"},
		{Opbroadcast1To32Int16x8, "int", 16, 8, 0, 32, "amd64", "zeroinitializer"},
		{Opbroadcast1To32Uint8x16, "uint", 8, 16, 0, 32, "amd64", "zeroinitializer"},
		{Opbroadcast1To32Uint16x8, "uint", 16, 8, 0, 32, "amd64", "zeroinitializer"},
		{Opbroadcast1To64Int8x16, "int", 8, 16, 0, 64, "amd64", "zeroinitializer"},
		{Opbroadcast1To64Uint8x16, "uint", 8, 16, 0, 64, "amd64", "zeroinitializer"},
	}
	seen := make(map[Op]bool)
	for _, test := range tests {
		seen[test.op] = true
		for _, arch := range strings.Split(test.arches, ",") {
			for _, carrier := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/carrier=%v", test.op, arch, carrier), func(t *testing.T) {
					kinds := map[string]map[int]types.Kind{
						"int":   {8: types.TINT8, 16: types.TINT16, 32: types.TINT32, 64: types.TINT64},
						"uint":  {8: types.TUINT8, 16: types.TUINT16, 32: types.TUINT32, 64: types.TUINT64},
						"float": {32: types.TFLOAT32, 64: types.TFLOAT64},
					}
					elem := types.Types[kinds[test.kind][test.bits]]
					output := llvmTestSIMDType("shuffle-output", elem, int64(test.resultLanes))
					resultType := getLLVMType(output)
					vector := func(lanes int) *types.Type {
						if carrier {
							return map[int]*types.Type{128: types.TypeVec128, 256: types.TypeVec256, 512: types.TypeVec512}[lanes*test.bits]
						}
						return llvmTestSIMDType("shuffle-vector", elem, int64(lanes))
					}
					output = vector(test.resultLanes)
					inputs := []*types.Type{vector(test.sourceLanes)}
					if test.otherLanes != 0 {
						inputs = append(inputs, vector(test.otherLanes))
					}
					params := make([]llvm.Type, len(inputs))
					for i, input := range inputs {
						params[i] = getLLVMType(input)
					}
					module := GlobalCtxt.NewModule("static-shuffle")
					CurrentModule = module
					builder := GlobalCtxt.NewBuilder()
					defer module.Dispose()
					defer builder.Dispose()
					function := llvm.AddFunction(module, "shuffle", llvm.FunctionType(resultType, params, false))
					builder.SetInsertPointAtEnd(llvm.AddBasicBlock(function, "entry"))
					context := &LLVMFuncContext{
						F:  &Func{Config: &Config{arch: arch}, Entry: &Block{CPUfeatures: CPUavx | CPUavx2 | CPUavx512}},
						Vs: make(map[ID]llvm.Value), b: builder,
					}
					args := make([]*Value, len(inputs))
					for i, input := range inputs {
						args[i] = &Value{ID: ID(i + 1), Op: OpArg, Type: input}
						context.Vs[args[i].ID] = function.Param(i)
					}
					v := &Value{ID: 3, Op: test.op, Type: output, Args: args}
					builder.CreateRet(context.GenLV(v))
					if err := llvm.VerifyModule(module, llvm.ReturnStatusAction); err != nil {
						t.Fatalf("invalid shuffle IR: %v\n%s", err, module.String())
					}
					ir := module.String()
					// The expected masks are explicit and independent of the lowering's index arithmetic.
					if !strings.Contains(ir, "shufflevector") || !strings.Contains(ir, test.mask) {
						t.Errorf("missing expected lane routing %s\n%s", test.mask, ir)
					}
					if strings.Contains(ir, " undef") || strings.Contains(ir, " poison") {
						t.Errorf("fixed shuffle must not introduce undefined lanes\n%s", ir)
					}
					if strings.Contains(ir, "@llvm.x86.") || strings.Contains(ir, "@llvm.aarch64.") {
						t.Errorf("static routing must use standard vector IR\n%s", ir)
					}
				})
			}
		}
	}
	for op := Op(0); int(op) < len(goALLCSIMDOpcodeIndex); op++ {
		info, ok := goALLCSIMDInfo(op)
		if ok && info.lowering >= goALLCSIMDLowerGetLow && info.lowering <= goALLCSIMDLowerInterleaveOdd && !seen[op] {
			t.Errorf("missing static shuffle test for %s", op)
		}
	}
}
