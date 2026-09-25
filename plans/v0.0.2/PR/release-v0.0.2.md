## v0.0.2

Second release of Spectre PS. `spectreps version` prints `0.0.2`. This note covers `master` through `5c13460`.

The ledger under `plans/v0.0.1/` numbered its build steps 0.0.1 through 0.0.5 for the first release. The `plans/v0.0.2/` ledger adds five phases. Those numbers are build steps inside this release. They are not product releases.

Spectre PS is a Go library and a `spectreps` command for a small slice of the jobs Ghostscript is used for. It reads a PostScript subset and a PDF subset, paints pages to pixels, writes new PDFs, stops on the first error, measures a page, and compares bytes or pixels. It does not link Ghostscript and it does not start `gs`.

- **License:** [MIT](https://github.com/chinmay-sawant/spectrePS/blob/master/LICENSE). Copyright (c) 2026 Chinmay Sawant.
- **Module:** `github.com/chinmay-sawant/spectrePS`, Go 1.26.4. One dependency, `golang.org/x/image` v0.46.0, for TIFF output and image scaling.
- **Library:** `github.com/chinmay-sawant/spectrePS/spectreps`.
- **Ledger:** `plans/v0.0.2/00-program.md`.
- **Commit:** `5c13460`.
- **Pull requests:** [#7](https://github.com/chinmay-sawant/spectrePS/pull/7) quick-win ledger, [#8](https://github.com/chinmay-sawant/spectrePS/pull/8) box, ink coverage, JPEG, and bitmap PDF, [#9](https://github.com/chinmay-sawant/spectrePS/pull/9) page ranges, TIFF, image colors, `gs` map, and PDF compression.

---

### Highlights

| Phase inside v0.0.2 | What shipped |
| --- | --- |
| **Phase 1** | `MeasureBox` and `MeasureInk`, with the `bbox` and `inkcov` commands. |
| **Phase 2** | JPEG raster from `raster -o`. |
| **Phase 3** | `pdfimage` writes one 24-bit RGB image per page into a new PDF. |
| **Phase 4** | PDF page ranges, PDF inputs in `compare raster`, TIFF raster, gray and CMYK image PDF, and the bounded `gs` switch map. |
| **Phase 5** | PDF compression levels 1 to 5 over any PDF the reader can open, built on an object pass-through writer. |
| **Closure** | `make test` runs packages with `-p`, from `nproc`. The lint and test transcripts are in the closure file. |

---

### Install / build

From source, on `master` at `5c13460`:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout 5c13460
make build
```

`make build` writes `bin/spectreps`. `make test` is `go test -p $(nproc) ./...`. `make lint` is `gofmt`, `golangci-lint`, and the 2,000-line Go file check.

```sh
./bin/spectreps version
./bin/spectreps bbox -w 200 -h 200 -r 72 square.ps
./bin/spectreps inkcov -w 200 -h 200 -r 72 square.ps
./bin/spectreps raster -o page.jpg -jpegq 90 -w 200 -h 200 -r 72 square.ps
./bin/spectreps raster -o page.tif -tiffcompress deflate -w 200 -h 200 -r 72 square.ps
./bin/spectreps raster -pages 2-4 -o page-%d.png in.pdf
./bin/spectreps pdfimage -colorspace gray -o page.pdf square.ps
./bin/spectreps rewrite -level 5 -o small.pdf in.pdf
./bin/spectreps compare raster a.pdf b.pdf
```

Library:

```go
package main

import (
    "context"
    "os"

    "github.com/chinmay-sawant/spectrePS/spectreps"
)

func main() {
    ctx := context.Background()
    in, err := spectreps.New()
    if err != nil {
        panic(err)
    }
    defer in.Close()

    src, err := os.ReadFile("in.pdf")
    if err != nil {
        panic(err)
    }
    doc, err := in.OpenPDF(ctx, src)
    if err != nil {
        panic(err)
    }
    opt := spectreps.DefaultRewriteOptions()
    opt.Level = 5
    out, err := in.RewritePDF(ctx, doc, opt)
    if err != nil {
        panic(err)
    }
    if err := os.WriteFile("small.pdf", out, 0o600); err != nil {
        panic(err)
    }
}
```

---

### What landed in v0.0.2

#### Box and ink coverage

[#8](https://github.com/chinmay-sawant/spectrePS/pull/8)

- `MeasureBox(img PageImage, dpi float64) (Box, bool)` returns the union of marked pixels in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. Stride padding is ignored, and `dpi` of 0 or less selects 72.
- `MeasureInk(img PageImage) Ink` returns RGB occupancy over `Width * Height`. A white page is `0 0 0`, a cyan page is `1 0 0`, and a red page is `0 1 1`. It is not a CMYK report and not Ghostscript `ink_cov`.
- `spectreps bbox` prints `%%BoundingBox` with floored minima and ceilinged maxima, then `%%HiResBoundingBox`. A blank page prints a zero box.
- `spectreps inkcov` prints `Page N` and three five-decimal fractions ending in `RGB`.
- Both commands accept `-w`, `-h`, and `-r`, and they rasterize every page of a PDF input.

#### JPEG raster

[#8](https://github.com/chinmay-sawant/spectrePS/pull/8)

- A `.jpg` or `.jpeg` output path encodes the pixmap with `image/jpeg`. `-jpegq` defaults to 75 and is clamped to 1 through 100. `run` and `compare raster` reject the flag.
- JPEG file bytes are not an equality oracle. `CompareRaster` and `PageImage` are.

#### Bitmap PDF

[#8](https://github.com/chinmay-sawant/spectrePS/pull/8)

- `ImagePDF` wraps each `PageImage` in one PDF page. The image is 24-bit RGB, 8 bits per component, `/ColorSpace /DeviceRGB`, `/Filter /FlateDecode`. `/MediaBox` is `[0 0 width*72/dpi height*72/dpi]`.
- The trailer `/ID` is the SHA-256 of the concatenated Flate image streams, both strings equal. No `/Info`, no `CreationDate`, no `ModDate`. Two calls return equal bytes.
- `spectreps pdfimage -o out.pdf` paints a PostScript or PDF input and writes the file at mode `0o600`.
- `Do` still returns `undefined` in the reader, so Spectre cannot rasterize its own image PDF yet. The tests decode the image stream from the bytes.

#### Page ranges and PDF raster

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- `-pages N|A-B|A-|-B` is accepted by `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster`. It is 1-based and inclusive, and an omitted flag selects every page.
- `raster` paints every page of a PDF input. It painted only page 0 before.
- `compare raster` accepts PDFs, opens them with `OpenPDF`, and compares the selected pages pairwise. Different selected page counts print `mismatch length`.
- A `%d` output path numbers emitted pages from 1, matching Ghostscript. A range that selects input pages 3 through 5 writes `page-1` through `page-3`.
- A start below 1 or past the last page returns `rangecheck`, exit 1. An end past the last page clamps. For PostScript the run executes every page and the filter applies after it, so a failing page outside the range still fails the command.

#### TIFF raster

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- A `.tif` or `.tiff` output path encodes the pixmap with `golang.org/x/image/tiff` as baseline TIFF. `-tiffcompress none|deflate` selects the compression and defaults to `deflate`.
- The library writes no compression or Deflate only. LZW and CCITT G3 and G4 are decode-only there, so the flag does not offer them.
- TIFF file bytes are not an equality oracle. The test decodes with `tiff.Decode` and compares pixels.

#### Gray and CMYK image PDF

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- `pdfimage -colorspace rgb|gray|cmyk` picks `/DeviceRGB` at 24 bits, `/DeviceGray` at 8 bits, or `/DeviceCMYK` at 32 bits. The default is `rgb`, so earlier bytes do not change.
- DeviceGray is BT.601 luma: `Y = round(0.299R + 0.587G + 0.114B)`.
- DeviceCMYK is K-first: `K = 1 - max(r,g,b)`, then `C = (1-r-K)/(1-K)` and the same for M and Y. Red `(255,0,0)` becomes gray `76` and CMYK `0 255 255 0`.
- The public API adds `ImageColor` and `ImagePDFColor`.

#### gs switch map

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- `documentation/gs-argv-mapping.md` maps the switches the subcommands can already express, and names the ones that stay rejected. It is a mapping document, not a compatibility mode. The binary still rejects a `gs` argv with exit 2.

#### Compression foundations

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- The PDF content interpreter composes a six-number matrix for `cm`, saved and restored by `q` and `Q`. A user point transforms through the matrix before the device scale, and the stroke width scales with it.
- `internal/pdf` exposes object access: `ObjectCount`, `RootNum`, `RawObject`, `ObjectValue`, and `SerializeValue`.
- `pdfout.WriteCopy` copies every source object it does not rewrite, with `CopyOptions.Overrides` replacing individual object bodies. Page trees, resources, fonts, annotations, and metadata survive.
- The reader discovers and decodes image XObjects: `ImageObjectNums` and `DecodeImage`, for Flate `DeviceRGB` and `DeviceGray` at 8 bits, and DCT through `image/jpeg`.
- `ScaleImage` resamples with CatmullRom. `EncodeDCT` and `EncodeFlateRGB` re-encode an image.

#### Compression levels

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- `spectreps rewrite -level N` accepts 0 through 5. A missing `-level` selects 0, which is byte for byte the previous writer.
- Levels 1 to 5 use the pass-through writer. Level 1 Flates uncompressed content streams. Level 2 also re-encodes Flate and raw image streams losslessly, with no resample. Levels 3 to 5 also re-encode images as DCT with a longest-side cap and a JPEG quality.
- The caps and qualities are 1754 px at 80, 1123 px at 60, and 842 px at 40. An image at or below its cap keeps its size, and the aspect ratio is kept.
- Text, fonts, and annotations survive because the writer copies them. CCITT and JPEG2000 streams, `/SMask` images, and images the decoder cannot read copy through unchanged.
- Two runs at the same level return equal bytes, and the output carries no `CreationDate` or `ModDate`.
- On `sampledata/compress/whatisthis.pdf`, a PDF 1.7 file with text, a `cm` matrix, and one 2480 by 3508 DCT image, the sizes are:

| Level | Bytes |
| ---: | ---: |
| input | 596,341 |
| 1 | 614,343 |
| 2 | 614,343 |
| 3 | 229,469 |
| 4 | 108,116 |
| 5 | 85,760 |

- Levels 1 and 2 are slightly larger than the input because the writer rebuilds the container and the image is already DCT. Levels 3 to 5 recover it. `sampledata/compress/path.pdf` compresses from 8,450 bytes to 667 bytes, and levels 1 to 5 are byte-identical for it because it has no images.

#### Tests, ledgers, and release plumbing

[#9](https://github.com/chinmay-sawant/spectrePS/pull/9)

- Every plan row is checked with a passing proof. New tests cover page ranges, PDF compare, TIFF, image colors, the content matrix, the copy writer, image decode and scale, the levels, and the sampledata acceptance.
- `documentation/features.md` is new. `test.md`, `devices.md`, `cli.md`, `public-api.md`, `covered-and-not-covered.md`, `folder-structure.md`, `copyright-and-rewrite.md`, and the root `README.md` carry the new surface.
- The v0.0.3 plan folder was folded into `plans/v0.0.2/` as phases 4 and 5, because no tag existed between them.
- `spectreps version` prints `0.0.2`.

---

### What this release does not do

These rows stay in `plans/v0.0.1/10-deferred.md`. Each one needs a new plan file before work starts.

- Paint `Do` and read images into a raster. The reader decodes image XObjects, but the content interpreter still returns `undefined` for `Do`.
- Decode CCITT and JPEG2000 image streams. They copy through unchanged.
- Report `ink_cov` weighted amounts. It needs a named weighting model.
- Extract text, `show`, and PDF `Tj`. Fonts are a separate machine.
- Create PDF/A-1b, PDF/A-2b, or PDF/A-3b. Validate is not a PDF/A check.
- Write PostScript with a `ps2write` style device.
- Accept the full `gs` argv grammar. A bounded switch map landed; the grammar stays out.
- PCL, PXL, XPS, and the printer device list.

Known limitations:

- Levels 1 and 2 grow an already-optimized file by about 3%, because the copy writer rebuilds the container and copies the source `/Type /XRef` and `/Type /ObjStm` objects as dead weight.
- Spectre cannot rasterize its own `pdfimage` output, because `Do` is still `undefined`.

Rewritten PDF bytes are not compared with Ghostscript. Pixel tests compare Spectre with Spectre.
