// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "reflect"

type reflectedReceiver struct{}
type reflectedResult struct{ Value int }

func (*reflectedReceiver) OnlyReflected() reflectedResult {
	return reflectedResult{Value: 42}
}

// The LLVM qualification subtest disables Go frontend inlining explicitly.
// The Go frontend must leave this call for LLVM to inline. The dynamic
// lookup's ReflectMethod fact must survive removal of the helper's body.
func lookupReflected(t reflect.Type, name string) bool {
	_, found := t.MethodByName(name)
	return found
}

func nestedLookup(t reflect.Type, name string) bool {
	return lookupReflected(t, name)
}

func main() {
	t := reflect.TypeOf(&reflectedReceiver{})
	if !nestedLookup(t, "OnlyReflected") {
		panic("reflection-only method was removed")
	}
}
