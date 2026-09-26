# v0.0.5 - Benchmark coverage

> **Parent:** `plans/v0.0.4/00-program.md` - the previous tag
> **Status:** planned. No row is checked. Phase 1 through 5 add benchmarks and re-record the baseline. Phase 5 is the cut line.
> **Estimated effort:** about a week for phase 1 and 2, about three days for phase 3 and 4, about two days for phase 5.

---

## Overview

v0.0.4 shipped a benchmark suite in `plans/v0.0.4/6-performance-profiling.md` phase 1. It holds 35 `Benchmark` functions across six packages, and `documentation/performance.md` records the baseline those produced. That suite is the accepted record and this file does not change it.

The suite is a map of the work that was easy to reach. It covers the six packages that already had a natural entry point and leaves eight packages with no benchmark at all, and it leaves nine hot paths unmeasured inside the packages it does cover. This file closes those holes and re-records the baseline. It changes no production code.

The count, measured rather than estimated. The tree had 35 benchmark functions producing 40 result rows before this file and has 67 producing 86 after, so 32 functions were added and they account for 46 rows, because sub-benchmarks expand. Five packages had no `bench_test.go` at all: `internal/engine`, `internal/font`, `internal/pdfa`, `internal/psout`, and `internal/tag`.

## Executive summary

Phase 1 benchmarks the graphics device paths the current suite barely touches. `internal/graphics/bench_test.go` fills a four point square and nothing else, so the `O(W*H*N)` cost in `Pixmap.Fill` (`internal/graphics/pixmap.go:137`) has never been measured above N=4, and the clip paths (`internal/graphics/clip.go:152,171,78`) have no benchmark at all despite allocating a fresh snapshot per call. Phase 2 covers the output encoders. The CLI raster benchmark emits PPM (`internal/cli/bench_test.go:69`), so no benchmark in the tree has ever encoded a PNG, and the gray and CMYK packers in `internal/pdfout/image.go:138,157` are per pixel float loops that nothing measures. Phase 3 adds the first benchmarks for `internal/font`, `internal/pdfa`, and `internal/engine`. Phase 4 covers `internal/tag`, `internal/psout`, and the three exported `spectreps` jobs that have no benchmark, where `tag.DerivePlan` carries an `O(n^2)` step at `internal/tag/reading.go:897` that has never been timed. Phase 5 re-records `documentation/performance.md` against the wider suite and adds allocation ceilings for the new stable counts.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-graphics-device.md` | 1 | Fill at path complexity, clip, glyph, and group composite in `internal/graphics`. |
| `2-encoders.md` | 2 | PNG, TIFF, JPEG, PPM, and the gray and CMYK image packers. |
| `3-font-and-metadata.md` | 3 | First benchmarks for `internal/font`, `internal/pdfa`, and `internal/engine`. |
| `4-reading-and-writing.md` | 4 | `internal/tag`, `internal/psout`, and the unbenchmarked `spectreps` jobs. |
| `5-baseline-and-budget.md` | 5 | Re-record the baseline and set the new allocation ceilings. |
| `v0.0.5-closure.md` | gate | `make lint` and `make test` transcripts for the merged tree. |

## What is not in v0.0.5

| Item | Why |
| --- | --- |
| Any change to production Go code | This file adds `_test.go` files and one documentation update. A fix that a benchmark motivates is its own row in a later file. |
| Editing the 35 existing benchmarks | Their numbers are the accepted baseline in `documentation/performance.md`. Changing them invalidates the recorded comparison for no gain. |
| Installing `benchstat`, or adding a CI workflow | `benchstat` is not on PATH, so `make bench-check` writes a skip line, and the repository has no CI. Both are real gaps. Neither is a coverage gap, and both need a tool decision this file does not make. |
| Wall-clock assertions in `make test` | Timing is machine-specific. Only allocation counts gate, in the shape of `spectreps/alloc_test.go`. |
| Comparing against Ghostscript | The supported subsets differ, so the verdict would belong to the tool and not to this tree. |
| Concurrency benchmarks | The tree has no goroutines outside the standard library. A parallel job needs its own plan before there is anything to measure. |

## Dependencies

Phases 1 through 4 need only the standard library `testing` package and the checked-in samples under `sampledata/`. They read `sampledata/compress/`, `sampledata/fixtures/`, `sampledata/pdfa/`, `sampledata/pdfua2/`, and `sampledata/validation/`, and a missing file skips the benchmark the way `spectreps/bench_test.go:39` already does. Phase 5 depends on phases 1 through 4 for the numbers it records.

No new module requirement. `make bench` already carries the six existing packages, and phase 5 adds the new ones to `BENCH_PKGS` in the Makefile.
