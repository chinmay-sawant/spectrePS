# v0.0.1 - Deferred work

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** deferred
> **Estimated effort:** not in the current tags

---

## Overview

Rows here are `[~]` on purpose. They are not a second active checklist. When one starts, add a new file under `plans/` for that tag, move the detail there, and leave this row as `[~]` with the path of the new file. When that tag lands the work, move the row to 10.4 with the tag path. The `[~]` count in 10.1 to 10.3 is the open deferred count.

## Executive summary

Ghostscript 9.55.0 exposes hundreds of printer devices, plus PCL and XPS in sister products. Spectre's v0.0.1 release is the PostScript subset, the path-only PDF, one pixmap, Flate rewrite, validate, and byte compare. v0.0.2 added the page summaries, JPEG and TIFF raster, page ranges, gray and CMYK image PDF, the `gs` switch map, the object pass-through writer, and compression levels 1 to 5. v0.0.3 added painting `Do`, CCITT and JPEG2000 decoding, `ink_cov` weights, the writer container cleanup, PDF/A-4, PDF/UA-2 preservation and preflight, PostScript output, the full `gs` grammar, and the font and text machine. v0.0.4 adds Type 1 programs, PDF/UA-2 tag generation, content graphics, transparency, separations with the RGB preview, font subsetting, the remaining stream filters, optional content, and `spectreps info`. What waits now is `tiffsep` plate output, bare CFF, standard 14 outline programs, two performance follow-ups, encryption, and the printer languages. The printer languages stay out of product.

## Phase 10: Deferred

### 10.1 Outputs that need an image, text, or font model

- [~] Bare CFF and CID-keyed CFF. Reason: the gated Type 1 plan phase 7 did not run in v0.0.4 because the tag budget ran out (`plans/v0.0.4/2-type1-fonts.md` phase 7). A simple `/FontFile3 /Subtype /Type1C` reader is the next gate, and `/CIDFontType0C` stays out until then.
- [~] Standard 14 outline programs. Reason: the standard 14 have metrics and extraction but no outlines, so `show` on a device is `invalidfont` by the language contract. The CUPS `cups-testfile.ps` corpus row is a recorded refusal because of this gate. Next gate: a font substitution plan that supplies license-clean outlines.

### 10.2 PDF jobs and variants

- [~] Spot-color separations (`tiffsep`). Reason: the reader resolves `/Separation` and `/DeviceN` to the RGB preview, but the plate accumulator and the `spectreps tiffsep` command were the first cut of `plans/v0.0.4/4-pdf-coverage.md` phase 3. Next gate: a separation device seam that carries one plane per ink.
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
- [x] Compression writer container cleanup and the optional packed form. Landed in v0.0.3 (`plans/v0.0.3/4-writer-cleanup.md`). The size guard in row 1.2 is accepted as the 610,034-byte ceiling in `plans/v0.0.4/1-writer-ceiling.md`.
- [x] Painting `Do` and reading images into a raster. Landed in v0.0.3 (`plans/v0.0.3/5-paint-do.md`).
- [x] PDF to PostScript via a `ps2write` style device. Landed in v0.0.3 (`plans/v0.0.3/6-ps2write.md`).
- [x] Full `gs` argv grammar. Landed in v0.0.3 (`plans/v0.0.3/7-gs-argv.md`).
- [x] PDF/A-4 creation, with the refusal policy. Landed in v0.0.3 (`plans/v0.0.3/8-pdfa4.md`). The claim is a profile preflight, not a certificate.
- [x] PDF/UA-2 preservation and preflight. Landed in v0.0.3 (`plans/v0.0.3/10-pdfua2.md`).
- [x] Text extraction in the style of `txtwrite`, `show`, and PDF `Tj`. Landed in v0.0.3 (`plans/v0.0.3/9-text-and-fonts.md`). Text pixels never byte-match Ghostscript; extraction compares text and geometry.
- [x] Type 1 charstrings and `seac`. Landed in v0.0.4 (`plans/v0.0.4/2-type1-fonts.md` phases 1 to 6). Bare CFF stays deferred in 10.1.
- [x] Tag generation for PDF/UA-2. Landed in v0.0.4 (`plans/v0.0.4/3-pdfua2-tags.md`). The claim is "generate and preflight", never certification.
- [x] PDF content graphics: clip `W`, Form XObjects, and `/ExtGState`. Landed in v0.0.4 (`plans/v0.0.4/4-pdf-coverage.md` phase 1).
- [x] Transparency: `/SMask`, `/Mask`, alpha, and blend modes. Landed in v0.0.4 (`plans/v0.0.4/4-pdf-coverage.md` phase 2).
- [x] Spot colors and separations, RGB preview only. Landed in v0.0.4 (`plans/v0.0.4/4-pdf-coverage.md` phase 3). `tiffsep` plate output stays deferred in 10.2.
- [x] Font embedding and subsetting. Landed in v0.0.4 (`plans/v0.0.4/4-pdf-coverage.md` phase 4).
- [x] Stream filters beyond Flate, DCT, CCITT, and JPX: LZW, ASCII85, ASCIIHex, RunLength, and predictors; optional content default visibility. Landed in v0.0.4 (`plans/v0.0.4/4-pdf-coverage.md` phases 5 and 6), plus the read-only `spectreps info` command.

### 10.5 Performance follow-ups

- [~] Font parse cache. Reason: `plans/v0.0.4/6-performance-profiling.md` row 5.4 did not measure a repeated `sfnt` parse large enough to change the accepted budget. Next gate: a profile that isolates a repeated parse on a font-heavy corpus page.
- [~] 300 dpi to 600 dpi scaling. Reason: the 9x gap recorded on 2026-09-25 did not reproduce on the merged tree (3.9x for 4x pixels), so no fix row follows. Next gate: a machine-specific profile on the current tree.

## Dependencies

Each row names its next gate. A row that names a plan file has its checklist there. Do not add a second copy of those rows here.
