package initialize

import _ "unsafe"

//go:linkname value example.com/lto-bss/decl.Value
var value = 42

func Address() *int { return &value }
