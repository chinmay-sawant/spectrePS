# v0.0.2 - Raster compare

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Lint and test passed on 2026-09-24. Tag 0.0.2 is checked.
> **Estimated effort:** 2 days

---

## Overview

Implement `CompareRaster` from `documentation/public-api.md` and the `compare raster` command from `documentation/cli.md`.

## Executive summary

Tag 0.0.2 is done when two PostScript programs can be painted and their pixmaps compared, and when two equal PPM bodies compare equal through `CompareFiles`. PNG bytes stay out of this test.

## Phase 5: Raster compare

### 5.1 CompareRaster

- [x] `CompareRaster` implements the width, height, and pixel rules in `documentation/public-api.md`, including stride padding that must be ignored. Proof: `go test -count=1 -run TestCompareRaster ./spectreps` exited 0 on 2026-09-24.

### 5.2 CLI

- [x] `spectreps compare raster -r 72 -w 200 -h 200 a.ps b.ps` exits 0 when both programs paint the same pixels, and exits 1 with `mismatch pixel N` or `mismatch width` on stdout when they do not. Proof: `go test -count=1 ./internal/cli -run TestCompareRasterCLI` exited 0 on 2026-09-24. The checked command uses `-w 20 -h 20`.

### 5.3 Same options

- [x] The command rasterizes both files with one `RunOptions` value. A test feeds two programs that differ only if the resolution differed, and shows a single `-r` applies to both. Proof: `go test -count=1 ./internal/cli -run TestCompareSameOptions` exited 0 on 2026-09-24.

### 5.4 Closure

- [x] `make lint` passes. Outcome on 2026-09-24: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24: exit 0.
- [x] Tag 0.0.2 note added to `plans/v0.0.1/09-release-records.md` with both transcripts.

## Dependencies

Phase 04. `compare bytes` from phase 02 stays as it is.
