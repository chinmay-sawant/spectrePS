# Spectre PS pull request: benchmark coverage

## Summary

Adds 32 benchmark functions across nine packages, taking the suite from 35 functions in six packages to 67 in eleven, and records the run in `documentation/benchmark.md`. The first run found a real defect: `EncodeFlateRGB` allocates one heap object per pixel, and `image.(*RGBA).At` plus `image.(*YCbCr).At` are 70.24 percent of every object the writer package allocates.

## Motivation / context

- Plans: `plans/v0.0.5/`, the ledger and five phase files
- Issues: none filed. The gap was found by reading the existing suite, not reported.

The v0.0.4 suite mapped the work that was easy to reach. `internal/graphics` filled a four point square and nothing else, so the `O(W*H*N)` cost in `Pixmap.Fill` had never been measured above N=4, and the clip paths, which allocate a snapshot per mark and are the most allocation-heavy code in the package, had no benchmark at all. Five packages had no `bench_test.go` whatsoever. The CLI raster benchmark emitted PPM, so nothing in the tree had ever encoded a PNG, which is why the per-pixel allocation in the Flate encoder survived four releases.

## Changes

### Benchmarks, 32 functions

| Package | Functions | What was unmeasured |
| --- | --- | --- |
| `internal/graphics` | 3 to 10 | Fill above N=4, all four clip paths, glyph blending, `ShowPage` |
| `internal/font` | 0 to 5 | Standard 14 lookups, encoding tables, the AGL map, Type 1 load, TrueType subsetting |
| `spectreps` | 18 to 23 | `Info`, `PreflightUA2`, `WritePostScript`, and the tagged and PDF/A writers |
| `internal/cli` | 6 to 10 | PNG, JPEG, TIFF, and the PPM double copy |
| `internal/pdfa` | 0 to 3 | Both preflights, the XMP packet builders |
| `internal/pdfout` | 4 to 6 | The gray and CMYK packers, the Flate RGB encoder |
| `internal/tag` | 0 to 2 | `DerivePlan`, the only superlinear loop on a public job |
| `internal/psout` | 0 to 2 | `Emit` and `Write` |
| `internal/engine` | 0 to 1 | `CompareBytes` |
| `internal/pdf` | 3 to 4 | Flate decode in isolation, not through `Open` |

### Findings

| Area | Measured |
| --- | --- |
| Flate RGB encode | 532,160 allocs for 532,144 pixels, 14.9 ms. 70.24 percent of all objects the package allocates |
| Clip snapshot | 890 us clipped against 37.6 us unclipped, because the whole page is snapshotted |
| Flate decode | 5.25 MB allocated to produce 1 MiB, growing from a 4 KB buffer by append |
| Fill complexity | 0.97 ms at 4 points, 7.7 ms at 32, 26.5 ms at 128 on a 200 by 200 page |
| PDF/A preflight | 1.65 ms, attributed to `File.ObjectCount` at 22.14 percent cumulative |
| Tagged rewrite | 18.6 times the PDF/A writer on the same input |

### Budget

Seven new allocation ceilings gate `make test`, thirteen in total. Each was proven by lowering it by one, confirming the test fails with the expected message, and restoring it. The budget table said the level 2 rewrite ceiling was 701 when `internal/cli/alloc_test.go:22` asserts 715; the table now matches the code.

### Build and documentation

- `BENCH_PKGS` in the Makefile gains `engine`, `font`, `pdfa`, `psout`, and `tag`.
- `documentation/benchmark.md` is new and records all 86 rows.
- `documentation/performance.md` gains the coverage extension, a profiles section, nine findings rows, and a rewritten budget and `bench-check` section.

