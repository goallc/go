package main

import "reflect"

var called bool

type methodTarget int

func (methodTarget) Exported() {
	called = true
}

func main() {
	reflect.ValueOf(methodTarget(0)).Method(0).Interface().(func())()
	if !called {
		panic("reflected method was not called")
	}
}
