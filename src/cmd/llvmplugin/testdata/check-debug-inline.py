#!/usr/bin/env python3

import argparse
import pathlib
import struct
import subprocess
import tempfile


MAGIC = bytes([0]) + b"go120ld"
PKGIDX_NONE = (1 << 31) - 1
PKGIDX_HASHED64 = PKGIDX_NONE - 1
PKGIDX_HASHED = PKGIDX_NONE - 2
PKGIDX_SELF = PKGIDX_NONE - 4
SYMBOL_SIZE = 21
AUX_SIZE = 9


def fail(message):
    raise SystemExit(message)


def run(command):
    result = subprocess.run(command, text=True, capture_output=True)
    if result.returncode:
        fail(
            f"command failed ({result.returncode}): {' '.join(command)}\n"
            f"{result.stdout}{result.stderr}"
        )


def string_at(data, offset):
    size, string_offset = struct.unpack_from("<II", data, offset)
    return data[string_offset : string_offset + size].decode()


def read_symbols(data, start, end):
    return [
        string_at(data, offset)
        for offset in range(start, end, SYMBOL_SIZE)
    ]


def read_uvarint(data, offset):
    value = 0
    shift = 0
    while True:
        byte = data[offset]
        offset += 1
        value |= (byte & 0x7F) << shift
        if byte < 0x80:
            return value, offset
        shift += 7


def read_varint(data, offset):
    value, offset = read_uvarint(data, offset)
    signed = value >> 1
    if value & 1:
        signed = ~signed
    return signed, offset


def decode_pctab(payload):
    ranges = []
    offset = 0
    pc = 0
    value = -1
    first = True
    while offset < len(payload):
        delta, offset = read_varint(payload, offset)
        if delta == 0 and not first:
            break
        value += delta
        pc_delta, offset = read_uvarint(payload, offset)
        next_pc = pc + pc_delta
        ranges.append((pc, next_pc, value))
        pc = next_pc
        first = False
    return ranges


def value_at(ranges, pc):
    for start, end, value in ranges:
        if start <= pc < end:
            return value
    fail(f"no PC table range covers PC {pc}: {ranges}")


