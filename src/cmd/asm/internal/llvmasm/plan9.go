// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

//go:build !compiler_bootstrap

package llvmasm

import (
	"cmd/asm/internal/arch"
	"cmd/asm/internal/asm"
	"cmd/asm/internal/lex"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"internal/buildcfg"
	"strings"
)

type plan9Symbol struct {
	Name   string
	ABI    obj.ABI
	Static bool
	Text   bool
}

// This is a versioned function context, not an alternative instruction IR.
// Instructions remain readable Plan 9 in the inline asm template. LLVM operands
// carry all symbol identities; this table connects original names to operands.
type plan9Context struct {
	Toolchain, Experiments string
	AMD64                  int
	ARM64                  string
	Version                int
	GOOS, GOARCH, Package  string
	Shared, Retpoline      bool
	MayMoreStack           string
	Function               plan9Symbol
	Source                 asm.Plan9Function
	References             []plan9Symbol
}

func plan9Key(s *obj.LSym, text bool) plan9Symbol {
	return plan9Symbol{s.Name, s.ABI(), s.Static(), text}
}

func lowerPlan9(ctxt *obj.Link, options Options) ([]function, error) {
	if options.GOOS != "linux" && options.GOOS != "darwin" {
		return nil, fmt.Errorf("Plan 9 LLVM carrier does not support %s", options.GOOS)
	}
	if options.GOARCH != "amd64" && options.GOARCH != "arm64" {
		return nil, fmt.Errorf("Plan 9 LLVM carrier does not support %s", options.GOARCH)
	}
	if ctxt.Flag_dynlink || ctxt.Flag_linkshared {
		return nil, fmt.Errorf("Plan 9 carrier does not support shared-library symbol binding")
	}
	var functions []function
	sources := make(map[*obj.LSym]asm.Plan9Function)
	for _, source := range options.Plan9 {
		sources[source.Sym] = source
	}
	for _, s := range ctxt.Text {
		source, ok := sources[s]
		if !ok {
			return nil, fmt.Errorf("missing Plan 9 source for %s", s.Name)
		}
		if len(s.P) != 0 {
			return nil, fmt.Errorf("Plan 9 source carrier received pre-encoded function %s", s.Name)
		}
		f := function{sym: s, body: strings.Join(source.Body, "\n") + "\n"}
		seen := make(map[*obj.LSym]int)
		add := func(sym *obj.LSym, text bool) {
			if sym == nil {
				return
			}
			text = text || sym.ABI() == obj.ABIInternal || sym.Type == objabi.STEXT || sym.Type == objabi.STEXTFIPS
			if i, ok := seen[sym]; ok {
				f.refs[i].text = f.refs[i].text || text
				return
			}
			seen[sym] = len(f.refs)
			f.refs = append(f.refs, reference{sym, text})
		}
		add(s, true)
		addAddr := func(a obj.Addr, text bool) {
			// FP/SP variable names describe slots, not linkable global symbols.
			if a.Name != obj.NAME_AUTO && a.Name != obj.NAME_PARAM {
				add(a.Sym, text)
			}
		}
		for p := s.Func().Text; p != nil; p = p.Link {
			addAddr(p.From, false)
			addAddr(p.To, p.As == obj.ACALL || p.As == obj.AJMP || p.To.Type == obj.TYPE_BRANCH)
			for _, a := range p.RestArgs {
				addAddr(a.Addr, false)
			}
		}
		c := plan9Context{Version: 1, Toolchain: buildcfg.Version, Experiments: strings.Join(buildcfg.Experiment.Enabled(), ","), AMD64: buildcfg.GOAMD64, ARM64: buildcfg.GOARM64.String(), GOOS: options.GOOS, GOARCH: options.GOARCH, Package: ctxt.Pkgpath,
			Shared: ctxt.Flag_shared, Retpoline: ctxt.Retpoline, MayMoreStack: ctxt.Flag_maymorestack,
			Function: plan9Key(s, true), Source: source}
		for _, r := range f.refs {
			c.References = append(c.References, plan9Key(r.sym, r.text))
		}
		data, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		f.plan9Context = string(data)
		functions = append(functions, f)
	}
	return functions, nil
}

