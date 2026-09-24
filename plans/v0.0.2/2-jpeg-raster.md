# v0.0.2 - JPEG raster

> **Parent:** `plans/v0.0.2/00-program.md` - quick-win ledger
> **Status:** implemented. Lint and test passed on 2026-09-25.
> **Estimated effort:** 1 day

---

## Overview

`spectreps raster -o` already writes PPM, or PNG when the path ends in `.png`. JPEG is the same pixels through `image/jpeg`. TIFF is not this file.

Ghostscript's `jpeg` device writes a color JFIF file. `-dJPEGQ` is an integer from 0 to 100 and defaults to 75. Source: [Devices, JPEG file format](https://ghostscript.readthedocs.io/en/latest/Devices.html). The page calls the format lossy. It does not say the bytes are a stable oracle.

Go's `image/jpeg.Encode` writes JPEG 4:2:0 baseline. `Options.Quality` is 1 to 100, and nil options use `DefaultQuality` 75. Quality below 1 is clipped to 1. Source: [pkg.go.dev/image/jpeg](https://pkg.go.dev/image/jpeg) and the Go 1.26.0 `writer.go` clip.

TIFF encode is `golang.org/x/image/tiff`, not `image/tiff`. That module stays in `plans/v0.0.1/10-deferred.md`.

## Executive summary

A path that ends in `.jpg` or `.jpeg` encodes the same `PageImage` as PNG. The default quality is 75, matching Ghostscript's `JPEGQ` and Go's `DefaultQuality`. The file is viewable. It is not the equality check. `CompareRaster` remains the oracle.

## Phase 2: JPEG file

### 2.1 Encode

- [x] `internal/cli` `writePages` calls `image/jpeg.Encode` when the path ends in `.jpg` or `.jpeg`. Quality comes from `-jpegq`, default 75, clamped to 1 through 100. The pixmap is RGB with alpha 255, the same conversion `encodePNG` already uses. PPM remains the fallback for every other suffix. Proof: `go test -count=1 ./internal/cli -run TestRasterJPEG` exited 0 on 2026-09-25.

### 2.2 Not an oracle

- [x] The JPEG test checks the SOI bytes `FF D8`, a successful `jpeg.Decode`, and width and height. It does not compare the JPEG file to a second encoder with `CompareFiles`. `CompareRaster` on the pixmap is unchanged. Proof: the same `TestRasterJPEG` run exited 0 on 2026-09-25.

## Dependencies

Phase 04 PNG encoding in `internal/cli/run.go`. No new module. Phase 1 is not required.
