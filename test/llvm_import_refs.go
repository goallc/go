// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	_ "encoding/hex"
	"fmt"
	"os"
	"reflect"
)

//go:noinline
func llvmImportBox[T any](value T) any {
	return value
}

func main() {
	if err := os.Setenv("GOALLC_IMPORT_VALUE", "dep"); err != nil {
		panic(err)
	}
	boxed := llvmImportBox(7)
	if got := reflect.TypeOf(boxed).Name(); got != "int" {
		panic(got)
	}
	if got := reflect.TypeOf((*int)(nil)).Elem().Kind(); got != reflect.Int {
		panic(got)
	}
	if got := fmt.Sprintf("%s/%v", os.Getenv("GOALLC_IMPORT_VALUE"), boxed); got != "dep/7" {
		panic(got)
	}
}
