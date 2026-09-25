# v0.0.3 - Future work

> **Parent:** `plans/v0.0.1/10-deferred.md` - deferred rows that outgrew one line
> **Status:** proposed. No row is active. The deferred rows in `plans/v0.0.1/10-deferred.md` point here.
> **Estimated effort:** days for the quick wins. Weeks for the compression plan.

---

## Overview

This folder holds future work that was deferred with a detail file. Nothing here is scheduled. A phase becomes active when its file is checked the way `plans/v0.0.2/` was checked.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-quick-wins.md` | 1 | PDF page ranges, PDF inputs for `compare raster`, TIFF raster, gray and CMYK image PDF, and the `gs` switch map. |
| `2-pdf-compression.md` | 2 | Compression levels 1 to 5 over any PDF the reader can open. |

## Remaining deferred after v0.0.3

| Item | Why it waits |
| --- | --- |
| Images inside a PDF, `Do` | The reader has no image XObject model. `2-pdf-compression.md` starts it. |
| DCT, CCITT, and downsampling | Rows 1.3 and 1.4 of `2-pdf-compression.md`. |
| Text extraction, `show`, `Tj` | Fonts are a separate machine. |
| PDF/A-1b, PDF/A-2b, PDF/A-3b creation | Needs a named level, a named policy, and metadata. |
| PDF to PostScript (`ps2write` style) | Another high-level device on the same marks. |
| `gs` argv compatibility mode | A bounded switch map is phase 4 of `1-quick-wins.md`. The full grammar stays out. |
| PCL, PXL, XPS, printer devices | Separate products from the PostScript and PDF interpreter. |

## Dependencies

The quick wins depend on the raster CLI from v0.0.2. `2-pdf-compression.md` depends on the object pass-through writer and the image model. The two phases are independent.
