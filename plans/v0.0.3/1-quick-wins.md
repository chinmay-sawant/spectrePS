# v0.0.3 - Quick wins

> **Parent:** `plans/v0.0.3/00-program.md` - future ledger
> **Status:** proposed. No row is active. The deferred rows in `plans/v0.0.1/10-deferred.md` point here.
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

- [ ] `go.mod` requires `golang.org/x/image` at v0.46.0, and the reason (the TIFF encoder) is recorded next to the change. `go mod tidy` writes `go.sum`, and a second run leaves no diff. Proof: `make tidy` twice, then `git diff --exit-code -- go.mod go.sum`.

### 2.2 Encoder

- [ ] `raster` encodes a path ending in `.tif` or `.tiff` with `tiff.Encode`, using Deflate compression by default and `-tiffcompress none|deflate` otherwise. The image is the same RGB pixmap the PPM and PNG writers use. TIFF bytes are not an equality oracle. Proof: `go test -count=1 ./internal/cli -run TestRasterTIFF` decodes the file with `tiff.Decode` and compares pixels.

## Phase 3: Gray and CMYK image PDF

### 3.1 Color policy

- [ ] The DeviceGray and DeviceCMYK conversions from the RGB pixmap are named and written in `documentation/devices.md`, with one worked example. Proof: the doc states both formulas and the example values.

### 3.2 Writer and command

- [ ] `pdfimage -colorspace rgb|gray|cmyk` writes `/DeviceGray` (8 bits per component) or `/DeviceCMYK` (32 bits) image streams. `rgb` stays the default, so existing bytes do not change. `ImagePDF` gains a color mode on the public API. Proof: `go test -count=1 ./internal/pdfout -run TestImageColorSpaces` and `go test -count=1 ./internal/cli -run TestPDFImageColor`.

## Phase 4: gs switch map

### 4.1 Written proposal

- [ ] A proposal maps the `gs` switches the current subcommands can already express (`-sDEVICE`, `-sOutputFile`, `-r`, `-g`, `-dFirstPage`, `-dLastPage`, `-dBATCH`, `-dNOPAUSE`, `-q`) onto commands and flags, and names the switches that stay rejected. `documentation/cli.md` links it. Proof: the proposal file exists and the link resolves. This phase is document-only, so it skips lint and test.

## Phase 5: Closure

- [ ] `make lint` passes. Outcome recorded on the day.
- [ ] `make test` passes. Outcome recorded on the day.

## Dependencies

1.1 and 1.2 share the page loop and can land together. 1.3 depends on 1.2 for the range flag. 2.1 gates 2.2. 3.1 gates 3.2. 4.1 is independent.

## Not in this file

- `cm`, the text operators, and the object pass-through writer. They are rows 1.1 and 1.2 in `2-pdf-compression.md`.
- `ink_cov` weighted amounts. Reason: Ghostscript prints `ink_cov` as a percent and its manual example disagrees with its source, so Spectre needs a named weighting model before any code. Next gate: a written model.
- DCT, CCITT, and downsampling. They are rows 1.3 and 1.4 in `2-pdf-compression.md`.
