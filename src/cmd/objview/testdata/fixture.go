package fixture

//go:noinline
func touch(*int) {}

// MetadataFixture keeps both an ordinary pointer and an address-taken,
// pointer-containing stack object live across a call.
//
//go:noinline
func MetadataFixture(p *int, choose bool) *int {
	obj := struct {
		p *int
		n int
	}{p: p, n: 7}
	q := &obj
	if choose {
		touch(&q.n)
	}
	return q.p
}

//go:noinline
func IndirectFixture(fn func(*int), p *int) {
	fn(p)
}
