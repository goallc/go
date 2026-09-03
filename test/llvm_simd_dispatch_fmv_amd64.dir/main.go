// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const dispatchFMVChild = "GOALLC_SIMD_FMV_CHILD"

func findDispatchFMVCase(name string) (dispatchFMVCase, bool) {
	for _, test := range dispatchFMVCases {
		if test.name == name {
			return test, true
		}
	}
	return dispatchFMVCase{}, false
}

func dispatchFMVChildEnv(test dispatchFMVCase) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GODEBUG=") && !strings.HasPrefix(entry, dispatchFMVChild+"=") {
			env = append(env, entry)
		}
	}
	return append(env, "GODEBUG="+test.godebug, dispatchFMVChild+"="+test.name)
}

func main() {
	if name := os.Getenv(dispatchFMVChild); name != "" {
		test, ok := findDispatchFMVCase(name)
		if !ok {
			panic("unknown child case " + name)
		}
		checkDispatchFMVCase(test)
		return
	}

	sort.Slice(dispatchFMVCases, func(i, j int) bool {
		return dispatchFMVCases[i].order < dispatchFMVCases[j].order
	})
	wanted := make(map[string]bool)
	for _, name := range os.Args[1:] {
		if _, ok := findDispatchFMVCase(name); !ok {
			panic("unknown case " + name)
		}
		wanted[name] = true
	}
	explicit := len(wanted) != 0

	for _, test := range dispatchFMVCases {
		if explicit && !wanted[test.name] {
			continue
		}
		if !test.support.available() {
			if explicit {
				panic(test.name + " requires " + test.support.String())
			}
			if os.Getenv("GOALLC_SIMD_FMV_PRINT") == "1" {
				fmt.Printf("%-24s skipped: requires %s\n", test.name, test.support)
			}
			continue
		}

		cmd := exec.Command(os.Args[0])
		cmd.Env = dispatchFMVChildEnv(test)
		output, err := cmd.CombinedOutput()
		if err != nil {
			panic("GODEBUG=" + test.godebug + ": " + err.Error() + ": " + string(output))
		}
		if os.Getenv("GOALLC_SIMD_FMV_PRINT") == "1" {
			fmt.Print(string(output))
		}
	}
}
