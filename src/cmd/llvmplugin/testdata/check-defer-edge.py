#!/usr/bin/env python3

import argparse
import json
import subprocess
import sys


FUNCTION = "defer_edge"
WRAPPER = "defer_wrapper"


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
        [args.objview, "-format=json", args.object],
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
        lambda item: item["name"] == FUNCTION,
        f"{FUNCTION} symbols",
    )
    metadata = function.get("function")
    if metadata is None:
        fail(f"{FUNCTION} has no function metadata")

    wrapper = only(
        obj["symbols"],
        lambda item: item["name"] == WRAPPER,
        f"{WRAPPER} symbols",
    )
    wrapper_metadata = wrapper.get("function")
    if wrapper_metadata is None or wrapper_metadata.get("info") is None:
        fail(f"{WRAPPER} has no function info")
    wrapper_info = wrapper_metadata["info"]
    if wrapper_info["func_id"] != 23 or wrapper_info["func_flags"] != 0:
        fail(f"{WRAPPER} lost GoObj wrapper identity: {wrapper_info}")

    references = {item["index"]: item["name"] for item in obj["references"]}

    def target_name(relocation):
        target = relocation["target"]
        if "name" in target:
            return target["name"]
        return references.get(target["sym_index"])

    deferreturn = only(
        function["relocations"],
        lambda item: item["type"] == "R_CALL"
        and target_name(item) == "runtime.deferreturn",
        "direct runtime.deferreturn relocations",
    )
    query = only(
        metadata["stack_map_queries"],
        lambda item: item["call_offset"] == deferreturn["offset"],
        "runtime.deferreturn stack-map queries",
    )
    if query["relocation_type"] != "R_CALL":
        fail(f"deferreturn is not represented as a direct call: {query}")
    if query["stack_map_index"] < 0:
        fail(f"deferreturn has no valid GC stack map: {query}")

    print(
        f"{FUNCTION}: runtime.deferreturn R_CALL at {deferreturn['offset']}, "
        f"stack map {query['stack_map_index']}; {WRAPPER}: FuncID 23, flags 0"
    )


if __name__ == "__main__":
    try:
        main()
    except (KeyError, RuntimeError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"check-defer-edge: {error}", file=sys.stderr)
        sys.exit(1)
