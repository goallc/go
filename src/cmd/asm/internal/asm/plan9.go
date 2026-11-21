// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license.

package asm

import (
	"cmd/asm/internal/lex"
	"cmd/internal/obj"
	"strings"
	"text/scanner"
)

// Plan9Function separates object definitions and PC directives from executable
// source. Text is already macro-expanded; no source files are needed by codegen.
type Plan9Function struct {
	Sym       *obj.LSym `json:"-"`
	Frame     int64
	Args      int32
	Flags     int
	Start     Plan9Position
	Body      []string `json:"-"`
	Positions []Plan9Position
	Events    []Plan9SourceEvent
}

type Plan9Position struct {
	File string
	Line uint
}
type Plan9SourceEvent struct {
	At       int
	Text     string
	Position Plan9Position
}

// ParseBody supplies TEXT's already parsed context without putting an object
// definition in the inline-assembly string.
func (p *Parser) ParseBody(text *obj.Prog) (*obj.Prog, bool) {
	if text.As != obj.ATEXT {
		panic("ParseBody requires TEXT context")
	}
	p.firstProg, p.lastProg, p.pc = text, text, 1
	text.Pc = 1
	return p.Parse()
}

func (p *Parser) plan9Position() Plan9Position {
	pos := p.ctxt.PosTable.Pos(p.pos())
	return Plan9Position{pos.AbsFilename(), pos.RelLine()}
}

func (p *Parser) capturePlan9Label(name string) {
	if !p.CapturePlan9 || len(p.Plan9) == 0 {
		return
	}
	f := &p.Plan9[len(p.Plan9)-1]
	f.Body = append(f.Body, name+":")
	f.Positions = append(f.Positions, p.plan9Position())
}

func (p *Parser) capturePlan9(word, cond string, operands [][]lex.Token) {
	if !p.CapturePlan9 || p.errorCount != 0 {
		return
	}
	if word == "TEXT" {
		t := p.lastProg
		if t == nil || t.As != obj.ATEXT {
			return
		}
		flags := 0
		if len(operands) == 3 {
			flags = int(p.evalInteger("TEXT", operands[1]))
		}
		p.Plan9 = append(p.Plan9, Plan9Function{Sym: t.From.Sym, Frame: t.To.Offset,
			Args: t.To.Val.(int32), Flags: flags, Start: p.plan9Position()})
		return
	}
	if len(p.Plan9) == 0 || word == "DATA" || word == "GLOBL" {
		return
	}
	var b strings.Builder
	b.WriteString(word + cond)
	for i, operand := range operands {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteString(", ")
		}
		for j, token := range operand {
			if j != 0 {
				b.WriteByte(' ')
			}
			s := token.String()
			// lex.Make canonicalizes middle-dot and division-slash identifiers.
			// Restore their lexical spelling, without changing numeric/string tokens.
			if token.ScanToken == scanner.Ident {
				s = strings.NewReplacer(".", "·", "/", "∕").Replace(s)
			}
			b.WriteString(s)
		}
	}
	f := &p.Plan9[len(p.Plan9)-1]
	if word == "PCDATA" || word == "FUNCDATA" {
		f.Events = append(f.Events, Plan9SourceEvent{len(f.Body), b.String(), p.plan9Position()})
	} else {
		f.Body = append(f.Body, b.String())
		f.Positions = append(f.Positions, p.plan9Position())
	}
}
