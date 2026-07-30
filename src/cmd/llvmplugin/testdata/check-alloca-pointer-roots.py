#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


def fail(message):
    print(f"alloca pointer root check failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def function_body(ir, name):
    match = re.search(
        rf"define goabiinternal [^@]+@{name}\b.*?^}}",
        ir,
        re.MULTILINE | re.DOTALL,
    )
    if not match:
        fail(f"missing function {name}")
    return match.group(0)


def require(text, pattern, description):
    if not re.search(pattern, text, re.MULTILINE | re.DOTALL):
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

    roots = re.findall(
        r"^\s*%[\w.]+ = load volatile ptr, ptr .*"
        r"!llvm\.statepoint\.fixed_stack_home !\d+",
        ir,
        re.MULTILINE,
    )
    if len(roots) != 11:
        fail(f"found {len(roots)} canonical roots, want 11")

    live_lines = [line for line in ir.splitlines() if '"gc-live"' in line]
    if len(live_lines) != 9:
        fail(f"found {len(live_lines)} gc-live bundles, want 9")
    for line in live_lines:
        bundle = line.split('"gc-live"', 1)[1]
        if ".gc.leaf." not in bundle or ".root" not in bundle:
            fail(f"alloca root missing from gc-live: {line.strip()}")
        if re.search(r"\bptr %slot(?=[,)])", bundle):
            fail(f"static alloca address survived in gc-live: {line.strip()}")
        if any(marker in bundle for marker in ("%nested", "{", "[")):
            fail(f"aggregate survived in gc-live: {line.strip()}")
    if re.search(r"%slot\.relocated\d* = call coldcc ptr", ir):
        fail("static alloca address received a gc.relocate")

    null_initializers = re.findall(r"^\s*store ptr null, ptr ", ir, re.MULTILINE)
    if len(null_initializers) != 10:
        fail(f"found {len(null_initializers)} null leaf initializers, want 10")

    pointer_slot = function_body(ir, "pointer_slot")
    require(
        pointer_slot,
        r"%slot\.gc\.leaf\.root\.relocated = call coldcc ptr "
        r"@llvm\.experimental\.gc\.relocate",
        "scalar alloca leaf relocation",
    )
    require(
        pointer_slot,
        r"store ptr %slot\.gc\.leaf\.root\.relocated, "
        r"ptr %slot",
        "scalar alloca leaf write-back",
    )

    nested = function_body(ir, "nested_whole_aggregate")
    for path in ("0", "2.0.1", "2.1.1"):
        require(
            nested,
            rf"%slot\.gc\.leaf\.{path}\.root = load volatile ptr, ptr "
            rf"%slot\.gc\.leaf\.{path}\.pre\.addr.*"
            rf"!llvm\.statepoint\.fixed_stack_home",
            f"nested canonical root {path}",
        )
        require(
            nested,
            rf"store ptr %slot\.gc\.leaf\.{path}\.root\.relocated, "
            rf"ptr %slot\.gc\.leaf\.{path}\.post\.addr",
            f"nested relocated write-back {path}",
        )
    require(
        nested,
        r"%reloaded = load %nested, ptr %slot",
        "whole aggregate reload from the fixed alloca",
    )

    call_skip = function_body(ir, "alloca_call_skip")
    if re.search(r"%slot.* = phi ptr", call_skip):
        fail("call/skip formed a relocation PHI for a static alloca address")

    multiple = function_body(ir, "alloca_multiple_calls")
    if multiple.count("!llvm.statepoint.fixed_stack_home") != 2:
        fail("multiple-call function did not reload its canonical home twice")
    if len(re.findall(r"\.root\d*\.relocated = call coldcc ptr", multiple)) != 2:
        fail("multiple-call function did not relocate both canonical roots")

    loop = function_body(ir, "alloca_loop")
    require(
        loop,
        r"%slot\.gc\.leaf\.root\.relocated = call coldcc ptr "
        r"@llvm\.experimental\.gc\.relocate",
        "loop canonical root relocation",
    )
    require(
        loop,
        r"store ptr %slot\.gc\.leaf\.root\.relocated, ptr %slot",
        "loop canonical root write-back before the backedge",
    )
    require(loop, r"br i1 %again, label %loop, label %exit", "loop backedge")

    gep = function_body(ir, "alloca_gep_address_across_call")
    gep_live = next(
        (line for line in gep.splitlines() if '"gc-live"' in line),
        None,
    )
    if not gep_live:
        fail("GEP-address function is missing gc-live")
    if re.search(r"\bptr %field(?=[,)])", gep_live):
        fail("static alloca GEP address survived in gc-live")
    if "%field.relocated" in gep:
        fail("static alloca GEP address received a gc.relocate")
    require(
        gep,
        r"%slot\.gc\.leaf\.1\.root\.relocated = call coldcc ptr "
        r"@llvm\.experimental\.gc\.relocate",
        "GEP-address alloca pointer leaf relocation",
    )

    escaped = function_body(ir, "alloca_address_passed_to_callee")
    require(
        escaped,
        r"@mutate_pointer_slot, i32 1, i32 0, ptr %slot,.*"
        r'"gc-live"\(ptr %slot\.gc\.leaf\.root\)',
        "address-passed alloca statepoint root",
    )
    require(
        escaped,
        r"store ptr %slot\.gc\.leaf\.root\.relocated, "
        r"ptr %slot",
        "address-passed alloca relocated write-back",
    )

    uninitialized = function_body(ir, "alloca_uninitialized_at_safepoint")
    require(
        uninitialized,
        r"store ptr null, ptr %slot.*?"
        r"%slot\.gc\.leaf\.root = load volatile ptr, ptr %slot",
        "null initialization before the first safepoint root load",
    )


if __name__ == "__main__":
    main()
