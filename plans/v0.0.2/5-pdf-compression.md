# v0.0.2 - PDF compression

> **Parent:** `plans/v0.0.2/00-program.md` - release ledger
> **Status:** implemented. Levels 0 to 5 landed and all rows are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** weeks. Depends on the object pass-through writer, then the image model.

---

## Overview

`spectreps rewrite` today re-emits the path subset and Flate-compresses the new content streams. This plan grows that into a compression job with named levels that accepts any PDF the reader can open: path content, text and fonts, image XObjects, and streams that are already compressed.

The contract:

- Every page reaches the output. Page count and page boxes match the input.
- Content Spectre interprets is re-emitted. Content it does not interpret is copied through unchanged.
- If the writer cannot copy an object, the command fails with a `JobError` and writes no file. Silent content loss is a bug.
- An encrypted or password-protected file still returns `invalidaccess`. Repair-and-continue stays a separate decision.
- An input whose streams are already Flate, DCT, or CCITT is accepted at every level.

## Executive summary

Levels are a Spectre policy, not a Ghostscript clone. Level 1 is lossless: Flate the content streams, leave images alone. Higher levels downsample and re-encode images, and the numbers below are the proposal. The image pipeline uses the standard library: `image/jpeg` for DCT and `compress/zlib` for Flate.

`sampledata/compress/whatisthis.pdf` is the acceptance fixture. It is a PDF 1.7 file with text, a `cm` matrix, and one DCT image. It fails today at `/undefined in cm`, which is the gap this plan closes.

## Phase 1: Levels over arbitrary PDFs

### 1.1 Object pass-through

- [x] The writer copies every object it does not rewrite: the page tree, `/Resources`, fonts, annotations, and metadata. It writes a new xref over the copied and new objects. Proof: `go test -count=1 ./internal/pdfout -run TestCopyObjects` exited 0 on 2026-09-25.

### 1.2 Content operators

- [x] The reader accepts `cm`. A six-number matrix composes into the CTM, `q` and `Q` save and restore it, path points transform through it before the device scale, and the stroke width scales by the same matrix scale. Proof: `go test -count=1 ./internal/pdf -run TestCTM` and `go test -count=1 ./spectreps -run TestCTM` exited 0 on 2026-09-25.
- [x] The pass-through writer copies a content stream it cannot interpret and edits only the parts it can, so `BT` `ET` `Tf` `Tj` `TJ` and the font resources reach the output unchanged. Proof: `go test -count=1 ./internal/pdfout -run TestLevelContentCopy` and `go test -count=1 ./spectreps -run TestRewriteLevelsStable` exited 0 on 2026-09-25.

### 1.3 Image XObjects

- [x] The reader opens `/Subtype /Image` XObjects with Flate and DCT streams. A DCT stream decodes through `image/jpeg`. Proof: `go test -count=1 ./internal/pdf -run TestImageXObject` exited 0 on 2026-09-25.
- [x] The writer half. `ScaleImage`, `EncodeDCT`, and `EncodeFlateRGB` are in `internal/pdfout/scale.go`, with `golang.org/x/image` v0.46.0 for CatmullRom. Proof: `go test -count=1 ./internal/pdfout -run 'TestScale|TestEncode'` exited 0 on 2026-09-25.

### 1.4 Levels

- [x] `spectreps rewrite -level N` accepts 0 through 5 and defaults to 0. Level 0 keeps the path-subset writer and the `-compress` switch. A level above 0 uses the pass-through writer and ignores `-compress`. The mapping is:

  | Level | Name | Content streams | Images |
  | --- | --- | --- | --- |
  | 1 | Light | Flate | unchanged |
  | 2 | Balanced | Flate | Flate and raw image streams re-encoded losslessly, no resample |
  | 3 | Medium | Flate | DCT, longest side capped at 1754 px, JPEG quality 80 |
  | 4 | Strong | Flate | DCT, longest side capped at 1123 px, JPEG quality 60 |
  | 5 | Hard | Flate | DCT, longest side capped at 842 px, JPEG quality 40 |

- [x] Proof: `go test -count=1 ./internal/cli -run TestRewriteLevels` exited 0 on 2026-09-25.
- [x] A missing `-level` keeps today's behavior. Level 5 does not promise a size, only the smallest of the five on the sample set.
  - [x] Writer half. `pdfout.LevelOverrides` builds the image column of the table: level 2 Flates raw rows and re-encodes Flate images, and levels 3 to 5 resample above the level's pixel cap and DCT-encode at the level's quality. Proof: `go test -count=1 ./internal/pdfout -run TestLevelImagePolicy` exited 0 on 2026-09-25.

### 1.5 Stable bytes

- [x] Two runs at the same level on the same input return buffers `CompareFiles` reports equal. The output contains no `CreationDate` or `ModDate`. Proof: `go test -count=1 ./spectreps -run TestRewriteLevelsStable` exited 0 on 2026-09-25.

### 1.6 Acceptance

- [x] `sampledata/compress/whatisthis.pdf` and `sampledata/compress/path.pdf` rewrite at every level. Page count matches the input, the JPEG image decodes in the output, and level 5 is the smallest file of the five. Proof: `go test -count=1 ./internal/cli -run TestRewriteSamples` exited 0 on 2026-09-25, guarded to skip when `sampledata/` is absent.

### 1.7 Documentation

- [x] `documentation/cli.md`, `documentation/devices.md`, `documentation/features.md`, `documentation/covered-and-not-covered.md`, and `documentation/test.md` state the level table and the pass-through rule. Proof: `grep -n 'level' documentation/cli.md documentation/features.md` showed the rows on 2026-09-25.

## Reference

Ghostscript's `pdfwrite` presets are the nearest published analogues. Source: `VectorDevices.html` and `Resource/Init/gs_pdfwr.ps` in the Ghostscript tree. `/screen` targets 72 dpi images, `/ebook` 150, and `/printer` and `/prepress` 300. Quality comes from `QFactor`, and the presets differ on color handling: `/sRGB` for screen and ebook, `/UseDeviceIndependentColor` for printer, and `/LeaveColorUnchanged` for prepress. The rendered manual table disagrees with `gs_pdfwr.ps` in a few rows, so the source is the better reference. Spectre's levels above stay its own contract.

## Dependencies

1.1 is independent and can land first. 1.2 needs either text support or the pass-through writer. 1.3 needs the image model. 1.4 to 1.6 need all of the above.

The existing deferred row for DCT, CCITT, and downsample in `plans/v0.0.1/10-deferred.md` is the same work as 1.3 and 1.4.

## Not in this plan

PDF/A output, output encryption, linearization, PDF 2.0 features, and byte parity with Ghostscript `pdfwrite`. Those stay in `plans/v0.0.1/10-deferred.md`.
