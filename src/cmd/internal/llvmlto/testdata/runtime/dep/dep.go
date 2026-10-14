package dep

import (
	"example.com/lto/leaf"
	"runtime"
)

var InitOrder string

func init() { InitOrder = leaf.InitOrder + ":dep" }

type Node struct {
	Value int
	Next  *Node
}

func Add(x int) int       { return x + 7 }
func Make(x int) *Node    { return &Node{Value: x, Next: &Node{Value: x + 1}} }
func Collect(n *Node) int { runtime.GC(); return n.Value + n.Next.Value }

type Value interface{ Get() int }

func (n *Node) Get() int { return n.Value }
func Read(v Value) int   { return v.Get() }
func Boom()              { panic("lto-panic") }
func Frames() []runtime.Frame {
	var pc [16]uintptr
	n := runtime.Callers(1, pc[:])
	it := runtime.CallersFrames(pc[:n])
	var out []runtime.Frame
	for {
		f, more := it.Next()
		out = append(out, f)
		if !more {
			return out
		}
	}
}

func Transitive(i int) int { return leaf.At(i) }

//go:noinline
func Opaque(x int) int { return x + 9 }

//go:noinline
func Deep(n int, p *Node) int {
	var roots [32]*Node
	roots[n&31] = p
	if n == 0 {
		runtime.GC()
		return roots[0].Value
	}
	x := Deep(n-1, p)
	return x + roots[n&31].Value
}
