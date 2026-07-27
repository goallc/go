//go:build dynamicllvm

package llvm

/*
#cgo darwin LDFLAGS: -L${SRCDIR}/llvm/lib -Wl,-rpath,${SRCDIR}/llvm/lib -Wl,-search_paths_first -Wl,-headerpad_max_install_names -lLLVM
#cgo linux LDFLAGS: -L${SRCDIR}/llvm/lib -Wl,-rpath,${SRCDIR}/llvm/lib -lLLVM
*/
import "C"

type llvmLinkModeSelected struct{}
