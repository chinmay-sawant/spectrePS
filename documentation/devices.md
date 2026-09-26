# Devices

Four jobs share words and must stay separate.

## Raster

`RunPostScript` and `RasterizePage` produce a `PageImage`. Row 0 is the top of the page. A mark at user coordinate y=0 lands on the last row. The graphics engine stores paths in user space, transforms them to device points, and the pixmap device flips y.

Default media is 612 by 792 points. Default resolution is 72 dpi, so the default pixmap is 612 by 792, RGB, 3 bytes per pixel, stride equal to `Width * 3` unless a later encoder needs alignment. Phase 04 uses stride `Width * 3`.

The file form used in tests is PPM raw, P6. The header is ASCII `P6\n{width} {height}\n255\n`, then tightly packed RGB bytes in the same order as `PageImage.Pixels`. Tests compare those pixel bytes with `CompareRaster`, or compare the PPM files with `CompareFiles` when the header is part of the case.

PNG is a second encoding of the same pixels, selected by an output path that ends in `.png`. `image/png` from the standard library does the encoding. PNG file bytes are not an equality oracle. Two encoders, or two library versions, may differ. The oracle is `PageImage`.

JPEG is a third encoding of the same pixels, selected by an output path that ends in `.jpg` or `.jpeg`. `image/jpeg` from the standard library does the encoding, with quality from `-jpegq`, default 75. The format is lossy, so decoded pixels can differ from the source by a small amount. JPEG file bytes are not an equality oracle either. `CompareRaster` and `PageImage` stay the oracle.

TIFF is a fourth encoding of the same pixels, selected by an output path that ends in `.tif` or `.tiff`. `golang.org/x/image/tiff` does the encoding with `tiff.Encode`. The pixel source is the same RGBA conversion the PNG and JPEG writers use, written as 8-bit RGBA with associated alpha, which is 24-bit RGB for opaque pages. `-tiffcompress none` writes uncompressed baseline TIFF, and `deflate` writes Deflate-compressed strips and is the default. The encoder has no write path for LZW, CCITT G3, or G4, so the flag accepts `none` and `deflate` only. TIFF file bytes are not an equality oracle either. `CompareRaster` and `PageImage` stay the oracle.

Anti-aliasing is off in this ledger. There is no `TextAlphaBits` equivalent yet. Turning it on would change pixels and invalidate fixtures, so it stays deferred.

## Compare

`CompareFiles` is a byte compare of two slices, including the first mismatch offset. It is the 0.0.1 behavior behind `spectreps compare bytes`.

`CompareRaster` compares width, height, then RGB bytes. It ignores stride padding. It is the 0.0.2 behavior behind `spectreps compare raster`. Both inputs are rasterized by Spectre with the same `RunOptions`. The command does not call Ghostscript.

A mismatch is exit code 1. It is not an interpreter error.

The CLI gives both inputs the same `RunOptions` value and the same `-pages` selection, so their page sizes always agree. `compare raster` prints `mismatch pixel N`, or `mismatch length` when the selected page counts differ. It never prints `mismatch width` or `mismatch height`, which `CompareRaster` can still report to a library caller. `compare bytes` prints `mismatch byte N` or `mismatch length N`.

## Box and ink coverage

`MeasureBox`, `MeasureInk`, and `MeasureInkAmount` read a finished `PageImage`. They do not paint a second time and they add no operator.

The box is the union of marked pixels in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. `dpi` of 0 selects 72.

`MeasureInk` is RGB occupancy: the fraction of pixels marked in each of R, G, and B. The pixmap is RGB, not CMYK, so the CLI line ends in `RGB` and not `CMYK OK`. These numbers are occupancy fractions, not Ghostscript `ink_cov` weighted amounts. `spectreps inkcov` prints them.

### Weighted ink amounts

`MeasureInkAmount` is the weighted ink amount. Let `N` be `Width * Height` and let `v_i` be the byte in the channel being measured, `c`, of pixel `i`, where `c` is R, G, or B. The amount is:

```
amount_c = (1/N) * sum (255 - v_i)/255
```

The sum ignores stride padding. A white byte contributes 0, a black byte contributes 1, and every other byte contributes its complement as a fraction. A zero-size image returns the zero `Ink`. `spectreps ink_cov` prints `amount * 100` with five decimals and the `RGB` suffix.

Worked example. A 20 by 20 page holds a cyan square (`0 1 1 setrgbcolor`) over 10 by 10 pixels. Those 100 pixels are `(0, 255, 255)` and the other 300 are white.

- R: 100 pixels contribute `(255-0)/255 = 1` and 300 contribute 0, so the amount is `100/400 = 0.25`.
- G and B: the square bytes are 255, so both amounts are 0.

`spectreps ink_cov -w 20 -h 20 -r 72` prints `Page 1`, then `25.00000 0.00000 0.00000 RGB`.

Second worked example. Every pixel on a byte-128 gray page is `(128, 128, 128)`. Each byte contributes `(255-128)/255 = 127/255 = 0.49803921...`, so the page prints `49.80392 49.80392 49.80392 RGB`.

The alternatives considered:

- Luma. One number per pixel, `0.299*R + 0.587*G + 0.114*B`, folds the channels into a display-weighted brightness. It loses which channel is heavy, and its weights are not ink weights. The report could not keep three RGB columns.
- Total occupancy. The existing `MeasureInk` counts a marked channel as 1 no matter how dark the byte is. A 1 percent cyan tint and a full cyan pixel both count once. That is the Ghostscript `inkcov` device, not `ink_cov`. It cannot tell a light page from a dark one.

The per-channel complement wins because it keeps both facts: which channel and how much. It is the continuous refinement of occupancy: a byte counts `(255 - v)/255` instead of 1, so a black byte counts 1 and a byte at 254 counts `1/255`. White and black land on the two ends. The channels stay R, G, and B because the pixmap is RGB, so the line keeps the `RGB` suffix.

Manual versus source. The 10.09.0 manual example line shows fractions, and its walk-through says a half cyan fill reports `0.50 0.00 0.00 0.00` for `ink_cov`. The source (`devices/gdevicov.c`, `cov_write_page_ink`) computes `c = dc_pix*100 / (total_pix*255)` and prints a percent. The source is the reference. Measured by hand on the installed 9.55.0:

