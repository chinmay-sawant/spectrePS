#!/usr/bin/env python3
"""Check the pinned PDF/A samples with veraPDF's explicit profile results."""

import csv
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parent.parent
CORPUS = ROOT / "sampledata/validation"
PROFILES = {"1a", "1b", "2a", "2b", "2u", "3a", "3b", "3u", "4", "4e", "4f"}


def verapdf_command():
    configured = os.environ.get("VERAPDF")
    if configured:
        return configured
    local = ROOT / "verapdf/verapdf"
    if local.is_file() and os.access(local, os.X_OK):
        return str(local)
    return shutil.which("verapdf")


def main():
    command = verapdf_command()
    if command is None:
        print("pdfa-corpus-check: veraPDF is required", file=sys.stderr)
        return 2

    with (ROOT / "scripts/pdfa-profiles.tsv").open(newline="", encoding="utf-8") as source:
        rows = list(csv.DictReader(source, delimiter="\t"))
    with (CORPUS / "manifest.tsv").open(newline="", encoding="utf-8") as source:
        manifest = {row["path"]: row for row in csv.DictReader(source, delimiter="\t")}

    positives = {row["profile"] for row in rows if row["compliant"] == "true"}
    if positives != PROFILES:
        print(f"pdfa-corpus-check: positive profiles {sorted(positives)}, want {sorted(PROFILES)}", file=sys.stderr)
        return 2

    failures = 0
    for row in rows:
        path = CORPUS / row["path"]
        profile = row["profile"]
        expected = row["compliant"] == "true"
        if profile not in PROFILES or row["compliant"] not in {"true", "false"} or not path.is_file():
            print(f"FAIL {row['path']}: invalid profile map row", file=sys.stderr)
            failures += 1
            continue
        pinned = manifest.get(row["path"])
        data = path.read_bytes()
        if pinned is None or len(data) != int(pinned["bytes"]) or hashlib.sha256(data).hexdigest() != pinned["sha256"]:
            print(f"FAIL {row['path']}: missing or changed manifest sample", file=sys.stderr)
            failures += 1
            continue

        result = subprocess.run(
            [command, "--flavour", profile, "--format", "json", str(path)],
            capture_output=True,
            text=True,
            check=False,
        )
        try:
            report = json.loads(result.stdout)["report"]["jobs"]
            verdict = report[0]["validationResult"][0]
            actual = verdict["compliant"]
            if len(report) != 1 or not isinstance(actual, bool) or verdict["jobEndStatus"] != "normal":
                raise ValueError("incomplete validation report")
        except (ValueError, KeyError, IndexError, TypeError) as error:
            print(f"FAIL {row['path']}: no complete veraPDF result: {error}", file=sys.stderr)
            failures += 1
            continue

        if actual != expected:
            print(f"FAIL {row['path']}: profile {profile} compliant={actual}, want {expected}")
            failures += 1
        else:
            print(f"PASS {row['path']}: profile {profile} compliant={actual}")

    print(f"pdfa-corpus-check: {len(rows) - failures} pass, {failures} fail, {len(rows)} samples")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
