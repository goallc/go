// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmasm

import (
	"cmd/internal/obj"
	a64 "cmd/internal/obj/arm64"
	"fmt"
	"strings"
)

func armreg(r int16, width int) (string, error) {
	prefix := "x"
	if width == 32 {
		prefix = "w"
	}
	switch {
	case r == a64.REGSP:
		if width == 32 {
			return "wsp", nil
		}
		return "sp", nil
	case r == a64.REGZERO:
		return prefix + "zr", nil
	case r >= a64.REG_R0 && r <= a64.REG_R30:
		return fmt.Sprintf("%s%d", prefix, r-a64.REG_R0), nil
	case r >= a64.REG_F0 && r <= a64.REG_F31:
		fp := "d"
		if width == 32 {
			fp = "s"
		}
		if width == 128 {
			fp = "q"
		}
		return fmt.Sprintf("%s%d", fp, r-a64.REG_F0), nil
	case r >= a64.REG_V0 && r <= a64.REG_V31:
		return fmt.Sprintf("q%d", r-a64.REG_V0), nil
	case r >= a64.REG_ARNG && r < a64.REG_ELEM:
		arrangements := []string{"8b", "16b", "4h", "8h", "2s", "4s", "1d", "2d", "b", "h", "s", "d", "1q", "q"}
		index := int(r>>5) & 15
		if index >= len(arrangements) {
			break
		}
		return fmt.Sprintf("v%d.%s", r&31, arrangements[index]), nil
	case r >= a64.REG_EXT && r < a64.REG_SPECIAL:
		ext := []string{"uxtb", "uxth", "uxtw", "uxtx", "sxtb", "sxth", "sxtw", "sxtx"}[(r>>8)&7]
		rp := "w"
		if strings.HasSuffix(ext, "x") {
			rp = "x"
		}
		return fmt.Sprintf("%s%d, %s #%d", rp, r&31, ext, (r>>5)&7), nil
	case r >= a64.REG_SPECIAL:
		return strings.ToLower(obj.Rconv(int(r))), nil
	}
	return "", fmt.Errorf("unsupported arm64 register %s", obj.Rconv(int(r)))
}

func (e *emitter) armaddr(a obj.Addr, width int, scond uint8) (string, error) {
	switch a.Type {
	case obj.TYPE_REG:
		return armreg(a.Reg, width)
	case obj.TYPE_CONST:
		return fmt.Sprintf("#%d", a.Offset), nil
	case obj.TYPE_BRANCH:
		return e.branch(a)
	case obj.TYPE_SPECIAL:
		return strings.ToLower(a64.SPCconv(a.Offset)), nil
	case obj.TYPE_SHIFT:
		r, err := armreg(a64.REG_R0+int16((a.Offset>>16)&31), width)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s, %s #%d", r, []string{"lsl", "lsr", "asr", "ror"}[(a.Offset>>22)&3], (a.Offset>>10)&63), nil
	case obj.TYPE_REGREG:
		r, err := armreg(a.Reg, width)
		if err != nil {
			return "", err
		}
		r2, err := armreg(int16(a.Offset), width)
		return r + ", " + r2, err
	case obj.TYPE_MEM:
		if a.Sym != nil && a.Name != obj.NAME_AUTO && a.Name != obj.NAME_PARAM {
			return "", fmt.Errorf("symbolic memory requires address materialization")
		}
		offset, reg := a.Offset, a.Reg
		frame := e.fn.Func().Text.To.Offset
		if a.Name == obj.NAME_AUTO {
			reg = a64.REGSP
			offset += int64(uint32(frame)) - int64(frame>>32)
		}
		if a.Name == obj.NAME_PARAM {
			reg = a64.REGSP
			offset += int64(uint32(frame)) + 8
		}
		base, err := armreg(reg, 64)
		if err != nil {
			return "", err
		}
		if a.Index != 0 {
			if offset != 0 {
				return "", fmt.Errorf("combined arm64 index and displacement is not supported")
			}
			idx, err := armreg(a.Index, 64)
			if err != nil {
				return "", err
			}
			return "[" + base + ", " + idx + "]", nil
		}
		switch scond {
		case 0:
			return fmt.Sprintf("[%s, #%d]", base, offset), nil
		case a64.C_XPRE:
			return fmt.Sprintf("[%s, #%d]!", base, offset), nil
		case a64.C_XPOST:
			return fmt.Sprintf("[%s], #%d", base, offset), nil
		}
	}
	return "", fmt.Errorf("unsupported arm64 operand %s", obj.Dconv(nil, &a))
}

