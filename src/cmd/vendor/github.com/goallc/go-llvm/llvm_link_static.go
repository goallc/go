//go:build staticllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm
#cgo linux LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -static-libstdc++ -static-libgcc -lm -ldl -lpthread -lrt
*/
import "C"

type llvmLinkModeSelected struct{}
