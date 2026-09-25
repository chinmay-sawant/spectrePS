# Tests for covered jobs

These are the tests for the jobs in `documentation/covered-and-not-covered.md`. Each bullet is one case. The expected result is the contract in `documentation/language.md`, `documentation/devices.md`, `documentation/cli.md`, and `documentation/public-api.md`.

Tests call package `spectreps` or the `spectreps` binary. They do not run `/usr/bin/gs`. Fixtures are Spectre output, checked in under `testdata/` when the first raster or PDF case lands. PNG file bytes are not an equality oracle. The oracle is `PageImage`, or a PPM raw body when the header is part of the case.

External tests use `package spectreps_test`, so they only see the exported API.

## Library and CLI

- `New` returns a non-nil instance. `Close` returns nil. A second `Close` on the same instance returns nil.
- Two instances from two `New` calls both run. There is no process-wide singleton.
- `Version` is `0.0.2`. `spectreps version` prints it and exits 0.
- `cmd/spectreps` imports `internal/cli` only. `internal/cli` imports `github.com/chinmay-sawant/spectrePS/spectreps`. No Go file imports `os/exec` or uses cgo.
- Unknown command, unknown flag, and a missing input file exit 2.
- A missing input path that the program tries to read exits 3.
- `raster` and `rewrite` without `-o` exit 2, including while the job itself is still `ErrNotImplemented`.
- A cancelled context passed to `RunPostScript`, `OpenPDF`, `RasterizePage`, `RewritePDF`, or `ImagePDF` returns `ctx.Err()` and no partial success.

## PostScript subset

- Scanner accepts an int, a real, an executable name, a literal name, a `%` comment, a parenthesis string with `\n`, `\r`, `\t`, `\\`, `\(`, `\)`, and `\ddd`, and a hex string. An odd final hex nibble is padded with 0. An integer token outside int32 is `rangecheck`.
- `{ { 1 2 add } }` is one executable array whose only element is an executable array. Unmatched braces are `syntaxerror`.
- Top-level `1 2 add` leaves 3. `{ 1 2 add }` leaves a procedure. `{ 1 2 add } exec` leaves 3. `{ { 1 2 add } } exec` leaves the inner procedure and does not run `add`.
- A procedure built while a name has one definition, then the name is redefined, runs the new definition. Lookup happens at execution time.
- `div` pushes a real. Division by zero is `undefinedresult`. Integer overflow on `add` is `rangecheck`. `copy` with a non-integer top operand is `typecheck`.
- `eq` treats int 1 and real 1.0 as equal. `eq` on two distinct arrays with the same elements is false. `eq` on the same array object is true.
- Dictionary `forall` walks entries in insertion order.
- `def` into `systemdict` is `invalidaccess`. `def` into `userdict` succeeds. `end` when only `systemdict` remains is `dictstackunderflow`. `]` with no mark is `unmatchedmark`. `exit` outside a loop is `invalidexit`. `show` is `undefined`.
- Operand stack past 8192 is `stackoverflow`. Execution stack past 500, dictionary stack past 20, and procedure nesting past 128 are `limitcheck`.
- `for` with a zero increment is `rangecheck`.

## Raster

- Default options produce a 612 by 792 RGB image at 72 dpi. Stride is `Width * 3`.
- On a 200 by 200 point page at 72 dpi, `0 0 moveto 100 0 lineto stroke` paints the bottom row. `currentpoint` after `0 0 moveto` is 0, 0 in user space. Row 0 of the pixmap stays the top of the page.
- `stroke`, `fill`, and `eofill` write RGB pixels. `eofill` uses the even-odd rule.
- `showpage` appends one image and clears the path. Two `showpage` calls return two images.
- A program that paints nothing and never calls `showpage` returns one blank page. A program that paints and never calls `showpage` returns one image.
- `translate`, `scale`, `rotate`, and `concat` change where the same path lands. `gsave` then `grestore` restores the matrix and the path. `gsave` past depth 32 is `limitcheck`.
- A page above 40000000 pixels, or a side above 20000 pixels, returns `limitcheck` and does not allocate that pixmap. A letter page at 600 dpi is over the cap. A letter page at 300 dpi is under it.
- `spectreps raster -o out.ppm in.ps` writes a P6 file. The header is `P6\n{width} {height}\n255\n`. The body matches `PageImage.Pixels`.
- `spectreps raster -o out.png in.ps` writes a PNG that decodes to the same pixels as the PPM from the same program. The test compares decoded pixels, not the PNG bytes.
- `spectreps raster -o out.jpg in.ps` writes a JPEG that starts with the SOI bytes `FF D8` and decodes to the page geometry. `.jpeg` selects the same encoder; every other suffix falls back to PPM. The test decodes with `image/jpeg` and does not compare JPEG bytes.
- `-jpegq` defaults to 75 and is clamped to 1 through 100. `-jpegq 0` and `-jpegq 500` still write a decodable JPEG. `run` and `compare raster` reject `-jpegq` with exit 2.
- `spectreps raster` paints every page of a PDF input. A two-page fixture and an `-o` path with `%d` write two PPM files whose bodies match each page's marks.
- `-pages` takes `N` or `A-B`, 1-based inclusive. `A-` runs to the last page and `-B` starts at page 1, and an omitted flag selects every page. `raster`, `pdfimage`, `bbox`, `inkcov`, and `compare raster` accept it, and `run` accepts and ignores it. A malformed value exits 2. A start below 1 or past the last page exits 1 with `Error: /rangecheck in pages`. An end past the last page clamps to the last page.
- For PostScript, `-pages` filters after `RunPostScript`, so a failing page outside the range still fails the command.
- `spectreps raster -o out.tif in.ps` writes a TIFF that `tiff.Decode` reads back to the same pixels as the PPM from the same program. `.tiff` selects the same encoder, `.png`, `.jpg`, and `.jpeg` keep theirs, and every other suffix falls back to PPM. The test compares decoded pixels, not the TIFF bytes.
- `-tiffcompress none` writes uncompressed TIFF and `-tiffcompress deflate` writes Deflate strips. The default is `deflate`, so two runs with the same input and the default return equal TIFF bytes. An unknown value exits 2 and writes no file. `run` and `compare raster` reject `-tiffcompress` with exit 2.
- Two pages and an `-o` path with no `%d` exit 2. `%d` is the one-based number of the emitted page. A range that selects input pages 2 and 3 writes `page-1` and `page-2`.

