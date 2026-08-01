#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


def fail(message):
    print(f"alloca nilcheck O2 check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--opt", required=True)
    parser.add_argument("--plugin", required=True)
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    optimized = subprocess.run(
        [args.opt, "-passes=default<O2>", "-S", "-o", "-", args.input],
        capture_output=True,
        text=True,
    )
    if optimized.returncode != 0:
        fail(f"opt failed:\n{optimized.stdout}{optimized.stderr}")
    if not re.search(
        r"load volatile i8, ptr %slot, align 8, !annotation !\d+",
        optimized.stdout,
    ):
        fail("SROA replacement load lost the frontend nilcheck annotation")

    lowered = subprocess.run(
        [
            args.llc,
            f"-load-pass-plugin={args.plugin}",
            "-verify-machineinstrs",
            "-filetype=null",
            "-o",
            "-",
            "-",
        ],
        input=optimized.stdout,
        capture_output=True,
        text=True,
    )
    if lowered.returncode != 0:
        fail(f"llc failed:\n{lowered.stdout}{lowered.stderr}")


if __name__ == "__main__":
    main()
