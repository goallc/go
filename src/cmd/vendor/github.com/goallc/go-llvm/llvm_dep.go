//===- llvm_dep.go - creates LLVM dependency ------------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//
//
// The vendored bindings target LLVM 23 and use dynamic linking by default.
// The staticllvm build tag selects static linking for toolchain builds.
//
//===----------------------------------------------------------------------===//

package llvm

var (
	_ llvmVersionSelected
	_ llvmLinkModeSelected
)
