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

## Box and ink coverage

`MeasureBox` and `MeasureInk` read a finished `PageImage`. They do not paint a second time and they add no operator.

The box is the union of marked pixels in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. `dpi` of 0 selects 72.

Ink output is RGB occupancy: the fraction of pixels marked in each of R, G, and B. The pixmap is RGB, not CMYK, so the CLI line ends in `RGB` and not `CMYK OK`. These numbers are occupancy fractions, not Ghostscript `ink_cov` weighted amounts.

## Image XObjects

The reader walks the xref for in-use objects whose dictionary has `/Subtype /Image`. `ImageObjectNums` returns their object numbers in ascending order. `DecodeImage` returns an `image.Image` or an error. It never returns a blank image for a failed decode.

Two stream forms decode at 8 bits per component:

- `/FlateDecode` with `/DeviceRGB` or `/DeviceGray`, through the same zlib path as content streams. A predictor above 1 is rejected.
- `/DCTDecode` through `image/jpeg`.

Any other filter, color space, or bit depth returns `undefined`, as does a Flate stream whose byte count does not match width by height by components. JPEG is lossy, so decoded pixels are not a byte oracle for the source.

## Rewrite

`RewritePDF` builds a new PDF from drawing operations on a `Document` at level 0. Stream compression uses `compress/flate` when `CompressStreams` is true. The CLI default is true.

The level 0 writer omits a wall-clock creation date and uses a fixed trailer id derived from the compressed content bytes. Two calls on the same document return equal buffers. `CompareFiles` is the proof.

The output is not a copy of the input xref, and it is not expected to match `pdfwrite` from any Ghostscript version.

The pass-through writer (`WriteCopy`) serves levels 1 through 5. It copies every object it does not replace: the page tree, `/Resources`, fonts, annotations, and metadata. An object stored in an object stream is written uncompressed through `SerializeValue`. An override replaces a whole object body by number. The trailer uses the source `/Root`, `/Size` as the highest in-use object number plus one, and `/ID` as the SHA-256 of the written object bodies. Two calls on the same source return equal buffers, and the file carries no `/Info` and no dates.

The level table:

| Level | Name | Content streams | Images |
| --- | --- | --- | --- |
| 0 | Path subset | re-emitted, Flate when `CompressStreams` is true | unchanged |
| 1 | Light | Flate every uncompressed stream | unchanged |
| 2 | Balanced | Flate | Flate and raw image streams re-encoded losslessly, no resample |
| 3 | Medium | Flate | re-encoded as DCT, longest side capped at 1754 px, quality 80 |
| 4 | Strong | Flate | re-encoded as DCT, longest side capped at 1123 px, quality 60 |
| 5 | Hard | Flate | re-encoded as DCT, longest side capped at 842 px, quality 40 |

An image at or below its cap keeps its size. An image Spectre cannot decode, and an image with an `/SMask`, is copied unchanged. A level above 0 ignores `CompressStreams`. Every page reaches the output with the same page count and boxes, because the writer copies the page tree.

The image helpers are three functions in `internal/pdfout`. `ScaleImage` takes any `image.Image` and returns RGBA resampled with the CatmullRom kernel from `golang.org/x/image/draw`; width and height below 1 clamp to 1. `EncodeDCT` wraps `image/jpeg` with the quality clamped to 1 through 100, and `EncodeFlateRGB` writes tightly packed RGB rows, top row first, inside zlib. All three are deterministic, so the same input returns the same bytes. Levels 3 through 5 call `ScaleImage` and `EncodeDCT`, and level 2 calls `EncodeFlateRGB`.

## PostScript output

`WritePostScript` builds a date-free PostScript program from drawing operations on a `Document`. It borrows `pdf.Paint`, so each page carries the same path subset as `RewritePDF` level 0, re-emitted as `setrgbcolor` or `setgray`, `setlinewidth`, `m` and `l`, and `S`, `f`, or `f*`. Coordinates are 72 dpi points.

The program starts with `%!PS-Adobe-3.0` and a fixed `%%BoundingBox: 0 0 612 792`. A prolog defines the short path names in terms of `moveto`, `lineto`, `stroke`, `fill`, and `eofill`, because a bare `m` or `S` is not a PostScript operator. Each page gets a `%%Page` comment and one `showpage`, and the program ends with `%%EOF`. There is no creation date, and two calls on the same document return equal buffers, so `CompareFiles` is the proof.

The PDF painter flattens `c` into straight segments before the recorder sees it, so the writer emits what `pdf.Paint` gives and never writes `curveto`. Text and images wait for the font and image machines: `Tj` returns `undefined in Tj`. The proof is a round trip: a PDF page with `re`/`f`, `m`/`l`/`S`, a curve, and `q`/`Q`/`cm` becomes PostScript, runs back through `RunPostScript` at 72 dpi, and matches under `CompareRaster`.

## Bitmap PDF

`ImagePDF` wraps each `PageImage` in one PDF page. The default image is 24-bit RGB, 8 bits per component, `/ColorSpace /DeviceRGB`, `/Filter /FlateDecode`. `ImagePDFColor` also writes DeviceGray and DeviceCMYK. The stored stream is the tightly packed rows, so stride padding is dropped. `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]` points, and a `dpi` of zero or less selects 72.

Each page has one content stream and one image XObject. The content stream is `q W 0 0 H 0 0 cm /Im0 Do Q`, and `/Resources` carries the XObject. Spectre's PDF interpreter still returns `undefined` for `Do`, so rasterizing this output is not the proof. The test decodes the image stream and compares it with `PageImage`.

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

## Validate

`validate` runs the interpreter in stop-on-first-error mode.

For PostScript, any `JobError` fails the command. For PDF, repair-and-continue is not the default of this command. A bad xref, a bad stream, or an unsupported operator fails the command with that `JobError`. Rendering commands may later warn and continue. `validate` does not.

`validate` does not write PDF/A metadata and does not claim conformance. PDF/A creation, if it is ever added, is a rewrite option and still not a certificate.

## PDF subset for the first PDF tag

Phase 06 reads:

- A header starting with `%PDF-`.
- Classic xref tables, then xref streams in a following row of the same phase.
- Flate-decoded content streams via `compress/flate`.
- Page content operators `m l c h re S s f f* n q Q cm w RG rg g G`.

Those operators map to the same path and color operations as `moveto` `lineto` `curveto` `closepath` `stroke` `fill` `eofill` `gsave` `grestore` `concat` `setlinewidth` `setrgbcolor` `setgray`.

`Tj`, `TJ`, `'`, `"`, and `Do` return `undefined` with the operator name filled in, unless a later phase defines them. A page that uses them does not rasterize as a blank success.

Encrypted files return `invalidaccess`. Unknown filters return `undefined`.

## Shared device interface inside the module

Unexported, owned by `internal/graphics` once phase 04 starts:

```go
type Device interface {
    Stroke(path Path, style Style)
    Fill(path Path, style Style, evenOdd bool)
    ShowPage()
}
```

The pixmap device and the PDF rewrite device both implement it. The PostScript operators and the PDF content interpreter call it. They do not call each other's parsers.
