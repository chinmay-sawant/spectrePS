# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file. When that tag lands the work, move the row to 10.4 with the tag path. The `[~]` count in 10.1 to 10.3 is the open deferred count.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's v0.0.1 release is the PostScript subset, the path-only PDF, one pixmap, Flate rewrite, validate, and byte compare. v0.0.2 added the page summaries, JPEG raster, and the bitmap PDF. v0.0.3 added TIFF raster, page ranges, gray and CMYK image PDF, the `gs` switch map, the object pass-through writer, and compression levels 1 to 5. What waits now is painting `Do`, CCITT and JPEG2000 decoding, `ink_cov` weights, text, PDF/A, PostScript output, the full `gs` grammar, and the printer languages.

## Phase 10: Deferred

### 10.1 Outputs that need an image or text model

- [~] Painting `Do` and reading images into a raster. Reason: the reader decodes image XObjects, but the content interpreter still returns `undefined` for `Do`, so an image PDF cannot be rasterized by Spectre. Next gate: a plan file for the `Do` operator and the image marker seam.
- [~] CCITT and JPEG2000 image streams. Reason: `DecodeImage` reads Flate and DCT only, so those streams copy through unchanged in the compression levels. Next gate: a CCITT or JPX decoder, then a plan file.
- [~] `ink_cov` weighted ink amounts. Reason: Ghostscript prints `ink_cov` as a percent and its manual example disagrees with its source, so Spectre needs a named weighting model before any code. Next gate: a written model.
- [~] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Reason: fonts are a separate machine from the path engine. Next gate: a new plan file. The phase 04 y-flip test has landed. Until then those operators return errors, not blank pages.

### 10.2 PDF jobs and variants

- [~] PDF/A-1b, PDF/A-2b, PDF/A-3b creation. Reason: needs output intents, metadata, and a finished rewrite. Creating the file is not a conformance certificate, and `PDFACompatibilityPolicy` 0 in Ghostscript can attach PDF/A metadata to a non-compliant file. Spectre will not copy that ambiguity. Next gate: a new plan file that states which PDF/A level and which policy.
- [~] PDF to PostScript via a `ps2write` style device. Reason: it is another high-level device on the same marks. Next gate: a new plan file.
- [~] Full `gs` argv grammar. Reason: the subcommands map to library methods, and a second flag grammar would fork the CLI. A bounded switch map landed in v0.0.3 (`plans/v0.0.3/1-quick-wins.md`, phase 4); the full grammar stays out. Next gate: a written proposal per switch family.

### 10.3 Out of product

- [~] PCL, PXL, XPS, and the printer device list from `gs -h`. Reason: GhostPCL, GhostXPS, and the printer drivers are separate products from the PostScript and PDF interpreter. Next gate: a new program plan only if a named device is requested. No work starts from this row alone.

### 10.4 Landed

- [x] `pdfimage24` style output, a page raster wrapped in a PDF. Landed in v0.0.2 (`plans/v0.0.2/3-pdfimage.md`).
- [x] JPEG encoder. Landed in v0.0.2 (`plans/v0.0.2/2-jpeg-raster.md`).
- [x] `bbox` and `inkcov` devices. Landed in v0.0.2 (`plans/v0.0.2/1-bbox-inkcov.md`).
- [x] DCT encode and downsample on rewrite. Landed in v0.0.3 (`plans/v0.0.3/2-pdf-compression.md`, rows 1.3 and 1.4).
- [x] PDF compression levels 1 to 5 over any PDF the reader can open. Landed in v0.0.3 (`plans/v0.0.3/2-pdf-compression.md`).
- [x] TIFF raster. Landed in v0.0.3 (`plans/v0.0.3/1-quick-wins.md`, phase 2).
- [x] Gray and CMYK image PDF (`pdfimage8`, `pdfimage32` style). Landed in v0.0.3 (`plans/v0.0.3/1-quick-wins.md`, phase 3).
- [x] Page selection for the raster and PDF jobs. Landed in v0.0.3 (`plans/v0.0.3/1-quick-wins.md`, phase 1).
- [x] PDF inputs in `compare raster`. Landed in v0.0.3 (`plans/v0.0.3/1-quick-wins.md`, phase 1).
- [x] `gs` argv compatibility mode, bounded switch map. Landed in v0.0.3 (`documentation/gs-argv-mapping.md`).

## Dependencies

Each row names its next gate. A row that names a plan file has its checklist there. Do not add a second copy of those rows here.
