#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys


FUNCTION = "abi0_pointer_arguments"


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


def stack_map(metadata, kind):
    return only(
        metadata["funcdata"],
        lambda item: item["kind"] == kind,
        f"{kind} tables",
    )["stack_map"]


def bitmap_bits(stack_map, index):
    return set(stack_map["bitmaps"][index]["set_bits"] or [])


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
    obj = only(
        [
            member["go_object"]
            for member in document["members"]
            if member.get("go_object") is not None
        ],
        lambda item: True,
        "Go objects",
    )
    symbol = only(
        obj["symbols"],
        lambda item: item["name"] == FUNCTION,
        f"{FUNCTION} symbols",
    )
    metadata = symbol.get("function")
    if metadata is None:
        fail(f"{FUNCTION} has no function metadata")
    if metadata["info"]["args"] != 24:
        fail(f"{FUNCTION} args={metadata['info']['args']}, want 24")

    args_maps = stack_map(metadata, "args_pointer_maps")
    locals_maps = stack_map(metadata, "locals_pointer_maps")
    if args_maps["count"] != 2 or args_maps["num_bits"] != 3:
        fail(f"unexpected args stack-map dimensions: {args_maps}")
    if locals_maps["count"] != 2:
        fail(f"unexpected locals stack-map dimensions: {locals_maps}")

    entry_args = bitmap_bits(args_maps, 0)
    ordinary_args = bitmap_bits(args_maps, 1)
    entry_locals = bitmap_bits(locals_maps, 0)
    ordinary_locals = bitmap_bits(locals_maps, 1)
    if entry_args != {0, 1, 2}:
        fail(f"entry args={sorted(entry_args)}, want [0, 1, 2]")
    if ordinary_args or entry_locals or ordinary_locals:
        fail(
            "non-entry roots are not empty: "
            f"ordinary-args={sorted(ordinary_args)}, "
            f"entry-locals={sorted(entry_locals)}, "
            f"ordinary-locals={sorted(ordinary_locals)}"
        )

    queries = metadata["stack_map_queries"]
    if len(queries) != 2:
        fail(f"unexpected call queries: {queries}")
    morestack_query = only(
        queries,
        lambda item: reference_name(obj, item["target"])
        == "runtime.morestack_noctxt",
        "morestack queries",
    )
    call_query = only(
        queries,
        lambda item: reference_name(obj, item["target"])
        == "use_three_pointers",
        "ordinary call queries",
    )
    if morestack_query["stack_map_index"] != 0:
        fail(f"morestack does not select entry map: {morestack_query}")
    if call_query["stack_map_index"] != 1:
        fail(f"ordinary call does not select map 1: {call_query}")

    print(
        f"{FUNCTION}: args={metadata['info']['args']} "
        f"entry-args={sorted(entry_args)} ordinary-roots=[]"
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
        print(f"check-x86-abi0-entry-args: {error}", file=sys.stderr)
        sys.exit(1)
