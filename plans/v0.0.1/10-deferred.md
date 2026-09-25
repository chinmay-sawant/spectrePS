# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file. When that tag lands the work, move the row to 10.4 with the tag path. The `[~]` count in 10.1 to 10.3 is the open deferred count.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's v0.0.1 release is the PostScript subset, the path-only PDF, one pixmap, Flate rewrite, validate, and byte compare. The quick summaries, the JPEG raster, and the bitmap PDF landed in `plans/v0.0.2/`. TIFF, the image model, text, PDF/A, PostScript output, `gs` argv, and the printer languages wait.

## Phase 10: Deferred

### 10.1 Outputs that need an image or text model

- [~] DCT encode, CCITT, and downsample on rewrite. Reason: phase 07 has no image samples to resample. Next gate: a PDF image phase that can paint `Do` for a Flate or DCT image XObject, then a new plan file for lossy rewrite. `plans/v0.0.3/2-pdf-compression.md` rows 1.3 and 1.4 cover the same work.
- [~] PDF compression levels 1 to 5 over any PDF the reader can open. Reason: the current `rewrite` re-emits path operators only, so it stops on `cm`, text, and image XObjects, and it has one Flate switch instead of a policy. Detail moved to `plans/v0.0.3/2-pdf-compression.md`. Next gate: the object pass-through writer, then the image model.
- [~] TIFF raster. Reason: TIFF encode is `golang.org/x/image/tiff`, not the standard library, and a new module requirement is its own plan row. Detail moved to `plans/v0.0.3/1-quick-wins.md` (phase 2). TIFF bytes must not become an equality oracle.
- [~] Gray and CMYK image PDF (`pdfimage8`, `pdfimage32` style). Reason: only the 24-bit RGB writer exists, and the gray and CMYK conversions from an RGB pixmap need a named policy. Detail moved to `plans/v0.0.3/1-quick-wins.md` (phase 3).
- [~] `ink_cov` weighted ink amounts. Reason: Ghostscript prints `ink_cov` as a percent and its manual example disagrees with its source, so Spectre needs a named weighting model before any code. Next gate: a written model.
- [~] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Reason: fonts are a separate machine from the path engine. Next gate: a new plan file. The phase 04 y-flip test has landed. Until then those operators return errors, not blank pages.

### 10.2 PDF jobs and variants

- [~] Page selection for the raster and PDF jobs (`-dFirstPage` and `-dLastPage` style). Detail moved to `plans/v0.0.3/1-quick-wins.md` (phase 1).
- [~] PDF inputs in `compare raster`. Reason: `rasterPair` sends both files through `RunPostScript`. Detail moved to `plans/v0.0.3/1-quick-wins.md` (phase 1).
- [~] PDF/A-1b, PDF/A-2b, PDF/A-3b creation. Reason: needs output intents, metadata, and a finished rewrite. Creating the file is not a conformance certificate, and `PDFACompatibilityPolicy` 0 in Ghostscript can attach PDF/A metadata to a non-compliant file. Spectre will not copy that ambiguity. Next gate: a new plan file that states which PDF/A level and which policy. Phase 07 has landed.
- [~] PDF to PostScript via a `ps2write` style device. Reason: it is another high-level device on the same marks. Next gate: a new plan file. Phase 07 has landed.
- [~] `gs` argv compatibility mode. Reason: the subcommands map to library methods, and a second flag grammar would fork the CLI. Detail moved to `plans/v0.0.3/1-quick-wins.md` (phase 4) for a bounded switch map. The full grammar stays out.

### 10.3 Out of product

- [~] PCL, PXL, XPS, and the printer device list from `gs -h`. Reason: GhostPCL, GhostXPS, and the printer drivers are separate products from the PostScript and PDF interpreter. Next gate: a new program plan only if a named device is requested. No work starts from this row alone.

### 10.4 Landed

- [x] `pdfimage24` style output, a page raster wrapped in a PDF. Landed in v0.0.2 (`plans/v0.0.2/3-pdfimage.md`).
- [x] JPEG encoder. Landed in v0.0.2 (`plans/v0.0.2/2-jpeg-raster.md`). TIFF moved to 10.1 as its own row.
- [x] `bbox` and `inkcov` devices. Landed in v0.0.2 (`plans/v0.0.2/1-bbox-inkcov.md`).

## Dependencies

Each row names its next gate. A row that names a plan file has its checklist there. Do not add a second copy of those rows here.
