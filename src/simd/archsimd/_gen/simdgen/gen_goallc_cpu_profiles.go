// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "simd/archsimd/_gen/internal/goallccpu"

// Use the upstream feature definitions in place; no ISA inputs are needed.
func generateGoALLCCPUProfiles(root string) error {
	virtuals := make(map[string][]string)
	for name, info := range goarchFeatureInfo["amd64"].features {
		if info.Virtual {
			virtuals[name] = info.Implies
		}
	}
	return goallccpu.Generate(root, virtuals)
}
