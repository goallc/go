// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build arm64

package main

import (
	"simd/archsimd"
)

//go:noinline
func ConvertLo2ToFloat64Float32x4(x archsimd.Float32x4) archsimd.Float64x2 {
	return x.ConvertLo2ToFloat64()
}

func checkArch() {
	check("ConvertLo2ToFloat64Float32x4", "float", 32, 4, "float", 64, 2, true, true, func(x []float32, got []float64) {
		result := ConvertLo2ToFloat64Float32x4(archsimd.LoadFloat32x4(x))
		result.Store(got)
	})
}
