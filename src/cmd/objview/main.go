// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Objview inspects Go object files and package archives.
//
// Usage:
//
//	go tool objview [-format=text|raw|json] [-json] object-or-archive
//
// Text is the default and prints function-oriented disassembly with decoded Go
// metadata. Raw prints exact Go object bytes beside their decoded records.
// JSON prints the canonical structured representation.
// -json is a compatibility alias for -format=json.
package main

import (
	"cmd/internal/telemetry/counter"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	outputFormat = flag.String("format", "text", "output format: text, raw, or json")
	jsonOutput   = flag.Bool("json", false, "print canonical JSON (alias for -format=json)")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go tool objview [-format=text|raw|json] [-json] object-or-archive\n\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("objview: ")
	counter.Open()

	flag.Usage = usage
	flag.Parse()
	counter.Inc("objview/invocations")
	counter.CountFlags("objview/flag:", *flag.CommandLine)
	if flag.NArg() != 1 {
		usage()
	}
	format, err := selectOutputFormat(*outputFormat, *jsonOutput, formatWasSet())
	if err != nil {
		log.Fatal(err)
	}
	path := flag.Arg(0)
	switch format {
	case "text":
		err = writeTextFile(os.Stdout, path)
	case "raw":
		err = writeRawFile(os.Stdout, path)
	case "json":
		var doc *canonicalDocument
		doc, err = parseCanonicalFile(path)
		if err == nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			err = enc.Encode(doc)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

func formatWasSet() bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "format" {
			set = true
		}
	})
	return set
}

func selectOutputFormat(format string, json, formatSet bool) (string, error) {
	switch format {
	case "text", "raw", "json":
	default:
		return "", fmt.Errorf("invalid -format %q: want text, raw, or json", format)
	}
	if json {
		if formatSet && format != "json" {
			return "", fmt.Errorf("-json conflicts with -format=%s", format)
		}
		return "json", nil
	}
	return format, nil
}
