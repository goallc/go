// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package llvmprofile writes LLVM IR instrumentation counters at program exit.
package llvmprofile

import (
	"bufio"
	"internal/runtime/exithook"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

// The plugin emits one slice of records per package. The linker fills packages.
type record struct {
	name     string
	hash     uint64
	counters []uint64
}

var packages []*[]record

func init() {
	if len(packages) != 0 {
		exithook.Add(exithook.Hook{F: dump, RunOnFailure: true})
	}
}

func dump() {
	path := os.Getenv("LLVM_PROFILE_FILE")
	if path == "" {
		path = "default.proftext"
	}
	path = strings.ReplaceAll(path, "%p", strconv.Itoa(os.Getpid()))
	if err := writeProfile(path); err != nil {
		os.Stderr.WriteString("LLVM PGO: " + err.Error() + "\n")
	}
}

func writeProfile(path string) error {
	// Publish complete runs even when several processes select the same path.
	f, err := os.CreateTemp(filepath.Dir(path), ".llvmprofile-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	w := bufio.NewWriter(f)
	line := func(n uint64) {
		buf := strconv.AppendUint(w.AvailableBuffer(), n, 10)
		w.Write(append(buf, '\n'))
	}
	w.WriteString(":ir\n")
	for _, pkg := range packages {
		for _, r := range *pkg {
			w.WriteString(r.name)
			w.WriteByte('\n')
			line(r.hash)
			line(uint64(len(r.counters)))
			for i := range r.counters {
				line(atomic.LoadUint64(&r.counters[i]))
			}
		}
	}
	err = w.Flush()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
