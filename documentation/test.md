# Tests for covered jobs

These are the tests for the jobs in `documentation/covered-and-not-covered.md`. Each bullet is one case. The expected result is the contract in `documentation/language.md`, `documentation/devices.md`, `documentation/cli.md`, and `documentation/public-api.md`.

Tests call package `spectreps` or the `spectreps` binary. They do not run `/usr/bin/gs`. Fixtures are Spectre output or hand-built, checked in under `sampledata/fixtures/`. PNG file bytes are not an equality oracle. The oracle is `PageImage`, or a PPM raw body when the header is part of the case. Text extraction is not a raster test either: the oracle is the text and the geometry, because text pixels never byte-match Ghostscript.

External tests use `package spectreps_test`, so they only see the exported API.

## Library and CLI

- `New` returns a non-nil instance. `Close` returns nil. A second `Close` on the same instance returns nil.
- Two instances from two `New` calls both run. There is no process-wide singleton.
- `Version` is `0.0.3`. `spectreps version` prints it and exits 0.
- `cmd/spectreps` imports `internal/cli` only. `internal/cli` imports `github.com/chinmay-sawant/spectrePS/spectreps`. No Go file imports `os/exec` or uses cgo.
- Unknown command, unknown flag, and a missing input file exit 2.
- A missing input path that the program tries to read exits 3.
- `raster` and `rewrite` without `-o` exit 2.
- A cancelled context passed to `RunPostScript`, `OpenPDF`, `RasterizePage`, `ExtractText`, `RewritePDF`, `ImagePDF`, or `ImagePDFColor` returns `ctx.Err()` and no partial success. Every job panics on a nil context with the package message.

## PostScript subset

- Scanner accepts an int, a real, an executable name, a literal name, a `%` comment, a parenthesis string with `\n`, `\r`, `\t`, `\\`, `\(`, `\)`, and `\ddd`, and a hex string. An odd final hex nibble is padded with 0. An integer token outside int32 is `rangecheck`.
- `{ { 1 2 add } }` is one executable array whose only element is an executable array. Unmatched braces are `syntaxerror`.
- Top-level `1 2 add` leaves 3. `{ 1 2 add }` leaves a procedure. `{ 1 2 add } exec` leaves 3. `{ { 1 2 add } } exec` leaves the inner procedure and does not run `add`.
- A procedure built while a name has one definition, then the name is redefined, runs the new definition. Lookup happens at execution time.
- `div` pushes a real. Division by zero is `undefinedresult`. Integer overflow on `add` is `rangecheck`. `copy` with a non-integer top operand is `typecheck`.
- `eq` treats int 1 and real 1.0 as equal. `eq` on two distinct arrays with the same elements is false. `eq` on the same array object is true.
- Dictionary `forall` walks entries in insertion order.
- `def` into `systemdict` is `invalidaccess`. `def` into `userdict` succeeds. `end` when only `systemdict` remains is `dictstackunderflow`. `]` with no mark is `unmatchedmark`. `exit` outside a loop is `invalidexit`. `save` is `undefined`.
- Operand stack past 8192 is `stackoverflow`. Execution stack past 500, dictionary stack past 20, and procedure nesting past 128 are `limitcheck`.
- `for` with a zero increment is `rangecheck`.

## Raster

- Default options produce a 612 by 792 RGB image at 72 dpi. Stride is `Width * 3`.
- On a 200 by 200 point page at 72 dpi, `0 0 moveto 100 0 lineto stroke` paints the bottom row. `currentpoint` after `0 0 moveto` is 0, 0 in user space. Row 0 of the pixmap stays the top of the page.
- `stroke`, `fill`, and `eofill` write RGB pixels. `eofill` uses the even-odd rule.
- `showpage` appends one image and clears the path. Two `showpage` calls return two images.
- A program that paints nothing and never calls `showpage` returns one blank page. A program that paints and never calls `showpage` returns one image.
- `translate`, `scale`, `rotate`, and `concat` change where the same path lands. `gsave` then `grestore` restores the matrix and the path. `gsave` past depth 32 is `limitcheck`.
- A page above 40000000 pixels, or a side above 20000 pixels, returns `limitcheck` and does not allocate that pixmap. A letter page at 600 dpi is 5100 by 6600, which is 33,660,000 pixels, under the cap. The area cap is crossed at 655 dpi. A letter page at 300 dpi is under it.
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