```
$ gs -q -dNOPAUSE -dBATCH -sDEVICE=ink_cov -g20x20 -r72 -o- quarter-cyan.ps
25.00000  0.00000  0.00000  0.00000 CMYK OK
$ gs -q -dNOPAUSE -dBATCH -sDEVICE=ink_cov -g20x20 -r72 -o- half-cyan.ps
49.80392  0.00000  0.00000  0.00000 CMYK OK
```

The first file fills a quarter page with 100 percent cyan. The second fills a whole page with a 50 percent cyan tint, which the CMYK8 device stores as byte 127, so the printed value is `127/255 * 100`. A full black page prints `100.00000` on K. Spectre follows the source: it scales the amount by 100 and keeps the same five decimals. The suffix is `RGB`, not `CMYK OK`, because the Spectre pixmap is RGB.

## Stream filters

`internal/pdf/filter.go` decodes one stage, and `LZWDecode` sits in `internal/pdf/lzw.go`. One decoded stream is capped at 32 MiB.

| Filter | Behavior |
| --- | --- |
| `FlateDecode` | zlib, the wrapper around `compress/flate`. Predictors 2 and 10 through 15 apply. |
| `LZWDecode` | 9 to 12 bit MSB-first codes, the clear and EOD markers, the KwKwK case, and `/EarlyChange` 0 or 1. Predictors apply. |
| `ASCIIHexDecode` | Pairs of hex digits, whitespace ignored, and a greater-than sign ends the data. An odd final digit pads with 0. |
| `ASCII85Decode` | `z` is four zero bytes and `~>` ends the data. A leading `<~` is accepted, and the final partial group writes n-1 bytes for n characters. |
| `RunLengthDecode` | A length of 0 through 127 copies the next length+1 bytes, 129 through 255 repeats the next byte 257-length times, and 128 ends the data. |

An empty filter name returns the bytes unchanged. A `/Filter` array decodes each stage in order, and each stage takes its own `/DecodeParms` entry. An unknown filter name returns `undefined` with that name, malformed data returns `syntaxerror` with the filter name, and a decoded stream past 32 MiB returns `limitcheck` with the filter name.

The reader does not decode `JBIG2Decode`, so `Decode` returns `undefined in JBIG2Decode`. The PDF/A preflight accepts the name because it is in the ISO 32000-2 filter table. That disagreement is deliberate: the preflight checks names, and the reader has no JBIG2 decoder. The PDF/A preflight still refuses a chain that names `LZWDecode`, because PDF/A disallows the filter even though the reader can decode it.

Predictors apply to xref streams, object streams, content streams, and image streams, because all four go through the same decoder. Predictor 2 is the TIFF horizontal differencing predictor, and its samples follow `/Colors`, `/BitsPerComponent`, and `/Columns`. Predictors 10 through 15 are the PNG predictors. Every PNG-predicted row begins with an algorithm tag byte that selects that row's algorithm, so `/Predictor 12` and `/Predictor 15` read the same per-row tags and may mix algorithms (ISO 32000-1 7.4.4.4). A `/Predictor` value other than 1, 2, and 10 through 15 returns `undefined` with the `Predictor` op name.

## Image XObjects

The reader walks the xref for in-use objects whose dictionary has `/Subtype /Image`. `ImageObjectNums` returns their object numbers in ascending order. `DecodeImage` returns an `image.Image` or an error. It never returns a blank image for a failed decode.

Four stream forms decode:

- `/FlateDecode` at 8 bits per component, through the same zlib path as content streams. The color space resolves through the reading-side rules below, and every sample converts to the preview RGB. The `/DecodeParms` predictor applies.
- `/DCTDecode` through `image/jpeg`. A gray source needs a gray space, a CMYK source a CMYK space, and any other source converts through its RGBA channels.
- `/CCITTFaxDecode` with `/DeviceGray` at 1 bit per component, through `golang.org/x/image/ccitt`. `/K < 0` is Group 4, `/K == 0` with `/EndOfLine true` is Group 3, and `/K > 0` is undefined. `/Columns` and `/Rows` default to `/Width` and `/Height`, `/BlackIs1` inverts the samples, and `/EncodedByteAlign` byte-aligns the codes. `Columns * Rows` above the 32 MiB decoded cap returns `limitcheck` before allocation.
- `/JPXDecode` through `github.com/mrjoshuak/go-jpeg2000`, a pure-Go decoder. `/ColorSpace` and `/BitsPerComponent` are optional and ignored for JPX: the codestream carries the color and the precision, so the branch runs before the shared parameter check. A header that declares more decoded sample bytes than the 32 MiB Flate cap returns `limitcheck` before the decoder allocates.

Any other filter or bit depth returns `undefined`, as does a Flate stream whose byte count does not match width by height by components. JPEG is lossy, so decoded pixels are not a byte oracle for the source; a JPEG2000 stream may be lossless or lossy. A failed decode returns `syntaxerror` or `limitcheck`, never a blank image.

### Reading-side color spaces

`imageParams` resolves `/ColorSpace` to a sample count and a preview RGB conversion. These forms resolve:

- DeviceRGB, DeviceGray, and DeviceCMYK, including the abbreviated names RGB, G, and CMYK.
- Indexed over any base space. The lookup is a string or a stream, and an index past hival clamps to hival.
- ICCBased. `/Alternate` wins. Without one, `/N` 1, 3, or 4 previews as gray, RGB, or CMYK. The profile bytes are never read, so a real profile transform is out of this subset.
- CalRGB as RGB and CalGray as gray. The white point, gamma, and matrix entries are ignored.
- Separation and DeviceN. The tint transform evaluates as a type 2 exponential interpolation or a type 4 PostScript calculator function. A type 0 sampled or type 3 stitching function is undefined.

A device sample scales by 1/255, and an Indexed sample is the index. A `/Decode` array remaps each sample to `low + sample*(high-low)` before the conversion, for gray, RGB, CMYK, and Indexed spaces; an Indexed space refuses a decode array. Any other space, a malformed array, a malformed decode array, and a cycle through indirect references return `undefined` with the operator name, `Image` or `Do`. An image with `/ImageMask true` skips the color space entirely and decodes to one bit per sample.

