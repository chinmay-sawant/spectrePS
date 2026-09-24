# v0.0.2 - Raster

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** not started
> **Estimated effort:** 1 week

---

## Overview

`RunPostScript` returns `[]PageImage` for the subset in `documentation/language.md`. Layout, y flip, caps, PPM raw, and PNG are specified in `documentation/devices.md`.

The first checked-in fixture goes in `testdata/`. Generate it with Spectre, then assert against those bytes. Do not call `/usr/bin/gs` from the test.

## Executive summary

The pixmap is the object `CompareRaster` will use. PPM raw is the file form of that pixmap. PNG is a convenience encoding and is not the oracle.

## Phase 4: Raster

### 4.1 Graphics state

- [ ] `internal/graphics` implements the default matrix, `translate`, `scale`, `rotate`, `concat`, `setlinewidth`, `setgray`, `setrgbcolor`, `gsave`, and `grestore` with the defaults and the gsave cap in `documentation/language.md`. Proof: `go test -count=1 ./internal/graphics -run TestMatrix`

### 4.2 Path and y flip

- [ ] `0 0 moveto 100 0 lineto stroke` on a 200 by 200 point page at 72 dpi paints the bottom row of the pixmap, not row 0. `currentpoint` after that `moveto` is 0, 0 in user space. Proof: `go test -count=1 . -run TestYFlip`

### 4.3 Paint

- [ ] `stroke`, `fill`, and `eofill` write RGB pixels. `eofill` uses the even-odd rule. `showpage` appends one image and clears the path. A program with no `showpage` and no paint returns one blank page. A program that paints and skips `showpage` returns one image. Proof: `go test -count=1 . -run TestPaint`

### 4.4 Caps

- [ ] A page whose pixel count is above 40000000, or whose side is above 20000, returns `limitcheck` and allocates no pixmap of that size. Proof: `go test -count=1 . -run TestPixelCap`

### 4.5 PPM and PNG

- [ ] `spectreps raster -o out.ppm in.ps` writes a P6 file whose body matches `PageImage.Pixels`. `spectreps raster -o out.png in.ps` writes a PNG that decodes to the same pixels. Two pages and an `-o` without `%d` exit 2. Proof: `go test -count=1 ./cmd/spectreps -run TestRasterFiles`

### 4.6 Public method

- [ ] `RunPostScript` no longer returns `ErrNotImplemented` for a program in the subset. An unsupported operator still returns `JobError`. Proof: `go test -count=1 -run 'TestRunPostScript|TestNotImplemented' .`

### 4.7 Closure

- [ ] `make lint` passes. Record the outcome here.
- [ ] `make test` passes. Record the outcome here.

## Dependencies

Phase 03. `CompareRaster` may still return `ErrNotImplemented` until phase 05. Phase 04 tests compare pixels in the test process with a direct slice compare, so phase 05 has a pixmap to lock onto.
