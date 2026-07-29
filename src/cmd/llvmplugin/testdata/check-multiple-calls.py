#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys


FUNCTION = "different_pointer_sets_across_calls"


def fail(message):
    raise RuntimeError(message)


def pc_value(ranges, pc):
    for item in ranges:
        if item["start"] <= pc < item["end"]:
            return item["value"]
    fail(f"PC {pc} is outside PCDATA ranges")


def only(items, predicate, description):
    matches = [item for item in items if predicate(item)]
    if len(matches) != 1:
        fail(f"found {len(matches)} {description}, want 1")
    return matches[0]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--objview", required=True)
    parser.add_argument("--object", required=True)
    args = parser.parse_args()

    result = subprocess.run(
        [args.objview, "-json", args.object],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    )
    document = json.loads(result.stdout)
    go_objects = [
        member["go_object"]
        for member in document["members"]
        if member.get("go_object") is not None
    ]
    obj = only(go_objects, lambda item: True, "Go objects")
    function = only(
        obj["symbols"],
        lambda item: item["name"] == FUNCTION,
        f"{FUNCTION} symbols",
    )
    metadata = function.get("function")
    if metadata is None:
        fail(f"{FUNCTION} has no function metadata")

    unsafe = only(
        metadata["pcdata"],
        lambda item: item["kind"] == "unsafe_point" and item["index"] == 0,
        "PCDATA_UnsafePoint tables",
    )
    if unsafe["ranges"] != [{"start": 0, "end": function["size"], "value": -1}]:
        fail(f"unexpected PCDATA_UnsafePoint ranges: {unsafe['ranges']}")

    stack_index = only(
        metadata["pcdata"],
        lambda item: item["kind"] == "stack_map_index" and item["index"] == 1,
        "PCDATA_StackMapIndex tables",
    )
    ranges = stack_index["ranges"]
    values = [item["value"] for item in ranges]
    if values != [-1, 1, 2, 0]:
        fail(f"unexpected stack-map range values: {values}")

    queries = metadata["stack_map_queries"]
    if len(queries) != 3:
        fail(f"found {len(queries)} call queries, want 3")
    if [item["stack_map_index"] for item in queries] != [1, 2, 0]:
        fail(f"unexpected call stack-map indexes: {queries}")
    for query in queries:
        if pc_value(ranges, query["lookup_pc"]) != query["stack_map_index"]:
            fail(f"query does not match raw PCDATA ranges: {query}")

    # X86 R_CALL relocations point at the four-byte displacement, one byte
    # after the CALL opcode. Go's range starts at the opcode itself.
    if ranges[1]["start"] != queries[0]["call_offset"] - 1:
        fail("first stack-map range does not start at the first CALL")
    if ranges[2]["start"] != queries[1]["call_offset"] - 1:
        fail("second stack-map range does not start at the second CALL")
    if ranges[3]["start"] != queries[2]["call_offset"] - 1:
        fail("empty stack-growth map does not start at the morestack CALL")

    locals_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "locals_pointer_maps",
        "FUNCDATA_LocalsPointerMaps tables",
    )["stack_map"]
    if locals_maps["count"] != 3:
        fail(f"locals stack-map count is {locals_maps['count']}, want 3")
    morestack_bits = set(locals_maps["bitmaps"][0]["set_bits"] or [])
    first_bits = set(locals_maps["bitmaps"][1]["set_bits"] or [])
    second_bits = set(locals_maps["bitmaps"][2]["set_bits"] or [])
    if len(first_bits) != 2 or len(second_bits) != 1:
        fail(f"unexpected live pointer sets: {first_bits}, {second_bits}")
    if morestack_bits:
        fail(f"morestack statepoint has GC live pointers: {morestack_bits}")

    print(
        f"{FUNCTION}: PCDATA values {values}, "
        f"call maps {[item['stack_map_index'] for item in queries]}, "
        f"locals bits {sorted(first_bits)} -> {sorted(second_bits)} "
        f"-> {sorted(morestack_bits)}"
    )


if __name__ == "__main__":
    try:
        main()
    except (KeyError, RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"check-multiple-calls: {error}", file=sys.stderr)
        sys.exit(1)
