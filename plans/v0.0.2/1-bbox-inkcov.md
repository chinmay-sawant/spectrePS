# v0.0.2 - Bounding box and ink coverage

> **Parent:** `plans/v0.0.2/00-program.md` - quick-win ledger
> **Status:** not started
> **Estimated effort:** 2 days

---

## Overview

`bbox` and `inkcov` read a finished `PageImage`. They do not paint a second time, and they do not add an operator.

Ghostscript documents both devices under special and test devices. Sources: [Devices, bounding box](https://ghostscript.readthedocs.io/en/latest/Devices.html#bounding-box-output) and [Devices, ink coverage](https://ghostscript.readthedocs.io/en/latest/Devices.html#ink-coverage-output). The installed binary is 9.55.0. The pages above are 10.09.0. The 10.08.0 copies of those sections match. Neither page states a 9.55.0 difference.

## Executive summary

Ghostscript `bbox` prints two lines on stderr. The box is the pixels that would be rendered at the selected resolution, converted back to points. White objects do not count unless `/WhiteIsOpaque true` is set. The documented default resolution for that device is 4000 dpi, and the numbers can move by up to two pixels. Spectre does not copy the 4000 dpi pass. It measures the pixmap the job already painted, at that job's dpi.

Ghostscript `inkcov` prints CMYK occupancy: how many device pixels contain each ink. A 50 percent cyan fill is still `1.00` on cyan because every pixel contains some cyan. `ink_cov` is the weighted amount and is not this phase. Spectre's pixmap is RGB, not CMYK. The Spectre line is three RGB occupancy fractions. It is not a CMYK report and it does not end in `CMYK OK`.

## Phase 1: Summaries of one pixmap

### 1.1 Bounding box

- [ ] `spectreps.MeasureBox(img PageImage, dpi float64) (Box, bool)` lives in `spectreps/`. `Box` holds `MinX`, `MinY`, `MaxX`, `MaxY` in points, origin at the lower left. `dpi` of 0 means 72. A pixel counts when any of R, G, or B is not 255. Row 0 is the top, so the bottom of pixel row `r` is `(height-r-1) * 72 / dpi` and the top is `(height-r) * 72 / dpi`. The left of column `c` is `c * 72 / dpi` and the right is `(c+1) * 72 / dpi`. The bool is false when no pixel counts. Proof: `go test -count=1 ./spectreps -run TestMeasureBox`.

### 1.2 Box text

- [ ] `spectreps bbox` reads one PostScript or PDF file. It uses the same `-w`, `-h`, and `-r` as `raster`. For each page it writes two lines to stdout. `%%BoundingBox:` uses the floor of each minimum and the ceiling of each maximum, as integers. `%%HiResBoundingBox:` prints the float edges with `strconv.FormatFloat(v, 'f', -1, 64)`. Ghostscript prints this on stderr and does not honor `-sOutputFile=` yet. Spectre prints the successful report on stdout. A page with no marked pixels prints `%%BoundingBox: 0 0 0 0` and `%%HiResBoundingBox: 0 0 0 0`. The manual does not say what an empty page prints, so this zero box is Spectre's rule. Proof: `go test -count=1 ./internal/cli -run TestBBox`.

### 1.3 RGB occupancy

- [ ] `spectreps.MeasureInk(img PageImage) Ink` returns the fraction of pixels whose R is not 255, and the same for G and B. The denominator is `Width * Height`. A white page is `0 0 0`. A page of red pixels is `1 0 0`. This matches Ghostscript `inkcov` occupancy, not `ink_cov` amounts, and the channels are RGB because that is the pixmap. Proof: `go test -count=1 ./spectreps -run TestMeasureInk`.

### 1.4 Coverage text

- [ ] `spectreps inkcov` uses the same inputs and page options as `bbox`. For each page it writes `Page N` and then three fractions with five digits after the point, then the word `RGB`. Page numbers start at 1. Example shape: `Page 1` then `1.00000 0.00000 0.00000 RGB`. Stdout, exit 0. Proof: `go test -count=1 ./internal/cli -run TestInkcov`.

## Dependencies

The pixmap and `RunPostScript` / `RasterizePage` from v0.0.1. `CompareRaster` stays the pixel equality check. These commands do not call it.
