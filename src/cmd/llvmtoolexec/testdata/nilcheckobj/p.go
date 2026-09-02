package nilcheckobj

// Read keeps p live after the explicit panicmem call. Its load is outside the
// first page, so the panic edge must retain its ordinary statepoint and map.
//
//go:noinline
func Read(p *[1024]int64) int64 {
	return p[1023]
}
