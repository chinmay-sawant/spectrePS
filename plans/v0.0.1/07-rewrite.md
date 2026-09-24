# v0.0.4 - PDF rewrite

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Lint and test passed on 2026-09-25. Tag 0.0.4 is checked.
> **Estimated effort:** 2 weeks

---

## Overview

`RewritePDF` writes a new PDF for a document this module can rasterize. `documentation/devices.md` is the contract. Stream compression is Flate. Bytes are stable across two calls.

This is the compression job. It is not a raster wrapped in a PDF. `pdfimage24` style output is a different device and is deferred.

## Executive summary

The writer omits wall-clock dates and uses a trailer id derived from the content bytes. Tests compare Spectre to Spectre. They do not compare Spectre to `pdfwrite`.

## Phase 7: PDF rewrite

### 7.1 Vector rewrite

- [x] `internal/pdfout` implements the device seam and emits path operators for the same subset phase 06 paints. `spectreps rewrite -o out.pdf in.pdf` exits 0. Opening `out.pdf` with `OpenPDF` and rasterizing page 0 matches `RasterizePage` of the input, via `CompareRaster`. Proof: `go test -count=1 ./spectreps -run TestRewritePixels` exited 0 on 2026-09-25.

### 7.2 Flate

- [x] With `DefaultRewriteOptions`, page content streams are Flate compressed. With `CompressStreams` false, those streams are not Flate. Both outputs still match pixels under row 7.1. Proof: `go test -count=1 ./internal/pdfout -run TestFlate` exited 0 on 2026-09-25.

### 7.3 Stable bytes

- [x] Two `RewritePDF` calls on the same bytes return buffers `CompareFiles` reports equal. The output contains no current timestamp. Proof: `go test -count=1 ./spectreps -run TestRewriteStable` exited 0 on 2026-09-25.

### 7.4 CLI

- [x] `spectreps rewrite` without `-o` exits 2. `-compress=false` selects the uncompressed option. Proof: `go test -count=1 ./internal/cli -run TestRewriteCLI` exited 0 on 2026-09-25.

### 7.5 Closure

- [x] `make lint` passes. Outcome on 2026-09-25: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0. `size-check` reported 0 over-limit files.
- [x] `make test` passes. Outcome on 2026-09-25: exit 0. Transcript is in `plans/v0.0.1/09-release-records.md` under tag 0.0.4.
- [x] Tag 0.0.4 note added to `plans/v0.0.1/09-release-records.md`.

## Dependencies

Phase 06. Image downsample, DCT, CCITT, font subsetting, and PDF/A stay in `10-deferred.md`.
