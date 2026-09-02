#!/usr/bin/env python3

import json
import os
import re
import sys
from pathlib import Path


POLICY = re.compile(r"LLVM (codegen|run) policy: (.*)")
BLACK_RESULT = re.compile(
    r'LLVM (codegen|run) blacklist result: NOT RUN test="([^"]+)" reason="([^"]+)"'
)
TESTDIR_PREFIX = "TestLLVM/testdir/"


def markdown(value):
    return str(value).replace("|", "\\|").replace("\n", " ")


def leaf_results(actions):
    names = sorted(actions)
    leaves = []
    for name in names:
        prefix = name + "/"
        if not any(other.startswith(prefix) for other in names):
            leaves.append((name, actions[name]))
    return leaves


def main():
    if len(sys.argv) != 3:
        raise SystemExit(f"usage: {sys.argv[0]} TEST-JSON SUMMARY")

    report_path = Path(sys.argv[1])
    summary_path = Path(sys.argv[2])
    arch = os.environ.get("GOALLC_CI_ARCH", "unknown")

    lines = [f"## GoALLC LLVM tests: linux/{markdown(arch)}", ""]
    if not report_path.is_file():
        lines.extend([
            "⚠️ Test report was not produced; inspect the preceding workflow steps.",
            "",
        ])
        summary_path.write_text("\n".join(lines), encoding="utf-8")
        return

    actions = {}
    elapsed = {}
    package_action = None
    policies = {}
    black_results = []

    with report_path.open(encoding="utf-8") as report:
        for raw_line in report:
            try:
                event = json.loads(raw_line)
            except json.JSONDecodeError:
                continue

            action = event.get("Action")
            test = event.get("Test")
            if test and action in {"pass", "fail", "skip"}:
                actions[test] = action
                duration = event.get("Elapsed")
                if isinstance(duration, (int, float)):
                    elapsed[test] = max(elapsed.get(test, 0), duration)
            elif not test and action in {"pass", "fail"}:
                package_action = action

            output = event.get("Output", "")
            match = POLICY.search(output)
            if match:
                policies[match.group(1)] = match.group(2)
            match = BLACK_RESULT.search(output)
            if match:
                black_results.append(match.groups())

    status = {
        "pass": "✅ Test command passed",
        "fail": "❌ Required test or infrastructure check failed",
    }.get(package_action, "⚠️ Test run did not complete")
    lines.extend([status, ""])

    if policies:
        lines.extend(["### Policy", "", "| Suite | Classification |", "| --- | --- |"])
        for suite in ("codegen", "run"):
            if suite in policies:
                lines.append(f"| {suite} | {markdown(policies[suite])} |")
        lines.append("")

    required_actions = {
        name: action
        for name, action in actions.items()
        if name.startswith(TESTDIR_PREFIX)
    }
    required = leaf_results(required_actions)
    required_counts = {
        action: sum(1 for _, result in required if result == action)
        for action in ("pass", "fail", "skip")
    }
    lines.extend([
        "### Required tests",
        "",
        f"{required_counts['pass']} passed, {required_counts['fail']} failed, "
        f"{required_counts['skip']} skipped.",
        "",
    ])
    failed_required = [name for name, action in required if action == "fail"]
    if failed_required:
        lines.extend(["Failed required tests:", ""])
        lines.extend(f"- `{markdown(name.removeprefix(TESTDIR_PREFIX))}`" for name in failed_required)
        lines.append("")

    if black_results:
        lines.extend([
            f"<details><summary>Blacklisted cases not run ({len(black_results)})</summary>",
            "",
            "| Suite | Test | Reason |",
            "| --- | --- | --- |",
        ])
        for suite, test, reason in sorted(set(black_results)):
            lines.append(f"| {markdown(suite)} | `{markdown(test)}` | {markdown(reason)} |")
        lines.extend(["", "</details>", ""])

    leaf_names = {name for name, _ in leaf_results(actions)}
    slow_cases = sorted(
        (
            (duration, name, actions[name])
            for name, duration in elapsed.items()
            if name in leaf_names
            and name.startswith(TESTDIR_PREFIX)
            and duration >= 10
        ),
        reverse=True,
    )
    if slow_cases:
        lines.extend([
            "### Slowest executed cases",
            "",
            "Cases taking at least 10 seconds. Move cases over the one-minute CI budget to the blacklist.",
            "",
            "| Test | Result | Elapsed |",
            "| --- | --- | ---: |",
        ])
        for duration, name, action in slow_cases:
            test_name = name.removeprefix(TESTDIR_PREFIX)
            lines.append(
                f"| `{markdown(test_name)}` | {markdown(action)} | {duration:.2f}s |"
            )
        lines.append("")

    summary_path.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
