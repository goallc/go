// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// copyGoallcLLVMHeaders prepares cgo's include directory without requiring
// Windows symlink privileges. Only a directory marked as our generated output
// may be replaced. The installed compiler does not depend on this directory.
func copyGoallcLLVMHeaders(destination, payload string) error {
	const marker = ".goallc-headers"
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s exists and is not an owned header directory", destination)
		}
		if _, err := os.Stat(filepath.Join(destination, marker)); err != nil {
			return fmt.Errorf("%s exists without the generated-header marker", destination)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Dir(destination), ".goallc-headers-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.CopyFS(filepath.Join(temporary, "include"), os.DirFS(filepath.Join(payload, "include"))); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, marker), []byte(payload+"\n"), 0644); err != nil {
		return err
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(temporary, destination)
}
