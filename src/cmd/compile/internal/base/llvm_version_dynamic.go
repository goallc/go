// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap && !staticllvm

package base

import "cmd/internal/llvmbackend"

func llvmVersionBackendIdentity() (string, error) { return llvmbackend.Identity() }
