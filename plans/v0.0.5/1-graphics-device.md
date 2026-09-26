# v0.0.5 - Graphics device

> **Parent:** `plans/v0.0.5/00-program.md` - program ledger
> **Status:** planned
> **Estimated effort:** about three days

---

## Overview

`internal/graphics` is the only package in the tree that rasterizes. Three of its benchmarks exist: `BenchmarkStroke` on a 200 point polyline, `BenchmarkFill` on a four point square, and `BenchmarkDrawImage` at two scales. The file is 93 lines.

The square is the problem. `Pixmap.Fill` (`internal/graphics/pixmap.go:137`) walks every pixel of the page and calls `inside` (`internal/graphics/pixmap.go:385`) for each one, and `inside` walks every subpath segment. The cost is `O(W*H*N)`. The only measurement of it is at N=4, where the per pixel work is trivial and the cost is indistinguishable from a page clear. Nothing records what happens at N=128.

The clip paths have no benchmark at all. `StrokeClipped`, `FillClipped`, `DrawGlyphClipped`, and `DrawImageClipped` each allocate a `rectSnapshot` of two fresh byte slices sized to the path bounds (`internal/graphics/clip.go:152`) and then call `restoreOutside` (`internal/graphics/clip.go:171`), which walks the same rectangle and tests every pixel against every clip subpath. `CompositeGroupClipped` (`internal/graphics/clip.go:78`) snapshots the whole page regardless of group bounds. This is the most allocation-heavy code in the package and none of it is timed.

`Pixmap.DrawGlyph` (`internal/graphics/pixmap.go:241`) blends a coverage mask per pixel and `Pixmap.ShowPage` (`internal/graphics/pixmap.go:154`) copies the full pixel plane and re-whitens two planes. `documentation/performance.md:310` records that glyph coverage did not reach the top 20 in the v0.0.4 profile. That is a fact about the corpus inputs, not a fact about the code, and a direct measurement settles it.

## Executive summary

Phase 1 adds the missing measurements to `internal/graphics/bench_test.go` and changes no file in the package. The fill benchmark becomes a scaling table over N so the `O(W*H*N)` claim is a number in `documentation/performance.md` rather than a reading of the source. The clip benchmarks record the snapshot allocations and the restore walk. The glyph and group composite benchmarks record the per pixel blend and the whole page snapshot. The fill benchmarks use a 200 by 200 page, not the 612 by 792 page the stroke benchmark uses, because `W*H*N` at full page size makes the benchmark too slow to run at `-count=3`.

## Phase 1: Fill at path complexity

`Pixmap.Fill` allocates nothing per pixel and its cost is entirely in the inside test. A scaling table over path complexity is the measurement.

- [x] 1.1 `benchStar` builds a closed N point star on a 200 by 200 page, and `BenchmarkFillComplexity` runs `Fill` at N of 4, 32, and 128 as sub-benchmarks. Proof: `go test -count=1 -run '^$' -bench BenchmarkFillComplexity -benchmem ./internal/graphics` printed three lines whose ns/op grows with N. Outcome on 2026-09-26: 968770, 7689271, and 26492447 ns/op, so 8 times the points cost 7.9 times the time and 4 times the points cost 3.4 times the time.
- [x] 1.2 `BenchmarkFillSubpaths` fills a path of 32 separate four point squares, so `subpaths` (`internal/graphics/pixmap.go:369`) returns 32 subpaths and `inside` walks all of them per pixel. Proof: `go test -count=1 -run '^$' -bench BenchmarkFillSubpaths -benchmem ./internal/graphics` exited 0. Outcome on 2026-09-26: 21577169 ns/op and 11480 B/op against 26492447 ns/op and 6272 B/op for one 128 point subpath, because `cross` rejects a segment when the pixel is outside its scanline.

## Phase 2: Clip

Every clipped mark allocates a snapshot and walks it twice. These are the numbers no file records.

- [x] 2.1 `BenchmarkStrokeClipped` and `BenchmarkFillClipped` run the existing stroke and fill inputs through `StrokeClipped` and `FillClipped` with one clip path, and the allocs/op line shows the two snapshot slices per call. Proof: `go test -count=1 -run '^$' -bench 'BenchmarkStrokeClipped|BenchmarkFillClipped' -benchmem ./internal/graphics` exited 0. Outcome on 2026-09-26: `BenchmarkStrokeClipped` 10995205 ns/op and 1900792 B/op over 6 allocs, `BenchmarkFillClipped` 10318785 ns/op and 164255 B/op over 8 allocs.
- [x] 2.2 `BenchmarkCompositeGroupClipped` composites a 200 by 200 group under one clip, which snapshots the whole page at `internal/graphics/clip.go:78`, and the sub-benchmark `group-only` runs `CompositeGroup` with no clip for the difference. Proof: `go test -count=1 -run '^$' -bench BenchmarkCompositeGroupClipped -benchmem ./internal/graphics` printed two lines. Outcome on 2026-09-26: `group-only` 37632 ns/op and 0 allocs, `clipped` 890755 ns/op and 164064 B/op over 6 allocs, so the whole page snapshot costs 23.7 times the composite.

## Phase 3: Glyph and page

- [x] 3.1 `BenchmarkDrawGlyph` blends an `*image.Alpha` coverage mask of 64 by 64 at full coverage, half coverage, and zero coverage, so the `alpha == 0` early return at `internal/graphics/pixmap.go:259` is visible as a separate line. Proof: `go test -count=1 -run '^$' -bench BenchmarkDrawGlyph -benchmem ./internal/graphics` printed three lines. Outcome on 2026-09-26: full 29528 ns/op, half 29117 ns/op, zero 9706 ns/op, all 0 allocs. The zero case is a third of the full case, so the early return still walks the whole mask.
- [x] 3.2 `BenchmarkShowPage` calls `ShowPage` on a 612 by 792 pixmap, which copies the full pixel plane and re-whitens the pixel and alpha planes at `internal/graphics/pixmap.go:154`. Proof: `go test -count=1 -run '^$' -bench BenchmarkShowPage -benchmem ./internal/graphics` exited 0 and the B/op line is at least the page size in bytes. Outcome on 2026-09-26: 774935 ns/op and 1458197 B/op, which is the 1454400 byte pixel plane plus growth. The retained page copies are dropped inside the loop, because every call keeps a full copy by design and a `-count=3` run would otherwise hold gigabytes.

## Phase 4: Closure

- [x] 4.1 Closure: `make lint` and `make test` exit 0, and the outcome is recorded in this row. Proof: `make lint` and `make test NPROC=4`. Outcome on 2026-09-26: both exit 0. `gofmt -l .` printed nothing, `golangci-lint run ./...` exited 0 after five findings in the new benchmark code were fixed, and `size-check` reported 0 over-limit files with the largest new file at 296 lines. `go test -p 4 ./...` reported ok for all 13 packages, `internal/graphics` in 0.005s.

## Not in this phase

- Any change to `internal/graphics/pixmap.go`, `internal/graphics/clip.go`, or any other file in the package. A benchmark that shows a win is a row in a later file with the behavior proof beside it.
- A scanline or bounding-box fill. The benchmark measures the cost as it is today. Changing the algorithm would invalidate the number this file records.
- Clip benchmarks that vary the clip path count. One clip path is the case the corpus exercises. The multi clip path case belongs with a corpus that has it.

## Dependencies

- `internal/graphics` has no dependency on any other package in the tree, so this phase needs nothing from phases 2 through 5.
- `documentation/performance.md` gains one findings row per benchmark here, written in `plans/v0.0.5/5-baseline-and-budget.md` phase 1.
