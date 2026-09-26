# v0.0.4 - Performance profiling

> **Parent:** `plans/v0.0.4/00-program.md` - program ledger
> **Status:** implemented. Rows 5.4 and 5.6 are deferred to `plans/v0.0.1/10-deferred.md` 10.5.
> **Estimated effort:** about a week across six phases. Phase 5 is the cut line.

---

## Overview

No Go benchmark exists in the module today. Every PR body that touched a job records "unbenchmarked" (`plans/PR/pr-postscript-subset.md:44`, `plans/PR/pr-pdf-rewrite.md:34`, `plans/PR/pr-pdf-open.md:41`). This file builds the harness, records a baseline, profiles the two surfaces a consumer can pick, and lands only the fixes the profiles confirm.

The application is `cmd/spectreps` plus `internal/cli`: the built binary, its flag parsing, file I/O, and one job per process. The library is the public `spectreps` package plus the `internal` layers it calls, used in process with one `Instance` over many jobs. A consumer that shells out to the binary pays the application cost, and one that imports the package pays the library cost. Both get a benchmark suite and a profile.

Measured on this machine on 2026-09-25: 13th Gen Intel Core i7-13700HX, 24 threads, 7.6 GiB RAM, Go 1.26.4, `bin/spectreps` 6,207,997 bytes. `spectreps version` runs in 2 to 7 ms across five runs, at 4.4 MB peak RSS. `spectreps raster` of `sampledata/compress/path.pdf` is under 10 ms at 72 dpi, 81 ms at 300 dpi, and 742 ms at 600 dpi. The 600 dpi cell is 4 times the pixels of 300 dpi and about 9 times the time, and phase 4 explains the gap. A synthetic program of 20,000 strokes takes 32 ms at 15 MB. `spectreps rewrite` of `sampledata/compress/whatisthis.pdf` takes 9 ms at level 2 and 302 ms at level 5, which is the DCT re-encode. `whatisthis.pdf` does not rasterize because it hits `undefined in W`, so the profiling corpus uses files the product accepts, and that reader gap stays with `plans/v0.0.4/4-pdf-coverage.md`.

Timing is machine-dependent and never gates `make test`, in the shape of `make pdfa-check`. Allocation counts are deterministic enough to gate. The benchmarks use the standard library `testing` and `runtime/pprof` only. hyperfine, perf, and benchstat are optional proof tools that print a skip when absent. Profiling inputs come from `sampledata/` and the corpus in `plans/v0.0.4/5-validation.md`. The integrator owns `plans/v0.0.4/00-program.md`, the closure file, and the `plans/v0.0.1/10-deferred.md` moves; this file touches none of them.

## Executive summary

Phase 1 builds the benchmark suite for the library, the CLI, and the hot layers, adds `make bench` and `make bench-profile`, and records the baseline in a new `documentation/performance.md`. Phase 2 profiles the application: process startup, per-command wall clock and RSS, and the scaling table. Phase 3 profiles the library: per-job CPU and allocation profiles, the instance reuse pattern, and escape analysis. Phase 4 turns the profiles into a findings table and separates the wins from the accepted costs. Phase 5 lands the measured fixes and is the cut line. Phase 6 sets allocation ceilings, adds `make bench-check`, and closes the file.

## Phase 1: Benchmark suite

Benchmarks live in the package under profile as `Benchmark` functions in `*_test.go`. Inputs are checked-in samples and corpus files, never network. Every benchmark calls `b.ReportAllocs()`, and the ones with a meaningful payload call `b.SetBytes`.

