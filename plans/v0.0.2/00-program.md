# v0.0.2 - Quick wins and compression

> **Parent:** `plans/v0.0.1/10-deferred.md` - deferred rows whose next gate has landed
> **Status:** implemented. Phases 1 to 5 and the closure rows are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** done with the plan

---

## Overview

v0.0.1 is released. This folder takes the shortest deferred rows, in the order a person can finish them, and leaves the rest where they are. It grew from three phases to five while the tag stayed open: the page summaries, JPEG, and bitmap PDF came first, then page ranges, TIFF, image colors, the `gs` switch map, and PDF compression levels.

The order is the amount of new machinery, not the order the deferred file lists them.

## Executive summary

Ghostscript's `bbox` device prints the painted box in points. Its `inkcov` device prints how many CMYK device pixels are marked. Spectre already has an RGB pixmap, so both summaries are a scan of `PageImage`. That is the first phase.

JPEG encode is in the standard library next to PNG. A `.jpg` output is a third encoding of the same pixels. TIFF needs `golang.org/x/image/tiff`, which became the first module dependency in phase 4.

`pdfimage24` wraps a page raster in a PDF. Spectre can write that from the pixmap it already paints, and phase 4 added the gray and CMYK variants. `Do` and DCT input stay deferred.

Phase 4 adds page ranges, PDF inputs for `compare raster`, TIFF, the `gs` switch map, and the image colors. Phase 5 adds the object pass-through writer, the image model, and compression levels 1 to 5. A real-world PDF 1.7 file compresses from 596,341 bytes to 85,760 bytes at level 5 with all 8 pages preserved.

Text, PDF/A, PostScript rewrite, the full `gs` argument grammar, and printer languages are not quick. They stay in `plans/v0.0.1/10-deferred.md`.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-bbox-inkcov.md` | 1 | Box and RGB mark coverage from the pixmap. |
| `2-jpeg-raster.md` | 2 | JPEG file from `spectreps raster -o`. |
| `3-pdfimage.md` | 3 | One 24-bit RGB image per page, inside a new PDF. |
| `4-quick-wins.md` | 4 | Page ranges, PDF compare, TIFF, gray and CMYK image PDF, and the `gs` switch map. |
| `5-pdf-compression.md` | 5 | Compression levels 1 to 5 over any PDF the reader can open. |
| `v0.0.2-closure.md` | gate | `make test` runs packages with `-p`. Lint and test are recorded there. |

## What is not in v0.0.2

| Deferred row | Why it waits |
| --- | --- |
| Painting `Do` and reading images into a raster | The reader decodes image XObjects, but the content interpreter still returns `undefined` for `Do`. |
| CCITT and JPEG2000 image streams | `DecodeImage` reads Flate and DCT only, so those streams copy through unchanged. |
| `ink_cov` weighted amounts | Needs a named weighting model. |
| Text, `show`, `Tj` | Fonts are a separate machine. |
| PDF/A | Needs a named level and a named policy. |
| PDF to PostScript | Another page description. |
| Full `gs` argv grammar | A bounded switch map landed in phase 4; the full grammar stays out. |
| PCL, PXL, XPS, printers | Out of product. |

## Dependencies

Phase 1 depends on the pixmap from v0.0.1. Phase 2 depends on the raster CLI from v0.0.1. Phase 3 depends on the pixmap and on the stable PDF writer from v0.0.1. Phase 4 depends on the raster CLI and the public API. Phase 5 depends on the pass-through writer and the image model from phase 4. The closure does not depend on any phase. It landed first.
