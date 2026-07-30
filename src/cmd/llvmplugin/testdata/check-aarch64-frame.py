#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys


FUNCTION = "aarch64_pointer_and_code_live"
POINTER_SIZE = 8


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

    pcsp = only(
        metadata["pc_tables"],
        lambda item: item["kind"] == "pcsp",
        "PCSP tables",
    )
    frame_sizes = {
        item["value"] for item in pcsp["ranges"] if item["value"] > 0
    }
    if len(frame_sizes) != 1:
        fail(f"unexpected framed PCSP values: {sorted(frame_sizes)}")
    frame_size = frame_sizes.pop()

    locals_size = metadata["info"]["locals"]
    if locals_size != frame_size - POINTER_SIZE:
        fail(
            f"FuncInfo.locals={locals_size}, want frame size {frame_size} "
            f"minus LR"
        )

    locals_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "locals_pointer_maps",
        "FUNCDATA_LocalsPointerMaps tables",
    )["stack_map"]
    want_bits = frame_size // POINTER_SIZE - 2
    if locals_maps["num_bits"] != want_bits:
        fail(
            f"locals bitmap has {locals_maps['num_bits']} bits, want "
            f"{want_bits} after excluding LR and FP"
        )
    if locals_maps["count"] != 2:
        fail(f"locals stack-map count is {locals_maps['count']}, want 2")

    morestack_bits = set(locals_maps["bitmaps"][0]["set_bits"] or [])
    ordinary_bits = set(locals_maps["bitmaps"][1]["set_bits"] or [])
    want_ordinary_bits = {1}
    if ordinary_bits != want_ordinary_bits:
        fail(
            "ordinary statepoint should contain only the data pointer live "
            f"after the call, excluding the call-only code pointer; got "
            f"{sorted(ordinary_bits)}, want {sorted(want_ordinary_bits)}"
        )
    if any(bit < 0 or bit >= want_bits for bit in ordinary_bits):
        fail(f"ordinary roots escape the GC locals range: {ordinary_bits}")
    if morestack_bits:
        fail(f"morestack statepoint has GC live pointers: {morestack_bits}")

    args_maps = only(
        metadata["funcdata"],
        lambda item: item["kind"] == "args_pointer_maps",
        "FUNCDATA_ArgsPointerMaps tables",
    )["stack_map"]
    if args_maps["count"] != locals_maps["count"] or args_maps["num_bits"] != 2:
        fail(f"unexpected args stack-map dimensions: {args_maps}")
    entry_args = set(args_maps["bitmaps"][0]["set_bits"] or [])
    ordinary_args = set(args_maps["bitmaps"][1]["set_bits"] or [])
    if entry_args != {1} or ordinary_args:
        fail(
            f"unexpected args maps: entry={sorted(entry_args)}, "
            f"ordinary={sorted(ordinary_args)}"
        )

    stack_index = only(
        metadata["pcdata"],
        lambda item: item["kind"] == "stack_map_index" and item["index"] == 1,
        "PCDATA_StackMapIndex tables",
    )
    values = [item["value"] for item in stack_index["ranges"]]
    if values != [-1, 1, 0]:
        fail(f"unexpected stack-map range values: {values}")
    queries = metadata["stack_map_queries"]
    if len(queries) != 1 or queries[0]["stack_map_index"] != 0:
        fail(f"unexpected morestack call query: {queries}")
    if stack_index["ranges"][-1]["start"] != queries[0]["call_offset"]:
        fail("morestack stack-map range does not start at the CALL")

    def check_entry_only(name, arg_size, num_bits, want_entry_bits):
        symbol = only(
            obj["symbols"],
            lambda item: item["name"] == name,
            f"{name} symbols",
        )
        function_metadata = symbol.get("function")
        if function_metadata is None:
            fail(f"{name} has no function metadata")
        if function_metadata["info"]["args"] != arg_size:
            fail(
                f"{name} args={function_metadata['info']['args']}, "
                f"want {arg_size}"
            )
        function_args = only(
            function_metadata["funcdata"],
            lambda item: item["kind"] == "args_pointer_maps",
            f"{name} FUNCDATA_ArgsPointerMaps tables",
        )["stack_map"]
        function_locals = only(
            function_metadata["funcdata"],
            lambda item: item["kind"] == "locals_pointer_maps",
            f"{name} FUNCDATA_LocalsPointerMaps tables",
        )["stack_map"]
        if (
            function_args["count"] != 1
            or function_locals["count"] != 1
            or function_args["num_bits"] != num_bits
        ):
            fail(
                f"{name} has unexpected pair dimensions: "
                f"args={function_args}, locals={function_locals}"
            )
        actual_entry_bits = set(
            function_args["bitmaps"][0]["set_bits"] or []
        )
        actual_locals_bits = set(
            function_locals["bitmaps"][0]["set_bits"] or []
        )
        if actual_entry_bits != want_entry_bits or actual_locals_bits:
            fail(
                f"{name} pair0 args={sorted(actual_entry_bits)}, "
                f"locals={sorted(actual_locals_bits)}"
            )
        function_queries = function_metadata["stack_map_queries"]
        if (
            len(function_queries) != 1
            or function_queries[0]["stack_map_index"] != 0
        ):
            fail(f"{name} has unexpected morestack query: {function_queries}")

    # ABI0's input pointer occupies word 0 and its pointer result occupies word
    # 1. The entry bitmap must leave the uninitialized result word clear.
    check_entry_only("aarch64_abi0_pointer_result", 16, 2, {0})
    # The seventeenth ABIInternal integer-class argument is stack-passed at
    # argp+0; the first sixteen scalar register homes follow it and stay clear.
    check_entry_only("aarch64_stack_pointer_arg", 136, 17, {0})

    print(
        f"{FUNCTION}: frame={frame_size} locals={locals_size} "
        f"entry-args={sorted(entry_args)} ordinary-roots={sorted(ordinary_bits)}"
    )


if __name__ == "__main__":
    try:
        main()
    except (KeyError, RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"check-aarch64-frame: {error}", file=sys.stderr)
        sys.exit(1)
