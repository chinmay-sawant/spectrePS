# Features

This file is the product inventory: what Spectre supports today and what waits. `documentation/covered-and-not-covered.md` maps the same ground against Ghostscript. `plans/v0.0.1/10-deferred.md` is the ledger for the work that waits, with a next gate on each row.

The released tag is v0.0.3. v0.0.2 added the page summaries, JPEG and TIFF raster, page selection, PDF inputs in `compare raster`, gray and CMYK image PDF, and PDF compression levels 1 to 5. v0.0.3 adds weighted `ink_cov`, CCITT and JPEG2000 decode, painting `Do`, `spectreps ps`, the `spectreps gs` argv mode, the PDF/A-4 and PDF/UA-2 preflights, and the font and text machine with `spectreps text`. `spectreps version` prints `0.0.3`.

## Input

- PostScript source over the subset in `documentation/language.md`. Tokens, three stacks, procedures, dictionaries, arrays, strings, control flow, math, matrix operators, and the path and paint operators all run.
- PDF path content. Classic xref tables and xref streams open. Object streams supply objects a type 2 xref row names. Content streams decode through Flate, LZW, ASCII85, ASCIIHex, and RunLength, with predictors 2 and 10 through 15, and the page content operators `m l c h re S s f f* n q Q cm w RG rg g G` paint through the same graphics engine as PostScript. `cm` composes a six-number matrix into the CTM, path points transform through it before the device scale, and the stroke width scales by it. `q` and `Q` save and restore it.
- PDF image XObjects paint through `Do`. The page's `/XObject` resources resolve, `/Resources` inherits from a `/Pages` ancestor, and an image decodes once per name per page. The image unit square maps through the CTM and the device scale, and the pixmap samples nearest neighbor with image row 0 at the top. An `/SMask` image is refused, not painted opaque. A missing name, a non-image subtype, and a decode error return `undefined` with the `Do` operator name.
- A PostScript header such as `%!PS-Adobe-3.0` is optional. It scans as a comment.
- An encrypted PDF returns `invalidaccess`. A PDF with an unknown stream filter returns `undefined`. A page that uses an operator outside the subset above fails with that operator name. The page is not a blank success.
- The page count is the number of page leaves in the tree, not the trailer `/Count`.

## Text and fonts

