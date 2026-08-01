#!/usr/bin/env python3

import argparse
import os
import subprocess
import sys


MODULE = r'''
target triple = "x86_64-unknown-linux-goobj"

declare goabiinternal void @callee()
declare token @llvm.experimental.gc.statepoint.p0(
    i64 immarg, i32 immarg, ptr, i32 immarg, i32 immarg, ...)

define goabiinternal void @test() #0 gc "goallc" {
entry:
  %slot = alloca [2 x ptr], align 8
  store [2 x ptr] zeroinitializer, ptr %slot, align 8
  %statepoint = call goabiinternal token (i64, i32, ptr, i32, i32, ...)
      @llvm.experimental.gc.statepoint.p0(
          i64 1, i32 0, ptr elementtype(void ()) @callee,
          i32 0, i32 0, i32 0, i32 0) [ "deopt"(__DEOPT__) ]
  ret void
}

attributes #0 = { "go-stack-growth-statepoint" }
'''


CASES = {
    "truncated": (
        "i64 1195461697, i64 15, i64 1, i64 1347703373",
        "protocol is truncated",
    ),
    "bad_length": (
        "i64 1195461697, i64 14, i64 1, i64 1347703373, i64 11, "
        "ptr %slot, i64 0, i64 16, i64 8, i64 8, i64 2, i64 64, "
        "i64 1, i64 3, i64 1095519299, i64 15",
        "protocol envelope is malformed",
    ),
    "duplicate": (
        "i64 1195461697, i64 26, i64 2, "
        "i64 1347703373, i64 11, ptr %slot, i64 0, i64 16, i64 8, "
        "i64 8, i64 2, i64 64, i64 1, i64 3, "
        "i64 1347703373, i64 11, ptr %slot, i64 0, i64 16, i64 8, "
        "i64 8, i64 2, i64 64, i64 1, i64 3, "
        "i64 1095519299, i64 26",
        "duplicate frame record",
    ),
    "overlap": (
        "i64 1195461697, i64 26, i64 2, "
        "i64 1347703373, i64 11, ptr %slot, i64 0, i64 8, i64 8, "
        "i64 8, i64 1, i64 64, i64 1, i64 1, "
        "i64 1347703373, i64 11, ptr %slot, i64 0, i64 16, i64 8, "
        "i64 8, i64 2, i64 64, i64 1, i64 3, "
        "i64 1095519299, i64 26",
        "records overlap",
    ),
    "padding": (
        "i64 1195461697, i64 15, i64 1, i64 1347703373, i64 11, "
        "ptr %slot, i64 0, i64 8, i64 8, i64 8, i64 1, i64 64, "
        "i64 1, i64 3, i64 1095519299, i64 15",
        "padding bits are nonzero",
    ),
    "non_direct": (
        "i64 1195461697, i64 15, i64 1, i64 1347703373, i64 11, "
        "i64 0, i64 0, i64 8, i64 8, i64 8, i64 1, i64 64, "
        "i64 1, i64 1, i64 1095519299, i64 15",
        "base is not a direct frame location",
    ),
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--llc", required=True)
    parser.add_argument("--plugin", required=True)
    args = parser.parse_args()

    for name, (deopt, diagnostic) in CASES.items():
        module = MODULE.replace("__DEOPT__", deopt)
        result = subprocess.run(
            [
                args.llc,
                f"-load-pass-plugin={args.plugin}",
                "-verify-machineinstrs",
                "-filetype=obj",
                "-o",
                os.devnull,
                "-",
            ],
            input=module,
            capture_output=True,
            text=True,
        )
        output = result.stdout + result.stderr
        if result.returncode == 0:
            print(f"{name}: malformed contract unexpectedly succeeded", file=sys.stderr)
            raise SystemExit(1)
        if diagnostic not in output:
            print(
                f"{name}: missing expected diagnostic {diagnostic!r}:\n{output}",
                file=sys.stderr,
            )
            raise SystemExit(1)


if __name__ == "__main__":
    main()
