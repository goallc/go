//go:build dynamicllvm

package llvm

/*
// Prefer the libraries next to an installed compiler. The absolute fallback
// also supports compiler test executables built outside the tool directory.
#cgo linux LDFLAGS: -Wl,-rpath,$ORIGIN/lib
#cgo darwin LDFLAGS: -Wl,-rpath,@loader_path/lib -Wl,-search_paths_first -Wl,-headerpad_max_install_names
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../../../../pkg/tool/linux_amd64/lib -Wl,-rpath,${SRCDIR}/../../../../../../pkg/tool/linux_amd64/lib
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../../../../pkg/tool/linux_arm64/lib -Wl,-rpath,${SRCDIR}/../../../../../../pkg/tool/linux_arm64/lib
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../../../../../pkg/tool/darwin_amd64/lib -Wl,-rpath,${SRCDIR}/../../../../../../pkg/tool/darwin_amd64/lib
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../../../../../pkg/tool/darwin_arm64/lib -Wl,-rpath,${SRCDIR}/../../../../../../pkg/tool/darwin_arm64/lib
#cgo LDFLAGS: -lLLVM
*/
import "C"

type llvmLinkModeSelected struct{}