- The PDF text operators `BT`, `ET`, `Tf`, `Td`, `TD`, `Tm`, `T*`, `Tc`, `Tw`, `Tz`, `TL`, `Ts`, `Tj`, `TJ`, `'`, and `"` run. The text state follows ISO 32000-1, `q` and `Q` save it, and `BT` resets the text matrices.
- A simple font reads `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, and `/BaseFont`, with the standard 14 metrics as the fallback when `/Widths` is absent. `/Encoding` names StandardEncoding, WinAnsiEncoding, or MacRomanEncoding, `/Differences` overrides codes by name, and a `/ToUnicode` CMap wins for extraction. A Type0 font reads Identity-H, a CIDFontType2 descendant, `/CIDToGIDMap`, `/W`, and `/DW`.
- Embedded `/FontFile2` and OpenType `/FontFile3` programs paint through `golang.org/x/image/font/sfnt` and `x/image/vector`. A standard 14 font, a Type 1 `/FontFile`, and a bare CFF stream have no outline program in this tag, so painting one of their glyphs returns `invalidfont`. Advances and extraction still work.
- `spectreps text file.pdf` prints the extracted text of the selected pages to stdout. Lines run top to bottom, glyphs on one line run left to right, a gap wider than a quarter box inserts a space, each line ends with CRLF, and a font with neither `/ToUnicode` nor a named encoding falls back to the code point.
- Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. Text tests compare shapes and advances, and extraction tests compare text and geometry, never raster bytes against `gs`.
- PostScript `findfont`, `scalefont`, `setfont`, and `show` resolve the standard 14 names and advance the current point. A device run returns `invalidfont`, the same no-outline policy.

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
- `spectreps ink_cov` prints `Page N` and three weighted RGB amounts as percentages with five digits after the point. The amount is the mean channel complement over `Width * Height`, the model in `documentation/devices.md`. The channels are the pixmap channels, so the line ends in `RGB`.
- All three commands accept `-w`, `-h`, and `-r` and share the `-pages` range. They rasterize a PDF input one selected page at a time.
- `spectreps info file.pdf` reads the document and writes a summary to stdout: the PDF version, the page count and each page `/MediaBox` in points, the tagged flag, fonts with an embedded flag, and the image count. It writes no file. An encrypted trailer is refused with `Error: /invalidaccess in Encrypt`. The line shape is in `documentation/cli.md`.

## PDF output

- `spectreps rewrite -level 0` writes a new PDF from a path-only PDF. Content streams carry the same path subset. `-compress` selects Flate content streams and defaults to true. Bytes are stable across two calls, and the file carries no wall-clock date. A missing `-level` selects 0.
- `spectreps rewrite -level 1` through `-level 5` use the pass-through writer, so text, fonts, and content Spectre cannot interpret are copied. A source `/Type /XRef` or `/Type /ObjStm` container is not copied: its number gets a free xref row and the trailer `/ID` covers only the written bodies. The writer option `CopyOptions.PackObjects` packs non-stream bodies into a Flate object stream with a `/Type /XRef` stream and a `%PDF-1.5` header. Level 1 Flates uncompressed content streams. Level 2 also re-encodes Flate, raw, LZW, and CCITT image streams losslessly, with no resample. Levels 3 through 5 also decode Flate, DCT, CCITT, JPEG2000, and LZW images and re-encode them as DCT with a longest-side cap and a quality. A CCITT stream decodes as Group 4 (`/K < 0`) or Group 3 (`/K == 0` with `/EndOfLine true`); `/K > 0` and any image Spectre cannot decode, and an image with an `/SMask`, is copied unchanged. The caps and qualities are the table in `documentation/devices.md`.
- `spectreps ps -o out.ps in.pdf` writes one date-free PostScript program from a path-only PDF. The marks are the same as `rewrite -level 0`, in 72 dpi points, with a fixed 612 by 792 box and one `showpage` per page. The program defines the short path names in a prolog, so it runs under `RunPostScript` and any PostScript interpreter. Two runs return equal bytes. Text and images are not emitted, so a page with `Tj` exits 1.
- A PDF/UA-2 input keeps its structure at levels 1 through 5. The pass-through writer copies `/StructTreeRoot`, the parent tree, MCIDs, `/Alt`, `/ActualText`, and `/Lang`, and a PDF 2.0 input keeps its header block, binary marker included, so it does not leave as a 1.4 shell. `-level 0` and `spectreps pdfimage` refuse a tagged input with `Error: /tagged` instead, because neither writer can keep the tree. The metadata reader and writer cover the catalog `/Metadata` packet with `pdfuaid` and `dc:title`, plus `/Lang`, `/MarkInfo`, and `/ViewerPreferences`. A `pdfuaid` claim is never written from nothing: it is kept when the source carried it, and added only when a caller opted in after a passing preflight. The claim for this work is preflight only, and it is not a certification.
- `spectreps pdfimage` wraps each painted page in a new PDF as one image XObject, 8 bits per component, `/Filter /FlateDecode`. `-colorspace rgb|gray|cmyk` picks `/DeviceRGB` at 24 bits, `/DeviceGray` at 8 bits, or `/DeviceCMYK` at 32 bits, and defaults to `rgb`. `/MediaBox` comes from the pixel size and the paint dpi. A `.pdf` input paints the selected pages with `RasterizePage`; any other input uses `RunPostScript`. Bytes are stable, and the trailer `/ID` is the SHA-256 of the image streams.
- The bitmap PDF round-trips. `Do` decodes the image and `RasterizePage` of the reopened file matches the source page under `CompareRaster`, for RGB and gray.
- `spectreps rewrite -pdfa 4|4f` writes a pass-through rewrite with a PDF/A-4 claim. The header becomes `%PDF-2.0` with a binary marker, the catalog gains `/Metadata` and `/OutputIntents`, and the XMP packet carries `pdfaid:part` 4, `pdfaid:rev` 2020, and the `F` letter for 4f. `-pdfa 4` is the base claim and `-pdfa 4f` is the embedded-file claim. The command runs the profile preflight first and refuses a known violation with exit 1 and `Error: /rule in PDFA`. The claim is a profile preflight, not a certificate.

## Compare and validate

- `spectreps compare bytes` compares two files byte by byte and prints `mismatch byte N` or `mismatch length N` on a mismatch. Exit 0 when equal, exit 1 on a mismatch.
- `spectreps compare raster` rasterizes both inputs with one `RunOptions` value and compares the pixmaps with `CompareRaster`. A `.pdf` input opens with `OpenPDF` and `RasterizePage`, and `-pages` applies to both sides. It prints `mismatch pixel N` or `mismatch width` or `mismatch height`.
- `spectreps validate` runs the interpreter in stop-on-first-error mode. A bad xref, a bad stream, an encrypted file, or an unsupported operator fails the command with that error. It does not claim PDF/A conformance. The PDF/UA-2 machine checks are `internal/pdfa.PreflightUA2`, a separate request from the PDF/A preflight. `validate` does not call it yet.
- `PreflightUA2` checks `/MarkInfo /Marked true`, `/StructTreeRoot`, one `Document` in the PDF 2.0 namespace, `/Lang` syntax, `/ViewerPreferences /DisplayDocTitle`, the `pdfuaid` values, `dc:title`, role-map resolution, and MCID coverage. A refusal is `Error: /ua2-<rule> in PDFUA`. The result is preflight only, never certification. The rule list is in `documentation/devices.md`.

## Library and CLI

- Package `spectreps` exposes `New`, `Close`, `RunPostScript`, `OpenPDF`, `PageCount`, `RasterizePage`, `ExtractText`, `RewritePDF`, `ImagePDF`, `MeasureBox`, `MeasureInk`, `MeasureInkAmount`, `CompareFiles`, `CompareRaster`, and `Version`. The full contract is `documentation/public-api.md`.
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
| PDF/UA-2 tag generation | Reading order and role assignment are not implemented. Preservation and preflight landed in v0.0.3, and the font and text machine now exist. | `plans/v0.0.4/3-pdfua2-tags.md`. |
| PCLm | A different image-PDF flavor. | A plan file. |
| Spot-color separations (`tiffsep`) | No separation model. | `plans/v0.0.4/4-pdf-coverage.md`. |
| Linearization and output encryption | Out of the current tags. Encryption and linearization need their own plan. | A new plan file. |
| Full PDF 1.7 and PDF 2.0, including transparency and optional content | The reader is a subset of ISO 32000-2; encryption and color management are out. | `plans/v0.0.4/4-pdf-coverage.md`. |
| `bind`, `save`, `restore`, `clip`, and PostScript filters | The PostScript operators stay out. The PDF stream filters (Flate, LZW, ASCII85, ASCIIHex, RunLength, and predictors 2 and 10 through 15) landed in v0.0.4. | `plans/v0.0.4/4-pdf-coverage.md`. |
| Font embedding and subsetting | The font machine reads metrics and embedded programs; writing or subsetting a font is a separate job. | `plans/v0.0.4/4-pdf-coverage.md`. |
| Printer languages: PCL, PXL, XPS, and the `gs -h` device list | GhostPCL, GhostXPS, and printer drivers are separate products from the PostScript and PDF interpreter. | A named device request opens a program plan. |
