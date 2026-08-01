package main

import "cmd/llvmtoolexec/testdata/nilcheck/p"

func main() {
	tail := &p.Node{Value: 13}
	live := &p.Node{Value: 11, Next: tail}
	checked := &p.Node{Value: 7}
	if got, want := p.Read(checked, live), 7+7+11+13; got != want {
		panic("non-nil explicit nil check or live pointer roots failed")
	}

	defer func() {
		if recover() == nil {
			panic("nil explicit nil check did not produce a recoverable panic")
		}
		if live.Value != 11 || live.Next != tail || tail.Value != 13 {
			panic("live pointers were corrupted after recovered nil check")
		}
	}()
	p.Read(nil, live)
}
