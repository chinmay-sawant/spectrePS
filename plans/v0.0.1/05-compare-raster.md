# v0.0.2 - Raster compare

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** not started
> **Estimated effort:** 2 days

---

## Overview

Implement `CompareRaster` from `documentation/public-api.md` and the `compare raster` command from `documentation/cli.md`.

## Executive summary

Tag 0.0.2 is done when two PostScript programs can be painted and their pixmaps compared, and when two equal PPM bodies compare equal through `CompareFiles`. PNG bytes stay out of this test.

## Phase 5: Raster compare

### 5.1 CompareRaster

- [ ] `CompareRaster` implements the width, height, and pixel rules in `documentation/public-api.md`, including stride padding that must be ignored. Proof: `go test -count=1 -run TestCompareRaster .`

### 5.2 CLI

- [ ] `spectreps compare raster -r 72 -w 200 -h 200 a.ps b.ps` exits 0 when both programs paint the same pixels, and exits 1 with `mismatch pixel N` or `mismatch width` on stdout when they do not. Proof: `go test -count=1 ./cmd/spectreps -run TestCompareRasterCLI`

### 5.3 Same options

- [ ] The command rasterizes both files with one `RunOptions` value. A test feeds two programs that differ only if the resolution differed, and shows a single `-r` applies to both. Proof: `go test -count=1 ./cmd/spectreps -run TestCompareSameOptions`

### 5.4 Closure

- [ ] `make lint` passes. Record the outcome here.
- [ ] `make test` passes. Record the outcome here.
- [ ] Tag 0.0.2 note added to `plans/v0.0.1/09-release-records.md` with both transcripts.

## Dependencies

Phase 04. `compare bytes` from phase 02 stays as it is.
