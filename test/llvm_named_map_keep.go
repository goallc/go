// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "reflect"

// The named map's R_KEEP edge retains the underlying descriptor and its
// hasher. Reflection can expose that descriptor without a static conversion.
type keepKey struct {
	_ int
	X int
	Y float64
}
type keepMap map[keepKey]int

var mapType = reflect.TypeFor[keepMap]()

func main() {
	named := reflect.MakeMap(mapType)
	key := reflect.ValueOf(keepKey{X: 7, Y: 1.5})
	named.SetMapIndex(key, reflect.ValueOf(42))
	// Obtain the unnamed map dynamically, so this program does not directly
	// reference its descriptor and accidentally replace the keep edge.
	underlying := reflect.MapOf(mapType.Key(), mapType.Elem())
	plain := named.Convert(underlying)
	if got := plain.MapIndex(key); !got.IsValid() || got.Int() != 42 {
		panic("named map lost its underlying descriptor or hasher")
	}
	plain.SetMapIndex(key, reflect.ValueOf(43))
	if named.MapIndex(key).Int() != 43 {
		panic("converted map changed lookup behavior")
	}
}
