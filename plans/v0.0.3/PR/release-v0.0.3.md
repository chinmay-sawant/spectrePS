## v0.0.3

Third release of Spectre PS. `spectreps version` prints `0.0.3`. This note covers `master` through `7fda620` plus the version bump in `cbdc0c2`.

The ledger under `plans/v0.0.1/` numbered its build steps 0.0.1 through 0.0.5 for the first release, and the `plans/v0.0.2/` ledger added five phases. The `plans/v0.0.3/` ledger takes the open deferred rows and gives each one a phase. Those numbers are build steps inside their releases. They are not product releases. An earlier folder named v0.0.3 was folded into `plans/v0.0.2/` as phases 4 and 5; this is the later folder.

Spectre PS is a Go library and a `spectreps` command for a small slice of the jobs Ghostscript is used for. It reads a PostScript subset and a PDF subset, paints pages to RGB pixels, writes new PDFs and PostScript, stops on the first error, measures a page, extracts text, and compares bytes or pixels. It does not link Ghostscript and it does not start `gs`. The `spectreps gs` mode scans an allowlisted argv itself.

- **License:** [MIT](https://github.com/chinmay-sawant/spectrePS/blob/master/LICENSE). Copyright (c) 2026 Chinmay Sawant.
- **Module:** `github.com/chinmay-sawant/spectrePS`, Go 1.26.4. Two direct dependencies, `golang.org/x/image` v0.46.0 and `github.com/mrjoshuak/go-jpeg2000` v1.5.12, plus the indirect `golang.org/x/sys` v0.48.0 and `golang.org/x/text` v0.42.0.
- **Library:** `github.com/chinmay-sawant/spectrePS/spectreps`.
- **Ledger:** `plans/v0.0.3/00-program.md`.
- **Commit:** `cbdc0c2`.
- **Pull requests:** [#11](https://github.com/chinmay-sawant/spectrePS/pull/11) the ten phases of `plans/v0.0.3`.

---

### Highlights

| Phase inside v0.0.3 | What shipped |
| --- | --- |
| **Phase 1** | `MeasureInkAmount` and the `ink_cov` command, the weighted RGB amounts beside the occupancy `inkcov`. |
| **Phase 2** | CCITT Group 3 and Group 4 image streams decode through `x/image/ccitt`. |
| **Phase 3** | JPEG2000 image streams decode through the pure-Go `go-jpeg2000` module. |
| **Phase 4** | The pass-through writer stops copying dead `/XRef` and `/ObjStm` containers, with an optional packed form. |
| **Phase 5** | `Do` paints image XObjects, so Spectre rasterizes its own image PDFs. |
| **Phase 6** | `spectreps ps` writes date-free PostScript for path pages. |
| **Phase 7** | `spectreps gs` accepts an allowlisted `gs` argv and `raster -format` selects the encoder. |
| **Phase 8** | `rewrite -pdfa 4|4f` writes a PDF/A-4 claim behind a refusal preflight. |
| **Phase 9** | The font and text machine, with text painting, PostScript `show`, and `spectreps text` extraction. |
| **Phase 10** | PDF/UA-2 preservation at levels 1 to 5, the UA-2 preflight, and tagged-input refusals. |
| **Closure** | The merged tree passed `make lint` and `make test`; the transcripts are in `plans/v0.0.3/v0.0.3-closure.md`. |

---

### Install / build

From source, on the release branch at `cbdc0c2`:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout cbdc0c2
make build
```

`make build` writes `bin/spectreps`. `make test` is `go test -p $(nproc) ./...`. `make lint` is `gofmt`, `golangci-lint`, and the 2,000-line Go file check. `make pdfa-check` and `make pdfua2-check` run veraPDF when a copy exists and are not part of `make test`.

A two-line text fixture and a two-page image fixture are checked in under `sampledata/fixtures/`.

```sh
./bin/spectreps version
./bin/spectreps ink_cov -w 200 -h 200 -r 72 in.pdf
./bin/spectreps raster -format png -o page.png in.pdf
./bin/spectreps raster -pages 1-2 -o page-%d.png sampledata/fixtures/gs-argv-input.pdf
./bin/spectreps pdfimage -colorspace gray -o page.pdf in.pdf
./bin/spectreps rewrite -level 5 -o small.pdf in.pdf
./bin/spectreps rewrite -pdfa 4 -o compliant.pdf in.pdf
./bin/spectreps ps -o page.ps in.pdf
./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
./bin/spectreps gs -sDEVICE=png16m -sOutputFile=page.png in.pdf
./bin/spectreps compare raster a.pdf b.pdf
```

Library. The new symbols are `PDFA`, `ExtractText`, `WritePostScript`, `Document.Tagged`, and `MeasureInkAmount`:

```go
package main

import (
    "context"
    "errors"
    "fmt"
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
    fmt.Printf("pages=%d tagged=%v\n", doc.PageCount(), doc.Tagged())

    text, err := in.ExtractText(ctx, doc, 0)
    if err != nil {
        panic(err)
    }
    fmt.Print(text)

    opt := spectreps.DefaultRewriteOptions()
    opt.Level = 2
    opt.PDFA = spectreps.PDFA4
    out, err := in.RewritePDF(ctx, doc, opt)
    if err != nil {
        var job spectreps.JobError
        if errors.As(err, &job) && job.Op == "PDFA" {
            panic("PDF/A preflight refused: /" + job.Msg)
        }
        panic(err)
    }
    if err := os.WriteFile("compliant.pdf", out, 0o600); err != nil {
        panic(err)
    }
}
```

---

### What landed in v0.0.3

#### Weighted ink coverage

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- `MeasureInkAmount(img PageImage) Ink` returns the mean per-channel complement `(1/N) * sum((255 - c)/255)` over `Width * Height`, with stride padding ignored. White is zero and black is one on every channel. A zero-size image returns the zero `Ink`.
- `spectreps ink_cov [-w points] [-h points] [-r dpi] [-pages range] file` prints `Page N` and three percentages with five decimals ending in `RGB`, the same shape as `inkcov`. The amount is printed times 100, matching the measured Ghostscript `ink_cov` device source rather than its manual example. `inkcov` occupancy is unchanged.
- A 20 by 20 page holding a quarter-cyan square prints `25.00000 0.00000 0.00000 RGB`. A byte-128 gray page prints `49.80392 49.80392 49.80392 RGB`.

#### CCITT and JPEG2000 decode

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- `/CCITTFaxDecode` decodes through `golang.org/x/image/ccitt`, already in the `x/image` module. `/K < 0` selects Group 4, and `/K == 0` with `/EndOfLine true` selects Group 3. `/K > 0`, a missing end-of-line marker, an 8-bit depth, `DeviceRGB`, and a stream truncated inside a row are refused with `undefined` or `syntaxerror`.
- `/JPXDecode` decodes through `github.com/mrjoshuak/go-jpeg2000` v1.5.12, Apache-2.0 and pure Go. The branch runs before the shared `/BitsPerComponent` and `/ColorSpace` checks, because the codestream carries the color and the precision. A failed decode is `syntaxerror`, never a blank image.
- `Columns * Rows` above the 32 MiB decoded cap is `limitcheck` before allocation for CCITT; a JPX header that declares more decoded sample bytes than the cap is `limitcheck` too.
- Level 2 decodes CCITT and re-encodes it as Flate RGB. Levels 3 to 5 decode CCITT and JPEG2000 and re-encode them as DCT with the level caps and qualities. An undecodable stream still copies through at every level.
- The fixtures are `sampledata/fixtures/jpx-rgb.j2k`, `jpx-gray.jp2`, `ccitt-g3.bin`, and `ccitt-g4.bin`, with generation commands and SHA-256 digests in `sampledata/fixtures/README.md`.

#### Painting `Do`

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- The PDF content scanner returns name text, `/Resources` resolves through the nearest `/Pages` ancestor, and `Do` requires `/Subtype /Image` and decodes each name once per painted page.
- The device seam `graphics.Marker.DrawImage(pic image.Image, ctm Matrix, scale float64)` maps the image unit square through the current matrix and the paint scale and samples nearest neighbor with image row 0 at the top. RGB and gray paint; alpha is ignored.
- Spectre now rasterizes its own `pdfimage` output. `RunPostScript` to `ImagePDF` to `OpenPDF` to `RasterizePage` matches the source `PageImage` under `CompareRaster` for RGB and gray.
- Level 0 keeps the gate: a page that paints an image returns `undefined in Do` instead of a path-only file that dropped or outlined it. Levels 1 to 5 carry the image through the pass-through writer under the level's image policy.

#### Compression writer

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- `pdfout.WriteCopy` skips a source `/Type /XRef` or `/Type /ObjStm` container. Those object numbers become free xref rows, and the trailer `/ID` covers only the written bodies in object-number order.
- `CopyOptions.PackObjects` writes the optional packed form: `%PDF-1.5`, every non-stream body in one Flate `/Type /ObjStm`, and a Flate `/Type /XRef` stream with `W [1 4 2]`. The `/ID` is computed before packing, so bytes do not depend on the mode. The levels 1 to 5 path does not select it.
- Phase 4 row 1.2 stays open with measured numbers. `sampledata/compress/whatisthis.pdf` is 596,341 bytes; levels 1 and 2 were 614,343 bytes before the container skip, 610,034 after it, and 596,491 with the packed writer, still 150 bytes over. The source packs 78 non-stream objects, so the not-larger-than-input guard cannot pass and was not added.

#### PostScript output

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- New `internal/psout`. `(*Instance).WritePostScript(ctx, doc, PostScriptOptions)` re-emits the same path subset as `RewritePDF` level 0: `setrgbcolor` or `setgray`, `setlinewidth`, `m` and `l`, and `S`, `f`, or `f*` in 72 dpi points.
- `spectreps ps -o out.ps in.pdf` writes a date-free `%!PS-Adobe-3.0` program with a prolog that defines the short names, a fixed 612 by 792 box, one `%%Page` and one `showpage` per page, and mode `0o600`. Two calls return equal bytes.
- Text and images are not emitted: a page with `Tj` exits 1 with `undefined in Tj` instead of a silently blank program.
- The proof is a round trip: a page with `re`/`f`, `m`/`l`/`S`, a curve, and `q`/`Q`/`cm` becomes PostScript, runs back through `RunPostScript` at 72 dpi, and matches under `CompareRaster`.

#### gs argv mode

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- `spectreps gs` scans an allowlisted argv and routes the job to the existing subcommands. The grammar is in the new `documentation/gs-argv-grammar.md`. A switch is accepted when it maps onto behavior Spectre already guarantees, accepted and ignored when the behavior is always on, and rejected when it would silently change pixels.
- Devices accepted: `ppmraw`, `png16m`, `jpeg`, `tiff24nc`, `bbox`, `inkcov`, `pdfimage24`, and `pdfwrite`. Switches include `-sOutputFile`, `-dFirstPage`, `-dLastPage`, `-sPageList` as one contiguous ascending list, `-r`, the point-size switches, `-g` at 72 dpi, `-dJPEGQ`, and `-f` for the input.
- `-dBATCH`, `-dNOPAUSE`, `-q`, `-dSAFER`, and `-dFIXEDMEDIA` are accepted and ignored. `-c`, `-dNOSAFER`, `-dDELAYSAFER`, stdin, `@file`, multiple inputs, stream output, and every unmapped switch exit 2.
- `raster -format ppm|png|jpeg|tiff` selects the encoder, and a `gs` device wins over the `-o` suffix. The end-to-end proof is a `-sDEVICE=pdfwrite` run against `sampledata/fixtures/gs-argv-input.pdf` that writes a PDF which reopens and rasterizes both pages.

#### PDF/A-4

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- New `internal/pdfa`. `RewriteOptions.PDFA` selects `PDFA4`, the base claim, or `PDFA4F`, the embedded-file claim. `spectreps rewrite -pdfa 4|4f` exposes the same choice. `PDFA4` is refused when the catalog carries `/Names /EmbeddedFiles`, and `PDFA4F` is refused when it does not.
- The writer emits `%PDF-2.0` with a binary marker above byte 127, keeps `/ID`, writes no `/Encrypt`, and appends `/Metadata` and one `/S /GTS_PDFA1` output intent with a generated D50 sRGB matrix-shaper ICC profile and no `/DestOutputProfileRef`. The XMP packet is static UTF-8 with `pdfaid:part` 4, `pdfaid:rev` 2020, and the `F` letter for 4f, with no dates, so two runs return equal bytes.
- The refusal policy mirrors Ghostscript `PDFACompatibilityPolicy` 2. A known violation returns `Error: /rule in PDFA`, exits 1, and writes no output file. The nine rules are `font-not-embedded`, `lzwdecode`, `filter-not-allowed`, `cmyk-without-profile`, `alternates-not-allowed`, `opi-not-allowed`, `blend-mode-not-allowed`, `embedded-files-need-4f`, and `4f-needs-embedded-files`.
- `make pdfa-check` runs veraPDF `--flavour 4` over `sampledata/pdfa/` and applies the same `negative/` exclusion filter as the UA-2 check. veraPDF 1.30.2 reported `compliant="2" nonCompliant="0"` on 2026-09-25 for `path-a4.pdf`, a Spectre write, and the copied `compliant-a4.pdf`. The claim stays a profile preflight, not a certificate.

#### Font model, text painting, and extraction

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- New `internal/font` holds generated tables: the standard 14 advances from the Adobe Core 14 AFM files, StandardEncoding, WinAnsiEncoding, and MacRomanEncoding, and Adobe Glyph List names. The generator is build-ignored and pins source commits and SHA-256 digests. Sources and licenses are in the new `documentation/fonts.md`. Helvetica `A` is 667 and `space` is 278; Courier is 600 for every glyph.
- The PDF text operators `BT`, `ET`, `Tf`, `Td`, `TD`, `Tm`, `T*`, `Tc`, `Tw`, `Tz`, `TL`, `Ts`, `Tj`, `TJ`, `'`, and `"` run, and `q`/`Q` saves the text state. A simple font reads `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, `/BaseFont`, `/Encoding` with `/Differences`, and `/ToUnicode` with `bfchar` and `bfrange`. A Type0 font reads Identity-H, a CIDFontType2 descendant, `/CIDToGIDMap`, `/W`, and `/DW`.
- Embedded TrueType `/FontFile2` and OpenType `/FontFile3` programs parse through `golang.org/x/image/font/sfnt` and paint through `x/image/vector`. A standard 14 glyph, a Type 1 `/FontFile`, and a bare CFF paint as `invalidfont`, because Spectre ships no substitute outlines; advances, encodings, and extraction still work for those fonts.
- PostScript `findfont`, `scalefont`, `setfont`, and `show` drive the same machine.
- `(*Instance).ExtractText(ctx, doc, pageIndex)` and `spectreps text [-pages range] file.pdf` return the laid-out text: lines top to bottom, glyphs left to right, a space for a gap wider than a quarter box, CRLF per line, and a code-point fallback when neither `/ToUnicode` nor the encoding names the code.
- Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. Text tests compare shapes and advances, and extraction tests compare text and geometry.
- Type 1 charstrings and `seac` were deferred to `plans/v0.0.1/10-deferred.md` 10.1, and bare CFF stays with them. CFF inside an OpenType wrapper is in scope.

#### PDF/UA-2

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- `internal/pdf/structtree.go` parses `/MarkInfo`, `/StructTreeRoot`, `/K`, `/S`, `/P`, `/Pg`, `/MCID`, `/Alt`, `/ActualText`, `/Lang`, `/Namespaces`, `/RoleMap`, `/RoleMapNS`, and `/ParentTree` into typed values. The tree walk caps depth at 64, the role chain at 32, and a cycle is `limitcheck`.
- `File.HasStructTree`, `File.StructTree`, and the public `Document.Tagged` expose the model. Levels 1 to 5 preserve a tagged input: tree shape, MCIDs, `/Alt`, `/ActualText`, and `/Lang` survive, and a tagged PDF 2.0 source keeps its header block with the binary marker instead of leaving as a 1.4 shell.
- Generated writers refuse a tagged input instead of dropping the tree: `rewrite -level 0` exits 1 with `Error: /tagged in RewritePDF`, and `pdfimage` exits 1 with `Error: /tagged in ImagePDF`.
- `internal/pdfa` adds `ReadUA2`, `UA2Write`, `UA2XMP`, `UA2ExtraObjects`, `UA2Catalog`, and `PreflightUA2`. A `pdfuaid` claim is never written from nothing: it is kept when the source carried it and added only after a passing preflight. The nine checks are `ua2-marked`, `ua2-structtree`, `ua2-document`, `ua2-lang`, `ua2-displaydoctitle`, `ua2-pdfuaid`, `ua2-title`, `ua2-rolemap`, and `ua2-mcid`, reported as `Error: /ua2-<rule> in PDFUA`.
- `make pdfua2-check` runs veraPDF `--flavour ua2 --format json` over `sampledata/pdfua2/` and excludes `negative/`. veraPDF 1.30.2 reported 1727 passed rules and 0 failed rules for each of `tagged-ua2.pdf` and `compliant-ua2.pdf` on 2026-09-25. Tag generation, reading order, and role assignment stay out, and `validate` does not call the preflight yet.

#### Public API

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- New functions: `MeasureInkAmount`, `WritePostScript`, `ExtractText`, and `Document.Tagged`. New types: `PostScriptOptions` and `PDFAMode` with `PDFANone`, `PDFA4`, and `PDFA4F`. New field: `RewriteOptions.PDFA`. No existing signature changed. `documentation/public-api.md` carries the contracts.
- `RewriteOptions` gained one field, so keyed literals compile unchanged and an unkeyed literal must add the third value. The zero value `PDFANone` leaves the claim off.

#### Tests, ledgers, and release plumbing

[#11](https://github.com/chinmay-sawant/spectrePS/pull/11)

- Of the 72 rows with a `Proof:` field, 70 are checked and pass. Phase 4 row 1.2 stays open by design with its measurements, and phase 9 row 3.6 (Type 1) went back to `plans/v0.0.1/10-deferred.md` 10.1. Phase 7 records its nine outcomes inline with the same date.
- `plans/v0.0.3/v0.0.3-closure.md` records `make lint` and `make test` exit 0 on the merged tree on 2026-09-25.
- New docs: `documentation/fonts.md` and `documentation/gs-argv-grammar.md`. The rest of the documentation set was refreshed for the new surface, and the claim wording stays "profile preflight" and "preflight only".
- `plans/v0.0.1/10-deferred.md` moved the landed rows to 10.4 and leaves Type 1 and PDF/UA-2 tag generation as the open text rows; the printer languages stay out of product.
- The module stays pure Go: no cgo, no `os/exec`, and no Ghostscript linkage. A `CGO_ENABLED=0 go build ./...` is recorded in the phase rows.

---

### What this release does not do

These rows stay out of this tag: `plans/v0.0.1/10-deferred.md`, the out-of-scope table in `plans/v0.0.3/00-program.md`, and the limits in `documentation/language.md` and `documentation/covered-and-not-covered.md`. Each one needs a new plan file before work starts.

- Type 1 charstrings, `seac`, and bare CFF outlines. CFF inside an OpenType wrapper is in scope.
- PDF/UA-2 tag generation. Reading order and role assignment are not implemented; preservation and preflight are.
- Font embedding, subsetting, and writing fonts. Font hinting stays out.
- JPEG2000 encoding. Decode is the job.
- Full PDF 1.7 and PDF 2.0, including transparency, optional content, encryption, and color management.
- `bind`, `save`, `restore`, `clip`, the `W` clipping operator, and PostScript filters other than Flate.
- PCLm output, spot-color separations, and PDF info, linearization, and output encryption.
- PCL, PXL, XPS, and the printer device list from `gs -h`.
- `validate` does not call `PreflightUA2` yet.

Known limitations:

- Levels 1 and 2 grow an already-packed file. The input is 596,341 bytes, the classic output is 610,034 after the container skip (614,343 before it), and the packed writer reaches 596,491. The not-larger-than-input guard cannot pass and was not added.
- Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. The oracle is text and geometry.
- A standard 14 font, a Type 1 `/FontFile`, or a bare CFF paints as `invalidfont`. Advances and extraction still work.
- The PDF/A-4 claim and the PDF/UA-2 checks are profile preflight, never certification.
- A tagged document is refused by `rewrite -level 0` and `pdfimage`, because neither writer can keep the tree.
- The content interpreter has no `W` clipping operator, so painting a PDF that uses clipping returns `undefined in W` even though rewrite copies the content through.

Rewritten PDF bytes are not compared with Ghostscript. Pixel tests compare Spectre with Spectre.