## Box and ink coverage

- `MeasureBox` returns the union of marked pixel edges in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. Stride padding is ignored, and `dpi` of 0 or less selects 72.
- `MeasureBox` on a blank pixmap returns the zero `Box` and false.
- `MeasureInk` divides each marked channel count by `Width * Height`. A white page is `0 0 0`, a cyan page is `1 0 0`, and a red page is `0 1 1`. A zero-size image returns the zero `Ink`.
- `spectreps bbox` prints `%%BoundingBox` with the floored minima and ceilinged maxima, then `%%HiResBoundingBox` with `strconv.FormatFloat(v, 'f', -1, 64)` edges. A page with no marked pixel prints `%%BoundingBox: 0 0 0 0` and `%%HiResBoundingBox: 0 0 0 0`. stdout, exit 0.
- `spectreps inkcov` prints `Page N` and three five-decimal RGB occupancy fractions ending in `RGB`. The line is not `CMYK OK`.
- Both commands rasterize every page of a PDF input, not only page 0. A missing input exits 2.
- Both commands apply `-pages` before printing. `inkcov` numbers emitted pages from 1, so selecting input page 2 prints `Page 1`.

## Pixel compare

- `CompareRaster` on two equal images sets `Equal` true, `Offset` -1, and an empty `Reason`.
- Different `Width` sets `Reason` `width` and `Offset` -1. Different `Height` with equal width sets `Reason` `height`.
- The first differing RGB byte sets `Reason` `pixel` and `Offset` to that byte index in row-major order. Bytes in the stride padding are ignored.
- `spectreps compare raster` uses one `RunOptions` value and one `-pages` selection for both files. A `.pdf` input opens with `OpenPDF` and paints each selected page with `RasterizePage`; any other input uses `RunPostScript`. Equal pixels exit 0. A mismatch exits 1 and prints `mismatch pixel N` or `mismatch width` on stdout. Different selected page counts print `mismatch length` and exit 1. stderr is empty.
- The compare command does not start `gs`.

## PDF open and rasterize

- A fixture with a `%PDF-` header, a classic xref, and a Flate content stream opens. The page count is the page tree length.
- A fixture that uses an xref stream and a Flate object stream opens, and the page count is right.
- Content operators `m l c h re S s f f* n q Q cm w RG rg g G` paint through the same device as the PostScript path operators. A one-page path PDF and the PostScript program of the same marks compare equal with `CompareRaster`.
- `Tj`, `TJ`, `'`, `"`, and `Do` each return `JobError` with the operator name filled in. The page is not a blank success.
- An encrypted file returns `invalidaccess`. An unknown stream filter returns `undefined`. A truncated xref returns `JobError`.
- `RasterizePage` with a negative index, or an index past the last page, returns `rangecheck`.
- `spectreps raster -o out.ppm in.pdf` writes the P6 file for a path-only fixture.

## PDF rewrite

