#!/usr/bin/env python3
"""Partition the complete dist test list; never maintain a second test allowlist."""

import argparse
import zlib
from pathlib import Path

GROUPS = (
    "stdlib-0", "stdlib-1", "cmd-compile", "cmd-cgo", "cmd-tools",
    "testdir-0", "testdir-1", "modes-crypto", "modes-experiments", "runtime",
)


def group_for(name):
    if name.startswith("cmd/internal/testdir:"):
        shard, count = name.split(":")[1].split("_")
        if count != "2" or shard not in ("0", "1"):
            raise ValueError(f"unexpected testdir shard: {name}")
        return f"testdir-{shard}"
    if ":" in name:
        # Keep related build modes together so they share their variant cache.
        # The first CI run spent 19 minutes in the single modes job on amd64.
        package, variant = name.split(":", 1)
        if package.startswith("crypto/"):
            return "modes-crypto"
        if variant in ("nojsonv2", "runtimesecret", "simd"):
            return "modes-experiments"
        return "runtime"
    if name == "runtime" or name.startswith("runtime/"):
        return "runtime"
    if name == "cmd/compile" or name.startswith("cmd/compile/"):
        return "cmd-compile"
    if name == "cmd/cgo" or name.startswith("cmd/cgo/"):
        return "cmd-cgo"
    if name.startswith("cmd/"):
        return "cmd-tools"
    # Stable across hosts and additions to the package list, unlike hash().
    return f"stdlib-{zlib.crc32(name.encode()) % 2}"


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
