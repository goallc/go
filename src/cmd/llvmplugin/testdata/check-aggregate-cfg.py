#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


def fail(message):
    print(f"aggregate CFG check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def function_body(ir, name):
    match = re.search(
        rf"define [^@]*@{re.escape(name)}\([^{{]*\) [^{{]*\{{\n(.*?)\n\}}",
        ir,
        re.DOTALL,
    )
    if not match:
        fail(f"missing rewritten function {name}")
    return match.group(1)


def require(body, pattern, description):
    if not re.search(pattern, body, re.DOTALL):
        fail(f"missing {description}: /{pattern}/")


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

    if re.search(
        r"^\s*(?:%[-\w.]+ = )?(?:alloca|load)\b|^\s*store\b",
        ir,
        re.MULTILINE,
    ):
        fail("temporary relocation memory traffic survived PromoteMemToReg")

    diamond = function_body(ir, "aggregate_diamond_call_skip")
    require(
        diamond,
        r"i64 -4232994149196383034",
        "stable aggregate diamond statepoint ID",
    )
    require(
        diamond,
        r"phi ptr \[ %value\.leaf\.0\.relocated, %call \], "
        r"\[ %value\.leaf\.0, %skip \]",
        "call/skip pointer-leaf merge",
    )
    require(
        diamond,
        r"insertvalue %pair poison, ptr %value\.leaf\.0\.relocated\.merge",
        "diamond aggregate reconstruction from current leaf",
    )

    branches = function_body(ir, "aggregate_branch_safepoints")
    if branches.count("@llvm.experimental.gc.statepoint") != 2:
        fail("two-branch case does not contain exactly two statepoints")
    require(
        branches,
        r"phi ptr \[ %value\.leaf\.0\.relocated\d*, %left \], "
        r"\[ %value\.leaf\.0\.relocated\d*, %right \]",
        "two-relocate branch merge",
    )

    sequential = function_body(ir, "aggregate_sequential_conditional")
    if sequential.count("@llvm.experimental.gc.statepoint") != 2:
        fail("sequential conditional case does not contain two statepoints")
    if sequential.count(" = phi ptr ") < 2:
        fail("sequential conditional case is missing relocation PHIs")
    require(
        sequential,
        r'"gc-live"\(ptr %value\.leaf\.0\.relocated\.merge',
        "second statepoint consuming the first current leaf",
    )

    loop = function_body(ir, "aggregate_natural_loop")
    require(loop, r"header:.*?phi ptr", "natural-loop backedge pointer PHI")
    require(
        loop,
        r'"gc-live"\(ptr %value\.leaf\.0\.relocated\.merge',
        "loop statepoint consuming the backedge current leaf",
    )

    irreducible = function_body(ir, "aggregate_irreducible")
    if irreducible.count(" = phi ptr ") < 3:
        fail("irreducible case is missing multi-entry relocation PHIs")
    require(
        irreducible,
        r"insertvalue %pair poison, ptr %value\.leaf\.0\.relocated\.merge",
        "irreducible reconstruction from merged leaf",
    )

    edge = function_body(ir, "aggregate_phi_edge_use")
    require(edge, r"%carried = phi %pair", "aggregate PHI edge use")
    if edge.count("insertvalue %pair poison") < 3:
        fail("aggregate PHI edge case is missing edge/current reconstructions")
    if edge.count("@llvm.experimental.gc.statepoint") != 2:
        fail("aggregate PHI edge case does not contain two statepoints")

    for name in (
        "aggregate_call_result_conditional",
        "aggregate_call_result_loop",
        "aggregate_call_result_irreducible",
    ):
        body = function_body(ir, name)
        require(
            body,
            r"call %pair @llvm\.experimental\.gc\.result\.",
            f"{name} aggregate gc.result",
        )
        require(body, r" = phi ptr ", f"{name} relocation PHI")
        require(
            body,
            r"insertvalue %pair poison, ptr %value\.leaf\.0",
            f"{name} reconstructed aggregate",
        )

    multiple = function_body(ir, "aggregate_multiple_safepoints")
    if multiple.count("@llvm.experimental.gc.statepoint") != 3:
        fail("multiple-safepoint case does not contain three statepoints")
    live_operands = re.findall(r'"gc-live"\(ptr ([^)]+)\)', multiple)
    if len(live_operands) != 3 or ".relocated" not in live_operands[1]:
        fail("multiple-safepoint live set does not consume relocated leaves")


if __name__ == "__main__":
    main()