- `RewritePDF` on a document this module can rasterize returns a PDF. Opening that PDF and rasterizing page 0 matches `RasterizePage` of the input, via `CompareRaster`.
- `DefaultRewriteOptions` selects level 0 and Flate-compresses page content streams. `CompressStreams` false leaves those streams uncompressed. Both outputs still match the input pixels.
- `RewriteOptions.Level` is 0 by default. Level 0 re-emits the path subset. Levels 1 through 5 use the pass-through writer, so text, fonts, and the page tree are copied and the page count and boxes match the input. A level outside 0 through 5 returns `rangecheck` from `RewritePDF` and exits 2 from the CLI.
- A source `/Type /XRef` or `/Type /ObjStm` container is not copied: its object number gets a free xref row, the `/ID` covers only the written bodies, and the page count and boxes hold. `CopyOptions.PackObjects` writes the optional packed form, `%PDF-1.5` with one Flate `/Type /ObjStm` for non-stream bodies and a Flate `/Type /XRef` stream. The packed and classic forms carry the same `/ID`, and two calls in either form are equal.
- Level 1 Flates every uncompressed content stream. Level 2 also re-encodes Flate and raw image streams losslessly with no resample. Levels 3 through 5 also re-encode images as DCT. The longest-side caps are 1754, 1123, and 842 pixels, at qualities 80, 60, and 40. An image at or below the cap keeps its size. An image Spectre cannot decode, and an image with an `/SMask`, is copied unchanged.
- Two `RewritePDF` calls at any level on the same input return buffers `CompareFiles` reports equal. The output contains no `CreationDate` or `ModDate`.
- The rewritten bytes are not required to equal the input bytes, and they are not compared with Ghostscript `pdfwrite`.
- `spectreps rewrite` without `-o` exits 2. `-compress=false` selects the uncompressed level 0 option. `-level 1` through `-level 5` succeed on a text stream that level 0 rejects with `undefined`. A successful rewrite exits 0.
- `sampledata/compress/whatisthis.pdf` and `sampledata/compress/path.pdf` rewrite at every level with the input page count. The JPEG image decodes, the caps hold, and level 5 is the smallest of the five. The test skips when `sampledata/` is absent.

## Bitmap PDF

- `ImagePDF` and `WriteImages` emit one `/Subtype /Image` XObject per page with `/ColorSpace /DeviceRGB`, `/BitsPerComponent 8`, and `/Filter /FlateDecode`. The stored stream is the tightly packed RGB rows, so stride padding is dropped.
- `ImagePDFColor` and `WriteImagesColor` keep those bytes for `ImageColorRGB` and `ImageRGB`, and add `/DeviceGray` with one byte per pixel and `/DeviceCMYK` with four. The test decodes both streams and compares each with the conversion below.
- Gray is `round(0.299*R + 0.587*G + 0.114*B)` per pixel. Pure red `(255, 0, 0)` decodes to the byte `76`.
- CMYK is `K = 1 - max(r, g, b)` with `C = (1 - r - K) / (1 - K)` and the same for M and Y, each rounded to a byte, and `C = M = Y = 0` when `K >= 1`. Pure red decodes to `0 255 255 0`.
- `spectreps pdfimage -colorspace gray|cmyk` writes those streams, `-colorspace rgb` is the same as the default, and any other value exits 2.
- `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]` points. `dpi` of 0 or less selects 72.
- The content stream paints `/Im0 Do` and the page resources carry the XObject. `Do` still returns `undefined` when Spectre opens the file, so the test decodes the image stream from the bytes instead of rasterizing the output.
- Two `ImagePDF` calls on the same pages return buffers `CompareFiles` reports equal. The bytes contain no `CreationDate`, `ModDate`, or `/Info`. Both trailer `/ID` strings are the SHA-256 of the concatenated image streams.
- `spectreps pdfimage` without `-o` exits 2. A `.pdf` input rasterizes every selected page with `RasterizePage`; any other input uses `RunPostScript`. `-pages` picks the pages either way, and the output has one page per selected input page. The output file is mode `0o600`.
- `pdfimage` is not `pdfwrite` and it does not DCT-encode.

## Validate

- `spectreps validate` on a subset PostScript program exits 0.
- `spectreps validate` on a PostScript `stackunderflow` exits 1. stderr is one line, `Error: /stackunderflow in add`. The `at file:line:col` tail is omitted because this subset does not record a source position.
- `spectreps validate` on a phase-06 PDF fixture exits 0. A truncated xref exits 1 with `JobError`. An encrypted file exits 1 with `invalidaccess`.
- `validate` does not write an output file and does not write PDF/A metadata.

## File access ban

- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` are defined and return `invalidaccess`.
- `spectreps validate` on a program that calls `deletefile` exits 1 with `invalidaccess`. A temp file created next to the input is still there after the command.
- A string path starting with `%pipe%` never runs, because `file` returns `invalidaccess` before it reads the path.
- No test, and no operator under test, opens a network connection.

## File byte compare

These cases belong to the library and the `compare bytes` command. Ghostscript has no compare command. Spectre does.

- Two equal slices, including two empty slices, set `Equal` true, `Offset` -1, and an empty `Reason`.
- The first differing byte sets `Reason` `byte` and `Offset` to that index.
- When one slice is a prefix of the other, `Offset` is the shorter length and `Reason` is `length`.
- `spectreps compare bytes` exits 0 for equal files and 1 for a mismatch, printing `mismatch byte N` on stdout.
