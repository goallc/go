// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package llvmasm

import (
	"cmd/internal/obj"
	"cmd/internal/obj/x86"
	"fmt"
	"strings"
)

func x86reg(r int16, width int) (string, error) {
	if r >= x86.REG_AX && r <= x86.REG_R15 {
		i := int(r - x86.REG_AX)
		names := map[int][]string{
			8:  {"al", "cl", "dl", "bl", "spl", "bpl", "sil", "dil", "r8b", "r9b", "r10b", "r11b", "r12b", "r13b", "r14b", "r15b"},
			16: {"ax", "cx", "dx", "bx", "sp", "bp", "si", "di", "r8w", "r9w", "r10w", "r11w", "r12w", "r13w", "r14w", "r15w"},
			32: {"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi", "r8d", "r9d", "r10d", "r11d", "r12d", "r13d", "r14d", "r15d"},
			64: {"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi", "r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15"},
		}
		return "%" + names[width][i], nil
	}
	name := strings.ToLower(obj.Rconv(int(r)))
	for _, spec := range []struct {
		lo, hi int16
		prefix string
	}{{x86.REG_X0, x86.REG_X31, "xmm"}, {x86.REG_Y0, x86.REG_Y31, "ymm"}, {x86.REG_Z0, x86.REG_Z31, "zmm"}, {x86.REG_F0, x86.REG_F7, "st"}} {
		if r >= spec.lo && r <= spec.hi {
			if spec.prefix == "st" {
				return fmt.Sprintf("%%st(%d)", r-spec.lo), nil
			}
			return fmt.Sprintf("%%%s%d", spec.prefix, r-spec.lo), nil
		}
	}
	if r >= x86.REG_AL && r <= x86.REG_R15B || r >= x86.REG_AH && r <= x86.REG_BH || r >= x86.REG_K0 && r <= x86.REG_K7 || r >= x86.REG_M0 && r <= x86.REG_M7 || name == "cs" || name == "ss" || name == "ds" || name == "es" || name == "fs" || name == "gs" {
		switch name {
		case "spb":
			name = "spl"
		case "bpb":
			name = "bpl"
		case "sib":
			name = "sil"
		case "dib":
			name = "dil"
		}
		return "%" + name, nil
	}
	return "", fmt.Errorf("unsupported x86 register %s", obj.Rconv(int(r)))
}

func (e *emitter) x86addr(a obj.Addr, width int, branch, lea bool) (string, error) {
	if a.Type == obj.TYPE_BRANCH {
		return e.branch(a)
	}
	switch a.Type {
	case obj.TYPE_REG:
		v, err := x86reg(a.Reg, width)
		if branch {
			v = "*" + v
		}
		return v, err
	case obj.TYPE_CONST:
		return fmt.Sprintf("$$%d", a.Offset), nil // escape the LLVM operand introducer
	case obj.TYPE_ADDR, obj.TYPE_MEM:
		if branch && a.Sym != nil && a.Reg == 0 && a.Index == 0 {
			return withOffset(e.ref(a.Sym, true), a.Offset), nil
		}
		addr := ""
		if a.Sym != nil && a.Name != obj.NAME_AUTO && a.Name != obj.NAME_PARAM {
			addr = e.ref(a.Sym, false)
		}
		if addr != "" {
			addr = withOffset(addr, a.Offset)
		} else if a.Offset != 0 {
			addr = fmt.Sprint(a.Offset)
		}
		if a.Name == obj.NAME_GOTREF {
			addr += "@GOTPCREL"
		}
		reg := a.Reg
		if a.Name == obj.NAME_AUTO || a.Name == obj.NAME_PARAM {
			reg = x86.REG_SP
		}
		if reg == x86.REG_TLS {
			segment := "%fs:"
			if e.options.GOOS == "darwin" {
				segment = "%gs:"
			}
			tls := e.ctxt.Lookup("runtime.tlsg")
			return segment + withOffset(e.ref(tls, false)+"@TPOFF", a.Offset), nil
		}
		if a.Type == obj.TYPE_ADDR && reg == 0 && a.Index == 0 && !lea {
			return "$$" + addr, nil
		}
		base := ""
		if reg != 0 {
			var err error
			base, err = x86reg(reg, 64)
			if err != nil {
				return "", err
			}
		}
		if a.Sym != nil && a.Name != obj.NAME_AUTO && a.Name != obj.NAME_PARAM && reg == 0 {
			base = "%rip"
		}
		if a.Index != 0 {
			index, err := x86reg(a.Index, 64)
			if err != nil {
				return "", err
			}
			if a.Scale == 0 {
				return "", fmt.Errorf("unsupported x86 register pair/address")
			}
			addr += fmt.Sprintf("(%s,%s,%d)", base, index, a.Scale)
		} else if base != "" {
			addr += "(" + base + ")"
		}
		if addr == "" {
			addr = "0"
		}
		if branch {
			addr = "*" + addr
		}
		return addr, nil
	}
	return "", fmt.Errorf("unsupported x86 operand %s", obj.Dconv(nil, &a))
}