An LZW image decodes through `DecodeLZWImageValue` for the level 2 writer. `DecodeImage` itself still returns `undefined in LZWDecode`, because its table is the four forms above.

## Painting images

The content interpreter resolves `Do` in the page's `/XObject` resources. `/Resources` inherits from the nearest `/Pages` ancestor, and the `/XObject` subdictionary, the image entry, and the image itself may each be indirect. An image decodes once per name per painted page.

An image paints into its unit square: `(0,0)` is the lower left and `(1,1)` is the upper right. The square maps through the current matrix (`cm`), then scales by the paint scale. The pixmap stamps it with nearest-neighbor sampling, image row 0 is the top of the square, and the source alpha composites the pixel. A CMYK, Indexed, Separation, or ICCBased image decodes to the preview RGB before it stamps, so `DrawImage` receives RGB pixels. `Marker.DrawImage(pic image.Image, ctm Matrix, scale float64)` is the device seam, and the pixmap and the rewrite recorder implement it.

A missing name, an entry whose `/Subtype` is not `/Image`, and a decode error all return `undefined` with the `Do` operator name. An image `/SMask`, a color-key `/Mask`, a stencil `/Mask`, and an `/ImageMask` decode and paint through the alpha channel. Level 0 of `RewritePDF` cannot write image pixels, so a page that paints one returns `undefined in Do` instead of dropping or outlining the image. The pass-through writer at levels 1 through 5 copies the image unchanged.

`Do` on a `/Subtype /Form` XObject runs the form content stream as content. `/Matrix` concatenates into the current matrix, `/BBox` becomes a clip on the form's marks, and a form `/Resources` subdictionary replaces the current resources. A form with no `/Resources` inherits the page resources. The form runs inside an implicit `q`/`Q`, so width, color, clip, resources, and text state return when it ends. Nested forms stop at depth 32 with `limitcheck` in `Do`. A form with no stream, no `/BBox`, a malformed `/Matrix` or `/BBox`, or a `/FormType` other than 1 returns `undefined` in `Do`. The level 0 recorder cannot express the box clip, so a page that runs a form returns `undefined in Do`.

## Clip, ExtGState, and marked content

`W` and `W*` intersect the current path into the clip applied to later marks, and `W*` uses the even-odd rule. `q` saves the clip and `Q` restores it. The runner keeps the clip as a list of device-space paths, and the pixmap applies it to fills, strokes, glyphs, and images by snapping back every pixel outside the region. A point survives the clip when its center is inside every path.

`graphics.ClipMarker` is the optional seam. `graphics.Marker` keeps its three methods, so a marker that cannot write a clip refuses `W` and `W*` with `undefined`. The level 0 recorder and the PostScript recorder keep that refusal.

`gs` resolves a name in `/Resources /ExtGState`. `/LW` sets the stroke width, and `q`/`Q` restore it. `/LC`, `/LJ`, `/ML`, and `/RI` are accepted as no-ops. Any other entry refuses with `undefined in gs` rather than skipping the state, and an unknown name is `undefined in gs`.

`J`, `j`, `M`, `i`, and `ri` are accepted as no-ops. The stroke model is a capsule with round caps and joins, so a cap, join, miter limit, flatness, or rendering intent has nowhere to land. This is a deviation from ISO 32000-2, which lets those operators shape the stroke.

`BMC`, `BDC`, `EMC`, `MP`, and `DP` parse. Nesting caps at 64 with `limitcheck`, an unmatched `EMC` is `syntaxerror in content`, and `BX` skips content through the matching `EX`. An unterminated compatibility section is `syntaxerror in content`, and an `EX` outside a section is ignored. A page with marked content rasterizes and extracts as if the markers were absent.

The read seams are separate from the number. `PaintOptions.MarkedContent` receives `BeginMarkedContent` at `BMC` and `BDC` and `EndMarkedContent` at the matching `EMC`, both at the same depth, with 1 for the outermost sequence. The properties value is the resolved `/Properties` entry for a name operand, the dictionary for an inline operand, and null for `BMC`. `MP` and `DP` fire no event because they have no `EMC`. `TextOptions.Runs` receives one `TextRun` per `Tj`, `TJ`, `'`, or `"` with the shown bytes, the font resource name, the size, the text and line matrices, rise, spacing, and horizontal scale. The run also carries the decoded character codes, the `/ToUnicode` result per code, the CTM, the paint scale, and the device advance box of the whole run. For `TJ` the bytes are the string elements concatenated and the numbers are omitted. `ImageNameMarker` receives the resource name and the resolved XObject dictionary before an image decodes. A nil or typed-nil sink is ignored, and a marker that accepts text runs but no glyph coverage skips glyph rasterization instead of refusing the text.

### Optional content

The catalog `/OCProperties /D` dictionary is read. An optional content group whose object number appears in `/OFF` is hidden, and every other group is visible. A `BDC` whose properties name a hidden group opens a skip: the marked-content frame and its sink events still open and close in order, but the painting operators between them never run. A `/OC` entry on an image or form XObject is checked before decode, and a hidden XObject is skipped as if the `Do` were absent. A membership dictionary with `/OCGs` evaluates its members, and `/P` selects the policy: `/AnyOn` (the default) is hidden only when every member is off, `/AllOn` when any is off, `/AllOff` when any is on, and `/AnyOff` when every member is on. Alternate configurations, the `/AS` usage map, and the visibility flag stay out, a deviation recorded in `covered-and-not-covered.md`.

### Content color operators

`K` and `k` select DeviceCMYK and set the four components. `CS` and `cs` pop a space name, resolve it in `/Resources /ColorSpace` first and as a device name second, and reset the components to 0. `SC` and `sc` pop one value per component, and `SCN` and `scn` do the same but refuse a trailing pattern name with `undefined`. The stroke and non-stroking forms share one current color in this subset, so an `RG` color still fills a path. The current space and components survive `q`/`Q`.

A mark reaches the RGB pixmap through the preview conversion the reading-side rules use. For DeviceCMYK the rule is frozen next to the writer's rule in the image color spaces section: `r = (1-C)*(1-K)`, and the matching green and blue terms. A Separation or DeviceN space evaluates its tint transform and previews the alternate space.

