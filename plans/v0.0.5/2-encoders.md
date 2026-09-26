# v0.0.5 - Output encoders

> **Parent:** `plans/v0.0.5/00-program.md` - program ledger
> **Status:** planned
> **Estimated effort:** about three days

---

## Overview

The CLI raster benchmark writes PPM (`internal/cli/bench_test.go:69`). PPM is a header and a raw copy of the RGB plane, so it is the cheapest of the four raster output formats and the least representative. `encodePNG` (`internal/cli/run.go:687`), `encodeJPEG` (`internal/cli/run.go:695`), and `encodeTIFF` (`internal/cli/tiff.go:35`) are reachable from the `raster` command and none of them has a benchmark. `rgbaFromPage` (`internal/cli/run.go:703`) allocates a fresh `image.RGBA` of `W*H*4` bytes for every one of them, and `encodePPM` (`internal/cli/run.go:677`) allocates the body and then a second full copy when it prepends the header, which `documentation/performance.md:346` records at 18.77 percent of CLI allocations.

The image packers in `internal/pdfout` are the other gap. `BenchmarkImagePDF` in `spectreps/bench_test.go:212` covers only `ImageColorRGB`, so the two other branches of `packSamples` (`internal/pdfout/image.go:124`) have never run inside a benchmark. `packedGray` (`internal/pdfout/image.go:138`) calls `luma` per pixel and `packedCMYK` (`internal/pdfout/image.go:157`) calls `rgbToCMYK` per pixel. `EncodeFlateRGB` (`internal/pdfout/scale.go:41`) reads through the `image.Image` interface per pixel and writes one zlib row per scanline, which is thousands of small writes where one would do.

## Executive summary

Phase 2 benchmarks the four raster output formats and the two unbenchmarked image packers. The format benchmarks share one painted page so the difference between them is the encoder and not the raster. The packer benchmarks share one painted page and vary only the color space, which is the public `ImagePDFColor` surface. Phase 2 also adds an isolated Flate decode benchmark, because `readLimited` (`internal/pdf/filter.go:649`) grows its buffer through 4096 byte appends and the only measurements of it are indirect, inside `BenchmarkOpenPDF/xref-stream`.

## Phase 1: Raster output formats

- [x] 1.1 `internal/cli/bench_test.go` adds `benchFormatPage`, one painted 200 by 200 page shared by the four encoder benchmarks, so no encoder benchmark pays the PostScript raster cost. Proof: `go test -count=1 -run '^$' -bench BenchmarkEncode -benchmem ./internal/cli` exited 0 and printed five lines.
- [x] 1.2 `BenchmarkEncodePPM` measures `encodePPM` and its B/op is at least twice the page size in bytes, which records the second full copy at `internal/cli/run.go:684`. Proof: `go test -count=1 -run '^$' -bench BenchmarkEncodePPM -benchmem ./internal/cli` printed one line. Outcome on 2026-09-26: 54055 ns/op and 246052 B/op over 4 allocs, against a 120000 byte page, so the body is copied twice.
- [x] 1.3 `BenchmarkEncodePNG` and `BenchmarkEncodeJPEG` measure `encodePNG` and `encodeJPEG` on the same page, and the B/op line for each carries the `W*H*4` RGBA conversion. Proof: `go test -count=1 -run '^$' -bench 'BenchmarkEncodePNG|BenchmarkEncodeJPEG' -benchmem ./internal/cli` printed two lines. Outcome on 2026-09-26: PNG 709869 ns/op and 1016390 B/op over 34 allocs, JPEG 504297 ns/op and 169776 B/op over 11 allocs.
- [x] 1.4 `BenchmarkEncodeTIFF` measures `encodeTIFF` at `tiffDeflate` and `tiffNone`, so the deflate cost is separated from the container cost. Proof: `go test -count=1 -run '^$' -bench BenchmarkEncodeTIFF -benchmem ./internal/cli` printed two lines. Outcome on 2026-09-26: `none` 164556 ns/op and 330558 B/op, `deflate` 540593 ns/op and 981423 B/op, so deflate costs 3.3 times the time for a 3.0 times larger file.

## Phase 2: Image packers

- [x] 2.1 `internal/pdfout/bench_test.go` adds `BenchmarkWriteImagesColor` with `ImageColorRGB`, `ImageColorGray`, and `ImageColorCMYK` sub-benchmarks over one `graphics.Image`, which is the three branches of `packSamples`. Proof: `go test -count=1 -run '^$' -bench BenchmarkWriteImagesColor -benchmem ./internal/pdfout` printed three lines. Outcome on 2026-09-26: rgb 221533 ns/op and 4635 B/op, gray 202610 ns/op and 65268 B/op, cmyk 640379 ns/op and 256786 B/op, so CMYK costs 2.9 times RGB.
- [x] 2.2 `BenchmarkEncodeFlateRGB` measures `EncodeFlateRGB` on a 842 by 632 image, the same geometry `BenchmarkEncodeDCT` uses, so the two encoders are comparable. Proof: `go test -count=1 -run '^$' -bench BenchmarkEncodeFlateRGB -benchmem ./internal/pdfout` exited 0 and printed one line. Outcome on 2026-09-26: 14909755 ns/op and 3193154 B/op over 532160 allocs, and 532144 is the pixel count, so the encoder allocates once per pixel. That is the largest allocation count in the tree and it was invisible before this phase.

## Phase 3: Flate decode

- [x] 3.1 `internal/pdf/bench_test.go` adds `BenchmarkDecodeFlate`, which inflates one 1 MiB buffer through `Decode` (`internal/pdf/filter.go:93`) with the `opFlate` name, and reports `SetBytes` against the uncompressed size, so `readLimited` (`internal/pdf/filter.go:649`) is measured on its own rather than through `Open`. Proof: `go test -count=1 -run '^$' -bench BenchmarkDecodeFlate -benchmem ./internal/pdf` printed one line with a MB/s value. Outcome on 2026-09-26: 1728564 ns/op at 606.62 MB/s and 5253305 B/op over 27 allocs, so the decode allocates 5.25 times its output.

## Phase 4: Closure

- [x] 4.1 Closure: `make lint` and `make test` exit 0, and the outcome is recorded in this row. Proof: `make lint` and `make test NPROC=4`. Outcome on 2026-09-26: both exit 0. `go test -p 4 ./...` reported ok for all 13 packages, `internal/cli` in 5.399s, `internal/pdf` in 0.848s, and `internal/pdfout` in 1.092s. No production file changed, so the encoder behaviour proofs are the existing `TestRaster` and `TestValidationPSOutImage` suites in those packages.

## Not in this phase

- A benchmark of a PDF/A or PDF/UA-2 output file. The tagged and PDF/A writers run their own preflight inside `Build` and `RewritePDF`, and phase 4 of `plans/v0.0.5/4-reading-and-writing.md` measures those whole jobs.
- Changing the PPM double allocation, the per scanline zlib write, or any other encoder. A benchmark that shows either is a row in a later file.
- Output to a real file. Every encoder here is measured in memory. Disk cost belongs to `scripts/bench-cli.sh`, which is the v0.0.4 phase 2 tool and is not changed here.

## Dependencies

- Phase 1 needs a painted page. It builds one with the same synthetic PostScript program `internal/cli/bench_test.go` already uses, so it depends on nothing from `plans/v0.0.5/1-graphics-device.md`.
- Phase 2 needs a `graphics.Image`, which `graphics.Pixmap` already produces, so it depends on the existing device and not on phase 1 of this file.
- Phase 3 needs the `Decode` entry point and a zlib buffer, both already in `internal/pdf/bench_test.go` as `benchFlate`.
