#!/usr/bin/env python3

"""Resolve the immutable LLVM payload selected by a GoALLC CI run."""

import argparse
import json
import os
import re
import sys
import time
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen


LLVM_REPOSITORY = "goallc/llvm-project"
LLVM_BASE_BRANCH = "llvm23.1.master"
LLVM_CI_RELEASE = "goallc-llvm-ci"
DEPENDENCY_PREFIX = "LLVM-PR:"
DEPENDENCY = re.compile(
    r"^LLVM-PR:\s*(?:goallc/llvm-project#)?([1-9][0-9]*)\s*$"
)
REVISION = re.compile(r"^[0-9a-f]{40}$")
ARCHITECTURES = {"amd64", "arm64"}


class PayloadError(RuntimeError):
    pass


def dependency_from_body(body):
    body = re.sub(r"<!--.*?-->", "", body or "", flags=re.DOTALL)
    declarations = [
        line.strip()
        for line in body.splitlines()
        if line.strip().startswith(DEPENDENCY_PREFIX)
    ]
    if not declarations:
        return None
    if len(declarations) != 1:
        raise PayloadError("the PR body must contain at most one LLVM-PR declaration")
    match = DEPENDENCY.fullmatch(declarations[0])
    if not match:
        raise PayloadError(
            "invalid LLVM-PR declaration; use "
            "'LLVM-PR: goallc/llvm-project#123'"
        )
    return int(match.group(1))


def selected_pr(event):
    if event.get("pull_request"):
        return dependency_from_body(event["pull_request"].get("body"))
    value = event.get("inputs", {}).get("llvm-pr", "").strip()
    if not value:
        return None
    return dependency_from_body(f"{DEPENDENCY_PREFIX} {value}")


def api_json(url):
    request = Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "goallc-go-ci",
            "X-GitHub-Api-Version": "2022-11-28",
        },
    )
    try:
        with urlopen(request, timeout=30) as response:
            return json.load(response)
    except HTTPError as error:
        raise PayloadError(f"GitHub API returned HTTP {error.code} for {url}") from error
    except (URLError, TimeoutError) as error:
        raise PayloadError(f"GitHub API request failed for {url}: {error}") from error


def resolve_pr(pr_number, api_url):
    pull = api_json(
        f"{api_url.rstrip('/')}/repos/{LLVM_REPOSITORY}/pulls/{pr_number}"
    )
    state = pull.get("state")
    if state != "open" and not (state == "closed" and pull.get("merged_at")):
        raise PayloadError(f"{LLVM_REPOSITORY}#{pr_number} is neither open nor merged")
    base = pull.get("base") or {}
    base_repo = (base.get("repo") or {}).get("full_name")
    if base_repo != LLVM_REPOSITORY or base.get("ref") != LLVM_BASE_BRANCH:
        raise PayloadError(
            f"{LLVM_REPOSITORY}#{pr_number} must target "
            f"{LLVM_REPOSITORY}:{LLVM_BASE_BRANCH}"
        )
    revision = ((pull.get("head") or {}).get("sha") or "").lower()
    if not REVISION.fullmatch(revision):
        raise PayloadError(f"{LLVM_REPOSITORY}#{pr_number} has no valid head revision")
    return revision


def release_asset_url(server_url, tag, asset):
    return (
        f"{server_url.rstrip('/')}/{LLVM_REPOSITORY}/releases/download/"
        f"{quote(tag, safe='')}/{quote(asset, safe='')}"
    )


def asset_exists(url):
    request = Request(url, method="HEAD", headers={"User-Agent": "goallc-go-ci"})
    try:
        with urlopen(request, timeout=30) as response:
            return response.status == 200
    except HTTPError as error:
        if error.code == 404:
            return False
        raise PayloadError(f"release asset check returned HTTP {error.code}: {url}") from error
    except (URLError, TimeoutError) as error:
        print(f"temporary release asset check failure: {error}", file=sys.stderr)
        return False


def wait_for_assets(server_url, tag, prefix, architectures, wait_seconds, poll_seconds):
    assets = [
        f"{prefix}-linux-{arch}.tar.zst{suffix}"
        for arch in architectures
        for suffix in ("", ".sha256")
    ]
    deadline = time.monotonic() + wait_seconds
    while True:
        missing = [
            asset
            for asset in assets
            if not asset_exists(release_asset_url(server_url, tag, asset))
        ]
        if not missing:
            return
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise PayloadError(
                "LLVM PR CI payload is not available yet; rerun this workflow "
                f"after its publish job succeeds. Missing: {', '.join(missing)}"
            )
        print(
            "Waiting for LLVM PR CI payloads: " + ", ".join(missing),
            flush=True,
        )
        time.sleep(min(poll_seconds, remaining))


def write_outputs(path, outputs):
    with Path(path).open("a", encoding="utf-8") as output:
        for name, value in outputs.items():
            if "\n" in value or "\r" in value:
                raise PayloadError(f"invalid newline in workflow output {name}")
            output.write(f"{name}={value}\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--event", required=True)
    parser.add_argument("--github-output", required=True)
    parser.add_argument("--pinned-release", required=True)
    parser.add_argument("--pinned-revision", required=True)
    parser.add_argument("--architectures", default="amd64,arm64")
    parser.add_argument("--wait-seconds", type=int, default=12_600)
    parser.add_argument("--poll-seconds", type=int, default=30)
    parser.add_argument(
        "--api-url", default=os.environ.get("GITHUB_API_URL", "https://api.github.com")
    )
    parser.add_argument(
        "--server-url", default=os.environ.get("GITHUB_SERVER_URL", "https://github.com")
    )
    args = parser.parse_args()

    if not REVISION.fullmatch(args.pinned_revision):
        raise PayloadError("--pinned-revision must be a full lowercase Git revision")
    architectures = args.architectures.split(",")
    if not architectures or any(arch not in ARCHITECTURES for arch in architectures):
        raise PayloadError("--architectures must contain only amd64 and arm64")
    if args.wait_seconds < 0 or args.poll_seconds <= 0:
        raise PayloadError("wait and poll durations must be positive")

    event = json.loads(Path(args.event).read_text(encoding="utf-8"))
    pr_number = selected_pr(event)
    if pr_number is None:
        outputs = {
            "mode": "release",
            "release_tag": args.pinned_release,
            "asset_prefix": args.pinned_release,
            "revision": args.pinned_revision,
            "source": f"pinned release {args.pinned_release}",
        }
    else:
        revision = resolve_pr(pr_number, args.api_url)
        prefix = f"goallc-llvm-pr{pr_number}-{revision}"
        wait_for_assets(
            args.server_url,
            LLVM_CI_RELEASE,
            prefix,
            architectures,
            args.wait_seconds,
            args.poll_seconds,
        )
        outputs = {
            "mode": "pull-request",
            "release_tag": LLVM_CI_RELEASE,
            "asset_prefix": prefix,
            "revision": revision,
            "source": f"{LLVM_REPOSITORY}#{pr_number} at {revision}",
        }

    write_outputs(args.github_output, outputs)
    print(f"Selected LLVM payload: {outputs['source']}")


if __name__ == "__main__":
    try:
        main()
    except (PayloadError, json.JSONDecodeError, OSError) as error:
        raise SystemExit(f"resolve LLVM payload: {error}")
