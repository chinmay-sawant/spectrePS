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

## Rewrite

`RewritePDF` builds a new PDF from drawing operations on a `Document`. Stream compression uses `compress/flate` when `CompressStreams` is true. The CLI default is true.

The writer omits a wall-clock creation date and uses a fixed trailer id derived from the compressed content bytes. Two calls on the same document return equal buffers. `CompareFiles` is the proof.

The output is not a copy of the input xref, and it is not expected to match `pdfwrite` from any Ghostscript version.

This tag compresses content streams. It does not downsample images, and it does not DCT-encode them. Those are image-model features and they are deferred.

The image side of the compression levels is three helpers in `internal/pdfout`. `ScaleImage` takes any `image.Image` and returns RGBA resampled with the CatmullRom kernel from `golang.org/x/image/draw`; width and height below 1 clamp to 1. `EncodeDCT` wraps `image/jpeg` with the quality clamped to 1 through 100, and `EncodeFlateRGB` writes tightly packed RGB rows, top row first, inside zlib. All three are deterministic, so the same input returns the same bytes. `spectreps rewrite` does not call them until the level wiring lands.

## Bitmap PDF

`ImagePDF` wraps each `PageImage` in one PDF page. The image is 24-bit RGB, 8 bits per component, `/ColorSpace /DeviceRGB`, `/Filter /FlateDecode`. The stored stream is the tightly packed RGB rows, so stride padding is dropped. `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]` points, and a `dpi` of zero or less selects 72.

Each page has one content stream and one image XObject. The content stream is `q W 0 0 H 0 0 cm /Im0 Do Q`, and `/Resources` carries the XObject. Spectre's PDF interpreter still returns `undefined` for `Do`, so rasterizing this output is not the proof. The test decodes the image stream and compares it with `PageImage`.

The writer emits objects in a fixed order, adds no `/Info`, and sets both trailer `/ID` strings to the SHA-256 of the concatenated Flate image streams. Two calls on the same pages return equal buffers, and `CompareFiles` is the proof. The output is not `pdfwrite` and it is not a DCT encode.

## Validate

`validate` runs the interpreter in stop-on-first-error mode.

For PostScript, any `JobError` fails the command. For PDF, repair-and-continue is not the default of this command. A bad xref, a bad stream, or an unsupported operator fails the command with that `JobError`. Rendering commands may later warn and continue. `validate` does not.

`validate` does not write PDF/A metadata and does not claim conformance. PDF/A creation, if it is ever added, is a rewrite option and still not a certificate.

## PDF subset for the first PDF tag

Phase 06 reads:

- A header starting with `%PDF-`.
- Classic xref tables, then xref streams in a following row of the same phase.
- Flate-decoded content streams via `compress/flate`.
- Page content operators `m l c h re S s f f* n q Q w RG rg g G`.

Those operators map to the same path and color operations as `moveto` `lineto` `curveto` `closepath` `stroke` `fill` `eofill` `gsave` `grestore` `setlinewidth` `setrgbcolor` `setgray`.

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
