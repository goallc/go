#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


def fail(message):
    print(f"call-only liveness check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
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

    for function in ("indirect_callee", "call_only_pointer_argument"):
        body = re.search(
            rf"define goabiinternal void @{function}.*?^}}",
            ir,
            re.MULTILINE | re.DOTALL,
        )
        if not body:
            fail(f"missing rewritten function {function}")
        if "@llvm.experimental.gc.statepoint" not in body.group(0):
            fail(f"{function} was not rewritten to a statepoint")
        if '"gc-live"' in body.group(0):
            fail(f"{function} recorded a call-only pointer in caller gc-live")
        if "@llvm.experimental.gc.relocate" in body.group(0):
            fail(f"{function} emitted a relocate for a call-only pointer")


if __name__ == "__main__":
    main()
