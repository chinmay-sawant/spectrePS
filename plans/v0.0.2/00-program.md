# v0.0.2 - Quick wins

> **Parent:** `plans/v0.0.1/10-deferred.md` - deferred rows whose next gate has landed
> **Status:** implemented. Phases 1 to 3 and the closure rows are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** a few days for the three phases after the closure

---

## Overview

v0.0.1 is released. The deferred list is not a queue. This folder takes the three shortest rows, in the order a person can finish them, and leaves the rest where they are.

The order is the amount of new machinery, not the order the deferred file lists them.

## Executive summary

Ghostscript's `bbox` device prints the painted box in points. Its `inkcov` device prints how many CMYK device pixels are marked. Spectre already has an RGB pixmap, so both summaries are a scan of `PageImage`. That is the first phase.

JPEG encode is in the standard library next to PNG. A `.jpg` output is a third encoding of the same pixels. TIFF is not in the standard library, so it stays deferred.

`pdfimage24` wraps a page raster in a PDF. Spectre can write that from the pixmap it already paints. It does not teach `Do`, and it does not DCT-encode the image. Those stay deferred.

Text, PDF/A, PostScript rewrite, a `gs` argument grammar, and printer languages are not quick. They stay in `plans/v0.0.1/10-deferred.md`.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-bbox-inkcov.md` | 1 | Box and RGB mark coverage from the pixmap. |
| `2-jpeg-raster.md` | 2 | JPEG file from `spectreps raster -o`. |
| `3-pdfimage.md` | 3 | One 24-bit RGB image per page, inside a new PDF. |
| `v0.0.2-closure.md` | gate | `make test` runs packages with `-p`. Lint and test are recorded there. |

## What is not in v0.0.2

| Deferred row | Why it is not quick |
| --- | --- |
| DCT, CCITT, downsample | Needs a PDF image phase that paints `Do`. |
| TIFF | Encoder is `golang.org/x/image/tiff`, a new module. |
| Text, `show`, `Tj` | Fonts are a separate machine. |
| PDF/A | Needs a named level and a named policy. |
| PDF to PostScript | Another page description. |
| `gs` argv mode | A second flag grammar. |
| PCL, PXL, XPS, printers | Out of product. |

## Dependencies

Phase 1 depends on the pixmap from v0.0.1. Phase 2 depends on the raster CLI from v0.0.1. Phase 3 depends on the pixmap and on the stable PDF writer from v0.0.1. The closure does not depend on phases 1 to 3. It can land first.
