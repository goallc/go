// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

var aggregateCallValue = 40

//go:noinline
func aggregateCallPointer() *int {
	return &aggregateCallValue
}

type aggregateCallPair struct {
	pointer *int
	number  int
}

//go:noinline
func consumeAggregateCallPair(value aggregateCallPair) int {
	return *value.pointer + value.number
}

func main() {
	pointer := aggregateCallPointer()
	value := aggregateCallPair{
		pointer: pointer,
		number:  2,
	}
	if consumeAggregateCallPair(value) != 42 {
		var failed *int
		*failed = 1
	}
}
