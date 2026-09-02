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

// SIMDArchData records the architecture-specific part of a generic SIMD
// operation. The generic operation's semantics and shape live in SIMDOpData;
// these fields describe the instruction selection contract that produced it.
type SIMDArchData struct {
	CPUFeature        string
	CPUProfile        string
	OperandOrder      string
	Input             string
	Output            string
	Immediate         string
	Mask              string
	Inputs            string
	Outputs           string
	MemoryFeature     string
	MemoryFeatureData string
}

// SIMDOpData is the GoALLC lowering descriptor carried from simdgen through
// the generic SSA generator. Inputs and Outputs are compact, lossless shape
// descriptions of the operands relevant to generic lowering.
type SIMDOpData struct {
	Lowering  string
	Width     int
	Lane      string
	LaneBits  int
	Lanes     int
	Input     string
	Output    string
	Immediate string
	Mask      string
	Memory    string
	Inputs    string
	Outputs   string
	Arch      map[string]SIMDArchData
}

func (d SIMDOpData) IsZero() bool {
	return d.Width == 0 && d.Input == "" && d.Output == "" && len(d.Arch) == 0
}

// EqualGeneric reports whether two architecture implementations describe the
// same generic operation. Architecture-specific feature and operand-order
// details are intentionally excluded.
func (d SIMDOpData) EqualGeneric(other SIMDOpData) bool {
	return d.Lowering == other.Lowering &&
		d.Width == other.Width &&
		d.Lane == other.Lane &&
		d.LaneBits == other.LaneBits &&
		d.Lanes == other.Lanes &&
		d.Input == other.Input &&
		d.Output == other.Output &&
		d.Immediate == other.Immediate &&
		d.Mask == other.Mask &&
		d.Memory == other.Memory &&
		d.Inputs == other.Inputs &&
		d.Outputs == other.Outputs
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
	merged := left
	if left.Lowering != "" {
		if !left.EqualGeneric(right) {
			return SIMDOpData{}, fmt.Errorf("simdgen: LLVM-lowered op %q has inconsistent generic descriptors: existing=%q, new=%q", opName, EncodeSIMDOpData(left), EncodeSIMDOpData(right))
		}
	} else {
		// Some of Go 1.27's architecture-specific APIs intentionally share
		// one generic op name despite differing scalar/result or immediate
		// details. Preserve the exact facts in Arch and mark only the common
		// view as unknown where they differ.
		if merged.Width != right.Width {
			merged.Width = 0
		}
		if merged.Lane != right.Lane || merged.LaneBits != right.LaneBits || merged.Lanes != right.Lanes {
			merged.Lane, merged.LaneBits, merged.Lanes = "none", 0, 0
		}
		mergeString := func(dst *string, other, unknown string) {
			if *dst != other {
				*dst = unknown
			}
		}
		mergeString(&merged.Input, right.Input, "invalid")
		mergeString(&merged.Output, right.Output, "invalid")
		mergeString(&merged.Immediate, right.Immediate, "invalid")
		mergeString(&merged.Mask, right.Mask, "invalid")
		mergeString(&merged.Memory, right.Memory, "arch-dependent")
		mergeString(&merged.Inputs, right.Inputs, "")
		mergeString(&merged.Outputs, right.Outputs, "")
	}
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
	if len(copyArch) == 0 {
		return SIMDOpData{}
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
	v.Set("v", "1")
	v.Set("lower", d.Lowering)
	v.Set("width", strconv.Itoa(d.Width))
	v.Set("lane", d.Lane)
	v.Set("laneBits", strconv.Itoa(d.LaneBits))
	v.Set("lanes", strconv.Itoa(d.Lanes))
	v.Set("in", d.Input)
	v.Set("out", d.Output)
	v.Set("imm", d.Immediate)
	v.Set("mask", d.Mask)
	v.Set("mem", d.Memory)
	v.Set("inputs", d.Inputs)
	v.Set("outputs", d.Outputs)
	for arch, data := range d.Arch {
		prefix := "arch." + arch + "."
		v.Set(prefix+"cpu", data.CPUFeature)
		v.Set(prefix+"profile", data.CPUProfile)
		v.Set(prefix+"order", data.OperandOrder)
		v.Set(prefix+"in", data.Input)
		v.Set(prefix+"out", data.Output)
		v.Set(prefix+"imm", data.Immediate)
		v.Set(prefix+"mask", data.Mask)
		v.Set(prefix+"inputs", data.Inputs)
		v.Set(prefix+"outputs", data.Outputs)
		v.Set(prefix+"mem", data.MemoryFeature)
		v.Set(prefix+"memdata", data.MemoryFeatureData)
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
	if version := v.Get("v"); version != "1" {
		return SIMDOpData{}, fmt.Errorf("unsupported GoALLC SIMD descriptor version %q", version)
	}
	parseInt := func(name string) (int, error) {
		n, err := strconv.Atoi(v.Get(name))
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q: %w", name, v.Get(name), err)
		}
		return n, nil
	}
	width, err := parseInt("width")
	if err != nil {
		return SIMDOpData{}, err
	}
	laneBits, err := parseInt("laneBits")
	if err != nil {
		return SIMDOpData{}, err
	}
	lanes, err := parseInt("lanes")
	if err != nil {
		return SIMDOpData{}, err
	}
	d := SIMDOpData{
		Lowering:  v.Get("lower"),
		Width:     width,
		Lane:      v.Get("lane"),
		LaneBits:  laneBits,
		Lanes:     lanes,
		Input:     v.Get("in"),
		Output:    v.Get("out"),
		Immediate: v.Get("imm"),
		Mask:      v.Get("mask"),
		Memory:    v.Get("mem"),
		Inputs:    v.Get("inputs"),
		Outputs:   v.Get("outputs"),
		Arch:      make(map[string]SIMDArchData),
	}
	for key := range v {
		if !strings.HasPrefix(key, "arch.") || !strings.HasSuffix(key, ".cpu") {
			continue
		}
		arch := strings.TrimSuffix(strings.TrimPrefix(key, "arch."), ".cpu")
		prefix := "arch." + arch + "."
		d.Arch[arch] = SIMDArchData{
			CPUFeature:        v.Get(prefix + "cpu"),
			CPUProfile:        v.Get(prefix + "profile"),
			OperandOrder:      v.Get(prefix + "order"),
			Input:             v.Get(prefix + "in"),
			Output:            v.Get(prefix + "out"),
			Immediate:         v.Get(prefix + "imm"),
			Mask:              v.Get(prefix + "mask"),
			Inputs:            v.Get(prefix + "inputs"),
			Outputs:           v.Get(prefix + "outputs"),
			MemoryFeature:     v.Get(prefix + "mem"),
			MemoryFeatureData: v.Get(prefix + "memdata"),
		}
	}
	return d, nil
}
