# v0.0.2 - Raster

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Lint and test passed on 2026-09-24.
> **Estimated effort:** 1 week

---

## Overview

`RunPostScript` returns `[]PageImage` for the subset in `documentation/language.md`. Layout, y flip, caps, PPM raw, and PNG are specified in `documentation/devices.md`.

The first checked-in fixture goes in `testdata/`. Generate it with Spectre, then assert against those bytes. Do not call `/usr/bin/gs` from the test.

## Executive summary

The pixmap is the object `CompareRaster` will use. PPM raw is the file form of that pixmap. PNG is a convenience encoding and is not the oracle.

## Phase 4: Raster

### 4.1 Graphics state

- [x] `internal/graphics` implements the default matrix, `translate`, `scale`, `rotate`, `concat`, `setlinewidth`, `setgray`, `setrgbcolor`, `gsave`, and `grestore` with the defaults and the gsave cap in `documentation/language.md`. Proof: `go test -count=1 ./internal/graphics -run TestMatrix` exited 0 on 2026-09-24.

### 4.2 Path and y flip

- [x] `0 0 moveto 100 0 lineto stroke` on a 200 by 200 point page at 72 dpi paints the bottom row of the pixmap, not row 0. `currentpoint` after that `moveto` is 0, 0 in user space. Proof: `go test -count=1 ./spectreps -run TestYFlip` exited 0 on 2026-09-24.

### 4.3 Paint

- [x] `stroke`, `fill`, and `eofill` write RGB pixels. `eofill` uses the even-odd rule. `showpage` appends one image and clears the path. A program with no `showpage` and no paint returns one blank page. A program that paints and skips `showpage` returns one image. Proof: `go test -count=1 ./spectreps -run TestPaint` exited 0 on 2026-09-24.

### 4.4 Caps

- [x] A page whose pixel count is above 40000000, or whose side is above 20000, returns `limitcheck` and allocates no pixmap of that size. Proof: `go test -count=1 ./spectreps -run TestPixelCap` exited 0 on 2026-09-24.

### 4.5 PPM and PNG

- [x] `spectreps raster -o out.ppm in.ps` writes a P6 file whose body matches `PageImage.Pixels`. `spectreps raster -o out.png in.ps` writes a PNG that decodes to the same pixels. Two pages and an `-o` without `%d` exit 2. Proof: `go test -count=1 ./internal/cli -run TestRasterFiles` exited 0 on 2026-09-24. The fixture is `testdata/line-bottom.ppm`.

### 4.6 Public method

- [x] `RunPostScript` no longer returns `ErrNotImplemented` for a program in the subset. An unsupported operator still returns `JobError`. Proof: `go test -count=1 -run 'TestRunPostScript|TestNotImplemented' ./spectreps` exited 0 on 2026-09-24.

### 4.7 Closure

- [x] `make lint` passes. Outcome on 2026-09-24: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24: exit 0. Transcript is in `plans/v0.0.1/09-release-records.md` under tag 0.0.2.

## Dependencies

Phase 03. `CompareRaster` may still return `ErrNotImplemented` until phase 05. Phase 04 tests compare pixels in the test process with a direct slice compare, so phase 05 has a pixmap to lock onto.
