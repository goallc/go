//go:build staticllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm -lz -lxml2
#cgo linux LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm -lz -lzstd -lxml2 -ldl -lpthread -lrt
*/
import "C"

type llvmLinkModeSelected struct{}
