# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file. When that tag lands the work, move the row to 10.4 with the tag path. The `[~]` count in 10.1 to 10.3 is the open deferred count.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's v0.0.1 release is the PostScript subset, the path-only PDF, one pixmap, Flate rewrite, validate, and byte compare. v0.0.2 added the page summaries, JPEG and TIFF raster, page ranges, gray and CMYK image PDF, the `gs` switch map, the object pass-through writer, and compression levels 1 to 5. v0.0.3 adds painting `Do`, CCITT and JPEG2000 decoding, `ink_cov` weights, the writer container cleanup, PDF/A-4, PDF/UA-2 preservation and preflight, PostScript output, the full `gs` grammar, and the font and text machine. What waits now is Type 1 fonts, PDF/UA-2 tag generation, the remaining PDF coverage, and the printer languages. The first three have phase files under `plans/v0.0.4/`; the printer languages stay out of product.

## Phase 10: Deferred

### 10.1 Outputs that need an image or text model

- [~] Type 1 charstrings and `seac`. Reason: `documentation/fonts.md` defers Type 1 and bare CFF, so the font model reads TrueType and OpenType through `sfnt` only. Detail moved to `plans/v0.0.4/2-type1-fonts.md` (bare CFF is the gated last phase).

### 10.2 PDF jobs and variants

- [~] Tag generation for PDF/UA-2. Reason: needs reading order and role assignment on top of the text and font machine, which landed in v0.0.3. Preservation and preflight landed in v0.0.3 (`plans/v0.0.3/10-pdfua2.md`). Detail moved to `plans/v0.0.4/3-pdfua2-tags.md`.
- [~] PDF content graphics: clip `W`, Form XObjects, and `/ExtGState`. Reason: the content interpreter returns `undefined` for them, and transparency and separations need forms and clip. Detail moved to `plans/v0.0.4/4-pdf-coverage.md`.
- [~] Transparency: `/SMask`, `/Mask`, alpha, and blend modes. Reason: the painter refuses `/SMask` and has no alpha or blend seam. Detail moved to `plans/v0.0.4/4-pdf-coverage.md`.
- [~] Spot colors and separations (`/Separation`, `/DeviceN`, and a `tiffsep`-style plate output). Reason: color is one RGB triple end to end. Detail moved to `plans/v0.0.4/4-pdf-coverage.md`.
- [~] Font embedding and subsetting. Reason: the font machine reads fonts; writing and subsetting them is a separate job. Detail moved to `plans/v0.0.4/4-pdf-coverage.md`.
- [~] Stream filters beyond Flate, DCT, CCITT, and JPX: LZW, ASCII85, ASCIIHex, RunLength, and predictors; optional content default visibility. Reason: the generic stream decoder decodes Flate only and holds no optional-content state. Detail moved to `plans/v0.0.4/4-pdf-coverage.md`.
- [~] Encryption, linearization, and output encryption. Reason: encryption needs key derivation (RC4, AES-128, AES-256), a crypt filter model, and decryption at every string and stream read. Next gate: a new plan file; v0.0.4 keeps it out.

### 10.3 Out of product

- [~] PCL, PXL, XPS, and the printer device list from `gs -h`. Reason: GhostPCL, GhostXPS, and the printer drivers are separate products from the PostScript and PDF interpreter. Explicitly deferred on 2026-09-25: no plan file exists, and no work starts from this row alone. A named device request opens a program plan.

### 10.4 Landed

- [x] `pdfimage24` style output, a page raster wrapped in a PDF. Landed in v0.0.2 (`plans/v0.0.2/3-pdfimage.md`).
- [x] JPEG encoder. Landed in v0.0.2 (`plans/v0.0.2/2-jpeg-raster.md`).
- [x] `bbox` and `inkcov` devices. Landed in v0.0.2 (`plans/v0.0.2/1-bbox-inkcov.md`).
- [x] DCT encode and downsample on rewrite. Landed in v0.0.2 (`plans/v0.0.2/5-pdf-compression.md`, rows 1.3 and 1.4).
- [x] PDF compression levels 1 to 5 over any PDF the reader can open. Landed in v0.0.2 (`plans/v0.0.2/5-pdf-compression.md`).
- [x] TIFF raster. Landed in v0.0.2 (`plans/v0.0.2/4-quick-wins.md`, phase 2).
- [x] Gray and CMYK image PDF (`pdfimage8`, `pdfimage32` style). Landed in v0.0.2 (`plans/v0.0.2/4-quick-wins.md`, phase 3).
- [x] Page selection for the raster and PDF jobs. Landed in v0.0.2 (`plans/v0.0.2/4-quick-wins.md`, phase 1).
- [x] PDF inputs in `compare raster`. Landed in v0.0.2 (`plans/v0.0.2/4-quick-wins.md`, phase 1).
- [x] `gs` argv compatibility mode, bounded switch map. Landed in v0.0.2 (`documentation/gs-argv-mapping.md`).
- [x] `ink_cov` weighted ink amounts. Landed in v0.0.3 (`plans/v0.0.3/1-ink-cov.md`).
- [x] CCITT G3 and G4 image streams. Landed in v0.0.3 (`plans/v0.0.3/2-ccitt-decode.md`).
- [x] JPEG2000 image streams. Landed in v0.0.3 (`plans/v0.0.3/3-jpeg2000-decode.md`).
- [x] Compression writer container cleanup and the optional packed form. Landed in v0.0.3 (`plans/v0.0.3/4-writer-cleanup.md`). The size guard in row 1.2 is open because the classic form must expand the source object streams.
- [x] Painting `Do` and reading images into a raster. Landed in v0.0.3 (`plans/v0.0.3/5-paint-do.md`).
- [x] PDF to PostScript via a `ps2write` style device. Landed in v0.0.3 (`plans/v0.0.3/6-ps2write.md`).
- [x] Full `gs` argv grammar. Landed in v0.0.3 (`plans/v0.0.3/7-gs-argv.md`).
- [x] PDF/A-4 creation, with the refusal policy. Landed in v0.0.3 (`plans/v0.0.3/8-pdfa4.md`). The claim is a profile preflight, not a certificate.
- [x] PDF/UA-2 preservation and preflight. Landed in v0.0.3 (`plans/v0.0.3/10-pdfua2.md`). Tag generation stays out of the ledger.
- [x] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Landed in v0.0.3 (`plans/v0.0.3/9-text-and-fonts.md`). Text pixels never byte-match Ghostscript; extraction compares text and geometry.

## Dependencies

Each row names its next gate. A row that names a plan file has its checklist there. Do not add a second copy of those rows here.