## Rewrite

`RewritePDF` builds a new PDF from drawing operations on a `Document` at level 0. Stream compression uses `compress/flate` when `CompressStreams` is true. The CLI default is true.

The level 0 writer omits a wall-clock creation date and uses a fixed trailer id derived from the compressed content bytes. Two calls on the same document return equal buffers. `CompareFiles` is the proof.

The output is not a copy of the input xref, and it is not expected to match `pdfwrite` from any Ghostscript version.

The pass-through writer (`WriteCopy`) serves levels 1 through 5. It copies every object it does not replace: the page tree, `/Resources`, fonts, annotations, and metadata. A font object is replaced only when the subset option is on. A source `/Type /XRef` or `/Type /ObjStm` container is not copied, so its object number becomes a free xref row and no dead container bytes reach the output. An object stored in an object stream is written uncompressed through `SerializeValue`. An override replaces a whole object body by number. The trailer uses the source `/Root`, `/Size` as the highest in-use object number plus one, and `/ID` as the SHA-256 of the written bodies only, in object-number order. Two calls on the same source return equal buffers, and the file carries no `/Info` and no dates.

`CopyOptions.PackObjects` selects the optional packed output: `%PDF-1.5`, every non-stream body in one Flate `/Type /ObjStm`, and a Flate `/Type /XRef` stream with `W [1 4 2]` in place of the classic xref. The `/ID` digest is computed over the unpacked bodies before packing, so it does not change with the mode. The levels 1 through 5 path does not select the packed output. `CopyOptions.PackObjects` is internal only, not on `RewriteOptions`, so no public call selects the packed output.

Levels 1 and 2 stay at or under 610,034 bytes on `sampledata/compress/whatisthis.pdf`. That is the recorded ceiling. The source packs 78 non-stream objects, so the classic form is 13,693 bytes over the 596,341-byte input, and `PackObjects` stays opt-in.

A tagged source is a separate case. Levels 1 through 5 copy the structure tree, the parent tree, MCIDs, `/Alt`, `/ActualText`, and `/Lang`, and the writer keeps the source header block, binary marker included, so a PDF 2.0 file with tags does not leave as a 1.4 shell. Level 0 returns `/tagged` instead of building a path-only file that dropped the tree. `ImagePDF` and `WriteImages` take page and image values and never see a source document, so the `pdfimage` command refuses a tagged PDF before it rasterizes.

`RewriteOptions.SubsetFonts` and `spectreps rewrite -subset-fonts` replace embedded TrueType fonts at levels 1 through 5. A reader-side collector returns the sorted unique codes per page and font resource name. The writer rebuilds each `glyf` program with stable glyph indices, rebuilds `loca`, appends the new program and a synthesized `/ToUnicode` stream, and rewrites the font dictionary through `CopyOptions.Overrides`. A Type0 font keeps its CIDs and its `/CIDToGIDMap` and gets `/W` trimmed to the used CIDs. An OpenType `/FontFile3` program and a Type 1 `/FontFile` program are copied whole. A font with no program is copied unchanged, so the PDF/A `font-not-embedded` rule still refuses it with or without the option. The subset tag is six uppercase letters from a digest of the subset bytes, so no date enters it, and two runs over the same source return equal bytes either way. The option is off by default. Level 0 ignores it and still refuses text with `undefined in Tj`. The shape is in `documentation/fonts.md`.

The level table:

| Level | Name | Content streams | Images |
| --- | --- | --- | --- |
| 0 | Path subset | re-emitted, Flate when `CompressStreams` is true | unchanged; a painted image is `undefined in Do` |
| 1 | Light | Flate every uncompressed stream | unchanged |
| 2 | Balanced | Flate | Flate, raw, LZW, and CCITT image streams re-encoded losslessly, no resample |
| 3 | Medium | Flate | re-encoded as DCT, longest side capped at 1754 px, quality 80 |
| 4 | Strong | Flate | re-encoded as DCT, longest side capped at 1123 px, quality 60 |
| 5 | Hard | Flate | re-encoded as DCT, longest side capped at 842 px, quality 40 |

An image at or below its cap keeps its size. An image Spectre cannot decode, and an image with an `/SMask`, is copied unchanged. Level 2 re-encodes Flate, raw, LZW, and CCITT streams, so DCT and JPEG2000 streams copy through. Levels 3 through 5 decode DCT, CCITT, JPEG2000, and LZW streams and re-encode them as DCT with the same caps and qualities. A level above 0 ignores `CompressStreams`. Every page reaches the output with the same page count and boxes, because the writer copies the page tree.

The image helpers are three functions in `internal/pdfout`. `ScaleImage` takes any `image.Image` and returns RGBA resampled with the CatmullRom kernel from `golang.org/x/image/draw`; width and height below 1 clamp to 1. `EncodeDCT` wraps `image/jpeg` with the quality clamped to 1 through 100, and `EncodeFlateRGB` writes tightly packed RGB rows, top row first, inside zlib. All three are deterministic, so the same input returns the same bytes. Levels 3 through 5 call `ScaleImage` and `EncodeDCT`, and level 2 calls `EncodeFlateRGB`.

## Tagged generation and preflight

`RewritePDF` with `RewriteOptions.Tag` generates a PDF/UA-2 structure tree for an untagged PDF. `spectreps rewrite -tags` is the CLI. Generation covers the content the text machine reads: paths, text, images, and the marked-content operators. A source operator the recorder cannot express, such as a clip `W` or `W*`, returns `undefined` with that operator name instead of writing a wrong tree. Form XObjects run as content, so a tagged write flattens them.

The recorder paints one page at 72 dpi, so one device pixel is one user point. Text is re-emitted with the recorded `Tm`, and a non-identity CTM or a paint scale is re-emitted as `cm` inside `q`/`Q`, so a translated, scaled, or rotated run keeps its device placement. An image event re-emits its draw matrix the same way, so the unit square stamps at the recorded device box. Path points are re-emitted in device space.

An untagged PDF stores no semantics, so the reading order is an approximation from device geometry, not a fact, open decision 5. The thresholds are:

| Rule | Threshold |
| --- | --- |
| One baseline | Runs whose baseline differs by at most 0.5 of the line box height. |
| One visual segment | Runs on one baseline with a horizontal gap at or under 3 line heights; a wider gap starts a new segment, so a column gutter separates. |
| One paragraph | Segments whose boxes overlap horizontally and whose vertical gap is at or under 1.35 times the smaller line height merge into one `/P`. |
| Reading order | Items that overlap vertically order left to right; otherwise top to bottom. |
| Heading | A run size at or above 1.2 times the body size, where the body size is the decoded-text weighted mode of the run sizes. The largest heading size is `/H1`, the next `/H2`, through `/H6`. |
| List | A bullet from `•`, `·`, `◦`, `‣`, `▪`, `–`, `—`, `-`, or `*`, or a decimal number of up to three digits followed by `.` or `)` and a space. The `/L` carries `/ListNumbering` `Disc` or `Decimal`, and each `/LI` carries `/Lbl` and `/LBody`. |
| Table | Two or more consecutive baseline rows of two or more cells whose column starts agree within 4 points across rows and are at least 2 line heights apart. The first row is `/TH` with `/A << /O /Table /Scope /Column >>`; later rows are `/TD`. |
| Figure | Every image XObject. The `/Alt` source is the innermost open BDC property dictionary, then the image XObject dictionary. |
| ActualText | Open decision 9 is narrow: a run carries `/ActualText` only when a glyph maps to a ligature code point (`U+FB00` through `U+FB06`, `U+0132`, `U+0133`) or has no Unicode mapping. The value expands a ligature and falls back to the code point. A run that shares an element with other runs becomes a `/Span` child. |
| Artifact | An event no element claims, such as a rule or a decoration, wraps in `/Artifact BMC`. A text block whose trimmed text is at most 64 runes long and repeats in the same vertical band (within 2 points) on two or more pages is page furniture and wraps in `/Artifact` too. |

Everything the recorder captured is covered: an event either joins a structure element, becomes a Figure, or wraps in an Artifact. The artifacts write no MCID and no parent tree entry, so the UA-2 content check ignores them and veraPDF sees no untagged content.

The claim is opt-in through `RewriteOptions.Claim` or `-claim`. The builder runs `pdf.Open` plus `pdfa.PreflightUA2` on its own first-pass bytes, and `pdfa.UA2Write` adds `pdfuaid:part 2` and `pdfuaid:rev 2024` only when that preflight passes. `dc:title` comes from `RewriteOptions.Title`, then the source XMP title; with neither, `ua2-title` fails and the tree still comes back with no claim. `/Lang` comes from `RewriteOptions.Lang`, then the source catalog. The claim wording is generate and preflight: it is not a certification.

The refusals are named:

| Request | Result |
| --- | --- |
| `Tag` on a tagged input | `Error: /tagged in RewritePDF`. |
| `Tag` with `PDFA` set | `Error: /unsupported in RewritePDF`, open decision 10. |
| An image with no `/Alt` source | `Error: /alt in Tag`. |
| `Claim` with no title | `Error: /ua2-title in PDFUA`. The tree is kept and no claim is written. |
| `Claim` that fails any other UA-2 rule | That rule, `Error: /ua2-<rule> in PDFUA`. The tree is kept and no claim is written. |
| `-claim`, `-tag-title`, or `-tag-lang` without `-tags` | Exit 2. |

The structure model is `internal/pdf/structtree.go`. It parses `/MarkInfo`, `/StructTreeRoot`, `/K`, `/S`, `/P`, `/Pg`, `/MCID`, `/Alt`, `/ActualText`, `/Lang`, `/Namespaces`, `/RoleMap`, `/RoleMapNS`, and `/ParentTree` into typed values. The tree walk caps depth at 64 and reports a cycle as `limitcheck`. The parent tree resolves an MCID to its structure element and back; a claim with no agreeing entry is `undefined in ParentTree`. A role map resolves a custom type to a standard type; a cycle or a chain past 32 hops is `limitcheck`, a mapping into its own namespace or to itself is `syntaxerror`, and an unmapped custom type is `undefined`.

The font check is dictionary-level only. A font passes when it has `/ToUnicode`, or when it is a simple font with a standard `/Encoding`. Otherwise an `/ActualText` on the structure element or an ancestor covers the run. The preflight does not decode glyphs. The claim is preflight only, never certification.

`internal/pdfa` adds the PDF/UA-2 metadata and the machine checks. `ReadUA2` reads the catalog `/Metadata` packet with the `pdfuaid` values and `dc:title`, plus `/Lang`, `/MarkInfo`, and `/ViewerPreferences`. `UA2Write` keeps the source claim and adds `pdfuaid:part 2` and `pdfuaid:rev 2024` only when the caller opted in after a passing preflight. `UA2XMP`, `UA2ExtraObjects`, and `UA2Catalog` produce the stream and the catalog.

`PreflightUA2` is the UA-2 request, separate from the PDF/A preflight. It returns `Error: /ua2-<rule> in PDFUA` for `ua2-marked`, `ua2-structtree`, `ua2-document`, `ua2-lang`, `ua2-displaydoctitle`, `ua2-pdfuaid`, `ua2-title`, `ua2-rolemap`, and `ua2-mcid`. A PDF/A-only problem such as an LZW stream or a non-embedded font does not fail the UA-2 request, and the UA-2 checks never run for a PDF/A request. The claim is preflight only, never certification.

`make pdfua2-check` runs `verapdf --flavour ua2 --format json` over the PDFs under `sampledata/pdfua2/` and skips when the CLI is absent. A `negative/` subfolder holds deliberate failures and is excluded. `sampledata/pdfua2/generated/report.pdf` is a Spectre tagged write, and `go run internal/tag/gen_samples.go` regenerates it together with `negative/no-title.pdf`, the refused claim case. On 2026-09-26, veraPDF 1.30.2 reported 1727 passed rules and 0 failed rules for `generated/report.pdf`, `tagged-ua2.pdf`, and `compliant-ua2.pdf`. The verdict is veraPDF's.

## PostScript output

