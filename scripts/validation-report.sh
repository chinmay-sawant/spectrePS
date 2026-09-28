#!/usr/bin/env bash
# validation-report runs the corpus tests with -json and writes a pass rate per
# manifest area and per basis to profiles/.
#
# It is a measurement tool, never a gate. The number that matters is the
# baseline share: a row whose basis is "baseline" records what this build did
# rather than what the specification requires, so a report whose baseline count
# grows is a report saying the corpus has stopped being a specification.
#
# A row can be exercised by more than one test, because a feature-specific test
# re-runs it with extra assertions on top of its area test. The report therefore
# counts distinct rows per group and reports the repeat count separately, so the
# duplication is visible instead of inflating the pass rate.
#
# It never runs under make test or make lint, because it needs the fetched tier
# and because a slow report gets ignored.
set -euo pipefail

cd "$(dirname "$0")/.."

out_dir=profiles
json="$out_dir/corpus.json"
report="$out_dir/corpus-report.tsv"

mkdir -p "$out_dir"

# -shuffle is on because a corpus that only passes in one order is not a corpus.
# The external tier skips itself when absent, so this works on a fresh clone.
printf '%s\n' 'validation-report: running the corpus tests'
go test -json -count=1 -shuffle=on ./internal/cli -run 'TestValidationCorpus' >"$json" 2>&1 || true

python3 - "$json" "$report" "sampledata/validation/manifest.tsv" <<'PYTHON'
import collections
import json
import sys

json_path, report_path, manifest_path = sys.argv[1], sys.argv[2], sys.argv[3]

# Read the label axes out of the manifest so the report joins on them.
#
# The join key is the mangled form, because the testing package rewrites spaces
# to underscores in a subtest name. Joining on the raw path silently drops every
# row whose path contains a space, which is all 2,691 of the fetched conformance
# tier, and the report then understates coverage instead of failing.
def mangle(path):
    return path.replace(" ", "_")


labels = {}
by_mangled = {}
with open(manifest_path) as fh:
    header = fh.readline().rstrip("\n").split("\t")
    ipath = header.index("path")
    iarea = header.index("area")
    ibasis = header.index("basis")
    for line in fh:
        if not line.strip():
            continue
        f = line.rstrip("\n").split("\t")
        labels[mangle(f[ipath])] = (f[iarea], f[ibasis], f[ipath])
        by_mangled[mangle(f[ipath])] = True

# state[(group, key)][row_path] is the worst verdict seen for that row.
VERDICT = {"pass": 0, "skip": 1, "fail": 2}
state = collections.defaultdict(dict)
runs = collections.Counter()

with open(json_path) as fh:
    for raw in fh:
        raw = raw.strip()
        if not raw.startswith("{"):
            continue
        try:
            event = json.loads(raw)
        except ValueError:
            continue
        name = event.get("Test")
        action = event.get("Action")
        if name is None or action not in VERDICT:
            continue
        # A subtest name is the parent test name, a slash, then the row path.
        if "/" not in name:
            continue
        row_path = name.split("/", 1)[1]
        if row_path not in labels:
            continue
        area, basis, _raw = labels[row_path]
        for group, key in (("area", area), ("basis", basis)):
            bucket = state[(group, key)]
            previous = bucket.get(row_path, 3)
            if VERDICT[action] < previous:
                bucket[row_path] = VERDICT[action]
        runs[row_path] += 1

rows = []
for (group, key), bucket in sorted(state.items()):
    counts = collections.Counter(bucket.values())
    total = len(bucket)
    ok = counts[0]
    bad = counts[2]
    skip = counts[1]
    ran = ok + bad
    rate = (100.0 * ok / ran) if ran else 0.0
    rows.append((group, key, total, ok, bad, skip, rate))

# Rows the corpus tests never touched. A committed row in this state is a gap
# in test selection, not in the file.
untouched = sorted(
    labels[key][2]
    for key in labels
    if key not in state.get(("area", labels[key][0]), {})
)

with open(report_path, "w") as fh:
    fh.write("group\tkey\trows\tpass\tfail\tskip\tpass-rate\n")
    for group, key, total, ok, bad, skip, rate in rows:
        fh.write("%s\t%s\t%d\t%d\t%d\t%d\t%.1f%%\n"
                 % (group, key, total, ok, bad, skip, rate))

width = max((len(k) for _, k, *_ in rows), default=8)
print()
print("%-6s %-*s %6s %6s %6s %6s %9s" % ("group", width, "key",
                                         "rows", "pass", "fail", "skip", "pass-rate"))
for group, key, total, ok, bad, skip, rate in rows:
    print("%-6s %-*s %6d %6d %6d %6d %8.1f%%"
          % (group, width, key, total, ok, bad, skip, rate))

repeated = sum(1 for p, n in runs.items() if n > 1)
print()
print("manifest rows        %d" % len(labels))
print("rows in a report     %d" % len({p for b in state.values() for p in b}))
print("rows run more than once %d (a feature test on top of its area test)" % repeated)
if untouched:
    print("rows no corpus test touched %d:" % len(untouched))
    for path in untouched:
        print("  %s" % path)
print()
print("wrote %s" % report_path)
PYTHON