## Alpha and blend

- The pixmap implements the optional `graphics.AlphaMarker` seam. A fill composites with the fill alpha and a stroke with the stroke alpha: `out = src*a + dst*(1-a)` per channel, rounded. Alpha 0 leaves the pixel, alpha 1 with `BlendNormal` replaces it, and an out-of-range alpha clamps to 0 or 1. Fill alpha does not leak into a stroke.
- The 12 separable blend modes paint with the ISO 32000-1 formulas. Over a backdrop of byte 200 and a source of byte 100 the exact bytes are Normal 100, Multiply 78, Screen 222, Overlay 188, Darken 100, Lighten 200, ColorDodge 255, ColorBurn 115, HardLight 157, SoftLight 191, Difference 100, and Exclusion 143. An unknown mode composites as Normal, and Multiply with fill alpha 0.5 over the same backdrop is 139.
- `blendModeName` in `internal/pdf` maps the 12 separable names and returns false for Hue, Saturation, Color, and Luminosity, so the PDF layer refuses those four by name.
- `gs` sets `/CA` and `/ca` (clamped to 0 through 1 with `typecheck` on a non-number), `/BM` through `blendModeName`, and `/SMask`. A `/BM` array, an unknown name, and the four non-separable names return `undefined in gs`. `q`/`Q` restore the alpha and the blend mode, so a mark after `Q` paints with the state from before `q`. An unknown `/ExtGState` name is `undefined in gs`.
- An `/ExtGState` `/SMask` with `/S /Alpha` renders its `/G` Form XObject into a scratch page and applies the coverage to later marks; `/S /Luminosity` uses `0.30*R + 0.59*G + 0.11*B` times the coverage. `/TR /Identity` is accepted and any other transfer function refuses. `/None` clears the mask. A `/G` form that does not validate refuses with the form error, and a marker without `SoftMaskMarker` refuses with `undefined in gs`.
- A Form XObject with `/Group /S /Transparency` renders into a transparent scratch page the size of the device and composites once. `/CA 0.5` halves the coverage, so red over white is `255 128 128`; `/CA 1` is exact red. Pixels the group never paints keep the page color, so a small mark in a group does not blank the page. `/CS` resolves, `/I` and `/K` must be booleans, and any other key refuses with `undefined in Do`. The scratch obeys the 40,000,000 pixel and 20,000 side caps with `limitcheck`.
- The rewrite recorder implements neither `AlphaMarker` nor `SoftMaskMarker`, so `Emit` keeps refusing `/ca`, `/CA`, `/BM`, and `/SMask` with `undefined in gs` instead of dropping the effect.

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
- `spectreps compare raster` uses one `RunOptions` value and one `-pages` selection for both files. A `.pdf` input opens with `OpenPDF` and paints each selected page with `RasterizePage`; any other input uses `RunPostScript`. Equal pixels exit 0. A mismatch exits 1 and prints `mismatch pixel N`. Different selected page counts print `mismatch length` and exit 1. Both inputs share the geometry, so `mismatch width` and `mismatch height` are library-only reasons and never reach the CLI. stderr is empty.
- The compare command does not start `gs`.

## PDF open and rasterize

