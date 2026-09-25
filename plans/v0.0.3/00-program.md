# v0.0.3 - Deferred work

> **Parent:** `plans/v0.0.1/10-deferred.md` - deferred rows whose next gate has landed
> **Status:** complete on 2026-09-25. Phases 1 to 10 landed; phase 9 row 3.6 (Type 1) went back to the deferred list, and phase 4 row 1.2 stays open with measured numbers.
> **Estimated effort:** days for phases 1 to 6, weeks for phases 7 to 10.

---

## Overview

v0.0.2 is released. This folder takes every open deferred row except the out-of-product printer languages, and gives each one a phase file. The order is the amount of new machinery: measurement and decoders first, then the image and PostScript devices, then the flag grammar, then the compliance and font machines.

The product stays pure Go. No cgo, no `os/exec`, and no Ghostscript linkage. A new module is a plan row of its own.

## Executive summary

Phase 1 adds the weighted `ink_cov` reading beside the occupancy `inkcov`. Phase 2 decodes CCITT G3 and G4, which `golang.org/x/image/ccitt` already covers. Phase 3 adds JPEG2000 through a pure-Go decoder behind a conformance gate. Phase 4 stops the compression writer from copying dead `/XRef` and `/ObjStm` containers. Phase 5 paints `Do`, so Spectre can rasterize image PDFs, including its own `pdfimage` output. Phase 6 writes PostScript for path pages.

Phase 7 grows the bounded `gs` switch map into a real argv mode. Phase 8 writes PDF/A-4 with a refusal policy, not Ghostscript's policy 0. Phase 9 builds the font and text machine. Phase 10 preserves and preflights PDF/UA-2, and does not generate tags.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-ink-cov.md` | 1 | Weighted `ink_cov` amounts beside the occupancy `inkcov`. |
| `2-ccitt-decode.md` | 2 | CCITT G3 and G4 image streams. |
| `3-jpeg2000-decode.md` | 3 | JPEG2000 image streams through a pure-Go decoder. |
| `4-writer-cleanup.md` | 4 | Stop copying dead containers, then re-measure levels 1 and 2. |
| `5-paint-do.md` | 5 | Paint `Do` and rasterize image PDFs. |
| `6-ps2write.md` | 6 | PDF to PostScript for path pages. |
| `7-gs-argv.md` | 7 | The full `gs` argv grammar, family by family. |
| `8-pdfa4.md` | 8 | PDF/A-4 output with a refusal policy. |
| `9-text-and-fonts.md` | 9 | Font model, text painting, and text extraction. |
| `10-pdfua2.md` | 10 | PDF/UA-2 preservation and preflight. |
| `v0.0.3-closure.md` | gate | Lint and test are recorded there. |

## What is not in v0.0.3

| Deferred row | Why it waits |
| --- | --- |
| PCL, PXL, XPS, and printer devices | Out of product. Separate programs from the PostScript and PDF interpreter. |
| Tag generation for PDF/UA-2 | Needs the text and font machine from phase 9, then reading order and role assignment. |
| Font hinting, font subsetting, and writing fonts | Not needed for reading, painting, or extraction. |
| JPEG2000 encoding | Decode is the job. |

## Dependencies

Phases 1 to 4 depend on the v0.0.2 surface only. Phase 5 depends on `DecodeImage` and the page tree from v0.0.2. Phase 6 depends on `pdf.Paint` and `RunPostScript`. Phase 7 depends on the existing commands and flags. Phase 8 depends on the pass-through writer and on a fixture that already embeds fonts. Phase 9 depends on `golang.org/x/image/font/sfnt` and `x/image/vector`. Phase 10 depends on the metadata writer from phase 8 and on phase 9 for tag generation. The closure does not depend on any phase.
