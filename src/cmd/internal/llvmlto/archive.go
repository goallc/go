// Copyright 2026 The GoALLC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

// Package llvmlto implements the linker's package-preserving LTO phase.
package llvmlto

import (
	"cmd/internal/archive"
	"cmd/internal/goobj"
	"cmd/internal/objabi"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const irMember = "__.LLVMBC"

// Prepare replaces bitcode archives in an isolated directory. The caller must
// keep that directory until the ordinary Go linker has finished loading them.
// No cache input is modified, including on failure.
func Prepare(main string, packages map[string]string, keep, pic bool, mode string) (string, map[string]string, *goobj.LTOManifest, func(), error) {
	work, err := os.MkdirTemp("", "go-lto-")
	if err != nil {
		return "", nil, nil, func() {}, err
	}
	cleanup := func() { os.RemoveAll(work) }
	if keep {
		fmt.Fprintln(os.Stderr, "LTO_WORK="+work)
		cleanup = func() {}
	}
	paths := map[string]bool{main: true}
	for _, path := range packages {
		paths[path] = true
	}
	var names []string
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	var inputs, originals []string
	for _, path := range names {
		ir, err := readIR(path)
		if err != nil {
			return "", nil, nil, cleanup, err
		}
		if ir == nil {
			continue
		}
		input := filepath.Join(work, fmt.Sprintf("%d.input.bc", len(inputs)))
		if err := os.WriteFile(input, ir, 0600); err != nil {
			return "", nil, nil, cleanup, err
		}
		inputs = append(inputs, input)
		originals = append(originals, path)
	}
	if len(inputs) == 0 {
		return "", nil, nil, cleanup, fmt.Errorf("LTO requires packages compiled with -llvm-lto")
	}
	out := filepath.Join(work, "optimized")
	if err := optimize(inputs, out, keep, mode); err != nil {
		return "", nil, nil, cleanup, err
	}
	var manifest *goobj.LTOManifest
	if mode == "full" {
		data, err := os.ReadFile(filepath.Join(out, "program.json"))
		if err != nil {
			return "", nil, nil, cleanup, err
		}
		manifest = new(goobj.LTOManifest)
		if err := json.Unmarshal(data, manifest); err != nil {
			return "", nil, nil, cleanup, err
		}
		if manifest.Version != 1 || len(manifest.Packages) != len(originals) || manifest.Main < 0 || manifest.Main >= len(originals) || originals[manifest.Main] != main {
			return "", nil, nil, cleanup, fmt.Errorf("invalid single-object LTO package manifest")
		}
		for i := range manifest.Packages {
			p := &manifest.Packages[i]
			if i == manifest.Main {
				p.Path = "main"
			} else {
				for name, path := range packages {
					if path == originals[i] {
						p.Path = name
						break
					}
				}
			}
			fp, err := hex.DecodeString(p.Fingerprint)
			if p.Path == "" || objabi.PathToPrefix(p.Path) != p.Prefix || err != nil || len(fp) != len(goobj.FingerprintType{}) {
				return "", nil, nil, cleanup, fmt.Errorf("invalid LTO identity for package %q", p.Prefix)
			}
		}
		if err := emit(filepath.Join(out, "program.bc"), filepath.Join(work, "program.o"), pic); err != nil {
			return "", nil, nil, cleanup, err
		}
		if err := finalizeSingleObject(filepath.Join(work, "program.o"), filepath.Join(out, "program.json"), manifest); err != nil {
			return "", nil, nil, cleanup, err
		}
	}
	replacements := make(map[string]string)
	for i, path := range originals {
		object := ""
		if manifest != nil {
			if i == manifest.Main {
				object = filepath.Join(work, "program.o")
			}
		} else {
			object = filepath.Join(work, fmt.Sprintf("%d.o", i))
			if err := emit(filepath.Join(out, fmt.Sprintf("%d.bc", i)), object, pic); err != nil {
				return "", nil, nil, cleanup, err
			}
		}
		archive := filepath.Join(work, fmt.Sprintf("%d.a", i))
		if err := rewriteArchive(path, archive, object); err != nil {
			return "", nil, nil, cleanup, err
		}
		replacements[path] = archive
	}
	result := make(map[string]string, len(packages))
	for name, path := range packages {
		if replacement := replacements[path]; replacement != "" {
			path = replacement
		}
		result[name] = path
	}
	if replacement := replacements[main]; replacement != "" {
		main = replacement
	}
	if manifest != nil {
		manifest.Archive = main
	}
	return main, result, manifest, cleanup, nil
}

func readIR(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	a, err := archive.Parse(f, false)
	if err != nil {
		return nil, err
	}
	var ir []byte
	for _, e := range a.Entries {
		if e.Name != irMember {
			continue
		}
		if ir != nil {
			return nil, fmt.Errorf("%s: duplicate %s member", path, irMember)
		}
		if e.Size == 0 {
			return nil, fmt.Errorf("%s: empty %s member", path, irMember)
		}
		ir, err = io.ReadAll(io.NewSectionReader(f, e.Offset, e.Size))
		if err != nil {
			return nil, err
		}
		if int64(len(ir)) != e.Size {
			return nil, io.ErrUnexpectedEOF
		}
	}
	return ir, nil
}

// writeArchiveEntry uses the same header as cmd/pack, but propagates write
// errors instead of archive.AddEntry's process-wide log.Fatal path.
func writeArchiveEntry(out io.Writer, name string, size int64, content io.Reader) error {
	var header [archive.HeaderSize]byte
	archive.FormatHeader(header[:], name, size)
	if n, err := out.Write(header[:]); err != nil {
		return err
	} else if n != len(header) {
		return io.ErrShortWrite
	}
	if _, err := io.CopyN(out, content, size); err != nil {
		return err
	}
	if size&1 != 0 {
		if n, err := out.Write([]byte{0}); err != nil {
			return err
		} else if n != 1 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func rewriteArchive(src, dst, object string) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	a, err := archive.Parse(f, false)
	if err != nil {
		return err
	}
	count, bitcode := 0, 0
	for _, e := range a.Entries {
		if e.Name == irMember {
			bitcode++
		}
		if e.Name == "_go_.o" {
			count++
		}
	}
	if count != 0 {
		return fmt.Errorf("%s: expected a slim bitcode archive with no compiler GoObj member, found %d", src, count)
	}
	if bitcode != 1 {
		return fmt.Errorf("%s: expected one bitcode member, found %d", src, bitcode)
	}
	var obj *os.File
	var objectSize int64
	if object != "" {
		obj, err = os.Open(object)
		if err != nil {
			return err
		}
		defer obj.Close()
		info, err := obj.Stat()
		if err != nil {
			return err
		}
		objectSize = info.Size()
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(dst)
		}
	}()
	if _, err := archive.New(out); err != nil {
		return err
	}
	for _, e := range a.Entries {
		if e.Name == irMember {
			if obj == nil {
				continue
			}
			if err := writeArchiveEntry(out, "_go_.o", objectSize, obj); err != nil {
				return err
			}
		} else {
			// Preserve the complete member, including its header and padding.
			size := archive.HeaderSize + e.Size + (e.Size & 1)
			if _, err := io.CopyN(out, io.NewSectionReader(f, e.Offset-archive.HeaderSize, size), size); err != nil {
				return err
			}
		}
	}
	return nil
}
