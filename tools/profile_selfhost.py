#!/usr/bin/env python3
"""Collect paired sync profiles in an isolated copy; see compiler3/performance.md."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import statistics
import subprocess
import sys
import tempfile
import time


def generated_snapshot(root):
    result = {}
    for path in sorted(root.rglob("*.go")):
        if path.name.endswith((".gen.go", ".gen_test.go")):
            result[str(path.relative_to(root))] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result


def source_digest(root):
    digest = hashlib.sha256()
    for path in sorted(root.rglob("*")):
        if path.is_file() and (path.suffix in (".go", ".mygo") or path.name in ("go.mod", "go.sum")):
            name = str(path.relative_to(root)).encode()
            digest.update(len(name).to_bytes(8, "big"))
            digest.update(name)
            digest.update(hashlib.sha256(path.read_bytes()).digest())
    return digest.hexdigest()


def parse_profile_log(text):
    phases, passes, logged_size = {}, {}, 0
    # Runtime GC traces can interleave on stderr with the compiler's logs.
    for timing in re.finditer(r"\[ir3-(phase|pass)\] (\S+) (\S+) (\d+)", text):
        totals = phases if timing.group(1) == "phase" else passes
        totals[timing.group(3)] = totals.get(timing.group(3), 0) + int(timing.group(4))
    for size_log in re.finditer(r"\[ir3-size\] (\S+) (\d+)", text):
        logged_size += int(size_log.group(2))
    return phases, passes, logged_size


def run(args, cwd, env, out, err):
    start = time.monotonic()
    with out.open("wb") as stdout, err.open("wb") as stderr:
        proc = subprocess.Popen(args, cwd=cwd, env=env, stdout=stdout, stderr=stderr)
        _, status, usage = os.wait4(proc.pid, 0)
        proc.returncode = os.waitstatus_to_exitcode(status)
    result = {
        "command": args,
        "returncode": proc.returncode,
        "wall_seconds": time.monotonic() - start,
        "user_seconds": usage.ru_utime,
        "system_seconds": usage.ru_stime,
        # Linux reports KiB; macOS reports bytes.
        "max_rss_bytes": usage.ru_maxrss if sys.platform == "darwin" else usage.ru_maxrss * 1024,
    }
    if proc.returncode:
        raise RuntimeError(f"{args!r} failed ({proc.returncode}); see {err}")
    return result


def save_results(output, records):
    (output / "runs.json").write_text(json.dumps(records, indent=2) + "\n")
    grouped = {}
    for item in records:
        key = (item["build_backend"], item["runtime_backend"], item.get("level"), item.get("cache"), item["profiled"])
        grouped.setdefault(key, []).append(item)
    summary = []
    for key, items in grouped.items():
        row = dict(zip(("build_backend", "runtime_backend", "level", "cache", "profiled"), key))
        row["count"] = len(items)
        for field in ("wall_seconds", "user_seconds", "system_seconds", "max_rss_bytes", "generated_bytes"):
            values = [item[field] for item in items if item.get(field) is not None]
            if values:
                row[field] = {"median": statistics.median(values), "min": min(values), "max": max(values)}
        summary.append(row)
    (output / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--codegen2", required=True, type=Path, help="profile-enabled codegen2-built compiler")
    parser.add_argument("--codegen3", required=True, type=Path, help="profile-enabled codegen3 O2-built compiler")
    parser.add_argument("--output", required=True, type=Path, help="new output directory for logs and profiles")
    parser.add_argument("--workload", default=".", help="relative sync target within the copied repository")
    parser.add_argument("--count", type=int, default=2, help="number of paired repetitions")
    options = parser.parse_args()
    if options.count < 1 or Path(options.workload).is_absolute() or ".." in Path(options.workload).parts:
        parser.error("count must be positive and workload must be a relative path inside the repository")
    root = Path(__file__).resolve().parent.parent
    binaries = {"codegen2-built": options.codegen2.resolve(strict=True), "codegen3-o2-built": options.codegen3.resolve(strict=True)}
    output = options.output.resolve()
    if output == root or root in output.parents:
        parser.error("output must be outside the repository")
    output.mkdir(parents=True, exist_ok=False)
    metadata = {
        "workload": options.workload,
        "count": options.count,
        "go_version": subprocess.check_output(["go", "version"], text=True).strip(),
        "go_environment": json.loads(subprocess.check_output(["go", "env", "-json", "GOOS", "GOARCH", "GOAMD64", "CGO_ENABLED"], text=True)),
        "platform": platform.platform(),
        "processor": platform.processor(),
        "revision": subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True).stdout.strip(),
        "dirty": bool(subprocess.run(["git", "status", "--porcelain"], cwd=root, capture_output=True, text=True).stdout),
        "gomaxprocs": os.environ.get("GOMAXPROCS", "Go default"),
        "binaries": {name: {"path": str(path), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()} for name, path in binaries.items()},
    }
    cpuinfo = Path("/proc/cpuinfo")
    if cpuinfo.exists():
        metadata["cpu_model"] = next((line.split(":", 1)[1].strip() for line in cpuinfo.read_text().splitlines() if line.startswith("model name")), "unknown")
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    env = os.environ.copy()
    env["GOCACHE"] = str(output / "go-cache")
    env["GOFLAGS"] = (env.get("GOFLAGS", "") + " -buildvcs=false").strip()
    env["GODEBUG"] = (env.get("GODEBUG", "") + ",gctrace=1").strip(",")
    env.pop("MYGO_PERF_CPU", None)
    env.pop("MYGO_PERF_MEM", None)
    env.pop("MYGO_PROFILE_TIMING", None)
    (output / "go-cache").mkdir()
    with tempfile.TemporaryDirectory(prefix="mygo-profile-") as tmp:
        checkout = Path(tmp) / "checkout"
        checkout.mkdir()
        subprocess.run(["rsync", "-rlt", "--exclude=/.git", "--exclude=/mygo", "--exclude=/mygo3", "--exclude=/build", str(root) + "/", str(checkout) + "/"], check=True)
        metadata["copied_source_sha256"] = source_digest(checkout)
        (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
        if not (checkout / options.workload).exists():
            raise RuntimeError(f"workload {options.workload!r} does not exist")
        # Warm the Go cache identically before recording either binary.
        with (output / "warm-go-build.log").open("wb") as warm_log:
            subprocess.run(["go", "build", "-o", str(Path(tmp) / "warm-mygo"), "./cmd/mygo"], cwd=checkout, env=env, stdout=warm_log, stderr=warm_log, check=True)
        records = []
        for repetition in range(options.count):
            order = list(binaries)
            if repetition % 2:
                order.reverse()
            for profiled in (False, True):
                for name in order:
                    label = f"default-{repetition}-{name}-{'profiled' if profiled else 'timed'}"
                    run_env = env.copy()
                    if profiled:
                        run_env.update(MYGO_PERF_CPU=str(output / (label + ".cpu")), MYGO_PERF_MEM=str(output / (label + ".heap")))
                    record = run([str(binaries[name]), "--bootstrap-timing", "sync", options.workload], checkout, run_env, output / (label + ".out"), output / (label + ".err"))
                    record.update(label=label, build_backend=name, runtime_backend="codegen2", repetition=repetition, profiled=profiled)
                    if profiled and (not (output / (label + ".cpu")).is_file() or not (output / (label + ".heap")).is_file()):
                        raise RuntimeError(f"{name} did not write profiles; build it with the hooks documented in compiler3/performance.md")
                    records.append(record)
                    save_results(output, records)
                    print(label, f"{record['wall_seconds']:.2f}s", flush=True)
        # Exercise each optimization level with independent cold caches and a
        # second warm run. Each repetition starts with a fresh cache.
        for level in range(3):
            for repetition in range(options.count):
                for profiled in (False, True):
                    cache = Path(tmp) / f"cache-o{level}-{repetition}-{profiled}"
                    for mode in ("cold", "warm", "off"):
                        label = f"codegen3-o{level}-{mode}-{repetition}-{'profiled' if profiled else 'timed'}"
                        run_env = env | {"MYGO_CACHE": str(cache) if mode != "off" else "off"}
                        if profiled:
                            run_env.update(MYGO_PERF_CPU=str(output / (label + ".cpu")), MYGO_PERF_MEM=str(output / (label + ".heap")), MYGO_PROFILE_TIMING="1")
                        emit = output / (label + "-generated")
                        before = generated_snapshot(checkout)
                        record = run([str(binaries["codegen3-o2-built"]), "--codegen3", f"--opt-level={level}", f"--codegen3-emit={emit}", "sync", options.workload], checkout, run_env, output / (label + ".out"), output / (label + ".err"))
                        if generated_snapshot(checkout) != before:
                            raise RuntimeError(f"{label} changed generated Go files in the source tree")
                        if profiled and (not (output / (label + ".cpu")).is_file() or not (output / (label + ".heap")).is_file()):
                            raise RuntimeError(f"{label} did not write profiles")
                        generated_files = list(emit.rglob("*.go"))
                        phases, passes, logged_size = parse_profile_log((output / (label + ".err")).read_text(errors="replace"))
                        if profiled and mode != "warm" and (not phases or (level > 0 and not passes)):
                            raise RuntimeError(f"{label} has no phase/pass timings; use a compiler built with MYGO_PROFILE_TIMING support and a MyGO workload")
                        size = sum(p.stat().st_size for p in generated_files) if generated_files else (logged_size or None)
                        size_source = "emit_copy" if generated_files else ("generation_log" if logged_size else "unavailable")
                        record.update(label=label, build_backend="codegen3-o2-built", runtime_backend="codegen3", level=level, cache=mode, repetition=repetition, profiled=profiled, generated_bytes=size, generated_size_source=size_source, phase_nanoseconds=phases, pass_nanoseconds=passes)
                        records.append(record)
                        save_results(output, records)
                        print(label, f"{record['wall_seconds']:.2f}s", flush=True)
    print(f"profiles and raw runs: {output}")


if __name__ == "__main__":
    main()