- A fixture with a `%PDF-` header, a classic xref, and a Flate content stream opens. The page count is the page tree length.
- A fixture that uses an xref stream and a Flate object stream opens, and the page count is right.
- Content operators `m l c h re S s f f* n q Q cm w RG rg g G` paint through the same device as the PostScript path operators. A one-page path PDF and the PostScript program of the same marks compare equal with `CompareRaster`.
- `Do` resolves a name in the page's `/XObject` resources, requires `/Subtype /Image`, decodes the image once per name, and stamps it into the unit square through the CTM and the paint scale. `/Resources` on a `/Pages` ancestor is inherited. A missing name, a non-image subtype, and a decode error each return `JobError` with `Do` and `undefined`. The rejected page is not a blank success.
- An image `/SMask` decodes to an alpha plane and blends the base image through `DrawImage`, so a red pixel at alpha 1/3 over white is `170 255 170`. `/Matte` is accepted and changes no pixel. A mask `/Decode [1 0]` inverts the alpha. A color-key `/Mask [min max ...]` keys out a pixel whose samples fall inside every range, and a stream `/Mask` becomes a stencil. An `/ImageMask true` image decodes one bit per sample, applies `/Decode`, and paints the current fill color where the mask is 1; a bit depth other than 1 refuses with `undefined in Do`. A `/Decode` array on a gray or RGB Flate image remaps before the preview: gray `[1 0]` turns `0 85 170 255` into `255 170 85 0`. A malformed decode array and a decode array on an Indexed image are `undefined in Image`.
- `Do` on a `/Subtype /Form` XObject runs the form content with `/Matrix` concatenated into the CTM, `/BBox` as a clip on the form's marks, and the form's `/Resources` or the inherited page resources. The form runs inside an implicit `q`/`Q`. Nested forms stop at depth 32 with `limitcheck in Do`; a form with no stream, no `/BBox`, a malformed `/Matrix` or `/BBox`, or a `FormType` other than 1 is `undefined in Do`.
- `W` and `W*` intersect the clip applied to later fills, strokes, images, and glyphs. `W*` uses the even-odd rule, and `q`/`Q` save and restore the clip. A marker without the optional clip seam refuses `W` with `undefined in W`.
- `B`, `B*`, `b`, and `b*` fill and stroke one path, even-odd for the starred forms and a close for `b` and `b*`. `b` and `b*` with no current subpath return `nocurrentpoint`.
- `gs` resolves a name in `/Resources /ExtGState`, applies `/LW`, `/CA`, `/ca`, `/BM`, and `/SMask`, and restores them on `Q`. An unknown name and an unsupported entry both return `undefined in gs`, so the state is never partly applied, and a non-number `/LW` or `/CA` is `typecheck`.
- `K` and `k` select DeviceCMYK and paint through the frozen preview: `0 0 0 0.5 k` fills byte 128 gray, `0 1 0 0 k` fills magenta, and `1 0 0 0 K` strokes cyan. `CS`/`cs`, `SC`/`sc`, and `SCN`/`scn` select a device or `/Resources /ColorSpace` space and its components, the space and components survive `q`/`Q`, and a trailing pattern name on `SCN`/`scn` returns `undefined`. An unknown space name is `undefined in cs`, and a non-number component is `typecheck`.
- A page whose `/ColorSpace` resource is a Separation or DeviceN space rasterizes through the preview: `0.5 sc` over a Separation to DeviceCMYK tint of 0.5 paints byte 128 gray, and a DeviceN over DeviceCMYK paints black at K 1.
- `J`, `j`, `M`, `i`, and `ri` are accepted as no-ops and leave the capsule stroke pixels equal to the same content without them. A bad operand still returns `typecheck` or `stackunderflow`.
- `BX` skips content through the matching `EX`, including a nested `BX`, and an `EX` outside a section is ignored. The end of a stream inside a section is `syntaxerror in content`.
- `BMC`, `BDC`, `EMC`, `MP`, and `DP` parse and track nesting. A `BDC` or `DP` dictionary operand and a `/Properties` name operand both resolve, the cap is 64 with `limitcheck`, and an unmatched `EMC` is `syntaxerror in content`. A marked page rasterizes as if the markers were absent, and `sampledata/pdfua2/tagged-ua2.pdf` rasterizes and extracts with no error.
- `PaintOptions.MarkedContent` pairs `BeginMarkedContent` and `EndMarkedContent` in stream order at the same depth and carries the resolved properties, and `MP` and `DP` fire no event. `TextOptions.Runs` fires one `TextRun` per `Tj`, `TJ`, `'`, and `"`, with the TJ string elements concatenated and the numbers omitted. `ImageNameMarker` fires with the resource name and the XObject dictionary before decode. A nil or typed-nil sink changes nothing.
- Optional content reads `/OCProperties /D`. Content under a group listed in `/OFF` is skipped, with the marked-content frame and its sink events still paired; a group not in `/OFF` paints. A `/Properties` name, an inline dictionary with `/OC`, and an inline membership dictionary with `/OCGs` all resolve. The `/P` policy maps `/AnyOn` (the default) to hidden when every member is off, `/AllOn` to hidden when any member is off, `/AllOff` to hidden when any member is on, and `/AnyOff` to hidden when every member is on. An image XObject whose `/OC` names an OFF group is skipped before decode and paints when the group is visible.
- An image `/ColorSpace` resolves to a sample count and a preview RGB conversion for DeviceRGB, DeviceGray, DeviceCMYK, Indexed over a string or stream table, ICCBased through `/Alternate` or `/N`, CalRGB, CalGray, Separation, and DeviceN. DeviceCMYK previews with `r = (1-C)(1-K)`, and the writer's K-first RGB to CMYK rule is its inverse. A CMYK Flate image, an Indexed Flate image, and a Separation Flate image decode to exact preview bytes. An index past hival clamps to hival, and a lookup shorter than `(hival+1) * base components` is `undefined`.
- The tint transform evaluator runs type 2 exponential interpolation and type 4 PostScript calculator functions, including arithmetic, comparison, `if`/`ifelse`, and the stack operators. A type 0 sampled or type 3 stitching function returns `undefined`, and a malformed calculator program returns `undefined` at resolve time. A type 4 code stream decodes under the 32 MiB cap.
- A DCT source must match the declared sample count: a CMYK space needs an `*image.CMYK` source, and a gray or Indexed space needs an `*image.Gray` source. A mismatch is `undefined`. Go's `image/jpeg` encoder writes no CMYK stream, so the CMYK DCT preview is checked at the conversion seam and the CMYK Flate image is checked end to end.
- An unsupported image color space returns `undefined` with the `Image` operator from `DecodeImage` and with `Do` from `DecodeImageValueOp`.
- An encrypted file returns `invalidaccess`. An unknown stream filter returns `undefined`. A truncated xref returns `JobError`.
- `LZWDecode` decodes a TIFF LZW strip, which pins the early change width switch, and round trips `/EarlyChange` 0 and 1 against a test encoder. Malformed data returns `syntaxerror` with the `LZWDecode` op and a stream past 32 MiB returns `limitcheck`.
- `ASCII85Decode` decodes a standard-library ASCII85 body, including the `z` shortcut, partial final groups, a leading `<~`, and ignored whitespace. A character outside `!` through `u`, a one-character final group, and a group above 2^32 - 1 return `syntaxerror`.
- `ASCIIHexDecode` decodes uppercase, lowercase, whitespace, the `>` end marker, and an odd final digit. A non-hex character returns `syntaxerror`.
- `RunLengthDecode` decodes literal runs, repeat runs, and the 128 end marker. A truncated run returns `syntaxerror`, and output past 32 MiB returns `limitcheck`.
- Predictors 2 and 10 through 15 apply on Flate for xref streams, object streams, content streams, and image streams. `/Predictor 12` and `/Predictor 15` both read the per-row tag byte. A predictor outside 1, 2, and 10 through 15 returns `undefined` with the `Predictor` op; a short row, a bad tag, and a bad `/BitsPerComponent` return `syntaxerror`.
- A `/Filter` chain decodes each stage in order with its own `/DecodeParms` entry. An unknown stage returns `undefined` with that filter name.
- Level 2 re-encodes an LZW image as Flate RGB, levels 3 through 5 as DCT, and an undecodable LZW stream copies through at every level. An LZW image decodes through `DecodeLZWImageValue`, because `DecodeImage` keeps its four-form table.
- `RasterizePage` with a negative index, or an index past the last page, returns `rangecheck`.
- `spectreps raster -o out.ppm in.pdf` writes the P6 file for a path-only fixture.

