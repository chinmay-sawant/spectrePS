# v0.0.3 - Paint Do and rasterize image PDFs

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** phase 1 landed. Phases 2 to 4 open.
> **Estimated effort:** about 4 days

---

## Overview

The reader decodes image XObjects, but the content interpreter has no `Do` case and the scanner throws name text away, so `/Im0 Do` returns `undefined`. That blocks rasterizing any PDF with images, including Spectre's own `pdfimage` output and the compressed samples.

## Executive summary

`Do` resolves a name in the page's `/XObject` resources, decodes the image once per name, and paints it into the unit square through the current matrix. A new `Marker.DrawImage` method carries the pixels to the pixmap and to the rewrite recorder. The recorder declines images so level 0 keeps returning `undefined`, byte for byte. Sampling is nearest neighbor, which round-trips Spectre's own RGB and gray image PDFs exactly at 1:1.

## Phase 1: Names and resources

### 1.1 Name-carrying lexer

- [x] The content scanner returns name text and `runner` gains a `popName` helper. `Do` on a non-name operand is `typecheck`, and `/Im Do` without resources stays `undefined`. Proof: `go test -count=1 ./internal/pdf -run TestPaintNameOperand` and `go test -count=1 ./internal/pdf -run TestPaintUndefined` exited 0 on 2026-09-25.

### 1.2 Page resources with inheritance

- [x] The page-tree walk keeps each page's nearest `/Resources`, with `/XObject` subdictionary lookup, and `PaintPage` passes it to the interpreter. `/Resources` inherited from a Pages ancestor resolves. Proof: `go test -count=1 ./internal/pdf -run TestPageResourcesInherited` and `go test -count=1 ./internal/pdf -run TestPageResourcesDirect` exited 0 on 2026-09-25.

### 1.3 Value-based image decode

- [x] The decode path is factored so a resolved stream value decodes the same way `DecodeImage(num)` does, and direct and indirect XObjects both work. Proof: `go test -count=1 ./internal/pdf -run TestDecodeImageValue` and `go test -count=1 ./internal/pdf -run TestImageXObject` exited 0 on 2026-09-25.

## Phase 2: The seam and the pixmap

### 2.1 Marker.DrawImage

- [ ] `internal/graphics/device.go` gains `DrawImage(pic image.Image, ctm Matrix, scale float64)`, implemented by `Pixmap`. `pdfout.recorder` implements it by recording that an image was seen. Proof: `go test -count=1 ./internal/graphics -run TestPixmapDrawImageUnitSquare`, `TestPixmapDrawImageCTM`, and `TestPixmapDrawImageNearest`.

### 2.2 Nearest-neighbor sampler

- [ ] The pixmap inverts the image-to-device matrix once per stamp, loops the mapped bounding box, and samples the nearest pixel with row 0 at the top. Opaque RGB and gray are painted; alpha is ignored in this phase. Proof: the same test run, plus a 1:1 round trip of an `ImagePDF` stream.

## Phase 3: Interpreter and rewrite gate

### 3.1 Do in the interpreter

- [ ] `Do` pops a name, resolves `/XObject`, requires `/Subtype /Image`, decodes once per name into a runner cache, and calls `DrawImage` with the current matrix and scale. Missing names, non-image subtypes, and decode errors return `undefined` with the `Do` operator name. Proof: `go test -count=1 ./internal/pdf -run TestPaintDoImageRGB`, `TestPaintDoImageGray`, `TestPaintDoCM`, `TestPaintDoMissing`, and `TestPaintDoRejectedSMask`.

### 3.2 Rewrite level 0 stays gated

- [ ] `pdfout.Emit` returns `undefined in Do` when the recorder saw an image, so level 0 does not silently outline or drop images. Proof: `go test -count=1 ./internal/pdfout -run TestEmitDoUnchanged`.

## Phase 4: Round trip and docs

### 4.1 Rasterize Spectre's own image PDF

- [ ] `RunPostScript` to `ImagePDF` to `OpenPDF` to `RasterizePage` compares equal to the source `PageImage` under `CompareRaster`, for the RGB and gray color modes. Proof: `go test -count=1 ./spectreps -run TestRasterizeOwnImagePDF` and `go test -count=1 ./internal/cli -run TestRasterOwnImagePDF`.

### 4.2 Docs and closure

- [ ] `documentation/devices.md`, `features.md`, `covered-and-not-covered.md`, `test.md`, and `public-api.md` state the new behavior, and the deferred row moves to 10.4. Proof: `grep -n 'DrawImage' documentation/devices.md`.
- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

`DecodeImage`, the page-tree walk, `graphics.Matrix.Invert`, and the pass-through writer from v0.0.2. No new module.

## Not in this plan

- `/ImageMask`, `/SMask`, `/Mask`, `/Decode` arrays, interpolation, and CMYK images.
- Form XObjects; `Do` on a non-image subtype returns `undefined`.
- Inline images (`BI`/`ID`/`EI`).
