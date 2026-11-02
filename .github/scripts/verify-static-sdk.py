#!/usr/bin/env python3
"""Audit and exercise an unpacked, relocated GoALLC static release SDK."""
import argparse
import json
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import subprocess
import tempfile


def run(args, timeout=600, **kwargs):
    process = subprocess.Popen([str(x) for x in args], text=True,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               start_new_session=os.name != "nt", **kwargs)
    try:
        output, _ = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            subprocess.run(["taskkill", "/F", "/T", "/PID", str(process.pid)], check=False)
        else:
            os.killpg(process.pid, signal.SIGKILL)
        output, _ = process.communicate()
        print(output, flush=True)
        raise RuntimeError(f"command exceeded {timeout}s: {args}")
    if process.returncode:
        print(output, flush=True)
        raise subprocess.CalledProcessError(process.returncode, args, output=output)
    return output


def dependencies(path, system):
    if system == "Darwin":
        output = run(["otool", "-L", path])
        deps = [line.strip().split(" (", 1)[0] for line in output.splitlines()[1:]]
        bad = [x for x in deps if not x.startswith(("/usr/lib/", "/System/Library/"))]
        load = run(["otool", "-l", path])
        versions = re.findall(r"^\s*minos ([0-9]+\.[0-9]+)(?:\.[0-9]+)?$", load, re.M)
        if any(tuple(map(int, v.split("."))) > (13, 0) for v in versions):
            raise RuntimeError(f"{path}: deployment target exceeds macOS 13.0")
    elif system == "Linux":
        output = run(["readelf", "-d", path])
        deps = re.findall(r"\(NEEDED\).*\[(.*?)\]", output)
        allowed = {"libc.so.6", "libm.so.6", "libpthread.so.0", "libdl.so.2", "librt.so.1",
                   "ld-linux-x86-64.so.2", "ld-linux-aarch64.so.1"}
        bad = [x for x in deps if x not in allowed]
        versions = re.findall(r"GLIBC_([0-9.]+)", run(["readelf", "--version-info", path]))
        if any(tuple(map(int, x.split("."))) > (2, 35) for x in versions):
            raise RuntimeError(f"{path}: requires newer than GLIBC_2.35")
    elif system == "Windows":
        reader = shutil.which("llvm-readobj")
        if not reader:
            raise RuntimeError("llvm-readobj is required for the Windows import audit")
        output = run([reader, "--coff-imports", path])
        deps = re.findall(r"^\s*Name: (.*\.dll)\s*$", output, re.I | re.M)
        allowed = {"kernel32.dll", "ntdll.dll", "advapi32.dll", "bcrypt.dll", "crypt32.dll",
                   "dnsapi.dll", "iphlpapi.dll", "netapi32.dll", "ole32.dll", "oleaut32.dll",
                   "psapi.dll", "secur32.dll", "shell32.dll", "user32.dll", "userenv.dll",
                   "version.dll", "winmm.dll", "ws2_32.dll", "ucrtbase.dll", "msvcrt.dll"}
        bad = [x for x in deps if x.lower() not in allowed and
               not x.lower().startswith("api-ms-win-")]
    else:
        raise RuntimeError(f"unsupported release host: {system}")
    if bad:
        raise RuntimeError(f"{path}: non-system dynamic dependencies: {bad}")
    return deps


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("sdk", type=Path)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--windows-target", action="store_true")
    parser.add_argument("--cgo", action="store_true", help="also verify cgo with the caller's C toolchain")
    args = parser.parse_args()
    sdk = args.sdk.resolve()
    system = platform.system()
    suffix = ".exe" if system == "Windows" else ""
    tools = list((sdk / "pkg/tool").glob("*/*")) + list((sdk / "bin").iterdir())
    binaries = [p for p in tools if p.is_file() and (p.suffix == ".exe" if suffix else os.access(p, os.X_OK))]
    if not any(p.name == "compile" + suffix for p in binaries):
        raise RuntimeError("SDK has no compiler")
    report = {"host": system, "dependencies": {str(p.relative_to(sdk)): dependencies(p, system) for p in binaries}}
    binding = sdk / "src/cmd/vendor/github.com/goallc/go-llvm"
    for p in [sdk / "llvm", sdk / ".goallc", sdk / "pkg/goallc-llvmplugin", binding / "llvm", binding / "libLLVMGoALLC.a"]:
        if p.exists() or p.is_symlink():
            raise RuntimeError(f"build input leaked into SDK: {p}")
    if list((sdk / "pkg/tool").glob("*/lib/*")):
        raise RuntimeError("static SDK contains dynamic runtime libraries")
    with tempfile.TemporaryDirectory(prefix="goallc-release-test-") as temp:
        work = Path(temp)
        env = dict(os.environ)
        for key in list(env):
            if key.startswith(("GO", "CGO_", "LLVM", "DYLD_", "LD_", "PKG_CONFIG")) or key in ("CC", "CXX"):
                env.pop(key, None)
        env.update(GOROOT=str(sdk), GOTOOLCHAIN="local", GOENV="off", CGO_ENABLED="0",
                   GOCACHE=str(work / "cache"), GOPATH=str(work / "gopath"))
        # Only OS commands are visible during pure Go builds. LLVM and developer
        # toolchains remain available to the audit process but not this child.
        env["PATH"] = (str(Path(os.environ["SystemRoot"]) / "System32") if suffix else "/usr/bin:/bin")
        go = sdk / "bin" / ("go" + suffix)
        def command(*command_args):
            return run([go, *command_args], env=env, cwd=work)
        report["version"] = command("version").strip()
        report["compiler"] = command("tool", "compile", "-V=full").strip()
        (work / "go.mod").write_text("module releaseprobe\n\ngo 1.26\n")
        (work / "main.go").write_text('''package main
import ("fmt"; "runtime"; "sync")
func main() { var w sync.WaitGroup; for i:=0;i<8;i++ {w.Add(1);go func(){defer w.Done();runtime.GC()}()};w.Wait();fmt.Println("release-ok") }
''')
        (work / "main_test.go").write_text('''package main
import "testing"
func TestRelease(t *testing.T) { defer func(){if recover()==nil{t.Fatal("recover")}}(); panic("release") }
''')
        command("build", "-o", work / ("probe" + suffix), ".")
        assert run([work / ("probe" + suffix)], env=env).strip() == "release-ok"
        command("test", ".")
        failures = []
        def check(name, action):
            try:
                action()
                report[name] = "passed"
            except (subprocess.CalledProcessError, RuntimeError) as error:
                report[name] = str(error)
                failures.append(name)
        check("native_tests", lambda: command("test", "-short", "-timeout=5m", "fmt", "reflect", "runtime", "sync"))
        targets = [("linux", "amd64"), ("linux", "arm64"), ("darwin", "arm64")]
        if args.windows_target:
            targets.append(("windows", "amd64"))
        for goos, goarch in targets:
            env.update(GOOS=goos, GOARCH=goarch)
            check(f"cross_{goos}_{goarch}", lambda: command("build", "-o", work / f"probe-{goos}-{goarch}", "."))
        report["cross_builds"] = ["/".join(t) for t in targets]
        if args.cgo:
            env.pop("GOOS", None)
            env.pop("GOARCH", None)
            env.update(CGO_ENABLED="1", PATH=os.environ["PATH"])
            for key in ("CC", "CXX", "MACOSX_DEPLOYMENT_TARGET"):
                if key in os.environ:
                    env[key] = os.environ[key]
            (work / "main.go").write_text('''package main
/*
#include <stdlib.h>
*/
import "C"
func main(){ if C.abs(-42) != 42 { panic("cgo") } }
''')
            def cgo_probe():
                command("build", "-ldflags=-linkmode=external", "-o", work / ("cgo-probe" + suffix), ".")
                run([work / ("cgo-probe" + suffix)], env=env)
            check("cgo_external_link", cgo_probe)
    report["status"] = "failed" if failures else "passed"
    args.report.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    if failures:
        raise SystemExit("SDK qualification failed: " + ", ".join(failures))


if __name__ == "__main__":
    main()