## PDF info

- `spectreps info file.pdf` prints `PDF version`, `Pages`, one `Page N: width x height` line per page, `Tagged`, a `Fonts:` block or `Fonts: none`, and `Images`. Page sizes resolve `/MediaBox` through the page tree and default to 612 by 792 points.
- The font block lists every in-use `/Type /Font` dictionary except CIDFont descendants, sorted by name, with `embedded=true` when `/FontDescriptor` carries `/FontFile`, `/FontFile2`, or `/FontFile3`. A Type 3 font counts as embedded, and a Type0 font needs every descendant to carry a program.
- A trailer with `/Encrypt` is refused at open time with `Error: /invalidaccess in Encrypt`. A malformed `/MediaBox` fails with `Error: /syntaxerror in Info`.
- `spectreps info` exits 0 on a clean read, 1 on a job error, 2 on a missing input or a bad flag, and 3 on an unreadable path. The command writes no file.

## Text and extraction

- `BT`, `ET`, `Tf`, `Td`, `TD`, `Tm`, `T*`, `Tc`, `Tw`, `Tz`, `TL`, and `Ts` maintain the text state. `q` and `Q` save and restore it, and `BT` resets the text matrices. `Q` does not restore the matrices, which are not part of the graphics state.
- A simple font reads `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, and `/BaseFont`, with the standard 14 metrics as the fallback when `/Widths` is absent. `/Encoding` with `/Differences` renames codes, and a `bfchar` or `bfrange` CMap overrides the encoding and the glyph list. A simple `/Subtype /Type1` font reads a PFA or PFB `/FontFile`, prefers its `/Length1-3`, resolves a code through the PDF encoding and then the built-in encoding of a symbolic font, and falls back to the charstring `hsbw` width when `/Widths` is absent. `/MMType1` keeps its PDF widths and paints `invalidfont`.
- A `/FontFile2` or OpenType `/FontFile3` program maps a code through the program's glyph names and then its cmap. `/Widths` wins for the advance and the program advance is the fallback. A Type0 Identity-H font maps two-byte codes through `/CIDToGIDMap`, with `/W` and `/DW` for the advances. The embedded-font fixtures are synthetic programs built in the test helper, so no third-party font bytes are checked in.
- `Tj`, `TJ`, `'`, and `"` deliver each positioned glyph to the sink with its code, Unicode, advance, and device box. TJ numbers, `'`, and `"` move the next glyph with the widths and the character and word spacing.
- Painting a standard 14 glyph or a bare CFF stream returns `invalidfont` and leaves the page white, while the sink still records the advance and Unicode. A simple `/Subtype /Type1` font reads its `/FontFile` program and paints through the same coverage path; `/MMType1` keeps its PDF widths and paints `invalidfont`. Embedded outlines blend coverage through `x/image/vector`; a checked-in PPM locks the result and a translated glyph moves the marked box. `sampledata/fixtures/type1-tj.ppm` locks the Type 1 `A`, and its pixels equal the synthetic TrueType `A` under `CompareRaster`.
- `File.ExtractText` and `spectreps.ExtractText` sort by Y then X, merge close runs, insert a space for a gap wider than a quarter box, end every line with CRLF, and fall back to the code point. A two-line fixture locks `Hello\r\nWorld\r\n`, a symbolic font locks `AB\r\n`, and a symbolic Type 1 font with a built-in encoding locks `ABZ\r\n`, where A and B come from the Adobe Glyph List and Z from the code point. A bad page index is `rangecheck`.
- `spectreps text [-pages range] file.pdf` prints the selected pages to stdout. The command accepts no other option, and `-pages` follows the shared grammar.
- PostScript `findfont`, `scalefont`, `setfont`, and `show` resolve the standard 14 names. `show` advances the current point, needs a current point, and returns `invalidfont` on a pixmap because the standard 14 have no outline program.