- [x] 1.1 `spectreps/bench_test.go` adds one benchmark per exported job: `BenchmarkRunPostScript`, `BenchmarkOpenPDF`, `BenchmarkRasterizePage`, `BenchmarkRewriteLevel0` through `BenchmarkRewriteLevel5`, `BenchmarkExtractText`, `BenchmarkImagePDF`, `BenchmarkMeasureBox`, `BenchmarkMeasureInk`, `BenchmarkCompareRaster`, and `BenchmarkCompareFiles`. Proof: `go test -run '^$' -bench . -benchmem ./spectreps`.
- [x] 1.2 `internal/cli/bench_test.go` adds `BenchmarkCLIVersion`, `BenchmarkCLIRaster`, `BenchmarkCLIRewrite`, `BenchmarkCLIPDFImage`, `BenchmarkCLIText`, and `BenchmarkCLIGS`, each calling `Run` in process with output under `b.TempDir()`. Proof: `go test -run '^$' -bench CLIVersion -benchmem ./internal/cli`.
- [x] 1.3 Layer benches: `internal/pdf` (open a classic xref file, open an xref stream file, paint one page), `internal/pdfout` (pass-through write, level 5 image re-encode, `ScaleImage`, `EncodeDCT`), `internal/graphics` (`Stroke`, `Fill`, `DrawImage` at 1:1 and scaled), and `internal/ps` (run the stroke program). Proof: `go test -run '^$' -bench . -benchmem ./internal/pdf ./internal/pdfout ./internal/graphics ./internal/ps`.
- [x] 1.4 `make bench` runs the suite with `-count=3` and writes `profiles/bench.txt`; `make bench-profile` writes the `-cpuprofile` and `-memprofile` files per package under `profiles/`; `/profiles/` joins `.gitignore` with a comment that it holds generated profiles. Proof: `make bench` and `make bench-profile` produce the files and `git status --porcelain` shows no new tracked file.
- [x] 1.5 `documentation/performance.md` records the machine, the Go version, the exact commands, and the first baseline: ns/op, B/op, and allocs/op per benchmark, plus the application table from the overview. Proof: `grep -n -e ns/op -e i7-13700HX documentation/performance.md`.
- [x] 1.6 `documentation/development.md` names `make bench` and `make bench-profile` and states that neither runs inside `make test`. Proof: `grep -n bench documentation/development.md`.
- [x] 1.7 Closure: `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 2: Application profiling

The application is the built binary and `internal/cli`. The in-process benchmark removes process startup, so the difference between `BenchmarkCLIRaster` and the wall clock of the binary is the startup and file I/O cost. RSS comes from `/usr/bin/time -v`.

- [x] 2.1 `scripts/bench-cli.sh` builds `bin/spectreps`, runs a fixed command list (`version`, `raster` of `path.pdf` at 72 and 300 dpi, `raster` of the stroke program, `rewrite` of `whatisthis.pdf` at levels 2 to 5, `text`, `pdfimage`, `validate`) over named inputs, uses hyperfine when present and a `/usr/bin/time -f '%e %M'` loop otherwise, and writes `profiles/cli.txt`. Proof: `bash scripts/bench-cli.sh` with the table pasted into `documentation/performance.md`.
- [x] 2.2 Startup split: record `version` (2 to 7 ms across five runs), `validate` on a small PostScript program, and the in-process `BenchmarkCLIRaster`, so the fixed per-process cost and the per-job cost are separate rows in the table. Proof: the three recorded rows and the stated delta.
- [x] 2.3 Profiles: `make bench-profile` over the CLI benchmarks, `go tool pprof -top -nodecount=20` per profile, and a `GODEBUG=gctrace=1` run of the heaviest job with the GC summary. The top 20 functions and the GC share go into `documentation/performance.md`. Proof: the pprof output and the gctrace summary.
- [x] 2.4 Scaling table: raster the stroke program and `path.pdf` at 72, 150, 300, 600, and 1200 dpi within the caps, and rewrite `whatisthis.pdf` at levels 2 to 5. Record seconds, pixels, and peak RSS per cell, and state for each superlinear cell whether the cause is pixel count, stroke width, allocation, or DCT encode. Proof: the table in `documentation/performance.md`.
- [x] 2.5 Closure: `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 3: Library profiling

