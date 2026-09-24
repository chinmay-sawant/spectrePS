# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's active ledger is PostScript, PDF, one pixmap, Flate rewrite, validate, and byte compare. Everything else waits.

## Phase 10: Deferred

### 10.1 Outputs that need an image or text model

- [~] DCT encode, CCITT, and downsample on rewrite. Reason: phase 07 has no image samples to resample. Next gate: a PDF image phase that can paint `Do` for a Flate or DCT image XObject, then a new plan file for lossy rewrite.
- [~] `pdfimage24` style output, a page raster wrapped in a PDF. Reason: it is a different device from vector `pdfwrite`, and tag 0.0.4 is the vector path. Next gate: phase 07 stable bytes, then a new plan file if a caller asks for bitmap PDFs.
- [~] JPEG and TIFF encoders. Reason: PNG and PPM already cover viewable and raw pixels. JPEG is lossy and must not become an equality oracle. Next gate: phase 04 PNG row checked, then a new plan file.
- [~] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Reason: fonts are a separate machine from the path engine. Next gate: phase 04 y-flip test checked, then a new plan file. Until then those operators return errors, not blank pages.
- [~] `bbox` and `inkcov` devices. Reason: both are summaries of a painted page, and the pixmap has to exist first. Next gate: phase 05, then a new plan file.

### 10.2 PDF variants

- [~] PDF/A-1b, PDF/A-2b, PDF/A-3b creation. Reason: needs output intents, metadata, and a finished rewrite. Creating the file is not a conformance certificate, and `PDFACompatibilityPolicy` 0 in Ghostscript can attach PDF/A metadata to a non-compliant file. Spectre will not copy that ambiguity. Next gate: phase 07, then a new plan file that states which PDF/A level and which policy.
- [~] PDF to PostScript via a `ps2write` style device. Reason: it is another high-level device on the same marks. Next gate: phase 07, then a new plan file.
- [~] `gs` argv compatibility mode. Reason: the subcommands map to library methods, and a second flag grammar would fork the CLI. Next gate: a written proposal that maps each accepted switch onto an existing method.

### 10.3 Out of product

- [~] PCL, PXL, XPS, and the printer device list from `gs -h`. Reason: GhostPCL, GhostXPS, and the printer drivers are separate products from the PostScript and PDF interpreter. Next gate: a new program plan only if a named device is requested. No work starts from this row alone.

## Dependencies

Each row names its next gate. No active phase file repeats these rows.
