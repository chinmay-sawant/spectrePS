# v0.0.3 - Quick wins

> **Parent:** `plans/v0.0.3/00-program.md` - future ledger
> **Status:** implemented. All rows are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** days per phase. Phase 4 is a document.

---

## Overview

This file takes the small deferred items that need no font machine, no image model, and no new page description language. Order is by risk.

`documentation/features.md` and `plans/v0.0.1/10-deferred.md` point here for these rows.

## Executive summary

`spectreps raster` paints only page 0 of a PDF while `bbox`, `inkcov`, and `pdfimage` walk every page. `compare raster` has the same gap on the read side, because `rasterPair` sends both files through `RunPostScript` even though `OpenPDF` and `RasterizePage` exist. Those are the first phase.

TIFF is the one new module. `golang.org/x/image/tiff` encodes baseline TIFF with no compression or Deflate. LZW, CCITT G3, and G4 are decode-only, so this plan does not promise them. Ghostscript's TIFF compression names do not map one to one.

Gray and CMYK image PDF carry a policy question, so a decision row comes first.

The `gs` argv item is a written switch map, not code.

## Phase 1: PDF pages

### 1.1 raster walks PDF pages

- [x] `spectreps raster` paints every page of a PDF input, matching `bbox`, `inkcov`, and `pdfimage`. Proof: `go test -count=1 ./internal/cli -run TestRasterPDFPages`. Outcome on 2026-09-25: exited 0.

### 1.2 Page range flag

- [x] `-pages A-B` (1-based, inclusive; a single `N`; omitted means every page) is accepted by `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster`. A `%d` output path numbers emitted pages from 1, matching Ghostscript `-sOutputFile` and `-dFirstPage`. A range outside the document returns `rangecheck`. For PostScript, the run executes every page and the filter applies after it, so a failing page that the range skips still fails the command. Proof: `go test -count=1 ./internal/cli -run TestPageSelection`. Outcome on 2026-09-25: exited 0.

### 1.3 compare raster PDF inputs

- [x] `rasterPair` opens a `.pdf` input with `OpenPDF` and `RasterizePage` instead of `RunPostScript`, and applies `-pages`. Proof: `go test -count=1 ./internal/cli -run TestCompareRasterPDF`. Outcome on 2026-09-25: exited 0.

## Phase 2: TIFF raster

### 2.1 Dependency row

- [x] `go.mod` requires `golang.org/x/image` at v0.46.0, and the reason (the TIFF encoder) is recorded next to the change. `go mod tidy` writes `go.sum`, and a second run leaves no diff. Proof: `make tidy` twice, then `git diff --exit-code -- go.mod go.sum`. 2026-09-25: both tidy runs exited 0, and the diff printed nothing and exited 0.

### 2.2 Encoder

- [x] `raster` encodes a path ending in `.tif` or `.tiff` with `tiff.Encode`, using Deflate compression by default and `-tiffcompress none|deflate` otherwise. The image is the same RGB pixmap the PPM and PNG writers use. TIFF bytes are not an equality oracle. Proof: `go test -count=1 ./internal/cli -run TestRasterTIFF` exited 0 on 2026-09-25; the test decodes the file with `tiff.Decode` and compares pixels with the PPM output of the same program.

## Phase 3: Gray and CMYK image PDF

### 3.1 Color policy

- [x] The DeviceGray and DeviceCMYK conversions from the RGB pixmap are named and written in `documentation/devices.md`, with one worked example. Proof: the doc states both formulas and the example values. Done 2026-09-25: `documentation/devices.md`, section Image color spaces, states `Y = round(0.299*R + 0.587*G + 0.114*B)` and the DeviceCMYK `K = 1 - max(r, g, b)` division, with pure red `(255, 0, 0)` as gray `76` and CMYK `0 255 255 0`.

### 3.2 Writer and command

- [x] `pdfimage -colorspace rgb|gray|cmyk` writes `/DeviceGray` (8 bits per component) or `/DeviceCMYK` (32 bits) image streams. `rgb` stays the default, so existing bytes do not change. `ImagePDF` gains a color mode on the public API. Proof: `go test -count=1 ./internal/pdfout -run TestImageColorSpaces` and `go test -count=1 ./internal/cli -run TestPDFImageColor` both exited 0 on 2026-09-25.

## Phase 4: gs switch map

### 4.1 Written proposal

- [x] `documentation/gs-argv-mapping.md` maps the `gs` switches the current subcommands can already express (`-sDEVICE`, `-sOutputFile`, `-r`, `-g`, `-dFirstPage`, `-dLastPage`, `-dBATCH`, `-dNOPAUSE`, `-q`) onto commands and flags, and names the switches that stay rejected. `documentation/cli.md` links it. This phase is document-only, so it skips lint and test. Proof (2026-09-25): `test -f documentation/gs-argv-mapping.md` exits 0, and `grep -n 'gs-argv-mapping.md' documentation/cli.md` prints line 5 with the linked sentence.

## Phase 5: Closure

- [x] `make lint` passes. Outcome on 2026-09-25: `make lint` exited 0. `golangci-lint run ./...` reported no issues, and `make size-check` printed `size-check: clean (0 over-limit files).`
- [x] `make test` passes. Outcome on 2026-09-25: `make test` exited 0. Every package with tests printed `ok`: `internal/cli`, `internal/graphics`, `internal/pdf`, `internal/pdfout`, `internal/ps`, and `spectreps`.

## Dependencies

1.1 and 1.2 share the page loop and can land together. 1.3 depends on 1.2 for the range flag. 2.1 gates 2.2. 3.1 gates 3.2. 4.1 is independent.

## Not in this file

- `cm`, the text operators, and the object pass-through writer. They are rows 1.1 and 1.2 in `2-pdf-compression.md`.
- `ink_cov` weighted amounts. Reason: Ghostscript prints `ink_cov` as a percent and its manual example disagrees with its source, so Spectre needs a named weighting model before any code. Next gate: a written model.
- DCT, CCITT, and downsampling. They are rows 1.3 and 1.4 in `2-pdf-compression.md`.
