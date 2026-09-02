// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sgutil

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SIMDArchData records the architecture-specific facts needed after generic
// SSA construction. Source instruction shapes stay in simdgen, where they are
// validated, rather than being copied into the compiler's generated tables.
type SIMDArchData struct {
	CPUProfile   string
	OperandOrder string
}

// SIMDOpData is the minimal GoALLC lowering descriptor carried from simdgen
// through the generic SSA generator. The Go SSA type carries vector width;
// lane kind and width select the operation semantics. opData already carries
// arity, Aux, and commutativity.
type SIMDOpData struct {
	Lowering string
	Lane     string
	LaneBits int
	Arch     map[string]SIMDArchData
}

func (d SIMDOpData) IsZero() bool {
	return d.Lowering == "" && len(d.Arch) == 0
}

// EqualGeneric reports whether two architecture implementations describe the
// same generic operation. Architecture-specific feature and operand-order
// details are intentionally excluded.
func (d SIMDOpData) EqualGeneric(other SIMDOpData) bool {
	return d.Lowering == other.Lowering &&
		d.Lane == other.Lane &&
		d.LaneBits == other.LaneBits
}

// MergeSIMDOpData combines architecture implementations of one generic op.
// A zero descriptor is accepted for operations that have no target-independent
// lowering annotation, while two populated descriptors must agree on every
// architecture-independent field.
func MergeSIMDOpData(opName string, left, right SIMDOpData) (SIMDOpData, error) {
	if left.IsZero() {
		return right, nil
	}
	if right.IsZero() {
		return left, nil
	}
	if left.Lowering != right.Lowering {
		return SIMDOpData{}, fmt.Errorf("simdgen: op %q has inconsistent GoALLC lowering kinds: existing=%q, new=%q", opName, left.Lowering, right.Lowering)
	}
	if !left.EqualGeneric(right) {
		return SIMDOpData{}, fmt.Errorf("simdgen: LLVM-lowered op %q has inconsistent generic descriptors: existing=%q, new=%q", opName, EncodeSIMDOpData(left), EncodeSIMDOpData(right))
	}
	merged := left
	if merged.Arch == nil {
		merged.Arch = make(map[string]SIMDArchData)
	}
	for arch, data := range right.Arch {
		if old, ok := merged.Arch[arch]; ok && old != data {
			return SIMDOpData{}, fmt.Errorf("simdgen: op %q has inconsistent %s GoALLC SIMD data: existing=%+v, new=%+v", opName, arch, old, data)
		}
		merged.Arch[arch] = data
	}
	return merged, nil
}

// WithoutArch removes data owned by arch while retaining the generic shape and
// other architecture implementations. It mirrors removal of an ARCH tag when
// a generator refreshes one architecture in a merged file.
func (d SIMDOpData) WithoutArch(arch string) SIMDOpData {
	if len(d.Arch) == 0 {
		return d
	}
	copyArch := make(map[string]SIMDArchData, len(d.Arch))
	for name, data := range d.Arch {
		if name != arch {
			copyArch[name] = data
		}
	}
	d.Arch = copyArch
	return d
}

// EncodeSIMDOpData returns a stable URL-query encoding. The generated Go file
// stores this as an ordinary quoted string so older merge logic can recognize
// entries without needing to parse Go composite literals.
func EncodeSIMDOpData(d SIMDOpData) string {
	if d.IsZero() {
		return ""
	}
	v := make(url.Values)
	v.Set("lower", d.Lowering)
	v.Set("lane", d.Lane)
	v.Set("laneBits", strconv.Itoa(d.LaneBits))
	for arch, data := range d.Arch {
		if data == (SIMDArchData{}) {
			continue
		}
		prefix := "arch." + arch + "."
		if data.CPUProfile != "" {
			v.Set(prefix+"profile", data.CPUProfile)
		}
		if data.OperandOrder != "" {
			v.Set(prefix+"order", data.OperandOrder)
		}
	}
	return v.Encode()
}

// DecodeSIMDOpData decodes the representation produced by
// EncodeSIMDOpData.
func DecodeSIMDOpData(encoded string) (SIMDOpData, error) {
	if encoded == "" {
		return SIMDOpData{}, nil
	}
	v, err := url.ParseQuery(encoded)
	if err != nil {
		return SIMDOpData{}, err
	}
	d := SIMDOpData{
		Lowering: v.Get("lower"),
		Lane:     v.Get("lane"),
		Arch:     make(map[string]SIMDArchData),
	}
	d.LaneBits, err = strconv.Atoi(v.Get("laneBits"))
	if err != nil {
		return SIMDOpData{}, fmt.Errorf("invalid laneBits %q: %w", v.Get("laneBits"), err)
	}
	seen := make(map[string]bool)
	for key := range v {
		if !strings.HasPrefix(key, "arch.") {
			continue
		}
		name := strings.TrimPrefix(key, "arch.")
		if i := strings.IndexByte(name, '.'); i >= 0 {
			seen[name[:i]] = true
		}
	}
	for arch := range seen {
		prefix := "arch." + arch + "."
		d.Arch[arch] = SIMDArchData{
			CPUProfile:   v.Get(prefix + "profile"),
			OperandOrder: v.Get(prefix + "order"),
		}
	}
	return d, nil
}
