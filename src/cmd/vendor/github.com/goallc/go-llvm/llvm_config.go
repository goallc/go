package llvm

/*
#cgo darwin CPPFLAGS: -I${SRCDIR}/llvm/include -D__STDC_CONSTANT_MACROS -D__STDC_FORMAT_MACROS -D__STDC_LIMIT_MACROS
#cgo linux CPPFLAGS: -I${SRCDIR}/llvm/include -D_GNU_SOURCE -D__STDC_CONSTANT_MACROS -D__STDC_FORMAT_MACROS -D__STDC_LIMIT_MACROS
#cgo darwin CXXFLAGS: -std=c++17
#cgo linux CXXFLAGS: -std=c++17
*/
import "C"
