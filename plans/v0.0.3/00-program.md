# v0.0.3 - Future work

> **Parent:** `plans/v0.0.1/10-deferred.md` - deferred rows that outgrew one line
> **Status:** implemented. Both phase files are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** days for the quick wins. Weeks for the compression plan.

---

## Overview

This folder holds future work that was deferred with a detail file. Both phases landed in v0.0.3. The remaining rows stay in `plans/v0.0.1/10-deferred.md`.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-quick-wins.md` | 1 | PDF page ranges, PDF inputs for `compare raster`, TIFF raster, gray and CMYK image PDF, and the `gs` switch map. |
| `2-pdf-compression.md` | 2 | Compression levels 1 to 5 over any PDF the reader can open. |

## Remaining deferred after v0.0.3

| Item | Why it waits |
| --- | --- |
| Painting `Do` and reading images into a raster | The reader decodes image XObjects, but the content interpreter still returns `undefined` for `Do`. |
| CCITT and JPEG2000 image streams | `DecodeImage` reads Flate and DCT only; those streams copy through unchanged. |
| `ink_cov` weighted amounts | Needs a named weighting model. |
| Text extraction, `show`, `Tj` | Fonts are a separate machine. |
| PDF/A-1b, PDF/A-2b, PDF/A-3b creation | Needs a named level, a named policy, and metadata. |
| PDF to PostScript (`ps2write` style) | Another high-level device on the same marks. |
| Full `gs` argv grammar | The bounded switch map landed; the full grammar stays out. |
| PCL, PXL, XPS, printer devices | Separate products from the PostScript and PDF interpreter. |

## Dependencies

The quick wins depend on the raster CLI from v0.0.2. `2-pdf-compression.md` depends on the object pass-through writer and the image model. The two phases are independent.
