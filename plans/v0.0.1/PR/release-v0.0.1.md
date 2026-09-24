## v0.0.1

First release of Spectre PS. `spectreps version` prints `0.0.1`. There is no earlier git tag. This note covers `master` through `96b6864`.

The ledger under `plans/v0.0.1/` numbered its build steps 0.0.1 through 0.0.5. Those numbers are phases inside this release. They are not product releases. v0.0.1 ships all of them.

Spectre PS is a Go library and a `spectreps` command for a small slice of the jobs Ghostscript is used for. It reads a PostScript subset and a path-only PDF, paints pages to pixels, writes a new PDF, stops on the first error, and compares bytes or pixels. It does not link Ghostscript and it does not start `gs`.

- **License:** [MIT](https://github.com/chinmay-sawant/spectrePS/blob/master/LICENSE). Copyright (c) 2026 Chinmay Sawant.
- **Module:** `github.com/chinmay-sawant/spectrePS`, Go 1.26.4.
- **Library:** `github.com/chinmay-sawant/spectrePS/spectreps`.
- **Ledger:** `plans/v0.0.1/00-program.md`.
- **Commit:** `96b6864`.
- **Pull requests:** [#1](https://github.com/chinmay-sawant/spectrePS/pull/1) library and CLI, [#2](https://github.com/chinmay-sawant/spectrePS/pull/2) PostScript paint and raster compare, [#3](https://github.com/chinmay-sawant/spectrePS/pull/3) PDF open, [#4](https://github.com/chinmay-sawant/spectrePS/pull/4) PDF rewrite, [#5](https://github.com/chinmay-sawant/spectrePS/pull/5) validate.

---

### Highlights

| Phase inside v0.0.1 | What shipped |
| --- | --- |
| **Ledger 0.0.1** | Module, `make` targets, public API, CLI, and `CompareFiles`. `cmd/spectreps` calls `internal/cli`, which calls package `spectreps`. |
| **Ledger 0.0.2** | PostScript subset in `documentation/language.md`. RGB pixmap with row 0 at the top. PPM raw and PNG. `CompareRaster`. |
| **Ledger 0.0.3** | PDF subset. Classic xref and xref streams. Flate decode. Path operators paint through the same pixmap as PostScript. |
| **Ledger 0.0.4** | `RewritePDF` writes a new PDF. Default content streams are Flate. Two calls on the same input return the same bytes. No creation date. |
| **Ledger 0.0.5** | `spectreps validate` stops on the first PostScript or PDF error. `deletefile` returns `invalidaccess` and does not delete a neighbor file. |

---

### Install / build

From source, on `master` at `96b6864`:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout 96b6864
make build
```

`make build` writes `bin/spectreps`. `make test` is `go test ./...`. `make lint` is `gofmt`, `golangci-lint`, and the 2,000-line Go file check.

```sh
./bin/spectreps version
./bin/spectreps run -o out.ppm box.ps
./bin/spectreps raster -o page.ppm in.pdf
./bin/spectreps rewrite -o out.pdf in.pdf
./bin/spectreps validate in.pdf
./bin/spectreps compare bytes a.pdf b.pdf
./bin/spectreps compare raster a.ps b.ps
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
    out, err := in.RewritePDF(ctx, doc, spectreps.DefaultRewriteOptions())
    if err != nil {
        panic(err)
    }
    if err := os.WriteFile("out.pdf", out, 0o600); err != nil {
        panic(err)
    }
}
```

---

### What landed in v0.0.1

The headings below follow the ledger phases. All of them ship in this release.

#### Library and CLI

[#1](https://github.com/chinmay-sawant/spectrePS/pull/1)

- Package `spectreps` exports `Version`, `New`, `Close`, `RunPostScript`, `OpenPDF`, `RasterizePage`, `RewritePDF`, `CompareFiles`, and `CompareRaster`.
- `CompareFiles` compares file bytes and reports the first mismatch offset.
- Exit codes are 0 for success, 1 for a job error or a compare mismatch, 2 for usage, and 3 for a read or write that fails before the interpreter runs.
- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` are banned. They return `invalidaccess`.

#### PostScript subset, raster, and pixel compare

[#2](https://github.com/chinmay-sawant/spectrePS/pull/2)

- The scanner, operand stack, dictionary stack, and execution stack live in `internal/ps`. Names resolve when they run, not when a procedure is scanned.
- Path operators paint through `internal/graphics`. Default media is 612 by 792 points at 72 dpi. Anti-aliasing is off.
- User y=0 is the bottom of the page. Pixmap row 0 is the top, so that mark lands on the last row.
- A side above 20000, or a pixel count above 40000000, returns `limitcheck` and allocates no pixmap. `gsave` depth stops at 32.
- `spectreps raster` writes P6 PPM, or PNG when `-o` ends in `.png`. PNG file bytes are not the equality check. `CompareRaster` compares width, height, then RGB bytes, and it ignores stride padding.
- A raster mismatch is exit 1. It is not an interpreter error.

#### PDF open

[#3](https://github.com/chinmay-sawant/spectrePS/pull/3)

- `OpenPDF` requires a `%PDF-` header, walks the page tree, and counts page leaves. `/Count` is not the answer when the tree disagrees.
- Classic xref tables and xref streams both open. Content streams use Flate through `compress/zlib`.
- An unknown filter returns `undefined`. A trailer `/Encrypt` returns `invalidaccess`.
- Page content operators `m l c h re S s f f* n q Q w RG rg g G` paint through the same pixmap as PostScript. A curve is three straight segments, the same rule as the PostScript subset.
- `Tj`, `TJ`, `'`, `"`, and `Do` return `undefined`. The page is not a blank success.
- `spectreps raster -o out.ppm in.pdf` writes page 0.

#### PDF rewrite

[#4](https://github.com/chinmay-sawant/spectrePS/pull/4)

- `RewritePDF` builds a new classic PDF 1.4 from the path subset. It does not copy the input xref, and it is not expected to match Ghostscript `pdfwrite`.
- `DefaultRewriteOptions` Flate-compresses page content streams. The zero `RewriteOptions` leaves those streams uncompressed. Both still match the input pixels under `CompareRaster`.
- The trailer `/ID` comes from the stored stream bytes. The file has no `/Info`, `/CreationDate`, or `/ModDate`. Two calls on the same input return equal bytes.
- `spectreps rewrite -o out.pdf in.pdf` writes that file. Missing `-o` exits 2. `-compress=false` selects the uncompressed option.

#### Validate

[#5](https://github.com/chinmay-sawant/spectrePS/pull/5)

- `spectreps validate good.ps` exits 0 for a subset program. `add` on an empty stack exits 1 and prints `Error: /stackunderflow in add`. The `at file:line:col` tail is omitted because this subset does not record a source position.
- `spectreps validate good.pdf` exits 0 for a phase 06 fixture. Validate opens the file and paints every page. A truncated xref exits 1. An encrypted file exits 1 with `invalidaccess`. `Tj` fails the page.
- `deletefile` exits 1 with `invalidaccess`. A file created next to the input is still there afterward.
- Validate writes no output file and no PDF/A metadata. It does not claim conformance.

---

### What this release does not do

These rows stay in `plans/v0.0.1/10-deferred.md`. Each one needs a new plan file before work starts.

- DCT, CCITT, downsample, and a page raster wrapped in a PDF.
- JPEG and TIFF encoders.
- Fonts, `show`, PDF `Tj` as text, and text extraction.
- `bbox` and `inkcov`.
- PDF/A creation. Validate is not a PDF/A check.
- PDF to PostScript (`ps2write`).
- A `gs` argv compatibility mode.
- PCL, PXL, XPS, and the printer device list.

Rewritten PDF bytes are not compared with Ghostscript. Pixel tests compare Spectre with Spectre.
