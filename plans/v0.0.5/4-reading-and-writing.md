# v0.0.5 - Reading and writing

> **Parent:** `plans/v0.0.5/00-program.md` - program ledger
> **Status:** planned
> **Estimated effort:** about three days

---

## Overview

Five of the twenty `Benchmark` functions in `spectreps/bench_test.go` cover five of the twelve job methods. `Document.Info` (`spectreps/info.go:30`), `WritePostScript` (`spectreps/pdf.go:155`), and `PreflightUA2` (`spectreps/validate.go:17`) have no benchmark. Neither does `RewritePDF` with `Tag: true` or `PDFA: PDFA4`, and those two writers are the most expensive paths in the file, because each one runs a preflight over the whole document before it writes a byte.

`internal/tag` has no benchmark. `DerivePlan` (`internal/tag/reading.go:147`) is the interesting part. It calls `buildBlocks` at `internal/tag/reading.go:612`, which is `O(lines * blocks)`, and `orderFlow` at `internal/tag/reading.go:897`, which is `O(n^2)` and shifts a slice on every removal. The tagged write is a public job and this is the only untimed superlinear loop in the tree.

`internal/psout` has no benchmark. `Emit` (`internal/psout/emit.go:25`) replays decoded page content through a recorder, and `Write` (`internal/psout/write.go:73`) frames the pages as a PostScript program. Together they are every byte of `WritePostScript`.

## Executive summary

Phase 4 covers the reading and writing jobs. The `spectreps` benchmarks go in the existing external test package and reuse its `benchRead` and `benchOpen` helpers, so they read the same checked-in samples. The `tag` and `psout` benchmarks live inside their own packages and build synthetic inputs, because both take already-decoded content rather than a file. The `DerivePlan` benchmark varies the number of text runs so the `O(n^2)` step is a measured slope rather than a claim about the source.

## Phase 1: Library jobs

- [x] 1.1 `spectreps/bench_test.go` adds `BenchmarkDocumentInfo` over the path PDF, which walks the page tree for sizes, sorts the font list, and counts images inside `pdf.Info` (`internal/pdf/info.go:48`). Proof: `go test -count=1 -run '^$' -bench BenchmarkDocumentInfo -benchmem ./spectreps` exited 0. Outcome on 2026-09-26: 1359 ns/op and 184 B/op over 7 allocs, so the page tree walk and the font sort are not a cost worth a fix.
- [x] 1.2 `BenchmarkPreflightUA2` runs `PreflightUA2` over the PDF/UA-2 sample from `sampledata/pdfua2/`, which is the second content scanner at `internal/pdfa/ua2_content.go:27` on its own. Proof: `go test -count=1 -run '^$' -bench BenchmarkPreflightUA2 -benchmem ./spectreps` printed one line. Outcome on 2026-09-26: 371776 ns/op and 168936 B/op over 4512 allocs.
- [x] 1.3 `BenchmarkWritePostScript` runs `WritePostScript` over the path PDF, which covers `psout.Emit` and `psout.Write` end to end and returns the program bytes. Proof: `go test -count=1 -run '^$' -bench BenchmarkWritePostScript -benchmem ./spectreps` exited 0. Outcome on 2026-09-26: 495039 ns/op and 208707 B/op over 5628 allocs.
- [x] 1.4 `BenchmarkRewritePDFTagged` and `BenchmarkRewritePDFA` cover the two `RewritePDF` writers that had no benchmark, at `RewriteOptions{Tag: true}` and `RewriteOptions{PDFA: PDFA4}`, so the recorder, `DerivePlan`, the build, and the preflight are all inside the timed loop. Proof: `go test -count=1 -run '^$' -bench 'BenchmarkRewritePDFTagged|BenchmarkRewritePDFA' -benchmem ./spectreps` printed two lines. Outcome on 2026-09-26: tagged 1505441 ns/op and 1440061 B/op over 12095 allocs, PDF/A-4 81110 ns/op and 89604 B/op over 147 allocs. The tagged writer is 18.6 times the archival writer on the same input.

## Phase 2: Tag plan derivation

