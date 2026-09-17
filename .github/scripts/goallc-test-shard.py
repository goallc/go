#!/usr/bin/env python3
"""Partition the complete dist test list; never maintain a second test allowlist."""

import argparse
from pathlib import Path

GROUPS = ("stdlib", "compiler", "tools", "runtime")


def group_for(name):
    if name.startswith("cmd/internal/testdir:"):
        if name != "cmd/internal/testdir:0_1":
            raise ValueError(f"unexpected testdir shard: {name}")
        return "runtime"
    if ":" in name:
        # Keep related build modes together so they share their variant cache.
        package, variant = name.split(":", 1)
        if package.startswith("crypto/"):
            return "tools"
        if variant in ("nojsonv2", "runtimesecret", "simd"):
            return "compiler"
        return "runtime"
    if name == "runtime" or name.startswith("runtime/"):
        return "runtime"
    if name == "cmd/compile" or name.startswith("cmd/compile/"):
        return "compiler"
    if name.startswith("cmd/"):
        return "tools"
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