func armImmediate(dst string, value uint64, width int) string {
	var out []string
	for shift := 0; shift < width; shift += 16 {
		part := (value >> shift) & 65535
		if part == 0 && len(out) > 0 {
			continue
		}
		op := "movk"
		if len(out) == 0 {
			op = "movz"
		}
		out = append(out, fmt.Sprintf("%s %s, #%d, lsl #%d", op, dst, part, shift))
	}
	return strings.Join(out, "\n")
}

func (e *emitter) armSymbolAddress(a obj.Addr, dst string) string {
	symbol := withOffset(e.ref(a.Sym, false), a.Offset)

	return fmt.Sprintf("adrp %s, %s\nadd %s, %s, :lo12:%s", dst, symbol, dst, dst, symbol)
}

func (e *emitter) armMove(p *obj.Prog) (string, error) {
	name := p.As.String()
	width := 64
	if name == "FMOVS" {
		width = 32
	}
	if name == "FMOVQ" {
		width = 128
	}
	if p.From.Type == obj.TYPE_CONST && p.To.Type == obj.TYPE_REG {
		// Immediate MOVW zero-extends 32 bits; register/memory MOVW sign-extends.
		// Do not apply the latter rule to constant materialization.
		switch name {
		case "MOVW", "MOVWU":
			width = 32
		case "MOVD":
		default:
			return "", fmt.Errorf("unsupported immediate move %s", name)
		}
		if name != "MOVD" && name != "MOVW" && name != "MOVWU" {
			return "", fmt.Errorf("unsupported immediate move %s", name)
		}
		if name != "MOVD" {
			width = 32
		}
		dst, err := armreg(p.To.Reg, width)
		if err != nil {
			return "", err
		}
		return armImmediate(dst, uint64(p.From.Offset), width), nil
	}
	if p.From.Type == obj.TYPE_ADDR {
		if name != "MOVD" {
			return "", fmt.Errorf("address move requires MOVD")
		}
		dst, err := armreg(p.To.Reg, 64)
		if err != nil {
			return "", err
		}
		if p.From.Sym != nil && p.From.Name != obj.NAME_AUTO && p.From.Name != obj.NAME_PARAM {
			return e.armSymbolAddress(p.From, dst), nil
		}
		mem := p.From
		mem.Type = obj.TYPE_MEM
		frame := e.fn.Func().Text.To.Offset
		if mem.Name == obj.NAME_AUTO {
			mem.Reg = a64.REGSP
			mem.Offset += int64(uint32(frame)) - int64(frame>>32)
		}
		if mem.Name == obj.NAME_PARAM {
			mem.Reg = a64.REGSP
			mem.Offset += int64(uint32(frame)) + 8
		}
		base, err := armreg(mem.Reg, 64)
		if err != nil {
			return "", err
		}
		if mem.Index != 0 || mem.Offset < 0 || mem.Offset > 4095 {
			return "", fmt.Errorf("unsupported arm64 address displacement")
		}
		return fmt.Sprintf("add %s, %s, #%d", dst, base, mem.Offset), nil
	}
	load := p.From.Type == obj.TYPE_MEM
	store := p.To.Type == obj.TYPE_MEM
	if load || store {
		mem, reg := p.From, p.To
		if store {
			mem, reg = p.To, p.From
		}
		op := "ldr"
		if store {
			op = "str"
		}
		switch name {
		case "MOVB":
			if load {
				op = "ldrsb"
			} else {
				op = "strb"
				width = 32
			}
		case "MOVBU":
			if load {
				op = "ldrb"
			} else {
				op = "strb"
			}
			width = 32
		case "MOVH":
			if load {
				op = "ldrsh"
			} else {
				op = "strh"
				width = 32
			}
		case "MOVHU":
			if load {
				op = "ldrh"
			} else {
				op = "strh"
			}
			width = 32
		case "MOVW":
			if load {
				op = "ldrsw"
			} else {
				width = 32
			}
		case "MOVWU":
			width = 32
		case "MOVD", "FMOVS", "FMOVD", "FMOVQ":
		default:
			return "", fmt.Errorf("unsupported memory move %s", name)
		}
		r, err := e.armaddr(reg, width, 0)
		if err != nil {
			return "", err
		}
		prefix := ""
		if mem.Sym != nil && mem.Name != obj.NAME_AUTO && mem.Name != obj.NAME_PARAM {
			if p.Scond != 0 || mem.Index != 0 || mem.Reg != 0 {
				return "", fmt.Errorf("unsupported symbolic arm64 memory mode")
			}
			if store && reg.Reg == a64.REGTMP {
				return "", fmt.Errorf("symbolic store would clobber R27")
			}
			prefix = e.armSymbolAddress(mem, "x27") + "\n"
			mem = obj.Addr{Type: obj.TYPE_MEM, Reg: a64.REGTMP}
		}
		m, err := e.armaddr(mem, 64, p.Scond)
		if err != nil {
			return "", err
		}
		return prefix + op + " " + r + ", " + m, nil
	}
	if p.Scond != 0 {
		return "", fmt.Errorf("writeback requires a memory operand")
	}
	dst, err := e.armaddr(p.To, width, 0)
	if err != nil {
		return "", err
	}
	sourceWidth := width
	op := "mov"
	switch name {
	case "MOVB":
		op = "sxtb"
		sourceWidth = 32
	case "MOVBU":
		op = "uxtb"
		sourceWidth = 32
		dst, err = e.armaddr(p.To, 32, 0)
	case "MOVH":
		op = "sxth"
		sourceWidth = 32
	case "MOVHU":
		op = "uxth"
		sourceWidth = 32
		dst, err = e.armaddr(p.To, 32, 0)
	case "MOVW":
		op = "sxtw"
		sourceWidth = 32
	case "MOVWU":
		sourceWidth = 32
		dst, err = e.armaddr(p.To, 32, 0)
	case "MOVD":
	case "FMOVS", "FMOVD":
		op = "fmov"
	default:
		return "", fmt.Errorf("unsupported register move %s", name)
	}
	if err != nil {
		return "", err
	}
	src, err := e.armaddr(p.From, sourceWidth, 0)
	if err != nil {
		return "", err
	}
	return op + " " + dst + ", " + src, nil
}

