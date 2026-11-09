package main

import (
	"example.com/lto-bss/decl"
	"example.com/lto-bss/initialize"
	"example.com/lto-bss/observer"
)

func main() {
	if decl.Value != 42 || initialize.Address() != &decl.Value || observer.Address() != &decl.Value {
		panic("indexed BSS did not resolve to its initialized definition")
	}
	*observer.Address() = 99
	if decl.Value != 99 || *initialize.Address() != 99 {
		panic("indexed BSS has multiple storage locations")
	}
	println("LTO INDEXED BSS OK")
}
