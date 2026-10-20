// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmbackend

// CodeGenOptions returns the LLVM code-generation options for a Go target.
func CodeGenOptions(goarch string) []string {
	options := []string{
		"-trap-unreachable",
		"-disable-machine-cse",
		// Target prologue emission models the morestack retry as a loop
		// backedge. Without profile data, MachineBlockPlacement otherwise
		// keeps the cold morestack block in the loop chain before the function
		// body. Honor the target-provided edge probabilities and outline it
		// after the body, matching the native Go prologue layout.
		"-force-loop-cold-block",
		"-disable-lsr",
	}
	if goarch == "arm64" {
		// Keep page-relative references compatible with Go linkers that require
		// one composite relocation instead of separate page and low-12 records.
		options = append(options, "-aarch64-goobj-composite-relocations")
	}
	return options
}