The library surface is the `spectreps` package. An embedded consumer calls `New` once and reuses the `Instance`, so the interesting cases are per call and per document, not per process.

- [x] 3.1 Per-job CPU profiles from the `spectreps` benchmarks, with the top functions, file:line, and percent per job in `documentation/performance.md`. The areas to name: Flate decode and encode, DCT decode and encode, image scale, pixmap stroke and fill, glyph coverage, PDF object parse, PostScript execution, text layout. Proof: `go tool pprof -top` output per profile.
- [x] 3.2 Allocation profile: `-memprofile` and `-benchmem` per job, and a `pprof -alloc_space` list of the largest sites. Proof: the allocation table with B/op and allocs/op per benchmark.
- [x] 3.3 Embedded pattern: `BenchmarkReuseInstance` (one `New`, one `OpenPDF`, rasterize every page) against `BenchmarkPerCallInstance` (a `New` and `Close` per page). Record the per-call overhead of the context check and the engine shim, which is the shape a consumer pays when it imports the package. Proof: both benchmarks and the recorded delta.
- [x] 3.4 Escape analysis: `go build -gcflags=-m ./spectreps ./internal/...` filtered to the hot files, with the heap escapes and any full-buffer copies recorded (page content, decoded image, Flate output, glyph boxes). Proof: the summary in `documentation/performance.md` with the file names.
- [x] 3.5 Closure: `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 4: Findings and scaling

- [x] 4.1 `documentation/performance.md` gains a findings table with one row per hot area: the top function with file:line, its percent of the job, and whether the cost is linear in the expected variable (pages, pixels, bytes, glyphs). Proof: `grep -n findings documentation/performance.md` and the table rows.
- [x] 4.2 Each finding with a plausible win gets a phase 5 row and names the measurement that will accept or reject it. Each finding without a win is recorded as accepted with the reason, so a later reader knows the cost was seen and kept. Proof: every findings table row names a phase 5 row or an accepted reason.
- [x] 4.3 The 300 dpi to 600 dpi gap measured on 2026-09-25 (81 ms to 742 ms, 4 times the pixels and about 9 times the time) is attributed to a cause or recorded as unexplained with the next step. Proof: the attribution row in the table.
- [x] 4.4 Closure: no timing assertion joins `make test`; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 5: Measured fixes

Every row here starts from a phase 4 finding and ends with a before and after number. The behavior proof is the existing test that covers the changed path, named in the row. A fix that changes a public signature needs a docs row in the same phase. These are the candidates the harness is likely to confirm. A row that phase 4 does not confirm moves to the integrator for `plans/v0.0.1/10-deferred.md`.

- [x] 5.1 Pixmap stroke and fill: act on the allocation finding for `internal/graphics` by reusing per-mark buffers or removing the short-lived slices, with B/op before and after. Behavior proof: `go test -count=1 ./internal/graphics ./spectreps -run 'TestPixmap|TestPaint'`.
- [x] 5.2 Flate: act on the allocation finding in `internal/pdf` and `internal/pdfout` by reusing `flate.Writer` and `flate.Reader` through a pool if the profile shows them. Behavior proof: `go test -count=1 ./internal/pdf ./internal/pdfout -run 'TestDecode|TestWrite|TestLevel'`.
- [x] 5.3 Level 0 writer: act on the copy finding in `internal/pdfout.EmitPage` or `pdf.SerializeValue` by removing a full-buffer copy. Behavior proof: `go test -count=1 ./internal/pdfout ./spectreps -run 'TestEmit|TestRewriteStable'`.
- [~] 5.4 Font parse: act on a repeated-parse finding by caching the `sfnt` parse per font object number, with the text job's ns/op and B/op before and after. Behavior proof: `go test -count=1 ./internal/pdf -run 'TestTrueTypeGlyph|TestIdentityHText'` and `go test -count=1 ./spectreps -run TestValidationExtractCases`. Deferred to `plans/v0.0.1/10-deferred.md` 10.5: the profile did not confirm a repeated parse large enough to change the budget.
- [x] 5.5 `CompareRaster`: act on the comparison finding by using `bytes.Equal` when both strides are tight and keeping the byte loop otherwise, with ns/op before and after. Behavior proof: `go test -count=1 ./spectreps -run TestCompareRaster`.
- [~] 5.6 The 300 dpi to 600 dpi scaling row, if phase 4 attributes it to an allocator or a per-scanline path, gets its own fix row with the same before and after shape. Behavior proof: `go test -count=1 ./spectreps -run 'TestPaint|TestYFlip'`. Deferred to `plans/v0.0.1/10-deferred.md` 10.5: the 9x gap did not reproduce on the merged tree (3.9x for 4x pixels), so no fix row follows.
- [x] 5.7 A row that phase 4 cannot confirm is rewritten as `[~]` and handed to the integrator for `plans/v0.0.1/10-deferred.md` with the reason and the next gate. Proof: the deferred row exists and this file points at it.
- [x] 5.8 Closure: `documentation/performance.md` records each landed fix with its before and after; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 6: Budgets, docs, and closure

- [x] 6.1 Allocation ceilings: `TestPerformanceAllocs` in `spectreps` and `internal/cli` locks the stable `testing.AllocsPerRun` counts for `MeasureBox`, `MeasureInk`, `MeasureInkAmount`, `CompareRaster`, `CompareFiles`, and the level 2 copy, using the accepted numbers from phase 4. Guard proof: with one ceiling lowered by one the test fails, then the ceiling is restored. Proof: `go test -count=1 ./spectreps ./internal/cli -run TestPerformanceAllocs`.
- [x] 6.2 `make bench-check` runs the suite with `-count=5`, compares against the recorded baseline with benchstat when it is on PATH, prints a skip when it is not, and writes `profiles/compare.txt` with the verdict. It never runs inside `make test`. Proof: `make bench-check` with the verdict pasted into `documentation/performance.md`.
- [x] 6.3 `documentation/performance.md` holds the accepted baseline, the budget (the allocation ceilings and the accepted wall-clock numbers), the tool matrix, and the statement that timing is machine-specific. `documentation/test.md` states that benchmarks and profiles do not gate `make test`. Proof: `grep -n -e budget -e machine-specific documentation/performance.md documentation/test.md`.
- [x] 6.4 Closure: `make lint` and `make test` exit 0 on the merged tree, the outcome is recorded in this row, and the integrator records it in `plans/v0.0.4/v0.0.4-closure.md`. Proof: `make lint` and `make test`.

## Dependencies

The landed v0.0.3 tree, the corpus and manifest from `plans/v0.0.4/5-validation.md` phase 1 for accepted inputs, and the existing samples under `sampledata/compress/`. Go 1.26.4 and the standard library `testing` and `runtime/pprof` only. No new module requirement. hyperfine, perf, and benchstat are optional proof tools that skip when absent. Ghostscript is not measured against and does not take part in this work.

## Not in this plan

- A behavior change for speed without the test that proves the change. Every phase 5 row names its behavior proof.
- Wall-clock assertions in `make test`. Timing is machine-dependent; only deterministic allocation counts gate.
- Performance comparison with Ghostscript. The verdict belongs to the tool and the machine, and the supported subsets differ.
- Concurrency or parallelism changes. If phase 4 shows a CPU-bound, parallelizable stage, a row opens here first and is measured like the others.
- Optimization of the `undefined in W` refusal path. The reader gap is owned by `plans/v0.0.4/4-pdf-coverage.md`.
- CI performance monitoring. The repository has no CI, and the baseline is a local file.
- Profiling the v0.0.4 feature phases before they land. Each of their own files records a measurement row when it changes a hot path.