type plan9Event struct {
	PC    int64  `json:"pc"`
	Kind  int64  `json:"kind"`
	Value int64  `json:"value"`
	File  string `json:"file"`
}
type plan9Reloc struct {
	Offset int32            `json:"offset"`
	Size   uint8            `json:"size"`
	Type   objabi.RelocType `json:"type"`
	Addend int64            `json:"addend"`
	Target int              `json:"target"`
}
type plan9Funcdata struct {
	Index  int64 `json:"index"`
	Target int   `json:"target"`
}
type plan9Result struct {
	Code     string          `json:"code"`
	Args     int32           `json:"args"`
	Locals   int32           `json:"locals"`
	ID       uint8           `json:"id"`
	Flags    uint8           `json:"flags"`
	Line     int32           `json:"line"`
	Events   []plan9Event    `json:"events"`
	Relocs   []plan9Reloc    `json:"relocs"`
	Funcdata []plan9Funcdata `json:"funcdata"`
}

func assemblePlan9(request []byte) ([]byte, error) {
	var input struct{ Body, Context string }
	if err := json.Unmarshal(request, &input); err != nil {
		return nil, err
	}
	var c plan9Context
	if err := json.Unmarshal([]byte(input.Context), &c); err != nil {
		return nil, err
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("unsupported Plan 9 context version %d", c.Version)
	}
	if c.GOOS != "linux" && c.GOOS != "darwin" {
		return nil, fmt.Errorf("unsupported Plan 9 OS %s", c.GOOS)
	}
	if c.GOARCH != "amd64" && c.GOARCH != "arm64" {
		return nil, fmt.Errorf("unsupported Plan 9 target %s", c.GOARCH)
	}
	if c.Toolchain != buildcfg.Version {
		return nil, fmt.Errorf("Plan 9 carrier requires its originating Go toolchain version")
	}
	experiments, err := buildcfg.ParseGOEXPERIMENT(c.GOOS, c.GOARCH, "none,"+c.Experiments)
	if err != nil {
		return nil, err
	}
	if c.AMD64 < 1 || c.AMD64 > 4 {
		return nil, fmt.Errorf("invalid Plan 9 GOAMD64 setting")
	}
	arm64, err := buildcfg.ParseGoarm64(c.ARM64)
	if err != nil {
		return nil, err
	}
	oldOS, oldArch := buildcfg.GOOS, buildcfg.GOARCH
	oldAMD64, oldARM64 := buildcfg.GOAMD64, buildcfg.GOARM64
	oldExperiments := buildcfg.Experiment
	buildcfg.GOOS, buildcfg.GOARCH = c.GOOS, c.GOARCH
	buildcfg.GOAMD64, buildcfg.GOARM64 = c.AMD64, arm64
	buildcfg.Experiment = *experiments
	defer func() {
		buildcfg.GOOS, buildcfg.GOARCH = oldOS, oldArch
		buildcfg.GOAMD64, buildcfg.GOARM64 = oldAMD64, oldARM64
		buildcfg.Experiment = oldExperiments
	}()
	architecture := arch.Set(c.GOARCH, c.Shared)
	ctxt := obj.Linknew(architecture.LinkArch)
	ctxt.Pkgpath, ctxt.IsAsm = c.Package, true
	ctxt.Flag_shared, ctxt.Retpoline, ctxt.Flag_maymorestack = c.Shared, c.Retpoline, c.MayMoreStack
	var diagnostics []string
	ctxt.DiagFunc = func(format string, args ...any) { diagnostics = append(diagnostics, fmt.Sprintf(format, args...)) }
	architecture.Init(ctxt)
	lookup := func(k plan9Symbol) *obj.LSym {
		if k.Static {
			return ctxt.LookupStatic(k.Name)
		}
		return ctxt.LookupABI(k.Name, k.ABI)
	}
	refs := make(map[*obj.LSym]int)
	for i, k := range c.References {
		s := lookup(k)
		if k.Text {
			s.Type = objabi.STEXT
		}
		refs[s] = i
	}
	sym := lookup(c.Function)
	position := func(p asm.Plan9Position) src.XPos {
		return ctxt.PosTable.XPos(src.MakePos(src.NewFileBase(p.File, p.File), p.Line, 0))
	}
	ctxt.InitTextSym(sym, c.Source.Flags, position(c.Source.Start))
	text := ctxt.NewProg()
	text.As, text.Pos, text.From.Sym = obj.ATEXT, position(c.Source.Start), sym
	text.From.Type, text.From.Name = obj.TYPE_MEM, obj.NAME_EXTERN
	text.To.Type, text.To.Offset, text.To.Val = obj.TYPE_TEXTSIZE, c.Source.Frame, c.Source.Args
	sym.Func().Text = text
	lines := strings.Split(strings.TrimSuffix(input.Body, "\n"), "\n")
	if len(lines) != len(c.Source.Positions) {
		return nil, fmt.Errorf("Plan 9 source position count does not match body")
	}
	var source strings.Builder
	positions := []asm.Plan9Position{{}}
	events := 0
	for i := 0; i <= len(lines); i++ {
		for events < len(c.Source.Events) && c.Source.Events[events].At == i {
			event := c.Source.Events[events]
			source.WriteString(event.Text + "\n")
			positions = append(positions, event.Position)
			events++
		}
		if i < len(lines) {
			source.WriteString(lines[i] + "\n")
			positions = append(positions, c.Source.Positions[i])
		}
	}
	if events != len(c.Source.Events) {
		return nil, fmt.Errorf("invalid Plan 9 event boundary")
	}
	parser := asm.NewParser(ctxt, architecture, lex.NewTokenizer("<plan9-inline>", strings.NewReader(source.String()), nil))
	first, ok := parser.ParseBody(text)
	if !ok {
		return nil, fmt.Errorf("Plan 9 function body did not parse")
	}
	if len(ctxt.Text) != 1 || len(ctxt.Data) != 0 {
		return nil, fmt.Errorf("Plan 9 inline body contains object definitions")
	}
	for p := first.Link; p != nil; p = p.Link {
		line := ctxt.PosTable.Pos(p.Pos).RelLine()
		if line == 0 || int(line) >= len(positions) {
			return nil, fmt.Errorf("invalid Plan 9 source position")
		}
		p.Pos = position(positions[line])
	}
	obj.PrepareText(ctxt, &obj.Plist{Firstpc: first}, nil)
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}
	ctxt.Arch.Assemble(ctxt, sym, ctxt.NewProg)
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("%s", strings.Join(diagnostics, "\n"))
	}
	target := func(s *obj.LSym) (int, error) {
		if s == nil {
			return -1, nil
		}
		i, ok := refs[s]
		if !ok {
			return 0, fmt.Errorf("late assembly introduced undeclared symbol %s (ABI %d)", s.Name, s.ABI())
		}
		return i, nil
	}
	info := sym.Func()
	result := plan9Result{Code: hex.EncodeToString(sym.P), Args: info.Args, Locals: info.Locals,
		ID: uint8(info.FuncID), Flags: uint8(info.FuncFlag), Line: info.StartLine,
		Events: []plan9Event{}, Relocs: []plan9Reloc{}, Funcdata: []plan9Funcdata{}}
	for _, r := range sym.R {
		i, err := target(r.Sym)
		if err != nil {
			return nil, err
		}
		result.Relocs = append(result.Relocs, plan9Reloc{r.Off, r.Siz, r.Type, r.Add, i})
	}
	sp := int64(0)
	seenData := map[int64]bool{}
	for p := info.Text; p != nil; p = p.Link {
		if p.Spadj != 0 {
			after := sym.Size
			if p.Link != nil {
				after = p.Link.Pc
			}
			sp += int64(p.Spadj)
			result.Events = append(result.Events, plan9Event{PC: after, Kind: -3, Value: sp})
		}
		switch p.As {
		case obj.APCDATA:
			result.Events = append(result.Events, plan9Event{PC: p.Pc, Kind: p.From.Offset, Value: p.To.Offset})
		case obj.AFUNCDATA:
			if p.From.Offset < 0 || p.From.Offset > 255 || p.To.Offset != 0 || seenData[p.From.Offset] {
				return nil, fmt.Errorf("invalid or duplicate FUNCDATA")
			}
			seenData[p.From.Offset] = true
			i, err := target(p.To.Sym)
			if err != nil {
				return nil, err
			}
			result.Funcdata = append(result.Funcdata, plan9Funcdata{p.From.Offset, i})
		}
		pos := ctxt.PosTable.Pos(p.Pos)
		if p.As != obj.ATEXT && p.As != obj.ANOP && pos.RelLine() != 0 {
			result.Events = append(result.Events, plan9Event{PC: p.Pc, Kind: -2, Value: int64(pos.RelLine()), File: pos.AbsFilename()})
		}
	}
	return json.Marshal(result)
}
