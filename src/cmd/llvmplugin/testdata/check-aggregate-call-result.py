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


def reference_name(obj, target):
    if target["pkg_kind"] != "none":
        fail(f"cannot resolve non-reference target: {target}")
    reference = only(
        obj["references"],
        lambda item: (
            item["class"] == "nonpackage_reference"
            and item["class_index"] == target["sym_index"]
        ),
        f"references for target {target}",
    )
    return reference["name"]


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
    queries = metadata["stack_map_queries"]
    if len(queries) != 4:
        fail(f"unexpected call queries: {queries}")
    target_names = [reference_name(obj, item["target"]) for item in queries]
    want_target_names = [
        "make_pair",
        "safepoint",
        "leaf_consume_pair",
        "runtime.morestack_noctxt",
    ]
    if target_names != want_target_names:
        fail(
            f"unexpected call targets: {target_names}, "
            f"want {want_target_names}"
        )
    if any(item["relocation_type"] != "R_CALL" for item in queries):
        fail(f"unexpected call relocation types: {queries}")
    indexes = [item["stack_map_index"] for item in queries]
    if indexes != [1, 2, 2, 0]:
        fail(f"unexpected call stack-map indexes: {indexes}")
    call_starts = [item["call_offset"] - 1 for item in queries]
    if any(
        item["return_pc"] != item["call_offset"] + item["instruction_size"]
        or item["lookup_pc"] != item["return_pc"] - 1
        for item in queries
    ):
        fail(f"unexpected x86 call query coordinates: {queries}")
    actual_ranges = [
        (item["start"], item["end"], item["value"]) for item in ranges
    ]
    want_ranges = [
        (0, call_starts[0], -1),
        (call_starts[0], call_starts[1], 1),
        (call_starts[1], call_starts[-1], 2),
        (call_starts[-1], function["size"], 0),
    ]
    if actual_ranges != want_ranges:
        fail(
            f"unexpected exact stack-map ranges: {actual_ranges}, "
            f"want {want_ranges}"
        )
    if not ranges[2]["start"] <= call_starts[2] < ranges[2]["end"]:
        fail("leaf aggregate consumer is not covered by live map 2")

    args_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "args_pointer_maps",
        "FUNCDATA_ArgsPointerMaps tables",
    )["stack_map"]
    if args_maps["count"] != 3 or args_maps["num_bits"] != 2:
        fail(f"unexpected argument-map dimensions: {args_maps}")
    args_bits = [
        set(item["set_bits"] or []) for item in args_maps["bitmaps"]
    ]
    # Map 0 is the stack-growth entry map and retains seed. Ordinary maps 1
    # and 2 move all live roots out of the incoming argument area.
    if args_bits != [{0}, set(), set()]:
        fail(f"unexpected argument roots: {args_bits}")

    locals_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "locals_pointer_maps",
        "FUNCDATA_LocalsPointerMaps tables",
    )["stack_map"]
    if locals_maps["count"] != 3 or locals_maps["num_bits"] != 4:
        fail(f"unexpected locals-map dimensions: {locals_maps}")
    locals_bits = [
        set(item["set_bits"] or []) for item in locals_maps["bitmaps"]
    ]
    # Map 1 covers make_pair before it produces a result. Map 2 then keeps
    # the aggregate result's pointer field live through both later calls.
    # The root uses the normal statepoint spill slot; there is no longer an
    # extra fixed-home frame slot ahead of it.
    if locals_bits != [set(), set(), {2}]:
        fail(f"unexpected local roots: {locals_bits}")

    print(
        f"{FUNCTION}: targets {target_names}, call maps {indexes}, "
        f"args {args_bits}, locals {locals_bits}"
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