## PDF rewrite

- `RewritePDF` on a document this module can rasterize returns a PDF. Opening that PDF and rasterizing page 0 matches `RasterizePage` of the input, via `CompareRaster`.
- `DefaultRewriteOptions` selects level 0 and Flate-compresses page content streams. `CompressStreams` false leaves those streams uncompressed. Both outputs still match the input pixels.
- `RewriteOptions.Level` is 0 by default. Level 0 re-emits the path subset, including `B`, `B*`, `b`, and `b*`. It refuses a clip (`W`, `W*`) with `undefined in W` and a form XObject with `undefined in Do`, because its recorder implements no clip seam. Levels 1 through 5 use the pass-through writer, so text, fonts, and the page tree are copied and the page count and boxes match the input. A level outside 0 through 5 returns `rangecheck` from `RewritePDF` and exits 2 from the CLI.
- A source `/Type /XRef` or `/Type /ObjStm` container is not copied: its object number gets a free xref row, the `/ID` covers only the written bodies, and the page count and boxes hold. `CopyOptions.PackObjects` writes the optional packed form, `%PDF-1.5` with one Flate `/Type /ObjStm` for non-stream bodies and a Flate `/Type /XRef` stream. The packed and classic forms carry the same `/ID`, and two calls in either form are equal.
- Level 1 Flates every uncompressed content stream. Level 2 also re-encodes Flate, raw, and CCITT image streams losslessly with no resample. Levels 3 through 5 also re-encode images as DCT. The longest-side caps are 1754, 1123, and 842 pixels, at qualities 80, 60, and 40. An image at or below the cap keeps its size. An image Spectre cannot decode, and an image with an `/SMask`, is copied unchanged.
- A Group 4 CCITT image decodes to exact gray pixels at every level: level 2 re-encodes it as Flate RGB and levels 3 through 5 as DCT. A Group 3 stream with `/EndOfLine true`, a Group 3 stream with no end-of-line markers, and a `/K > 0` mixed one- and two-dimensional stream decode with `/EndOfBlock true` and with `/EndOfBlock false`, and a byte-aligned Group 3 stream needs `/EncodedByteAlign true`. An 8-bit depth and a `DeviceRGB` space return `undefined`; a stream truncated inside a row returns `syntaxerror`; `Columns * Rows` above 32 MiB returns `limitcheck`. An undecodable CCITT stream copies through at every level.
- Two `RewritePDF` calls at any level on the same input return buffers `CompareFiles` reports equal. The output contains no `CreationDate` or `ModDate`.
- The rewritten bytes are not required to equal the input bytes, and they are not compared with Ghostscript `pdfwrite`.
- `spectreps rewrite` without `-o` exits 2. `-compress=false` selects the uncompressed level 0 option. `-level 1` through `-level 5` succeed on a text stream that level 0 rejects with `undefined`. A successful rewrite exits 0.
- `sampledata/compress/whatisthis.pdf` and `sampledata/compress/path.pdf` rewrite at every level with the input page count. The JPEG image decodes, the caps hold, and level 5 is the smallest of the five. The level 1 and 2 sizes for `whatisthis.pdf` stay at or under the recorded 610,034-byte ceiling. The test skips when `sampledata/` is absent.

