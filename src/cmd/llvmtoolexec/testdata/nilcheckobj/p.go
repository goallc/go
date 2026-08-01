package nilcheckobj

// Read keeps p live on the normal edge after the explicit panicmem call. The
// panic edge must therefore receive its own ordinary statepoint and stack map.
//
//go:noinline
func Read(p *int) int {
	return *p
}
