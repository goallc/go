#!/usr/bin/env python3

import argparse
import json
import re
import subprocess
import sys
import tempfile


BEGIN = 0x474F4446
END = 0x46444F47


def fail(message):
    raise RuntimeError(message)


def only(items, predicate, description):
    matches = [item for item in items if predicate(item)]
    if len(matches) != 1:
        fail(f"found {len(matches)} {description}, want 1")
    return matches[0]


def read_uvarint(payload, offset):
    value = 0
    shift = 0
    while offset < len(payload):
        byte = payload[offset]
        offset += 1
        value |= (byte & 0x7F) << shift
        if byte < 0x80:
            return value, offset
        shift += 7
        if shift >= 64:
            fail("open-defer funcdata contains an overflowing uvarint")
    fail("open-defer funcdata contains a truncated uvarint")


def check_goobj(llc, plugin, objview, input_path):
    with tempfile.TemporaryDirectory() as temporary:
        object_path = f"{temporary}/open-defer.goobj"
        subprocess.run(
            [
                llc,
                f"-load-pass-plugin={plugin}",
                "-verify-machineinstrs",
                "-filetype=obj",
                "-o",
                object_path,
                input_path,
            ],
            check=True,
        )
        result = subprocess.run(
            [objview, "-format=json", object_path],
            check=True,
            stdout=subprocess.PIPE,
            text=True,
        )

    document = json.loads(result.stdout)
    objects = [
        member["go_object"]
        for member in document["members"]
        if member.get("go_object") is not None
    ]
    obj = only(objects, lambda item: True, "Go objects")
    function = only(
        obj["symbols"],
        lambda item: item["name"] == "open_defer",
        "open_defer symbols",
    )
    metadata = function.get("function")
    if metadata is None:
        fail("open_defer has no function metadata")

    funcdata = metadata.get("funcdata", [])
    if [item["index"] for item in funcdata] != list(range(5)):
        fail(f"open_defer funcdata is not positional through index 4: {funcdata}")
    if funcdata[2]["kind"] != "stack_objects" or funcdata[2]["symbol"]["pkg_kind"] != "invalid":
        fail(f"open_defer has a non-nil StackObjects entry: {funcdata[2]}")
    if funcdata[3]["kind"] != "inline_tree" or funcdata[3]["symbol"]["pkg_kind"] != "invalid":
        fail(f"open_defer has a non-nil inline-tree entry: {funcdata[3]}")
    open_defer = funcdata[4]
    if open_defer["kind"] != "open_coded_defer":
        fail(f"funcdata index 4 is not open-coded defer information: {open_defer}")
    payload = bytes.fromhex(open_defer["raw_hex"])
    bits_offset, offset = read_uvarint(payload, 0)
    slots_offset, offset = read_uvarint(payload, offset)
    if offset != len(payload) or bits_offset == 0 or slots_offset == 0:
        fail(f"malformed open-defer frame offsets: {open_defer['raw_hex']}")

    locals_maps = funcdata[1]["stack_map"]["bitmaps"]
    ordinary_indices = {
        query["stack_map_index"]
        for query in metadata["stack_map_queries"]
        if query["stack_map_index"] > 0
    }
    if len(ordinary_indices) != 2:
        fail(f"found ordinary stack-map indices {ordinary_indices}, want two")
    slot_pairs = []
    for index in sorted(ordinary_indices):
        bits = set(locals_maps[index].get("set_bits") or [])
        pairs = [bit for bit in bits if bit + 1 in bits]
        if not pairs:
            fail(f"open-defer slots are not consecutive in locals map {index}: {bits}")
        slot_pairs.append(pairs[0])
    if slot_pairs[0] != slot_pairs[1]:
        fail(f"open-defer slot positions change between calls: {slot_pairs}")

    print(
        "GoObj funcdata[4] locates open-defer bits and two consecutive "
        f"locals-ptrmap slots at bit {slot_pairs[0]}"
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--plugin", required=True)
    parser.add_argument("--input", required=True)
    parser.add_argument("--objview")
    args = parser.parse_args()

    result = subprocess.run(
        [
            args.llc,
            f"-load-pass-plugin={args.plugin}",
            "-goallc-pass-plugin-emit-ir",
            "-filetype=null",
            "-o",
            "-",
            args.input,
        ],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    ir = result.stdout
    body_match = re.search(
        r"define goabiinternal ptr @open_defer\b.*?^}",
        ir,
        re.MULTILINE | re.DOTALL,
    )
    if not body_match:
        fail("rewritten open_defer function is missing")
    body = body_match.group(0)
    bundles = re.findall(r'"deopt"\((.*?)\)', body)
    if len(bundles) != 2:
        fail(f"found {len(bundles)} ordinary statepoint deopt bundles, want 2")
    for bundle in bundles:
        tokens = [token.strip() for token in bundle.split(",")]
        begin = f"i64 {BEGIN}"
        if begin not in tokens:
            fail(f"open-defer begin marker is missing: {bundle}")
        start = tokens.index(begin)
        expected = [
            begin,
            "i64 6",
            "i64 2",
            "ptr %bits",
            "ptr %slots",
            f"i64 {END}",
            "i64 6",
        ]
        if tokens[start : start + len(expected)] != expected:
            fail(f"open-defer frame record is malformed: {bundle}")
    if len(re.findall(r'"gc-live"\([^)]*ptr %slots', body)) != 2:
        fail("the open-defer slots array is not live at both statepoints")
    if "goallc.open_defer" in body:
        fail("frontend open-defer alloca metadata was not consumed")
    print("open-defer bits and two closure slots are carried at both calls")
    if args.objview:
        check_goobj(args.llc, args.plugin, args.objview, args.input)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.CalledProcessError) as error:
        print(f"check-open-defer: {error}", file=sys.stderr)
        sys.exit(1)
