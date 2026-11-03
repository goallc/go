//go:build staticllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm
#cgo linux LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -static-libstdc++ -static-libgcc -lm -ldl -lpthread -lrt
#cgo windows LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -static -lpsapi -lshell32 -lole32 -luuid -ladvapi32 -lws2_32 -lntdll
*/
import "C"

type llvmLinkModeSelected struct{}
