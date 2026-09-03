// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestGoALLCSIMDPlanLookup(t *testing.T) {
	tests := []struct {
		name string
		want goALLCSIMDPlan
		ok   bool
	}{
		{"ConcatAddPairsInt16x8", goALLCSIMDPlanCompose, true},
		{"ConvertToInt32Float32x4", goALLCSIMDPlanConvert, true},
		{"PermuteUint8x16", goALLCSIMDPlanShuffle, true},
		{"blendInt8x16", goALLCSIMDPlanLegacy, true},
		{"blendInt8x32", goALLCSIMDPlanCompose, true},
		{"UnknownInt8x16", goALLCSIMDPlanInvalid, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := goALLCSIMDPlanForGenericOp(test.name)
			if got != test.want || ok != test.ok {
				t.Fatalf("goALLCSIMDPlanForGenericOp(%q) = (%s, %v), want (%s, %v)", test.name, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestGoALLCSIMDPlanCoversGeneratedOps(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "cmd", "compile", "internal", "ssa", "_gen", "simdgenericOps.go")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	counts := map[string]int{}
	seenFamilies := map[string]bool{}
	seenLegacy := map[string]bool{}
	var missing []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		nameStart := strings.Index(line, `{name: "`)
		archStart := strings.Index(line, "// ARCH:")
		if nameStart < 0 || archStart < 0 {
			continue
		}
		archs := strings.Fields(line[archStart+len("// ARCH:"):])[0]
		if !slices.Contains(strings.Split(archs, ","), "amd64") && !slices.Contains(strings.Split(archs, ","), "arm64") {
			continue
		}
		nameStart += len(`{name: "`)
		nameEnd := strings.IndexByte(line[nameStart:], '"')
		if nameEnd < 0 {
			t.Fatalf("malformed generated SIMD op line: %s", line)
		}
		name := line[nameStart : nameStart+nameEnd]
		if strings.Contains(line[:archStart], `simd: "`) {
			counts["implemented"]++
			continue
		}
		plan, ok := goALLCSIMDPlanForGenericOp(name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		counts[plan.String()]++
		if plan == goALLCSIMDPlanLegacy {
			seenLegacy[name] = true
		} else {
			family := goALLCSIMDTypeSuffixRE.ReplaceAllString(name, "")
			seenFamilies[family] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("generated amd64/arm64 SIMD ops lack a GoALLC lowering plan: %s", strings.Join(missing, ", "))
	}

	for family := range goALLCSIMDPlanByFamily {
		if !seenFamilies[family] {
			t.Errorf("GoALLC SIMD plan contains unused family %q", family)
		}
	}
	for name := range goALLCSIMDLegacyOps {
		if !seenLegacy[name] {
			t.Errorf("GoALLC SIMD legacy exception %q is no longer present", name)
		}
	}

	for _, status := range []string{"implemented", "standard", "compose", "convert", "shuffle", "mask", "shift", "target-intrinsic", "legacy"} {
		t.Logf("%-16s %d", status, counts[status])
	}
}