No production file changes. Every benchmark function is in a `_test.go` file.

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No product code changed, so no product timing changes. The suite found one defect that is documented and not fixed here |
| **Memory** | Same. The 532,160 allocations per `EncodeFlateRGB` call are recorded, not removed |
| **Behavior / correctness** | None. `make test` passes on all 13 packages and every existing behaviour test is untouched |
| **API / CLI** | None. No exported symbol, flag, or exit code changed |
| **Dependencies** | None in `go.mod` and `go.sum`, which are byte-identical. `benchstat` is installed as a tool via `go install`, which runs outside the module |
| **Binary size / build time** | Unchanged. `make build` produces the same 7577021 byte binary |

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | - |

## Test plan

- [x] `make test`
- [x] `make lint`
- [x] `make build`

### Commands

```sh
make lint
make test NPROC=4
make build
make bench
make bench-profile
make bench-check
```

Each exited 0 on 2026-09-26 on `feature/005-benchmark-coverage`.

| Command | Outcome |
| --- | --- |
| `make lint` | exit 0. gofmt clean, golangci-lint clean after five findings in the new benchmark code were fixed, size-check 0 over-limit |
| `make test NPROC=4` | exit 0, all 13 packages ok |
| `make build` | exit 0, `bin/spectreps version` reports 0.0.4 |
| `make bench` | exit 0, 258 benchmark rows, 6 min 7 s |
| `make bench-profile` | exit 0, 11 CPU and 11 heap profiles |
| `make bench-check` | exit 0, first real benchstat verdict on this tree, see below |

## Screenshots / sample output

The `make bench-check` verdict is the notable one, and it is a negative result worth reading. benchstat is installed now, so the comparison runs for the first time on this tree. It is still not usable:

| Reading | Value |
| --- | --- |
| Rows compared | 323 |
| Called significant | 107 |
| Of those, at exactly p=0.036 | 107 |
| Marked `all samples are equal` | 22 |
| Per-package `allocs/op` geomean | +0.00 percent in every package |

`make bench` captures `-count=3` and `make bench-check` captures `-count=5`, which hands benchstat fewer samples than the six it needs before it computes a confidence interval. All 107 flagged rows sit at `p=0.036`, the floor a Mann-Whitney U test can reach at that sample size, so the tool is reporting non-overlap rather than ranking anything. It fired on a tree where no production file had changed, with per-package `sec/op` geomeans from -2.22 to +46.67 percent at a load average of 1.04. The fix is `-count=10` and is deferred row 4.3 rather than guessed at, and every `ns/op` column in the new documentation is marked as a first reading rather than a baseline.

## Related issues

- Relates to the deferred ledger at `plans/v0.0.1/10-deferred.md` 10.5, which holds the v0.0.4 performance follow-ups
- No issue filed. The coverage gap and the encoder defect were found by reading the tree

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs, none exists for this work
- [x] Filled body committed under `plans/PR/pr-benchmark-coverage.md`

## Follow-ups (out of scope)

- Prove a benchmark fails on a real regression. The 13 allocation ceilings are proven detectors; the 32 new benchmark functions are not. This is the gap that matters most in this change and it is recorded in the plan
- Run `-count=10` and re-capture, so the timing columns become a baseline. Deferred row 4.3
- Review the ceilings that pin implementation detail. `DocumentInfo` at 7 allocs and PNG encode at 34 will trip on a legitimate change, unlike the five 0-alloc ceilings which are contracts
- Fix `EncodeFlateRGB`. One interface call per pixel is the whole cost and it is a contained change
- Decide whether `documentation/benchmark.md` is generated from the capture or treated as a dated artifact

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [ ] No unrelated changes in diff
- [ ] Public API / CLI changes documented, none here
- [ ] New rules have fixture coverage when applicable
- [ ] PR has assignee and labels
- [ ] Related issues use correct Closes/Relates keywords
- [ ] No secrets or generated artifacts committed
- [ ] Diff-stat-by-extension table pasted at the bottom

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 12 | 1010 | 1 |
| `.md` | 14 | 965 | 20 |
| No extension | 1 | 1 | 1 |
| **Total** | **27** | **1976** | **22** |