func (e *emitter) arm64(p *obj.Prog) (string, error) {
	name := p.As.String()
	if strings.HasPrefix(name, "MOV") || strings.HasPrefix(name, "FMOV") {
		return e.armMove(p)
	}
	if name == "RET" {
		if p.To.Sym != nil {
			v, err := e.branch(p.To)
			return "b " + v, err
		}
		if p.To.Reg != 0 {
			v, err := armreg(p.To.Reg, 64)
			return "ret " + v, err
		}
		return "ret", nil
	}
	if name == "WORD" || name == "DWORD" {
		if p.From.Type != obj.TYPE_CONST {
			return "", fmt.Errorf("literal word must be constant")
		}
		op := ".inst"
		if name == "DWORD" {
			op = ".quad"
		}
		return fmt.Sprintf("%s %d", op, p.From.Offset), nil
	}
	if name == "B" || name == "JMP" || name == "BL" || name == "CALL" {
		op := "b"
		if name == "BL" || name == "CALL" {
			op = "bl"
		}
		if p.To.Type == obj.TYPE_REG || p.To.Type == obj.TYPE_MEM && p.To.Sym == nil && p.To.Offset == 0 {
			r, err := armreg(p.To.Reg, 64)
			return op + "r " + r, err
		}
		v, err := e.branch(p.To)
		return op + " " + v, err
	}
	conditions := map[string]string{"BEQ": "eq", "BNE": "ne", "BCS": "hs", "BHS": "hs", "BCC": "lo", "BLO": "lo", "BMI": "mi", "BPL": "pl", "BVS": "vs", "BVC": "vc", "BHI": "hi", "BLS": "ls", "BGE": "ge", "BLT": "lt", "BGT": "gt", "BLE": "le"}
	if cond, ok := conditions[name]; ok {
		v, err := e.branch(p.To)
		return "b." + cond + " " + v, err
	}
	width := 64
	if strings.HasSuffix(name, "W") {
		width = 32
		name = strings.TrimSuffix(name, "W")
	}
	op := strings.ToLower(name)
	operand := func(a obj.Addr) (string, error) { return e.armaddr(a, width, p.Scond) }
	if name == "CBZ" || name == "CBNZ" {
		src, err := operand(p.From)
		if err != nil {
			return "", err
		}
		to, err := e.branch(p.To)
		return op + " " + src + ", " + to, err
	}
	if name == "TBZ" || name == "TBNZ" {
		reg, err := armreg(p.Reg, width)
		if err != nil {
			return "", err
		}
		to, err := e.branch(p.To)
		return fmt.Sprintf("%s %s, #%d, %s", op, reg, p.From.Offset, to), err
	}
	if name == "LDP" || name == "STP" || name == "FLDPD" || name == "FSTPD" || name == "FLDPS" || name == "FSTPS" {
		load := strings.Contains(name, "LDP")
		mem, pair := p.From, p.To
		if !load {
			mem, pair = p.To, p.From
		}
		if strings.HasSuffix(name, "S") {
			width = 32
		}
		regs, err := e.armaddr(pair, width, 0)
		if err != nil {
			return "", err
		}
		addr, err := e.armaddr(mem, 64, p.Scond)
		if err != nil {
			return "", err
		}
		op = "stp"
		if load {
			op = "ldp"
		}
		return op + " " + regs + ", " + addr, nil
	}
	if p.Scond != 0 {
		return "", fmt.Errorf("unsupported arm64 instruction suffix")
	}
	switch name {
	case "ADD", "ADDS", "SUB", "SUBS", "AND", "ANDS", "ORR", "EOR", "BIC", "BICS", "ORN", "EON", "ADC", "ADCS", "SBC", "SBCS", "LSL", "LSR", "ASR", "ROR", "MUL", "SDIV", "UDIV", "SMULH", "UMULH":
		dst, err := operand(p.To)
		if err != nil {
			return "", err
		}
		src, err := operand(p.From)
		if err != nil {
			return "", err
		}
		r := p.Reg
		if r == 0 {
			r = p.To.Reg
		}
		lhs, err := armreg(r, width)
		if err != nil {
			return "", err
		}
		if p.From.Type == obj.TYPE_CONST && p.From.Offset < 0 && (name == "ADD" || name == "SUB") {
			if name == "ADD" {
				op = "sub"
			} else {
				op = "add"
			}
			src = fmt.Sprintf("#%d", -p.From.Offset)
		}
		return op + " " + dst + ", " + lhs + ", " + src, nil
	case "CMP", "CMN", "TST":
		src, err := operand(p.From)
		if err != nil {
			return "", err
		}
		r := p.Reg
		if r == 0 {
			r = p.To.Reg
		}
		lhs, err := armreg(r, width)
		return op + " " + lhs + ", " + src, err
	case "NEG", "NEGS", "MVN", "CLZ", "CLS", "RBIT", "REV", "REV16", "REV32":
		dst, err := operand(p.To)
		if err != nil {
			return "", err
		}
		src, err := operand(p.From)
		return op + " " + dst + ", " + src, err
	case "NOOP":
		return "nop", nil
	case "YIELD", "WFE", "WFI", "SEV", "SEVL", "ISB":
		return op, nil
	case "SVC", "HVC", "SMC", "BRK", "HLT":
		return fmt.Sprintf("%s #%d", op, p.From.Offset), nil
	case "MRS":
		dst, err := operand(p.To)
		if err != nil {
			return "", err
		}
		src, err := operand(p.From)
		return "mrs " + dst + ", " + src, err
	case "MSR":
		dst, err := operand(p.To)
		if err != nil {
			return "", err
		}
		src, err := operand(p.From)
		return "msr " + dst + ", " + src, err
	}
	return "", fmt.Errorf("unsupported arm64 instruction %s", p.As)
}