## PDF/A-4 profile preflight

- The PDF/A writer emits `%PDF-2.0` followed by a binary marker whose four bytes are above byte 127. The trailer keeps `/ID` and writes no `/Encrypt`. Both `WriteWithOptions` and `WriteCopy` take the option.
- `CopyOptions.AppendObjects` writes complete bodies after the highest source object number, and `CopyOptions.CatalogOverride` replaces the root body. The copied catalog keeps its other entries.
- The XMP packet is static UTF-8 with `pdfaid:part` 4, `pdfaid:rev` 2020, and the `F` letter for 4f. It carries no dates, and two calls return equal bytes.
- The generated ICC profile is a D50 sRGB matrix-shaper with the `desc`, `cprt`, `wtpt`, `rXYZ`, `gXYZ`, `bXYZ`, `rTRC`, `gTRC`, and `bTRC` tags. The output intent is `/S /GTS_PDFA1` with `/DestOutputProfile` and no `/DestOutputProfileRef`.
- The preflight refuses a font with no embedded file, `LZWDecode`, a filter outside the ISO 32000-2 table, `DeviceCMYK`, `/Alternates`, `/OPI`, and a `/BM` other than `Normal`. Every refusal is a `JobError` with `Op` `PDFA` and the failed rule in `Msg`.
- A `/Separation` or `/DeviceN` whose alternate names `DeviceCMYK`, nested in a page or form `/Resources /ColorSpace` dictionary through a name, an array, or a reference, refuses with `cmyk-without-profile`. An ICCBased profile whose `/Alternate` names `DeviceCMYK` refuses too, and a separation over `DeviceRGB` passes.
- `PDFA4` is refused when the catalog carries `/Names /EmbeddedFiles`, and `PDFA4F` is refused when it does not.
- `RewritePDF` with a PDF/A mode returns bytes that open with the same page count. Two calls on the same document return equal buffers, and the output carries no `CreationDate`, `ModDate`, or `xmp:MetadataDate`.
- `spectreps rewrite -pdfa 4|4f` writes the file and exits 0, a refusal exits 1 with `Error: /rule in PDFA`, and any other `-pdfa` value exits 2. A refusal writes no output file.
- `make pdfa-check` runs `verapdf --flavour 4` over the PDFs under `sampledata/pdfa/`, excludes a `negative/` subfolder, prefers a local copy at `./verapdf/verapdf`, falls back to `verapdf` on PATH, and prints a skip when neither exists. The local copy is gitignored. veraPDF is a proof tool, not a dependency, and it stays out of `make test`. On 2026-09-25, veraPDF 1.30.2 reported `compliant="2" nonCompliant="0"` for `path-a4.pdf` and `compliant-a4.pdf`. The verdict is veraPDF's; the claim wording stays "profile preflight".
- `make pdfua2-check` runs `verapdf --flavour ua2 --format json` over the PDFs under `sampledata/pdfua2/`, uses the same local-copy preference and skip, and excludes `negative/`. On 2026-09-25, veraPDF 1.30.2 reported 1727 passed rules and 0 failed rules for `tagged-ua2.pdf` and `compliant-ua2.pdf`. The `negative/untagged.pdf` fixture fails `ua2-marked` by design.