- [x] 2.1 `internal/tag/bench_test.go` reuses `recordStrokes` and `recordRun` from `internal/tag/fixture_test.go`, which take no `*testing.T`, and builds the fixture file from `newTagDoc` and `kidRefs` for the same reason, so the benchmark input matches the shape the reader tests already use. Proof: `go test -count=1 -run '^$' -bench . -benchmem ./internal/tag` exited 0.
- [x] 2.2 `BenchmarkDerivePlan` runs `DerivePlan` (`internal/tag/reading.go:147`) over 50, 200, and 800 recorded text runs on one page, and the ns/op slope across the three lines records the cost of the `orderFlow` loop at `internal/tag/reading.go:897`. Proof: `go test -count=1 -run '^$' -bench BenchmarkDerivePlan -benchmem ./internal/tag` printed three lines. Outcome on 2026-09-26: 74667, 379228, and 2448927 ns/op. Four times the runs cost 5.1 times the time from 50 to 200 and 6.5 times from 200 to 800, so the curve steepens where `orderFlow` starts to dominate.
- [x] 2.3 `BenchmarkTaggedBuild` runs `Build` (`internal/tag/build.go:44`) over the fixture, which includes the `pdfa.PreflightUA2` call the builder makes on its own output, so the whole tagged write is timed once at the package level. Proof: `go test -count=1 -run '^$' -bench BenchmarkTaggedBuild -benchmem ./internal/tag` printed one line. Outcome on 2026-09-26: 116797 ns/op and 95515 B/op over 1350 allocs for 20 recorded runs.

## Phase 3: PostScript writer

- [x] 3.1 `internal/psout/bench_test.go` adds `benchPageContent`, a synthetic content stream of 2,000 stroked path segments, and `BenchmarkEmit` measures `Emit` (`internal/psout/emit.go:25`) over it. The stream is path operators only, because a page that paints an image returns `undefined` in `Do` at `internal/psout/emit.go:42` and the writer emits no image operators. Proof: `go test -count=1 -run '^$' -bench BenchmarkEmit -benchmem ./internal/psout` exited 0. Outcome on 2026-09-26: 2323015 ns/op and 760994 B/op over 25979 allocs, which is 13 allocations per path segment.
- [x] 3.2 `BenchmarkWrite` measures `Write` (`internal/psout/write.go:73`) over ten framed pages, so the header, the `%%Page` comments, and the bounding box are all inside the timed loop. Proof: `go test -count=1 -run '^$' -bench BenchmarkWrite -benchmem ./internal/psout` printed one line. Outcome on 2026-09-26: 438270 ns/op and 1523984 B/op over 9 allocs.

## Phase 4: Closure

- [x] 4.1 Closure: `make lint` and `make test` exit 0, and the outcome is recorded in this row. Proof: `make lint` and `make test NPROC=4`. Outcome on 2026-09-26: both exit 0. `go test -p 4 ./...` reported ok for all 13 packages, `spectreps` in 1.437s, `internal/tag` in 0.012s, and `internal/psout` in 0.017s. The new `TestJobAllocs` in `spectreps` ran as part of that and passed at the recorded ceilings.

## Not in this phase

- A benchmark of `ExtractText` on a tagged or multi page document. The existing `BenchmarkExtractText` covers the one page case, and the cost is in the same font path phase 3 of `plans/v0.0.5/3-font-and-metadata.md` measures.
- A parallel raster or a parallel page loop. The tree has no goroutines outside the standard library, so there is nothing to measure until a job is parallel.
- Any change to a file in `spectreps`, `internal/tag`, or `internal/psout`.

## Dependencies

- Phase 1 needs the existing `benchRead` and `benchOpen` helpers in `spectreps/bench_test.go` and the samples under `sampledata/pdfua2/`.
- Phase 2 needs `recordStrokes` and `recordRun` from `internal/tag/fixture_test.go` and the fixture built by `openTagFixture`. A helper that takes a `*testing.T` is not reusable from a benchmark, so the benchmark builds its own recorder where the fixture helper needs a test handle.
- Phase 3 needs a content stream and no file. `psout.Emit` takes decoded bytes, so the benchmark supplies them directly.
- Nothing in this phase depends on `plans/v0.0.5/1-graphics-device.md` or `plans/v0.0.5/2-encoders.md`.
