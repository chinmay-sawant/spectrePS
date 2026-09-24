# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file. When that tag lands the work, the row becomes `[x]` with the path.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's v0.0.1 release is the PostScript subset, the path-only PDF, one pixmap, Flate rewrite, validate, and byte compare. The quick summaries, the JPEG raster, and the bitmap PDF landed in `plans/v0.0.2/`. Everything else waits.

## Phase 10: Deferred

### 10.1 Outputs that need an image or text model

- [~] DCT encode, CCITT, and downsample on rewrite. Reason: phase 07 has no image samples to resample. Next gate: a PDF image phase that can paint `Do` for a Flate or DCT image XObject, then a new plan file for lossy rewrite.
- [x] `pdfimage24` style output, a page raster wrapped in a PDF. Landed in v0.0.2 (`plans/v0.0.2/3-pdfimage.md`).
- [x] JPEG and TIFF encoders. JPEG landed in v0.0.2 (`plans/v0.0.2/2-jpeg-raster.md`). TIFF stays here. Reason: TIFF encode is `golang.org/x/image/tiff`, not the standard library, and a new module requirement is its own plan row. Next gate: that dependency row, then a new plan file. JPEG must not become an equality oracle.
- [~] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Reason: fonts are a separate machine from the path engine. Next gate: phase 04 y-flip test checked, then a new plan file. Until then those operators return errors, not blank pages.
- [x] `bbox` and `inkcov` devices. Landed in v0.0.2 (`plans/v0.0.2/1-bbox-inkcov.md`).

### 10.2 PDF variants

- [~] PDF/A-1b, PDF/A-2b, PDF/A-3b creation. Reason: needs output intents, metadata, and a finished rewrite. Creating the file is not a conformance certificate, and `PDFACompatibilityPolicy` 0 in Ghostscript can attach PDF/A metadata to a non-compliant file. Spectre will not copy that ambiguity. Next gate: phase 07, then a new plan file that states which PDF/A level and which policy.
- [~] PDF to PostScript via a `ps2write` style device. Reason: it is another high-level device on the same marks. Next gate: phase 07, then a new plan file.
- [~] `gs` argv compatibility mode. Reason: the subcommands map to library methods, and a second flag grammar would fork the CLI. Next gate: a written proposal that maps each accepted switch onto an existing method.

### 10.3 Out of product

- [~] PCL, PXL, XPS, and the printer device list from `gs -h`. Reason: GhostPCL, GhostXPS, and the printer drivers are separate products from the PostScript and PDF interpreter. Next gate: a new program plan only if a named device is requested. No work starts from this row alone.

## Dependencies

Each row names its next gate. A row that names a `plans/v0.0.2/` file has its checklist there. Do not add a second copy of those rows here.
