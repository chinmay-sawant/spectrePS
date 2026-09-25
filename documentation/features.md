# Features

This file is the product inventory: what Spectre supports today and what waits. `documentation/covered-and-not-covered.md` maps the same ground against Ghostscript. `plans/v0.0.1/10-deferred.md` is the ledger for the work that waits, with a next gate on each row.

The released tag is v0.0.1. v0.0.2 adds the page summaries, JPEG and TIFF raster, page selection, PDF inputs in `compare raster`, gray and CMYK image PDF, and PDF compression levels 1 to 5. `spectreps version` prints `0.0.2`.

## Input

- PostScript source over the subset in `documentation/language.md`. Tokens, three stacks, procedures, dictionaries, arrays, strings, control flow, math, matrix operators, and the path and paint operators all run.
- PDF path content. Classic xref tables and xref streams open. Object streams supply objects a type 2 xref row names. Content streams decode through Flate, and the page content operators `m l c h re S s f f* n q Q cm w RG rg g G` paint through the same graphics engine as PostScript. `cm` composes a six-number matrix into the CTM, path points transform through it before the device scale, and the stroke width scales by it. `q` and `Q` save and restore it.
- A PostScript header such as `%!PS-Adobe-3.0` is optional. It scans as a comment.
- An encrypted PDF returns `invalidaccess`. A PDF with an unknown stream filter returns `undefined`. A page that uses `Tj`, `TJ`, `'`, `"`, or `Do` fails with that operator name. The page is not a blank success.
- The page count is the number of page leaves in the tree, not the trailer `/Count`.

## Raster output

- `spectreps raster` writes PPM raw (P6) for any suffix other than `.png`, `.jpg`, `.jpeg`, `.tif`, or `.tiff`.
- A `.png` path encodes the pixmap with `image/png`.
- A `.jpg` or `.jpeg` path encodes the same pixmap with `image/jpeg`. `-jpegq` takes 1 through 100 and defaults to 75. JPEG is lossy, and its file bytes are not an equality oracle.
- A `.tif` or `.tiff` path encodes the pixmap with `golang.org/x/image/tiff`. `-tiffcompress none|deflate` picks the compression and defaults to `deflate`. TIFF file bytes are not an equality oracle.
- `-pages` selects a 1-based inclusive range for `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster`. A single `N` selects one page, `A-B` a range, `A-` from page A to the last page, and `-B` from page 1 to B. An output path with `%d` numbers the emitted pages from 1. A start past the last page returns `rangecheck`, an end past it clamps to the last page, and omitting the flag selects every page.
- The pixmap is RGB8, row 0 at the top, stride `Width * 3`. Default media is 612 by 792 points and the default resolution is 72 dpi.

## Page measurement

- `spectreps bbox` prints `%%BoundingBox` with floored minima and ceilinged maxima, then `%%HiResBoundingBox` with the float edges. The box is the union of marked pixels in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. A blank page prints a zero box.
- `spectreps inkcov` prints `Page N` and three RGB occupancy fractions with five digits after the point. It is not a CMYK report and it does not end in `CMYK OK`. These are occupancy fractions, not `ink_cov` weighted amounts.
- Both commands accept `-w`, `-h`, and `-r`, and they rasterize every page of a PDF input.

## PDF output

- `spectreps rewrite -level 0` writes a new PDF from a path-only PDF. Content streams carry the same path subset. `-compress` selects Flate content streams and defaults to true. Bytes are stable across two calls, and the file carries no wall-clock date. A missing `-level` selects 0.
- `spectreps rewrite -level 1` through `-level 5` use the pass-through writer, so text, fonts, and content Spectre cannot interpret are copied. Level 1 Flates uncompressed content streams. Level 2 also re-encodes Flate and raw image streams losslessly, with no resample. Levels 3 through 5 also re-encode images as DCT with a longest-side cap and a quality. The caps and qualities are the table in `documentation/devices.md`.
- `spectreps pdfimage` wraps each painted page in a new PDF as one image XObject, 8 bits per component, `/Filter /FlateDecode`. `-colorspace rgb|gray|cmyk` picks `/DeviceRGB` at 24 bits, `/DeviceGray` at 8 bits, or `/DeviceCMYK` at 32 bits, and defaults to `rgb`. `/MediaBox` comes from the pixel size and the paint dpi. A `.pdf` input paints the selected pages with `RasterizePage`; any other input uses `RunPostScript`. Bytes are stable, and the trailer `/ID` is the SHA-256 of the image streams.
- The bitmap PDF says nothing about `Do` on the reading side. Spectre still returns `undefined` for `Do`, so it cannot rasterize its own image PDF yet.

