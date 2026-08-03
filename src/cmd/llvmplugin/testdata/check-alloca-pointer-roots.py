#!/usr/bin/env python3

import argparse
import json
import re
import subprocess
import sys
import tempfile


BEGIN = 1195461697
LOCALS_TAG = 1280262988
STACK_OBJECT_TAG = 1398033231
END = 1095519299
WORD_BITS = 64


def fail(message):
    print(f"alloca pointer-map check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def run(command, *, input_text=None):
    result = subprocess.run(
        command, input=input_text, capture_output=True, text=True
    )
    if result.returncode != 0:
        fail(
            f"command failed ({' '.join(command)}):\n"
            f"{result.stdout}{result.stderr}"
        )
    return result.stdout + result.stderr


def function_body(ir, name):
    match = re.search(
        rf"define goabiinternal [^@]+@{name}\b.*?^}}",
        ir,
        re.MULTILINE | re.DOTALL,
    )
    if not match:
        fail(f"missing function {name}")
    return match.group(0)


def deopt_bundles(function):
    return re.findall(r'"deopt"\((.*?)\)', function)


def parse_i64(token, description):
    match = re.fullmatch(r"i64 (-?[0-9]+)", token)
    if not match:
        fail(f"{description} is not an i64 constant: {token}")
    return int(match.group(1))


def parse_protocol(bundle):
    tokens = [token.strip() for token in bundle.split(",")]
    if len(tokens) < 5:
        fail(f"truncated protocol: {bundle}")
    protocol_length = parse_i64(tokens[-1], "trailing protocol length")
    protocol_start = len(tokens) - protocol_length - 1
    if protocol_start < 0:
        fail(f"protocol length exceeds deopt bundle: {bundle}")
    if parse_i64(tokens[protocol_start], "begin magic") != BEGIN:
        fail(f"wrong begin magic: {bundle}")
    if parse_i64(tokens[protocol_start + 1], "protocol length") != protocol_length:
        fail(f"protocol length copies disagree: {bundle}")
    if parse_i64(tokens[-2], "end magic") != END:
        fail(f"wrong end magic: {bundle}")

    record_count = parse_i64(tokens[protocol_start + 2], "record count")
    cursor = protocol_start + 3
    records = []
    for record_index in range(record_count):
        if cursor + 10 > len(tokens) - 1:
            fail(f"truncated record {record_index}: {bundle}")
        tag = parse_i64(tokens[cursor], "record tag")
        if tag == LOCALS_TAG:
            kind = "locals"
        elif tag == STACK_OBJECT_TAG:
            kind = "stack_object"
        else:
            fail(f"wrong record tag {record_index}: {bundle}")
        record_length = parse_i64(tokens[cursor + 1], "record length")
        base = tokens[cursor + 2]
        if not re.fullmatch(r"ptr %[A-Za-z0-9_.]+", base):
            fail(f"record {record_index} has non-alloca base: {base}")
        byte_offset = parse_i64(tokens[cursor + 3], "byte offset")
        byte_size = parse_i64(tokens[cursor + 4], "byte size")
        alignment = parse_i64(tokens[cursor + 5], "alignment")
        pointer_size = parse_i64(tokens[cursor + 6], "pointer size")
        bit_count = parse_i64(tokens[cursor + 7], "bit count")
        word_bits = parse_i64(tokens[cursor + 8], "word width")
        word_count = parse_i64(tokens[cursor + 9], "word count")
        if record_length != 10 + word_count:
            fail(f"record {record_index} length is inconsistent: {bundle}")
        if cursor + record_length > len(tokens) - 2:
            fail(f"record {record_index} overruns protocol: {bundle}")
        words = [
            parse_i64(tokens[cursor + 10 + word], "bitmap word")
            & ((1 << WORD_BITS) - 1)
            for word in range(word_count)
        ]
        records.append(
            (
                kind,
                base.removeprefix("ptr %"),
                byte_offset,
                byte_size,
                alignment,
                pointer_size,
                bit_count,
                word_bits,
                tuple(words),
            )
        )
        cursor += record_length
    if cursor != len(tokens) - 2:
        fail(f"records do not cover protocol payload: {bundle}")
    return tokens[:protocol_start], protocol_length, records


def expect_records(ir, function_name, expected, expected_prefixes=None):
    function = function_body(ir, function_name)
    bundles = deopt_bundles(function)
    if len(bundles) != len(expected):
        fail(
            f"{function_name} has {len(bundles)} deopt bundles, "
            f"want {len(expected)}"
        )
    if expected_prefixes is None:
        expected_prefixes = [[] for _ in expected]
    for bundle, want_records, want_prefix in zip(
        bundles, expected, expected_prefixes
    ):
        got_prefix, _, got_records = parse_protocol(bundle)
        if got_prefix != want_prefix:
            fail(
                f"{function_name} ordinary deopt prefix={got_prefix}, "
                f"want {want_prefix}"
            )
        if got_records != want_records:
            fail(
                f"{function_name} records={got_records}, "
                f"want {want_records}"
            )
    return function


def record(kind, base, size, bits, words):
    return (kind, base, 0, size, 8, 8, bits, WORD_BITS, tuple(words))


def check_rewritten_ir(ir):
    statepoints = re.findall(r"@llvm\.experimental\.gc\.statepoint", ir)
    # Seventeen calls plus the intrinsic declaration.
    if len(statepoints) != 18:
        fail(f"found {len(statepoints) - 1} statepoints, want 17")
    if len(re.findall(r'"deopt"\(', ir)) != 17:
        fail("pointer allocas do not have seventeen deopt records")
    if "llvm.statepoint.fixed_stack_home" in ir:
        fail("obsolete fixed-home metadata remains in rewritten IR")

    null_initializers = re.findall(r"^\s*store ptr null, ptr ", ir, re.MULTILINE)
    if len(null_initializers) != 17:
        fail(f"found {len(null_initializers)} null initializers, want 17")

    locals_one = [record("locals", "slot", 8, 1, [1])]
    stack_one = [record("stack_object", "slot", 8, 1, [1])]
    expect_records(ir, "pointer_slot", [stack_one], [["i64 7"]])
    expect_records(
        ir,
        "nested_whole_aggregate",
        [[record("locals", "slot", 48, 6, [0x29])]],
    )
    expect_records(ir, "alloca_call_skip", [locals_one])
    expect_records(ir, "alloca_multiple_calls", [locals_one, locals_one])
    expect_records(ir, "alloca_loop", [locals_one])
    expect_records(
        ir,
        "alloca_gep_address_across_call",
        [[record("locals", "slot", 16, 2, [0x2])]],
    )
    expect_records(ir, "alloca_uninitialized_at_safepoint", [locals_one])
    expect_records(
        ir,
        "alloca_high_bitmap_word",
        [[record("locals", "slot", 512, 64, [1 << 63])]],
    )
    expect_records(
        ir,
        "alloca_multiple_records",
        [[
            record("locals", "left", 8, 1, [1]),
            record("locals", "right", 8, 1, [1]),
        ]],
    )
    expect_records(ir, "alloca_select_same_base", [locals_one])

    escaped = expect_records(
        ir, "alloca_address_passed_to_callee", [stack_one]
    )
    expect_records(ir, "alloca_nocapture_writable", [stack_one])
    expect_records(
        ir, "alloca_escaped_before_unknown_write", [stack_one, stack_one]
    )
    expect_records(
        ir, "alloca_readonly_and_readnone", [stack_one, stack_one]
    )

    selected = function_body(ir, "alloca_select_same_base")
    if '"gc-live"(ptr %selected' not in selected or "%selected.relocated" not in selected:
        fail("alloca-address select is not represented as an ordinary live pointer")

    if "gc.relocate" in escaped or ".relocated" in escaped:
        fail("callee-writable alloca received a relocated write-back")
    statepoint_end = escaped.index("@llvm.experimental.gc.statepoint")
    if re.search(r"store ptr .*ptr %slot", escaped[statepoint_end:]):
        fail("callee-writable alloca is stored after the call")

    relocates = re.findall(
        r"= call coldcc ptr @llvm\.experimental\.gc\.relocate", ir
    )
    if len(relocates) != 2:
        fail(f"found {len(relocates)} scalar relocates, want 2")
    uninitialized = function_body(ir, "alloca_uninitialized_at_safepoint")
    if '"gc-live"(ptr %pointer' not in uninitialized:
        fail("ordinary scalar SSA pointer is missing from gc-live")
    if "%pointer.relocated" not in uninitialized:
        fail("ordinary scalar SSA pointer was not relocated")


def check_objview(objview, object_path):
    document = json.loads(run([objview, "-format=json", object_path]))
    symbols = document["members"][0]["go_object"]["symbols"]

    def funcdata_kinds(name):
        matches = [symbol for symbol in symbols if symbol.get("name") == name]
        if len(matches) != 1 or not matches[0].get("function"):
            fail(f"objview did not find one function symbol {name}")
        return [entry["kind"] for entry in matches[0]["function"]["funcdata"]]

    locals_kinds = funcdata_kinds("alloca_multiple_calls")
    if "locals_pointer_maps" not in locals_kinds:
        fail("locals-only alloca has no LocalsPointerMaps")
    if "stack_objects" in locals_kinds:
        fail("locals-only alloca unexpectedly emitted StackObjects")

    stack_object_kinds = funcdata_kinds("alloca_address_passed_to_callee")
    if "locals_pointer_maps" not in stack_object_kinds:
        fail("address-observable alloca has no LocalsPointerMaps")
    if "stack_objects" not in stack_object_kinds:
        fail("address-observable alloca has no StackObjects")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--opt", required=True)
    parser.add_argument("--plugin", required=True)
    parser.add_argument("--input", required=True)
    parser.add_argument("--objview")
    args = parser.parse_args()

    rewritten = run(
        [
            args.llc,
            f"-load-pass-plugin={args.plugin}",
            "-goallc-pass-plugin-emit-ir",
            "-filetype=null",
            "-o",
            "-",
            args.input,
        ]
    )
    check_rewritten_ir(rewritten)
    run(
        [
            args.opt,
            f"-load-pass-plugin={args.plugin}",
            "-passes=verify",
            "-disable-output",
            "-",
        ],
        input_text=rewritten,
    )

    with tempfile.TemporaryDirectory() as directory:
        optimized = run(
            [
                args.opt,
                "-passes=default<O2>,verify",
                "-S",
                "-o",
                "-",
                args.input,
            ]
        )
        optimized_rewritten = run(
            [
                args.llc,
                f"-load-pass-plugin={args.plugin}",
                "-goallc-pass-plugin-emit-ir",
                "-filetype=null",
                "-o",
                "-",
                "-",
            ],
            input_text=optimized,
        )
        if len(re.findall(r'"deopt"\(', optimized_rewritten)) != 7:
            fail("default<O2> did not preserve stack-object records")
        if "goallc.source_addrtaken" in optimized_rewritten:
            fail("source address-taken provenance escaped final classification")
        if "alloca %nested" in function_body(
            optimized_rewritten, "nested_whole_aggregate"
        ):
            fail("source address-taken provenance prevented SROA")

        machine_ir = run(
            [
                args.llc,
                f"-load-pass-plugin={args.plugin}",
                "-verify-machineinstrs",
                "-stop-after=finalize-isel",
                "-o",
                "-",
                args.input,
            ]
        )
        statepoint_lines = re.findall(r"(?m)^.*STATEPOINT.*$", machine_ir)
        if len(statepoint_lines) != 17:
            fail(f"MIR has {len(statepoint_lines)} statepoints, want 17")
        alloca_statepoints = [
            line for line in statepoint_lines if str(BEGIN) in line
        ]
        if len(alloca_statepoints) != 17:
            fail("MIR does not contain seventeen alloca statepoints")
        for statepoint in alloca_statepoints:
            if (
                str(LOCALS_TAG) not in statepoint
                and str(STACK_OBJECT_TAG) not in statepoint
            ) or "%stack." not in statepoint:
                fail(f"MIR alloca record is malformed: {statepoint}")

        output = f"{directory}/alloca-pointer-roots.goobj"
        run(
            [
                args.llc,
                f"-load-pass-plugin={args.plugin}",
                "-verify-machineinstrs",
                "-filetype=obj",
                "-o",
                output,
                args.input,
            ]
        )
        if args.objview:
            check_objview(args.objview, output)
        optimized_output = f"{directory}/alloca-pointer-roots-o2.goobj"
        run(
            [
                args.llc,
                f"-load-pass-plugin={args.plugin}",
                "-verify-machineinstrs",
                "-filetype=obj",
                "-o",
                optimized_output,
                "-",
            ],
            input_text=optimized,
        )


if __name__ == "__main__":
    main()
