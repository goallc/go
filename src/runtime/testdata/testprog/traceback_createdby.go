// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "runtime"

func init() {
	register("TracebackCreatedBy", TracebackCreatedBy)
}

type tracebackReporter chan string

func (r tracebackReporter) start() {
	go r.collect()
}

func (r tracebackReporter) collect() {
	go func() {
		buf := make([]byte, 16<<10)
		r <- string(buf[:runtime.Stack(buf, false)])
	}()
}

func TracebackCreatedBy() {
	r := make(tracebackReporter)
	go r.start()
	print(<-r)
}
