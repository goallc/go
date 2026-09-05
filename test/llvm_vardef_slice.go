// run

// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type freshness string

// With the receiver and group string, this seven-word aggregate exceeds
// amd64's argument registers and is passed in memory.
type discovery struct {
	version   string
	resources []*int
	freshness freshness
}

type manager interface {
	add(string, discovery)
}

type receiver struct{ calls int }

//go:noinline
func (r *receiver) add(group string, value discovery) {
	if group != "group" || value.freshness != "Stale" {
		panic("incorrect string argument")
	}
	if value.version == "v1" {
		if value.resources != nil || len(value.resources) != 0 || cap(value.resources) != 0 {
			panic("zero aggregate contains a corrupted slice header")
		}
	} else if value.version != "poison" || len(value.resources) != 1 || *value.resources[0] != 42 {
		panic("incorrect populated aggregate")
	}
	r.calls++
}

//go:noinline
func update(m manager, cached *map[string]discovery, version string) {
	var entry discovery
	if cached == nil {
		// SSA can reuse the declaration's zero bytes for resources. VarDef
		// at this assignment must not invalidate that earlier initialization.
		entry = discovery{version: version}
	} else {
		var ok bool
		entry, ok = (*cached)[version]
		if !ok {
			entry = discovery{version: version}
		}
	}
	entry.freshness = "Stale"
	m.add("group", entry)
}

func main() {
	n := 42
	values := map[string]discovery{"v1": {version: "poison", resources: []*int{&n}}}
	empty := map[string]discovery{}
	r := new(receiver)
	for i := 0; i < 100; i++ {
		// First populate the frame, then exercise both zero-value paths.
		update(r, &values, "v1")
		update(r, nil, "v1")
		update(r, &empty, "v1")
	}
	if r.calls != 300 {
		panic("missing interface calls")
	}
}