## Bitmap PDF

- `ImagePDF` and `WriteImages` emit one `/Subtype /Image` XObject per page with `/ColorSpace /DeviceRGB`, `/BitsPerComponent 8`, and `/Filter /FlateDecode`. The stored stream is the tightly packed RGB rows, so stride padding is dropped.
- `ImagePDFColor` and `WriteImagesColor` keep those bytes for `ImageColorRGB` and `ImageRGB`, and add `/DeviceGray` with one byte per pixel and `/DeviceCMYK` with four. The test decodes both streams and compares each with the conversion below.
- Gray is `round(0.299*R + 0.587*G + 0.114*B)` per pixel. Pure red `(255, 0, 0)` decodes to the byte `76`.
- CMYK is `K = 1 - max(r, g, b)` with `C = (1 - r - K) / (1 - K)` and the same for M and Y, each rounded to a byte, and `C = M = Y = 0` when `K >= 1`. Pure red decodes to `0 255 255 0`.
- `spectreps pdfimage -colorspace gray|cmyk` writes those streams, `-colorspace rgb` is the same as the default, and any other value exits 2.
- `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]` points. `dpi` of 0 or less selects 72.
- The content stream paints `/Im0 Do` and the page resources carry the XObject. The output reopens and rasterizes: `RasterizePage` matches the source `PageImage` under `CompareRaster` for RGB and gray, and `spectreps raster` on the `pdfimage` output matches the PPM of the source program.
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

## Benchmarks and profiles

`make test` runs `go test ./...`, and `go test` runs a benchmark only when `-bench` is passed, so no benchmark gates `make test`. `make bench`, `make bench-profile`, and `make bench-check` are manual targets, and `profiles/` is gitignored. Benchmark timing is machine-specific and never gates a phase. The allocation ceilings in `TestPerformanceAllocs` are ordinary tests, so they do run under `make test`. The baseline, the budget, and the tool matrix are in `documentation/performance.md`.
