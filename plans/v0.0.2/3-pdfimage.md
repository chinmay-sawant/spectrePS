# v0.0.2 - Bitmap PDF

> **Parent:** `plans/v0.0.2/00-program.md` - quick-win ledger
> **Status:** not started
> **Estimated effort:** 3 days

---

## Overview

Ghostscript `pdfimage24` renders the page to a bitmap and wraps that bitmap in a PDF. The Devices page says those devices "render input to a bitmap then wraps the bitmap(s) up as the content of a PDF file," at colour depth 8 gray, 24 RGB, or 32 CMYK. `pdfwrite` is the high-level device that keeps vectors. Source: [Devices](https://ghostscript.readthedocs.io/en/latest/Devices.html) and [Vector devices](https://ghostscript.readthedocs.io/en/latest/VectorDevices.html).

Spectre's `RewritePDF` is the vector path. `pdfout.Page` holds content operators only. `Do` is still `undefined`. This phase does not change that.

## Executive summary

`spectreps pdfimage -o out.pdf` paints with the existing raster path, then writes one PDF page per pixmap. Each page carries one 24-bit RGB image, 8 bits per channel, `/ColorSpace /DeviceRGB`, `/Subtype /Image`. The image stream is Flate, not DCT. DCT is the deferred downsample row, and Flate keeps the file stable. Two calls on the same input return equal bytes. There is no creation date.

Opening that PDF and calling `RasterizePage` is not the proof. The content stream has to invoke the image, and `Do` still fails. The proof decodes the image stream and compares those bytes with the pixmap.

## Phase 3: One image per page

### 3.1 Writer

- [ ] `internal/pdfout` grows a writer that accepts `[]PageImage` plus the dpi used to paint them, and returns PDF bytes. Each page object has a `/MediaBox` in points, `width * 72 / dpi` by `height * 72 / dpi`. The image dictionary is `/Subtype /Image`, `/Width`, `/Height`, `/ColorSpace /DeviceRGB`, `/BitsPerComponent 8`, `/Filter /FlateDecode`. The stored stream is the tightly packed RGB bytes, zlib wrapped. The trailer `/ID` is the SHA-256 of those stored streams, both strings the same, with no `/Info` and no date. A cancelled context returns `ctx.Err()`. A nil context panics with `pdfout: nil context`. Proof: `go test -count=1 ./internal/pdfout -run TestImagePDF`.

### 3.2 Command

- [ ] `spectreps pdfimage` requires `-o` and one input path. Missing `-o` exits 2. `-w`, `-h`, and `-r` match `raster`. A `.pdf` input uses `RasterizePage` for every page. Any other input uses `RunPostScript`. The command writes the PDF at mode `0o600` and exits 0. `RewritePDF` is unchanged and still emits path operators. Proof: `go test -count=1 ./internal/cli -run TestPDFImage`.

### 3.3 Stable bytes

- [ ] Two `pdfimage` runs on the same input and the same options return buffers `CompareFiles` reports equal. The bytes contain neither `CreationDate` nor `ModDate`. The inflated image matches `PageImage` RGB with stride padding removed. Proof: `go test -count=1 ./spectreps -run TestImagePDFStable` and the `TestImagePDF` run from row 3.1.

## Dependencies

`RasterizePage`, `RunPostScript`, and the classic PDF writer from v0.0.1. Phase 2 is not required. A later image phase that paints `Do` can consume this file. This phase does not implement `Do`.