## Compare and validate

- `spectreps compare bytes` compares two files byte by byte and prints `mismatch byte N` or `mismatch length N` on a mismatch. Exit 0 when equal, exit 1 on a mismatch.
- `spectreps compare raster` rasterizes both inputs with one `RunOptions` value and compares the pixmaps with `CompareRaster`. A `.pdf` input opens with `OpenPDF` and `RasterizePage`, and `-pages` applies to both sides. It prints `mismatch pixel N` or `mismatch width` or `mismatch height`.
- `spectreps validate` runs the interpreter in stop-on-first-error mode. A bad xref, a bad stream, an encrypted file, or an unsupported operator fails the command with that error. It does not claim PDF/A conformance.

## Library and CLI

- Package `spectreps` exposes `New`, `Close`, `RunPostScript`, `OpenPDF`, `PageCount`, `RasterizePage`, `RewritePDF`, `ImagePDF`, `MeasureBox`, `MeasureInk`, `CompareFiles`, `CompareRaster`, and `Version`. The full contract is `documentation/public-api.md`.
- Every job takes a `context.Context`. A canceled context returns `ctx.Err()` with no partial success, and a nil context panics.
- The `spectreps` command parses flags and maps errors to exit codes 0 through 3. The contract is `documentation/cli.md`.
- `internal/cli` calls the public package. `cmd/spectreps` calls `internal/cli` only.

## Limits and safety

- Caps: operand stack 8192, execution stack 500, dictionary stack 20, `gsave` depth 32, procedure nesting 128, path points 100000, pixels per page 40000000, page side 20000 pixels. Crossing a cap returns `limitcheck`.
- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` are defined and return `invalidaccess`. A `%pipe%` path never runs.
- The module does not use cgo, does not link Ghostscript, and does not start `gs` or any other process. No operator opens a network connection.

## Deferred

| Feature | Why it waits | Next gate |
| --- | --- | --- |
| Painting `Do` and reading images into a raster | The reader decodes image XObjects, but the content interpreter still returns `undefined` for `Do`. | A plan file for the `Do` operator and the image marker seam. |
| CCITT and JPEG2000 image streams on rewrite | `DecodeImage` reads Flate and DCT only, so those streams copy through unchanged. | A CCITT or JPX decoder. |
| Text extraction, `show`, `Tj` | Fonts are a separate machine from the path engine. | A new plan file after the font decision. |
| PDF/A-1b, PDF/A-2b, PDF/A-3b creation | Needs a named level, a named policy, and metadata. The file is not a conformance certificate. | A plan file that states the level and the policy. |
| PDF to PostScript (`ps2write` style) | It is another high-level device on the same marks. | A plan file. |
| PCLm | A different image-PDF flavor. | A plan file. |
| Spot-color separations (`tiffsep`) | No separation model. | A plan file. |
| Full `gs` argv grammar | The subcommands map to library methods, and a second flag grammar would fork the CLI. The bounded switch map landed. | A written proposal per switch family. |
| `ink_cov` weighted ink amounts | Ghostscript prints `ink_cov` as a percent and its manual example disagrees with its source. | A written weighting model. |
| PDF info, linearization, output encryption | Out of the current tags. | A new plan file. |
| Full PDF 1.7 and PDF 2.0, including transparency and optional content | The current reader is a path-only subset. | A new plan file. |
| `bind`, `save`, `restore`, `clip`, and filters other than Flate | Out of the current PostScript subset. | A new plan file. |
| Font embedding and subsetting | Needs the font machine first. | A new plan file. |
| Printer languages: PCL, PXL, XPS, and the `gs -h` device list | GhostPCL, GhostXPS, and printer drivers are separate products from the PostScript and PDF interpreter. | A named device request opens a program plan. |
