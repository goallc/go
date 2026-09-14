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

type aggregateCallArray [8]int

//go:noinline
func consumeAggregateCallArray(value aggregateCallArray) int {
	return value[0] + value[7]
}

var indirectAggregateCall = consumeAggregateCallArray

type aggregateArrayConsumer interface {
	consume(aggregateCallArray) int
}

type aggregateConsumer struct{ bias int }

//go:noinline
func (c *aggregateConsumer) consume(value aggregateCallArray) int {
	return c.bias + consumeAggregateCallArray(value)
}

//go:noinline
func checkAggregateMemoryCalls(p *aggregateCallArray, consumer aggregateArrayConsumer) {
	// A later memory change must not replace this value snapshot with p.
	snapshot := *p
	p[0] = 100
	if consumeAggregateCallArray(snapshot) != 9 || indirectAggregateCall(snapshot) != 9 || consumer.consume(snapshot) != 10 {
		panic("aggregate argument lost its snapshot")
	}
	closure := func(value aggregateCallArray) int { return p[0] + consumeAggregateCallArray(value) }
	if callAggregateClosure(closure, snapshot) != 109 || snapshot[0] != 1 || p[0] != 100 {
		panic("aggregate closure argument changed")
	}
}

//go:noinline
func callAggregateClosure(fn func(aggregateCallArray) int, value aggregateCallArray) int {
	return fn(value)
}

func main() {
	array := aggregateCallArray{1, 2, 3, 4, 5, 6, 7, 8}
	checkAggregateMemoryCalls(&array, &aggregateConsumer{bias: 1})
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