`WritePostScript` builds a date-free PostScript program from drawing operations on a `Document`. It borrows `pdf.Paint`, so each page carries the same path subset as `RewritePDF` level 0, re-emitted as `setrgbcolor` or `setgray`, `setlinewidth`, `m` and `l`, and `S`, `f`, or `f*`. Coordinates are 72 dpi points.

The program starts with `%!PS-Adobe-3.0` and a fixed `%%BoundingBox: 0 0 612 792`. A prolog defines the short path names in terms of `moveto`, `lineto`, `stroke`, `fill`, and `eofill`, because a bare `m` or `S` is not a PostScript operator. Each page gets a `%%Page` comment and one `showpage`, and the program ends with `%%EOF`. There is no creation date, and two calls on the same document return equal buffers, so `CompareFiles` is the proof.

The PDF painter flattens `c` into straight segments before the recorder sees it, so the writer emits what `pdf.Paint` gives and never writes `curveto`. The recorder refuses text and images: `Tj` returns `undefined in Tj`, and an image draw returns `undefined in Do`, so a text page or an image page does not leave as a silently blank program. `WritePostScript` resolves no `/XObject` resources, so a page whose content runs `/Im0 Do` refuses with `undefined in Do` at the name lookup. The proof is a round trip: a PDF page with `re`/`f`, `m`/`l`/`S`, a curve, and `q`/`Q`/`cm` becomes PostScript, runs back through `RunPostScript` at 72 dpi, and matches under `CompareRaster`.

## Bitmap PDF

`ImagePDF` wraps each `PageImage` in one PDF page. The default image is 24-bit RGB, 8 bits per component, `/ColorSpace /DeviceRGB`, `/Filter /FlateDecode`. `ImagePDFColor` also writes DeviceGray and DeviceCMYK. The stored stream is the tightly packed rows, so stride padding is dropped. `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]` points, and a `dpi` of zero or less selects 72.

Each page has one content stream and one image XObject. The content stream is `q W 0 0 H 0 0 cm /Im0 Do Q`, and `/Resources` carries the XObject. Rasterizing this output is a round trip: the reopened file decodes the image and stamps it back at 1:1 through the same `Do` path, so `RasterizePage` matches the source `PageImage` under `CompareRaster` for the RGB and gray spaces.

The writer emits objects in a fixed order, adds no `/Info`, and sets both trailer `/ID` strings to the SHA-256 of the concatenated Flate image streams. Two calls on the same pages return equal buffers, and `CompareFiles` is the proof. The output is not `pdfwrite` and it is not a DCT encode.

## Image color spaces

`ImagePDFColor` and `WriteImagesColor` pick the stream color space. `ImageColorRGB` and `ImageRGB` store 24-bit RGB, 3 bytes per pixel, with `/ColorSpace /DeviceRGB`. This is the default, and `ImagePDF` and `WriteImages` keep writing those bytes. `ImageColorGray` and `ImageGray` store 8-bit gray, 1 byte per pixel, with `/DeviceGray`. `ImageColorCMYK` and `ImageCMYK` store 32-bit CMYK, 4 bytes per pixel, with `/DeviceCMYK`. Every page uses `/BitsPerComponent 8` and `/Filter /FlateDecode`. `spectreps pdfimage -colorspace rgb|gray|cmyk` selects one, and `rgb` is the default.

DeviceGray is BT.601 luma. For a pixel `(R, G, B)` with bytes 0 through 255:

```
Y = round(0.299*R + 0.587*G + 0.114*B)
```

The stream stores `Y`.

DeviceCMYK divides each channel by 255, so `r`, `g`, and `b` run 0 through 1, then:

```
K = 1 - max(r, g, b)
```

If `K >= 1`, then `C = M = Y = 0`. Otherwise:

```
C = (1 - r - K) / (1 - K)
M = (1 - g - K) / (1 - K)
Y = (1 - b - K) / (1 - K)
```

Each of C, M, Y, and K scales by 255 and rounds to the nearest byte. The stream stores the four bytes in C, M, Y, K order.

Worked example. Pure red is `(255, 0, 0)`.

- Gray: `round(0.299*255 + 0.587*0 + 0.114*0)` is `round(76.245)`, so the stream stores `76`.
- CMYK: `r = 1`, `g = 0`, `b = 0`, so `K = 1 - 1 = 0`, `C = (1 - 1 - 0) / 1 = 0`, `M = (1 - 0 - 0) / 1 = 1`, and `Y = 1`. The stream stores `0 255 255 0`.

The reader reverses the CMYK rule. Divide each channel by 255, so C, M, Y, and K run 0 through 1, then:

```
r = (1 - C) * (1 - K)
g = (1 - M) * (1 - K)
b = (1 - Y) * (1 - K)
```

Each channel scales by 255 and rounds to the nearest byte. Worked example: the quadruple `0 255 255 0` is pure red, and `255 0 0 0` is cyan `(0, 255, 255)`. This is the inverse of the K-first rule above, so a red pixel written as CMYK reads back red.

## Validate

`validate` runs the interpreter in stop-on-first-error mode.

For PostScript, any `JobError` fails the command. For PDF, repair-and-continue is not the default of this command. A bad xref, a bad stream, or an unsupported operator fails the command with that `JobError`. Rendering commands may later warn and continue. `validate` does not.

`validate` does not write PDF/A metadata and does not run the PDF/A preflight. PDF/A creation is the rewrite option below, and its result is not a certificate.

A tagged input also runs the PDF/UA-2 machine checks after its pages paint, open decision 11. `Document.Tagged` reads the catalog `/StructTreeRoot` or a true `/MarkInfo /Marked`, and a document that carries either one is a UA-2 request for `validate`, so a tree the machine rules refuse fails the command with `Error: /ua2-<rule> in PDFUA`. An untagged PDF never runs the UA-2 checks and keeps the interpreter-only behavior.

## PDF/A-4 profile preflight

`RewritePDF` with `RewriteOptions.PDFA` set writes a pass-through rewrite and appends a PDF/A-4 claim. `PDFA4` is the base claim and `PDFA4F` claims PDF/A-4f. PDF/A-4e stays out of scope.

The claim is a profile preflight, not a certificate. Every line in this file and in the CLI treats it as a claim.

The writer changes three things:

- The header becomes `%PDF-2.0` followed by a marker line whose four bytes are above byte 127. The trailer keeps `/ID` and writes no `/Encrypt`.
- The catalog gains `/Metadata` on an XMP stream and `/OutputIntents` on one output intent. Every other catalog entry is copied unchanged, and every other source object is copied by the pass-through writer.
- The XMP stream is a static UTF-8 packet with `pdfaid:part` 4 and `pdfaid:rev` 2020. PDF/A-4f adds the `F` conformance letter. The packet carries no dates.

The output intent is `/S /GTS_PDFA1`. It carries `/DestOutputProfile` on a generated D50 sRGB matrix-shaper ICC profile and writes no `/DestOutputProfileRef`. The profile is built in `internal/pdfa`, not copied from another file.

`RewriteOptions.Level` still selects stream and image handling. Level 0 copies streams unchanged, and levels 1 through 5 use the level table above. A PDF/A rewrite uses the pass-through writer and never the level 0 path re-emitter, so text and fonts are copied.

The preflight refuses the claim with a `JobError` whose `Op` is `PDFA` and whose `Msg` is the failed rule:

| Rule | Refusal |
| --- | --- |
| `font-not-embedded` | A font dictionary with no `/FontFile`, `/FontFile2`, or `/FontFile3` on its descriptor. A Type 3 font is exempt. |
| `lzwdecode` | A stream filter chain that names `LZWDecode`. |
| `filter-not-allowed` | A filter name outside the ISO 32000-2 filter table, including `Crypt`. |
| `cmyk-without-profile` | A color space value that names `DeviceCMYK`, including the alternate of a `/Separation`, `/DeviceN`, or ICCBased space nested in a page `/ColorSpace` resource. The output intent is RGB, so no matching CMYK profile exists. |
| `alternates-not-allowed` | An image dictionary with `/Alternates`. |
| `opi-not-allowed` | An image dictionary with `/OPI`. |
| `blend-mode-not-allowed` | A `/BM` entry whose value is not `Normal`. |
| `embedded-files-need-4f` | `PDFA4` on an input whose catalog has `/Names /EmbeddedFiles`. |
| `4f-needs-embedded-files` | `PDFA4F` on an input with no `/Names /EmbeddedFiles`. |

The scan walks every in-use object, not only the page tree, so an unused font or stream can still refuse the claim. The scan reads dictionaries and not content streams, so a `k` or `K` color operator in page content is not caught. A page or form `/Resources /ColorSpace` dictionary is dictionary-level and is reached, so a nested separation is caught. The preflight does not embed fonts, convert color, or decode LZW.

`make pdfa-check` runs `verapdf --flavour 4` over the PDFs under `sampledata/pdfa/` and skips when the CLI is absent. A `negative/` subfolder is excluded. On 2026-09-25, veraPDF 1.30.2 reported both `path-a4.pdf`, a Spectre write, and the copied `compliant-a4.pdf` valid for PDF/A-4 with 0 failed jobs. That is veraPDF's verdict for those files, not a Spectre certificate.

Two rewrites of the same input and mode return equal bytes, because the packet, the profile, and the trailer `/ID` are fixed.

## Text and fonts

Fonts come from `internal/font` for the standard 14 metrics, encodings, glyph names, and the Type 1 program decoder, and from `golang.org/x/image/font/sfnt` for embedded TrueType and OpenType programs. The model, its sources, and the painting policy are in `documentation/fonts.md`.

PDF text operators: `BT`, `ET`, `Tf`, `Td`, `TD`, `Tm`, `T*`, `Tc`, `Tw`, `Tz`, `TL`, `Ts`, `Tj`, `TJ`, `'`, and `"`. The text state and the text matrices follow ISO 32000-1. `q` and `Q` save and restore the text state, and `BT` resets both matrices.

A simple font reads `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, and `/BaseFont`, with the standard 14 metrics as the fallback when `/Widths` is absent. `/Encoding` names StandardEncoding, WinAnsiEncoding, or MacRomanEncoding, and `/Differences` overrides codes by name. A symbolic Type 1 font with no `/Encoding` starts from the program's built-in encoding. `/ToUnicode` CMaps (`bfchar` and `bfrange`) win over the encoding and the Adobe Glyph List. A Type0 font reads `/Encoding /Identity-H`, a CIDFontType2 descendant, `/CIDToGIDMap`, `/W`, and `/DW`.

The show operators deliver each positioned glyph to the `TextOptions.Sink` seam with its code, Unicode, advance, and device box. `File.ExtractText` reads that sink and lays the glyphs out: lines sort top to bottom, glyphs on one baseline sort left to right, a gap wider than a quarter of the box height inserts a space, and each line ends with CRLF. A font with no `/ToUnicode`, no PDF encoding, and no built-in encoding falls back to the code point.

Painting needs an outline program. The standard 14 ship no outlines and Spectre does not substitute host fonts, so painting a standard 14 glyph returns `invalidfont`. Advances, encodings, and extraction still work, because the glyph box and the text need metrics only. A PFA or PFB `/FontFile`, a `/FontFile2`, or an OpenType `/FontFile3` stream is the outline source when one exists. Bare CFF and Type0 fonts outside Identity-H are out of this tag.

Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. Text tests compare shapes and advances, and extraction tests compare text and geometry, never `CompareRaster` against `gs`.

## PDF subset for the first PDF tag

Phase 06 reads:

- A header starting with `%PDF-`.
- Classic xref tables, then xref streams in a following row of the same phase.
- Content streams through the stream filters in this file: Flate, LZW, ASCII85, ASCIIHex, and RunLength, with predictors 2 and 10 through 15.
- Page content operators `m l c h re S s f f* B B* b b* W W* n q Q cm w J j M i ri gs RG rg g G Do BT ET Tf Td TD Tm T* Tc Tw Tz TL Ts Tj TJ ' " BMC BDC EMC MP DP BX EX`.

Those operators map to the same path and color operations as `moveto` `lineto` `curveto` `closepath` `stroke` `fill` `eofill` `gsave` `grestore` `concat` `setlinewidth` `setrgbcolor` `setgray`.

