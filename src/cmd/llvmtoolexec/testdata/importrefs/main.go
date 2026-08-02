package main

import (
	_ "encoding/hex"
	"fmt"
	"os"
	"reflect"

	"cmd/llvmtoolexec/testdata/importrefs/dep"
)

//go:noinline
func box[T any](v T) any { return v }

func main() {
	value := dep.Value()
	if value == "getenv" {
		value = os.Getenv("GOALLC_IMPORT_VALUE")
	}
	boxed := box(7)
	if got := reflect.TypeOf(boxed).Name(); got != "int" {
		panic(got)
	}
	got := fmt.Sprintf("%s/%v", value, boxed)
	if got != "dep/7" {
		panic(got)
	}
}
