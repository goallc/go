package p

import "runtime"

type Node struct {
	Value int
	Next  *Node
}

type roots struct {
	checked *Node
	live    *Node
}

// useRoots keeps the pointer-containing address-taken local live across the
// ordinary runtime.GC safepoint after the explicit nil check.
//
//go:noinline
func useRoots(r *roots) int {
	return r.checked.Value + r.live.Value + r.live.Next.Value
}

//go:noinline
func Read(checked, live *Node) int {
	r := roots{checked: checked, live: live}
	checkedValue := r.checked.Value
	runtime.GC()
	return checkedValue + useRoots(&r)
}