`Do` paints an image XObject, runs a form XObject, and returns `undefined` with the `Do` operator name when the name, image, or form cannot run. The text operators paint and extract through the font machine above. A font with no outline source paints as `invalidfont`; bare CFF and non-Identity Type0 fonts are out of this tag. A page that uses an unsupported operator does not rasterize as a blank success.

Encrypted files return `invalidaccess`. Unknown filters return `undefined`.

## Alpha and blend modes

The pixmap implements an optional seam beside `Marker`, so the rewrite recorder keeps refusing effects that level 0 cannot write:

```go
type BlendMode uint8

const (
    BlendNormal BlendMode = iota
    BlendMultiply
    BlendScreen
    BlendOverlay
    BlendDarken
    BlendLighten
    BlendColorDodge
    BlendColorBurn
    BlendHardLight
    BlendSoftLight
    BlendDifference
    BlendExclusion
)

type AlphaMarker interface {
    SetFillAlpha(alpha float64)
    SetStrokeAlpha(alpha float64)
    SetBlendMode(mode BlendMode)
}
```

A fill or stroke mark composites as `out = blend(src, dst)*a + dst*(1-a)` per channel, rounded to the nearest byte. `SetFillAlpha` and `SetStrokeAlpha` clamp to 0 through 1, and the pixmap starts at alpha 1 and `BlendNormal`. The 12 separable modes use the ISO 32000-1 table 136 formulas. The four non-separable modes, Hue, Saturation, Color, and Luminosity, have no constant here. `blendModeName` in `internal/pdf` maps a `/BM` name to the enum and returns false for those four, so the PDF layer refuses them by name.

The rewrite recorder does not implement `AlphaMarker`, so `Emit` keeps refusing a page that needs an alpha or blend effect. The `gs` operator is the one content operator that sets the seam: `/CA` sets the stroke alpha, `/ca` the fill alpha, and `/BM` the blend mode, and `q`/`Q` save and restore all three. A recorder marker without the seam turns any of those entries into `undefined in gs`, so level 0 never drops the effect.

## Transparency groups and soft masks

Two more optional seams sit beside `AlphaMarker`. A marker that cannot host them refuses the operator by name, exactly as the recorder refuses a clip.

```go
type SoftMaskMarker interface {
    SetSoftMask(mask []byte)
}

type GroupMarker interface {
    PageSize() (int, int)
    CompositeGroup(group *Pixmap, alpha float64, mode BlendMode)
}
```

The scratch page a group or a soft mask renders into has the page pixel size, and it obeys the same caps as a page: 40,000,000 pixels and a 20,000 pixel side, with `limitcheck` when crossed.

A Form XObject with `/Group << /S /Transparency >>` renders into a scratch `Pixmap` that starts transparent, then composites once through `CompositeGroup`. The group alpha comes from `/CA`, the blend mode from the current `/BM`, and the current fill alpha multiplies the coverage. `/CS` must resolve through the reading-side color space rules, and `/I` and `/K` must be booleans. This subset composites every supported group once into the scratch, which is the isolated shape; `/I false` and `/K true` are read and validated but not given a different composite, a deviation recorded here. A marker without `GroupMarker` refuses the form with `undefined in Do`.

An `/ExtGState /SMask` entry with `/S /Alpha` or `/S /Luminosity` and a `/G` Form XObject builds a per-pixel coverage plane applied to later marks. `/Alpha` takes the group coverage. `/Luminosity` takes the ISO 32000-1 weighted sum `0.30*R + 0.59*G + 0.11*B` of the group composited on black, times the coverage, so an untouched pixel stays 0. `/TR` carries `/Identity` only, `/None` clears the mask, and any other subtype, transfer function, or entry refuses with `undefined in gs`. A marker without `SoftMaskMarker` refuses the same way. The coverage plane is sampled on the device pixel grid the group was rendered with; a later change to the CTM does not move it, a deviation from the ISO 32000-1 mask space.

## Image masks, soft masks, and Decode

An image dictionary with `/ImageMask true` decodes at one bit per sample, most significant bit first, with `/Decode` defaulting to `[0 1]`. A decoded sample at or above 0.5 paints the current fill color; `Decode [1 0]` inverts the mask. The mask caches as an alpha plane and is tinted at each `Do`, because the fill color can change between paints. Only `/FlateDecode` image masks decode in this tag; a CCITT or DCT image mask refuses with that filter name.

An image `/SMask` decodes to an alpha plane, and `/Mask` accepts two forms: a color-key array of `2n` sample ranges, where a pixel inside every range is masked out, and a stream used as a stencil. A soft mask stream decodes as an 8-bit gray image or as a bilevel image mask. A `/Decode` array on a Flate image remaps each sample before the color conversion, for gray, RGB, CMYK, and Indexed spaces; an Indexed space refuses a `/Decode` array because its sample is a table index. A `/Matte` entry is accepted and ignored, a deviation: the matte color is not removed before compositing. The composed image carries the coverage in its alpha channel, and `Pixmap.DrawImage` composites it, so a masked-out pixel leaves the page. The level 0 recorder still refuses any image with `undefined in Do`.

## Shared device interface inside the module

Owned by `internal/graphics`. The pixmap device and the PDF rewrite recorder both implement it. The PostScript operators and the PDF content interpreter call it. They do not call each other's parsers.

```go
type Marker interface {
    Stroke(pts []Point, width, red, green, blue float64)
    Fill(pts []Point, red, green, blue float64, evenOdd bool)
    DrawImage(pic image.Image, ctm Matrix, scale float64)
}
```

`ClipMarker` is optional and lives beside `Marker` in the same package. The pixmap implements it and the rewrite recorder does not, so a clip never reaches a device that cannot write one. It is four methods, each with the clip list first:

```go
type ClipMarker interface {
    StrokeClipped(clips []Clip, pts []Point, width, red, green, blue float64)
    FillClipped(clips []Clip, pts []Point, red, green, blue float64, evenOdd bool)
    DrawGlyphClipped(clips []Clip, mask *image.Alpha, originX, originY int, red, green, blue float64)
    DrawImageClipped(clips []Clip, pic image.Image, ctm Matrix, scale float64)
}

type Clip struct {
    Pts     []Point
    EvenOdd bool
}
```
