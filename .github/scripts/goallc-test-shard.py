#!/usr/bin/env python3
"""Partition the complete dist test list; never maintain a second test allowlist."""

import argparse
from pathlib import Path

GROUPS = ("stdlib", "cmd", "runtime", "testdir-0", "testdir-1", "modes")


def group_for(name):
    if name.startswith("cmd/internal/testdir:"):
        shard, count = name.split(":")[1].split("_")
        if count != "2" or shard not in ("0", "1"):
            raise ValueError(f"unexpected testdir shard: {name}")
        return f"testdir-{shard}"
    if ":" in name:
        return "modes"
    if name == "runtime" or name.startswith("runtime/"):
        return "runtime"
    if name.startswith("cmd/"):
        return "cmd"
    return "stdlib"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("test_list", type=Path)
    parser.add_argument("group", choices=GROUPS)
    args = parser.parse_args()
    names = args.test_list.read_text().splitlines()
    if not names or any(not name or name.strip() != name for name in names):
        parser.error("empty or malformed dist test list")
    if len(names) != len(set(names)):
        parser.error("duplicate dist test names")
    groups = {group: [] for group in GROUPS}
    for name in names:
        groups[group_for(name)].append(name)
    if any(not tests for tests in groups.values()):
        parser.error("empty test group; check GO_TEST_SHARDS and dist registration")
    print("\n".join(groups[args.group]))


if __name__ == "__main__":
    main()
