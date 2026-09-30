#!/usr/bin/env python3
"""Audit the original 664 batch2 failures with Spectre and a real GS device."""

import argparse
import csv
import re
import subprocess
import tempfile
import time
from collections import Counter
from concurrent.futures import FIRST_COMPLETED, ThreadPoolExecutor, wait
from pathlib import Path

ROOT = Path("sampledata/validation/external/_bulk/batch2")
INPUT = Path(".notes/wt/refuse664.tsv")
STATE = Path(".notes/wt/batch2-render-audit")
FIELDS = ["path", "rc", "pages", "bytes", "seconds", "detail"]


def read_rows(path: Path) -> dict[str, dict[str, str]]:
    if not path.exists():
        return {}
    with path.open(newline="") as file:
        return {row["path"]: row for row in csv.DictReader(file, delimiter="\t")}


def execute(args: list[str], timeout: float) -> tuple[int, str, str]:
    try:
        result = subprocess.run(args, capture_output=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        return 124, "", "timeout"
    stdout = result.stdout.decode("utf-8", errors="replace")
    stderr = result.stderr.decode("utf-8", errors="replace")
    return result.returncode, stdout, stderr


def detail_line(stdout: str, stderr: str) -> str:
    line = next((line for line in (stderr + "\n" + stdout).splitlines() if line.strip()), "")
    return line.replace("\t", " ")[:240]


def audit_one(phase: str, rel: str, timeout: float) -> dict[str, str]:
    path = ROOT / rel
    started = time.monotonic()
    if not path.is_file():
        return dict(path=rel, rc="2", pages="0", bytes="0", seconds="0", detail="missing input")

    if phase == "info":
        rc, stdout, stderr = execute(["./bin/spectreps", "info", str(path)], timeout)
        match = re.search(r"^Pages: (\d+)$", stdout, re.MULTILINE)
        pages = match.group(1) if match else "0"
        detail = detail_line(stderr, stdout) if rc or pages == "0" else ""
        byte_count = "0"
    elif phase == "raster":
        info = read_rows(STATE / "info.tsv").get(rel)
        if info is None or info["rc"] != "0":
            return dict(path=rel, rc="-2", pages="0", bytes="0", seconds="0", detail="info did not pass")
        with tempfile.TemporaryDirectory(prefix="spectre-batch2-") as temp:
            output = str(Path(temp) / "spectre-%d.ppm")
            rc, stdout, stderr = execute(
                ["./bin/spectreps", "raster", "-pages", "1", "-r", "12", "-o", output, str(path)], timeout
            )
            rendered = [item for item in Path(temp).glob("spectre-*.ppm") if item.stat().st_size > 0]
            pages = str(len(rendered))
            byte_count = str(sum(item.stat().st_size for item in rendered))
        detail = detail_line(stderr, stdout) if rc or pages == "0" else ""
    else:
        with tempfile.TemporaryDirectory(prefix="ghostscript-batch2-") as temp:
            output = str(Path(temp) / "gs-%d.ppm")
            rc, stdout, stderr = execute(
                ["gs", "-q", "-dNOPAUSE", "-dBATCH", "-dSAFER", "-r12", "-sDEVICE=ppmraw", f"-sOutputFile={output}", str(path)],
                timeout,
            )
            rendered = [item for item in Path(temp).glob("gs-*.ppm") if item.stat().st_size > 0]
            pages = str(len(rendered))
            byte_count = str(sum(item.stat().st_size for item in rendered))
        detail = detail_line(stderr, stdout) if rc or pages == "0" else ""

    return dict(
        path=rel,
        rc=str(rc),
        pages=pages,
        bytes=byte_count,
        seconds=f"{time.monotonic() - started:.2f}",
        detail=detail,
    )


def print_summary() -> None:
    info = read_rows(STATE / "info.tsv")
    raster = read_rows(STATE / "raster.tsv")
    gs = read_rows(STATE / "gs.tsv")
    rendered = {path for path, row in gs.items() if row["rc"] == "0" and int(row["pages"]) > 0}
    info_pass = {path for path, row in info.items() if row["rc"] == "0"}
    raster_pass = {path for path, row in raster.items() if row["rc"] == "0" and int(row["pages"]) > 0}
    print(f"rows: info={len(info)} raster={len(raster)} gs={len(gs)}")
    print(f"ghostscript statuses: {dict(Counter(row['rc'] for row in gs.values()))}")
    print(f"ghostscript emitted pages: {len(rendered)}")
    print(f"info passed: {len(info_pass)}; info passed and GS emitted pages: {len(info_pass & rendered)}")
    print(f"Spectre page 1 painted: {len(raster_pass)}; both Spectre and GS painted: {len(raster_pass & rendered)}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("phase", choices=["info", "raster", "gs", "summary"])
    parser.add_argument("--budget", type=float, default=26)
    parser.add_argument("--timeout", type=float, default=25)
    parser.add_argument("--max-items", type=int, default=0)
    args = parser.parse_args()

    if args.phase == "summary":
        print_summary()
        return

    files = [line.strip() for line in INPUT.read_text().splitlines() if line.strip()]
    STATE.mkdir(parents=True, exist_ok=True)
    output = STATE / f"{args.phase}.tsv"
    done = read_rows(output)
    pending = [rel for rel in files if rel not in done]
    if args.max_items:
        pending = pending[: args.max_items]
    queued = len(pending)
    already_done = len(done)

    deadline = time.monotonic() + args.budget
    with output.open("a", newline="") as file:
        writer = csv.DictWriter(file, fieldnames=FIELDS, delimiter="\t", lineterminator="\n")
        if not done:
            writer.writeheader()
        if args.phase == "gs":
            with ThreadPoolExecutor(max_workers=4) as pool:
                active = {}

                def submit_next() -> bool:
                    if not pending:
                        return False
                    job_timeout = min(args.timeout, deadline - time.monotonic() - 0.5)
                    if job_timeout < 1:
                        return False
                    rel = pending.pop(0)
                    future = pool.submit(audit_one, args.phase, rel, job_timeout)
                    active[future] = (rel, job_timeout)
                    return True

                while len(active) < 4 and submit_next():
                    pass
                while active:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        break
                    finished, _ = wait(active, timeout=remaining, return_when=FIRST_COMPLETED)
                    if not finished:
                        break
                    for future in finished:
                        rel, job_timeout = active.pop(future)
                        row = future.result()
                        if row["rc"] != "124" or job_timeout >= args.timeout:
                            writer.writerow(row)
                            file.flush()
                            done[rel] = row
                        submit_next()
        else:
            for rel in pending:
                remaining = deadline - time.monotonic()
                if remaining < 1:
                    break
                row = audit_one(args.phase, rel, min(args.timeout, remaining - 0.5))
                writer.writerow(row)
                file.flush()
                done[rel] = row

    print(f"{args.phase}: {len(done)}/{len(files)} complete; {max(0, queued - (len(done) - already_done))} queued")


if __name__ == "__main__":
    main()
