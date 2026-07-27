//===- llvm_dep.go - creates LLVM dependency ------------------------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//
//
// This file makes selecting an LLVM API version and link mode mandatory.
//
//===----------------------------------------------------------------------===//

package llvm

var (
	_ llvmVersionSelected
	_ llvmLinkModeSelected
)
