#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys


FUNCTION = "aggregate_call_result_goobj"


def fail(message):
    raise RuntimeError(message)


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

    stack_index = only(
        metadata["pcdata"],
        lambda item: item["kind"] == "stack_map_index" and item["index"] == 1,
        "PCDATA_StackMapIndex tables",
    )
    ranges = stack_index["ranges"]
    if [item["value"] for item in ranges] != [-1, 0, 1, 0]:
        fail(f"unexpected stack-map ranges: {ranges}")

    queries = metadata["stack_map_queries"]
    indexes = [item["stack_map_index"] for item in queries]
    if indexes != [0, 1, 1, 0]:
        fail(f"unexpected call stack-map indexes: {indexes}")
    if ranges[1]["start"] != queries[0]["call_offset"] - 1:
        fail("empty entry map does not begin at the first statepoint CALL")
    if ranges[2]["start"] != queries[1]["call_offset"] - 1:
        fail("aggregate-result map does not begin at the live statepoint CALL")
    if ranges[3]["start"] != queries[-1]["call_offset"] - 1:
        fail("empty stack-growth map does not begin at the morestack CALL")

    args_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "args_pointer_maps",
        "FUNCDATA_ArgsPointerMaps tables",
    )["stack_map"]
    if args_maps["count"] != 2 or args_maps["num_bits"] != 2:
        fail(f"unexpected argument-map dimensions: {args_maps}")
    if any(item["set_bits"] for item in args_maps["bitmaps"]):
        fail(f"x86 aggregate test unexpectedly has argument roots: {args_maps}")

    locals_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "locals_pointer_maps",
        "FUNCDATA_LocalsPointerMaps tables",
    )["stack_map"]
    if locals_maps["count"] != 2:
        fail(f"locals stack-map count is {locals_maps['count']}, want 2")
    morestack_bits = set(locals_maps["bitmaps"][0]["set_bits"] or [])
    live_bits = set(locals_maps["bitmaps"][1]["set_bits"] or [])
    if len(live_bits) != 1:
        fail(f"aggregate result has unexpected live pointer bits: {live_bits}")
    if morestack_bits:
        fail(f"morestack statepoint has GC live pointers: {morestack_bits}")

    print(
        f"{FUNCTION}: call maps {indexes}, "
        f"locals bits {sorted(live_bits)} -> {sorted(morestack_bits)}"
    )


if __name__ == "__main__":
    try:
        main()
    except (
        KeyError,
        RuntimeError,
        subprocess.CalledProcessError,
        json.JSONDecodeError,
    ) as error:
        print(f"check-aggregate-call-result: {error}", file=sys.stderr)
        sys.exit(1)
