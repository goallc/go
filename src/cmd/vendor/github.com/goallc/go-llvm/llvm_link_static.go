//go:build staticllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR}/llvm/lib -lLLVMGoALLC -lm -lz -lxml2
#cgo linux LDFLAGS: -L${SRCDIR}/llvm/lib -lLLVMGoALLC -lm -lz -lxml2 -ldl -lpthread
*/
import "C"

type llvmLinkModeSelected struct{}
