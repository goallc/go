//go:build staticllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm -lz -lxml2
#cgo darwin pkg-config: libzstd
#cgo windows LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -static -lpsapi -lshell32 -lole32 -luuid -ladvapi32 -lws2_32 -lntdll
#cgo linux LDFLAGS: -L${SRCDIR} -lLLVMGoALLC -lm -lz -lzstd -lxml2 -ldl -lpthread -lrt
*/
import "C"

type llvmLinkModeSelected struct{}