func (e *emitter) x86(p *obj.Prog) (string, error) {
	name := p.As.String()
	if p.Scond != 0 {
		return "", fmt.Errorf("x86 instruction suffix is not implemented")
	}
	if name == "ADJSP" {
		if p.From.Offset == 0 {
			return "", nil
		}
		if p.From.Offset > 0 {
			return fmt.Sprintf("subq $$%d, %%rsp", p.From.Offset), nil
		}
		return fmt.Sprintf("addq $$%d, %%rsp", -p.From.Offset), nil
	}
	if name == "RET" && p.To.Type == obj.TYPE_NONE {
		return "retq", nil
	}
	if name == "RET" {
		name = "JMP"
	}
	mnemonic := strings.ToLower(name)
	aliases := map[string]string{"JEQ": "je", "JNE": "jne", "JCS": "jb", "JCC": "jae", "JHI": "ja", "JLS": "jbe", "JLT": "jl", "JLE": "jle", "JGT": "jg", "JGE": "jge", "JMI": "js", "JPL": "jns", "JOS": "jo", "JOC": "jno", "JPS": "jp", "JPC": "jnp", "MOVBQZX": "movzbq", "MOVBQSX": "movsbq", "MOVWQZX": "movzwq", "MOVWQSX": "movswq", "MOVLQSX": "movslq", "MOVBLZX": "movzbl", "MOVBLSX": "movsbl", "MOVWLZX": "movzwl", "MOVWLSX": "movswl", "MOVBWZX": "movzbw", "MOVBWSX": "movsbw", "MOVLQZX": "movl", "IMUL3Q": "imulq", "IMUL3L": "imull", "IMUL3W": "imulw", "PSLLDQ": "pslldq", "PSRLDQ": "psrldq", "BYTE": ".byte", "WORD": ".short", "LONG": ".long", "QUAD": ".quad"}
	_, supported := aliases[name]
	if v, ok := aliases[name]; ok {
		mnemonic = v
	}
	// Plan 9 and AT&T differ for several SIMD and multi-operand families.
	// Admit known integer families explicitly instead of guessing by casing.
	if len(name) > 1 && strings.ContainsRune("BWLQ", rune(name[len(name)-1])) {
		switch name[:len(name)-1] {
		case "ADC", "ADD", "AND", "BSF", "BSR", "BSWAP", "BT", "BTC", "BTR", "BTS", "CMP", "CMPXCHG", "DEC", "DIV", "IDIV", "IMUL", "INC", "LEA", "LZCNT", "MOV", "MUL", "NEG", "NOT", "OR", "POP", "POPCNT", "PUSH", "RCL", "RCR", "ROL", "ROR", "SAR", "SBB", "SHL", "SHR", "SUB", "TEST", "TZCNT", "XADD", "XCHG", "XOR":
			supported = true
		}
	}
	switch name {
	case "CALL", "JMP", "LOCK", "REP", "CLD", "STD", "CLC", "STC", "CMC", "CPUID", "RDTSC", "RDTSCP", "XGETBV", "XSETBV", "SYSCALL", "HLT", "UD2", "PAUSE", "LFENCE", "MFENCE", "SFENCE", "PUSHFQ", "POPFQ", "CMPXCHG8B", "CMPXCHG16B":
		supported = true
	}
	if !supported {
		return "", fmt.Errorf("unsupported amd64 instruction %s", name)
	}
	width := 64
	if strings.HasSuffix(name, "B") {
		width = 8
	}
	if strings.HasSuffix(name, "W") {
		width = 16
	}
	if strings.HasSuffix(name, "L") {
		width = 32
	}
	if strings.HasPrefix(name, "SET") {
		width = 8
	}
	branch := name == "CALL" || name == "JMP" || strings.HasPrefix(name, "J") || strings.HasPrefix(name, "LOOP")
	var addrs []obj.Addr
	if p.From.Type != obj.TYPE_NONE {
		addrs = append(addrs, p.From)
	}
	if p.Reg != 0 {
		addrs = append(addrs, obj.Addr{Type: obj.TYPE_REG, Reg: p.Reg})
	}
	for _, a := range p.RestArgs {
		addrs = append(addrs, a.Addr)
	}
	if p.To.Type != obj.TYPE_NONE {
		addrs = append(addrs, p.To)
	}
	if p.RegTo2 != 0 {
		return "", fmt.Errorf("unexpected second x86 destination")
	}
	if (name == "CMPB" || name == "CMPW" || name == "CMPL" || name == "CMPQ") && len(addrs) == 2 {
		addrs[0], addrs[1] = addrs[1], addrs[0]
	}

	var args []string
	for i, a := range addrs {
		w := width
		if strings.HasPrefix(name, "MOV") && (strings.HasSuffix(name, "SX") || strings.HasSuffix(name, "ZX")) {
			if len(name) != 7 {
				return "", fmt.Errorf("unsupported extending move %s", name)
			}
			sizes := map[byte]int{'B': 8, 'W': 16, 'L': 32, 'Q': 64}
			w = sizes[name[4]]
			if i == 0 {
				w = sizes[name[3]]
			}
			if name == "MOVLQZX" {
				w = 32
			}
		}
		if (strings.HasPrefix(name, "SHL") || strings.HasPrefix(name, "SHR") || strings.HasPrefix(name, "SAR") || strings.HasPrefix(name, "ROL") || strings.HasPrefix(name, "ROR")) && i == 0 && len(addrs) > 1 && a.Type == obj.TYPE_REG {
			w = 8
		}
		var v string
		var err error
		if strings.HasPrefix(mnemonic, ".") && a.Type == obj.TYPE_CONST {
			v = fmt.Sprint(a.Offset)
		} else {
			v, err = e.x86addr(a, w, branch, strings.HasPrefix(name, "LEA"))
		}
		if err != nil {
			return "", err
		}
		args = append(args, v)
	}
	return mnemonic + " " + strings.Join(args, ", "), nil
}
