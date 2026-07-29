#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


def fail(message):
    print(f"aggregate scalarization check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--opt", required=True)
    parser.add_argument("--plugin", required=True)
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    rewrite = subprocess.run(
        [
            args.llc,
            f"-load-pass-plugin={args.plugin}",
            "-goallc-pass-plugin-emit-ir",
            "-filetype=null",
            "-o",
            "-",
            args.input,
        ],
        capture_output=True,
        text=True,
    )
    if rewrite.returncode != 0:
        fail(f"llc failed:\n{rewrite.stdout}{rewrite.stderr}")
    ir = rewrite.stdout + rewrite.stderr

    verify = subprocess.run(
        [
            args.opt,
            f"-load-pass-plugin={args.plugin}",
            "-passes=verify",
            "-disable-output",
            "-",
        ],
        input=ir,
        capture_output=True,
        text=True,
    )
    if verify.returncode != 0:
        fail(f"opt verifier failed:\n{verify.stdout}{verify.stderr}")

    live_lines = [line for line in ir.splitlines() if '"gc-live"' in line]
    if not live_lines:
        fail("rewritten IR has no gc-live bundle")
    for line in live_lines:
        bundle = line.split('"gc-live"', 1)[1]
        if any(marker in bundle for marker in ("{", "[", "<")):
            fail(f"aggregate survived in gc-live: {line.strip()}")
        if not re.search(r'"gc-live"\(ptr ', line):
            fail(f"gc-live contains a non-scalar-pointer operand: {line.strip()}")

    required = {
        "two-pointer aggregate leaves":
            r'gc-live"\(ptr %value\.leaf\.1, ptr %value\.leaf\.2\)',
        "nested array leaf extraction": r"extractvalue %nested %value, 1, 1, 0",
        "relocated leaf reconstruction":
            r"insertvalue %pair .*%value\.leaf\.0\.relocated",
        "aggregate phi scalarization": r"%value\.leaf\.0 = extractvalue %pair %value, 0",
        "aggregate call result":
            r"call %pair @llvm\.experimental\.gc\.result\.[^(]+",
        "aggregate current call argument":
            r'@consume_pair.*"gc-live"\(ptr %value\.leaf\.0\)',
        "multiple relocation chain": r"%value\.leaf\.0\.relocated2",
        "aggregate load result": r"%value\.leaf\.0 = extractvalue %pair %value, 0",
        "freeze preserved before extraction": r"%value = freeze %pair poison",
    }
    for description, pattern in required.items():
        if not re.search(pattern, ir):
            fail(f"missing {description}: /{pattern}/")

    if re.search(r'"gc-live"\([^)]*(?:%pair|%triple|%nested)', ir):
        fail("named aggregate type/value survived in gc-live")


if __name__ == "__main__":
    main()