class GoObj:
    def __init__(self, path):
        raw = pathlib.Path(path).read_bytes()
        base = raw.index(MAGIC)
        self.data = raw[base:]
        self.offsets = struct.unpack_from("<19I", self.data, 20)
        self.symdef = read_symbols(
            self.data, self.offsets[3], self.offsets[4]
        )
        self.hashed64 = read_symbols(
            self.data, self.offsets[4], self.offsets[5]
        )
        self.hashed = read_symbols(
            self.data, self.offsets[5], self.offsets[6]
        )
        self.nonpkgdef = read_symbols(
            self.data, self.offsets[6], self.offsets[7]
        )
        self.nonpkgref = read_symbols(
            self.data, self.offsets[7], self.offsets[8]
        )
        self.defined = (
            self.symdef + self.hashed64 + self.hashed + self.nonpkgdef
        )
        self.aux_indexes = [
            struct.unpack_from("<I", self.data, self.offsets[12] + 4 * i)[0]
            for i in range((self.offsets[13] - self.offsets[12]) // 4)
        ]
        self.data_indexes = [
            struct.unpack_from("<I", self.data, self.offsets[13] + 4 * i)[0]
            for i in range((self.offsets[14] - self.offsets[13]) // 4)
        ]

    def resolve(self, pkg_index, sym_index):
        if pkg_index == PKGIDX_SELF and sym_index < len(self.symdef):
            return self.symdef[sym_index]
        if pkg_index == PKGIDX_HASHED64 and sym_index < len(self.hashed64):
            return self.hashed64[sym_index]
        if pkg_index == PKGIDX_HASHED and sym_index < len(self.hashed):
            return self.hashed[sym_index]
        if pkg_index == PKGIDX_NONE:
            symbols = self.nonpkgdef + self.nonpkgref
            if sym_index < len(symbols):
                return symbols[sym_index]
        return f"{pkg_index}:{sym_index}"

    def data_index(self, pkg_index, sym_index):
        if pkg_index == PKGIDX_SELF and sym_index < len(self.symdef):
            return sym_index
        if pkg_index == PKGIDX_HASHED64 and sym_index < len(self.hashed64):
            return len(self.symdef) + sym_index
        if pkg_index == PKGIDX_HASHED and sym_index < len(self.hashed):
            return len(self.symdef) + len(self.hashed64) + sym_index
        if pkg_index == PKGIDX_NONE and sym_index < len(self.nonpkgdef):
            return (
                len(self.symdef)
                + len(self.hashed64)
                + len(self.hashed)
                + sym_index
            )
        return None

    def symbol_data(self, pkg_index, sym_index):
        index = self.data_index(pkg_index, sym_index)
        if index is None:
            fail(f"auxiliary data is not local: {pkg_index}:{sym_index}")
        start = self.offsets[16] + self.data_indexes[index]
        end = self.offsets[16] + self.data_indexes[index + 1]
        return self.data[start:end]

    def auxiliaries(self, symbol):
        try:
            symbol_index = self.defined.index(symbol)
        except ValueError:
            fail(f"GoObj has no defined symbol {symbol!r}")
        result = {}
        first = self.aux_indexes[symbol_index]
        last = self.aux_indexes[symbol_index + 1]
        for aux_index in range(first, last):
            offset = self.offsets[15] + aux_index * AUX_SIZE
            aux_type = self.data[offset]
            pkg_index, sym_index = struct.unpack_from(
                "<II", self.data, offset + 1
            )
            result.setdefault(aux_type, []).append(
                (self.resolve(pkg_index, sym_index),
                 self.symbol_data(pkg_index, sym_index))
            )
        return result


def one_aux(auxiliaries, aux_type, description, symbol="main.outer"):
    values = auxiliaries.get(aux_type, [])
    if len(values) != 1:
        fail(f"{symbol} has {len(values)} {description} auxiliaries, want 1")
    return values[0][1]


def check_func_info(obj, payload, quantum):
    if len(payload) < 24:
        fail(f"short FuncInfo: {len(payload)} bytes")
    start_line, file_count = struct.unpack_from("<iI", payload, 12)
    if start_line != 5:
        fail(f"FuncInfo.StartLine={start_line}, want 5")
    file_end = 20 + file_count * 4
    inline_count = struct.unpack_from("<I", payload, file_end)[0]
    if inline_count != 2:
        fail(f"FuncInfo has {inline_count} inline nodes, want 2")
    nodes = []
    for index in range(inline_count):
        offset = file_end + 4 + index * 24
        parent, file_index, line, callee_pkg, callee_sym, parent_pc = (
            struct.unpack_from("<iIiIIi", payload, offset)
        )
        nodes.append(
            (parent, file_index, line,
             obj.resolve(callee_pkg, callee_sym), parent_pc)
        )
    if [(n[0], n[2], n[3]) for n in nodes] != [
        (-1, 10, "main.mid"),
        (0, 20, "main.inner"),
    ]:
        fail(f"unexpected inline tree: {nodes}")
    parent_pcs = [node[4] for node in nodes]
    if parent_pcs[1] - parent_pcs[0] != quantum:
        fail(
            f"inline anchors are not adjacent {quantum}-byte instructions: "
            f"ParentPC={parent_pcs}"
        )
    if any(parent_pc % quantum for parent_pc in parent_pcs):
        fail(f"unaligned inline ParentPC values: {parent_pcs}")
    return parent_pcs


def check_target(llc, plugin, input_path, triple, quantum, output):
    run(
        [
            llc,
            f"-load-pass-plugin={plugin}",
            f"-mtriple={triple}",
            "-verify-machineinstrs",
            "-filetype=obj",
            "-o",
            str(output),
            input_path,
        ]
    )
    obj = GoObj(output)
    auxiliaries = obj.auxiliaries("main.outer")
    parent_pcs = check_func_info(
        obj, one_aux(auxiliaries, 1, "FuncInfo"), quantum
    )
    pcfile = decode_pctab(one_aux(auxiliaries, 8, "pcfile"))
    pcline = decode_pctab(one_aux(auxiliaries, 9, "pcline"))
    # The linker installs AuxPcinline as runtime PCDATA_InlTreeIndex (slot 2).
    pcinline = decode_pctab(one_aux(auxiliaries, 10, "pcinline"))

    parent_units = [parent_pc // quantum for parent_pc in parent_pcs]
    if [value_at(pcline, pc) for pc in parent_units] != [10, 20]:
        fail(f"pcline does not describe the inline anchors: {pcline}")
    if [value_at(pcinline, pc) for pc in parent_units] != [-1, 0]:
        fail(f"pcinline does not unwind at ParentPC values: {pcinline}")
    if value_at(pcfile, parent_units[0]) < 0:
        fail(f"pcfile has no source file at the first inline anchor: {pcfile}")
    child_pc = parent_units[1] + 1
    if value_at(pcinline, child_pc) != 1:
        fail(f"pcinline does not enter the innermost frame: {pcinline}")

    unlocated = obj.auxiliaries("main.unlocated")
    func_info = one_aux(
        unlocated, 1, "FuncInfo", symbol="main.unlocated"
    )
    start_line, file_count = struct.unpack_from("<iI", func_info, 12)
    if start_line != 40 or file_count != 1:
        fail(
            "unlocated FuncInfo does not preserve its source: "
            f"StartLine={start_line} files={file_count}"
        )
    file_index = struct.unpack_from("<I", func_info, 20)[0]
    unlocated_pcfile = decode_pctab(
        one_aux(unlocated, 8, "pcfile", symbol="main.unlocated")
    )
    unlocated_pcline = decode_pctab(
        one_aux(unlocated, 9, "pcline", symbol="main.unlocated")
    )
    if value_at(unlocated_pcfile, 0) != file_index:
        fail(
            "unlocated pcfile does not use its CU file index: "
            f"FuncInfo.File={file_index} pcfile={unlocated_pcfile}"
        )
    if value_at(unlocated_pcline, 0) != start_line:
        fail(
            "unlocated pcline does not use its StartLine: "
            f"StartLine={start_line} pcline={unlocated_pcline}"
        )

    zero = obj.auxiliaries("main.zero")
    zero_func_info = one_aux(
        zero, 1, "FuncInfo", symbol="main.zero"
    )
    _, zero_file_count = struct.unpack_from("<iI", zero_func_info, 12)
    zero_inline_offset = 20 + zero_file_count * 4
    zero_inline_count = struct.unpack_from(
        "<I", zero_func_info, zero_inline_offset
    )[0]
    if zero_inline_count != 1:
        fail(
            "zero-line callsite has unexpected inline node count: "
            f"{zero_inline_count}"
        )
    zero_node = struct.unpack_from(
        "<iIiIIi", zero_func_info, zero_inline_offset + 4
    )
    zero_line = zero_node[2]
    zero_parent_pc = zero_node[5]
    if zero_line != 0 or zero_parent_pc < 0:
        fail(
            "zero-line callsite did not retain its line and final-layout "
            f"anchor: line={zero_line} ParentPC={zero_parent_pc}"
        )

    shared = obj.auxiliaries("main.shared")
    shared_func_info = one_aux(
        shared, 1, "FuncInfo", symbol="main.shared"
    )
    _, shared_file_count = struct.unpack_from("<iI", shared_func_info, 12)
    shared_inline_offset = 20 + shared_file_count * 4
    shared_inline_count = struct.unpack_from(
        "<I", shared_func_info, shared_inline_offset
    )[0]
    if shared_inline_count != 2:
        fail(
            "shared callsite has unexpected inline node count: "
            f"{shared_inline_count}"
        )
    shared_nodes = []
    for index in range(shared_inline_count):
        node = struct.unpack_from(
            "<iIiIIi",
            shared_func_info,
            shared_inline_offset + 4 + index * 24,
        )
        shared_nodes.append(
            (node[0], node[2], obj.resolve(node[3], node[4]), node[5])
        )
    if [(n[0], n[1], n[2]) for n in shared_nodes] != [
        (-1, 71, "main.sharedLeft"),
        (-1, 71, "main.sharedRight"),
    ]:
        fail(f"shared callsite did not retain separate callees: {shared_nodes}")
    shared_parent_pcs = [node[3] for node in shared_nodes]
    if any(pc < 0 or pc % quantum for pc in shared_parent_pcs):
        fail(f"shared callsite has invalid ParentPC values: {shared_parent_pcs}")
    if shared_parent_pcs[0] == shared_parent_pcs[1]:
        fail(f"shared callsite reused one ParentPC: {shared_parent_pcs}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--plugin", required=True)
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    with tempfile.TemporaryDirectory() as directory:
        directory = pathlib.Path(directory)
        for name, triple, quantum in [
            ("x86_64", "x86_64-unknown-linux-goobj", 1),
            ("aarch64", "aarch64-unknown-linux-goobj", 4),
        ]:
            check_target(
                args.llc,
                args.plugin,
                args.input,
                triple,
                quantum,
                directory / f"{name}.goobj",
            )


if __name__ == "__main__":
    main()
