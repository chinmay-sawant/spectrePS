## Summary

Implement the ten phases of `plans/v0.0.3`: weighted `ink_cov`, CCITT and JPEG2000 decode, image painting through `Do`, a container-clean writer with an optional packed form, PDF-to-PostScript, the `gs` argv mode, a PDF/A-4 claim with a refusal preflight, the font and text machine, and PDF/UA-2 preservation and preflight. The work adds four commands, two flags, six public API declarations, one new option field, and three packages: `internal/font`, `internal/pdfa`, and `internal/psout`. Existing defaults stay, with two named behavior changes: level 0 and `pdfimage` refuse a tagged input instead of dropping its tags, and the pass-through writer drops dead `/XRef` and `/ObjStm` containers.

---

## Motivation / context

- Plans: `plans/v0.0.3/00-program.md` and the ten phase files in `plans/v0.0.3/`, with the merged-tree gate in `plans/v0.0.3/v0.0.3-closure.md`. Parent ledger: `plans/v0.0.1/10-deferred.md`.
- v0.0.2 is released. v0.0.3 takes every open deferred row except the out-of-product printer languages, PCL, PXL, XPS, and PDF/UA-2 tag generation.
- Two v0.0.3 phase rows stay open. Phase 4 row 1.2 is open because the classic writer cannot beat the source size on an already packed sample; the numbers are in the phase file. Phase 9 row 3.6, Type 1 charstrings and `seac`, went back to `plans/v0.0.1/10-deferred.md` 10.1 with the Type 1 scope decision.
- Issues: see **Related issues**

---

## Changes

### Weighted ink coverage

- `MeasureInkAmount(img PageImage) Ink` returns the mean complement of each channel over `Width * Height`: `(1/N) * sum((255 - c)/255)`, with stride padding ignored. White is zero and black is one on every channel. `spectreps/measure.go`.
- `spectreps ink_cov [-w points] [-h points] [-r dpi] [-pages range] file` prints `Page N` and three percentages with five decimals ending in `RGB`, the same shape as `inkcov`. The command prints the amount times 100 to match the measured `ink_cov` device source, not the manual example. `inkcov` occupancy is unchanged.
- The model, both worked examples, and the manual-versus-source note are in `documentation/devices.md`.

### Image decoding

- `/CCITTFaxDecode` decodes through `golang.org/x/image/ccitt` in `internal/pdf/ccitt.go`. `/K < 0` selects Group 4, `/K == 0` with `/EndOfLine true` selects Group 3, and `/K > 0`, a missing end-of-line marker, an 8-bit depth, `DeviceRGB`, and a stream truncated inside a row are refused with `undefined` or `syntaxerror`. `Columns * Rows` above the 32 MiB decoded cap is `limitcheck` before allocation.
- `/JPXDecode` decodes through the new pure-Go `github.com/mrjoshuak/go-jpeg2000 v1.5.12` in `internal/pdf/jpx.go`. The branch runs before the shared parameter checks because the codestream carries color and precision. A failed decode is `syntaxerror`, a declared size past the 32 MiB cap is `limitcheck`, and neither is a blank image.
- Level 2 decodes CCITT and re-encodes it as Flate RGB. Levels 3 to 5 decode CCITT and JPEG2000 and re-encode them as DCT. DCT and JPX streams copy through at level 2, and any stream Spectre cannot decode copies through at every level.
- Fixtures under `sampledata/fixtures/` hold the JPX codestream and container written by Pillow with OpenJPEG, a `README.md` with the JPX generation commands and SHA-256 digests, and the CCITT binary streams with the `gen_ccitt.py` generator script.

### Painting images

- The PDF content scanner now returns name text, `/Resources` resolves through the nearest `/Pages` ancestor, and `Do` requires `/Subtype /Image` and decodes each image once per page. `internal/pdf/do.go`, `internal/pdf/resource.go`, `internal/pdf/image.go`.
- `graphics.Marker` gains `DrawImage(pic image.Image, ctm Matrix, scale float64)`. The pixmap maps the image unit square through the current matrix and the paint scale and samples nearest neighbor with image row 0 at the top. RGB and gray are painted; alpha is ignored. `internal/graphics/device.go`, `internal/graphics/pixmap.go`.
- Spectre now rasterizes its own image PDF. `RunPostScript` to `ImagePDF` to `OpenPDF` to `RasterizePage` matches the source `PageImage` under `CompareRaster` for RGB and gray.
- Level 0 rewrite stays gated. A page that paints an image returns `undefined in Do` instead of a path-only file that dropped or outlined it. A missing name, a non-image subtype, an `/SMask` image, and a decode error return `undefined` with the `Do` operator name.

### Compression writer

- `pdfout.WriteCopy` skips source `/Type /XRef` and `/Type /ObjStm` container objects and gives their numbers free xref rows. The trailer `/ID` now covers only the written bodies in object-number order.
- `CopyOptions.PackObjects` writes the optional packed form: `%PDF-1.5`, non-stream bodies in one Flate `/Type /ObjStm`, and a Flate `/Type /XRef` stream with `W [1 4 2]`. The `/ID` is computed before packing, so it does not change with the mode. The levels 1 to 5 path does not select it.
- Phase 4 row 1.2 stays open with measured numbers. `sampledata/compress/whatisthis.pdf` is 596,341 bytes, levels 1 and 2 were 614,343 before the container skip, 610,034 after it, and 596,491 with the packed writer, so the "not larger than the input" guard cannot pass and was not added.
- Both writer changes are documented in `documentation/devices.md`, and the v0.0.2 container limitation note is closed in `documentation/features.md`.

### PostScript output

- New `internal/psout` holds a `graphics.Marker` recorder and a writer. `(*Instance).WritePostScript(ctx, doc, PostScriptOptions)` re-emits the same path subset as `RewritePDF` level 0: `setrgbcolor` or `setgray`, `setlinewidth`, `m` and `l`, and `S`, `f`, or `f*`, in 72 dpi points.
- `spectreps ps -o out.ps in.pdf` writes a date-free `%!PS-Adobe-3.0` program with a prolog that defines the short names, a fixed 612 by 792 box, one `%%Page` and `showpage` per page, and mode `0o600`. Two calls return equal bytes. Text and images are refused, so a page with `Tj` exits 1 with `undefined in Tj` instead of a silently blank program.
- The round trip is the proof: a page with `re`/`f`, `m`/`l`/`S`, a curve, and `q`/`Q`/`cm` becomes PostScript, runs back through `RunPostScript` at 72 dpi, and compares equal under `CompareRaster`.

### gs argv mode

- `spectreps gs` scans an allowlisted argv and routes the job to the existing subcommands. `internal/cli/gs.go`, with the grammar in the new `documentation/gs-argv-grammar.md`. The policy is: accept a switch when it maps onto behavior Spectre already guarantees, accept and ignore when the behavior is always on, and reject anything that would silently change pixels.
- Devices accepted: `ppmraw`, `png16m`, `jpeg`, `tiff24nc`, `bbox`, `inkcov`, `pdfimage24`, and `pdfwrite`. The rest of the switch set is `-sOutputFile`, `-dFirstPage`, `-dLastPage`, `-sPageList`, `-r`, `-dDEVICEWIDTHPOINTS`, `-dDEVICEHEIGHTPOINTS`, `-g` at 72 dpi, and `-dJPEGQ`.
- `-sPageList` accepts a contiguous ascending comma list of pages and ranges and maps it onto one `-pages` value. Even and odd selections, open or reversed ranges, overlaps, gaps, and page 0 stay rejected.
- `-dBATCH`, `-dNOPAUSE`, `-q`, `-dSAFER`, and `-dFIXEDMEDIA` are accepted and ignored. `-c`, `-dNOSAFER`, `-dDELAYSAFER`, stdin, and `@file` are refused with exit 2, as is every switch without a mapping.
- `raster -format ppm|png|jpeg|tiff` selects the encoder, and a `-sDEVICE` wins over the `-o` suffix. The end-to-end proof is a `-sDEVICE=pdfwrite` run against `sampledata/fixtures/gs-argv-input.pdf` that writes a PDF which reopens and rasterizes both pages.

### PDF/A-4

- New `internal/pdfa` holds a static UTF-8 XMP packet, a generated D50 sRGB matrix-shaper ICC profile, the output intent, and the profile preflight. `RewriteOptions.PDFA` selects `PDFA4`, the base claim, or `PDFA4F`, the embedded-file claim. `spectreps rewrite -pdfa 4|4f`.
- `PDFA4` is refused when the catalog carries `/Names /EmbeddedFiles`, and `PDFA4F` is refused when it does not. PDF/A-4e, the earlier parts, and PDF/X stay out.
- The writer moves the header to `%PDF-2.0` with a binary marker above byte 127, keeps `/ID`, writes no `/Encrypt`, and appends `/Metadata` and one `/S /GTS_PDFA1` output intent with `/DestOutputProfile` and no `/DestOutputProfileRef`. Other catalog entries are copied unchanged. Two runs return equal bytes with no dates.
- The refusal policy mirrors Ghostscript `PDFACompatibilityPolicy` 2. A known violation returns `Error: /rule in PDFA`, exits 1, and writes no output file. The nine rules are `font-not-embedded`, `lzwdecode`, `filter-not-allowed`, `cmyk-without-profile`, `alternates-not-allowed`, `opi-not-allowed`, `blend-mode-not-allowed`, `embedded-files-need-4f`, and `4f-needs-embedded-files`.
- `make pdfa-check` runs `verapdf --flavour 4` over the PDFs under `sampledata/pdfa/` and excludes a `negative/` folder. veraPDF 1.30.2 reports `path-a4.pdf`, a Spectre write, and the copied `compliant-a4.pdf` valid for PDF/A-4, so the external verdict is recorded.

### Text and fonts

- New `internal/font` holds generated tables: the standard 14 advances from the Adobe Core 14 AFM files, the StandardEncoding, WinAnsiEncoding, and MacRomanEncoding tables from pdf.js, and the Adobe Glyph List names. The generator `internal/font/gen.go` is build-ignored and pins the source commits and SHA-256 digests. Source and license are in the new `documentation/fonts.md`.
- The PDF text operators `BT`, `ET`, `Tf`, `Td`, `TD`, `Tm`, `T*`, `Tc`, `Tw`, `Tz`, `TL`, `Ts`, `Tj`, `TJ`, `'`, and `"` run. A simple font reads `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, `/BaseFont`, `/Encoding` with `/Differences`, and `/ToUnicode` with `bfchar` and `bfrange`. A Type0 font reads Identity-H with a CIDFontType2 descendant, `/CIDToGIDMap`, `/W`, and `/DW`. `internal/pdf/font.go`, `internal/pdf/cmap.go`, `internal/pdf/text.go`.
- Embedded TrueType `/FontFile2` and OpenType `/FontFile3` programs parse through `golang.org/x/image/font/sfnt`, and embedded outlines paint through `x/image/vector`. A standard 14 glyph or a Type 1 `/FontFile` paints as `invalidfont`, because Spectre ships no substitute outlines and does not look up host fonts. Advances, encodings, and extraction still work for those fonts.
- PostScript `findfont`, `scalefont`, `setfont`, and `show` drive the same machine. `internal/ps/op_text.go`.
- `(*Instance).ExtractText(ctx, doc, pageIndex)` and `spectreps text [-pages range] file.pdf` return the laid-out text: lines top to bottom, glyphs left to right, a space for a gap wider than a quarter box, CRLF per line, and a code-point fallback when neither `/ToUnicode` nor the encoding names the code.
- Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. Text tests compare shapes and advances, and extraction tests compare text and geometry.
- Type 1 charstrings and `seac` were deferred to `plans/v0.0.1/10-deferred.md` 10.1, and bare CFF stays with them. CFF inside an OpenType wrapper is in scope.

### PDF/UA-2

- `internal/pdf/structtree.go` parses `/MarkInfo`, `/StructTreeRoot`, `/K`, `/S`, `/P`, `/Pg`, `/MCID`, `/Alt`, `/ActualText`, `/Lang`, `/Namespaces`, `/RoleMap`, `/RoleMapNS`, and `/ParentTree` into typed values. The tree walk caps depth at 64, the role chain at 32, and a cycle is `limitcheck`.
- `File.HasStructTree`, `File.StructTree`, and the public `Document.Tagged` expose the model. `Document.Tagged` reports a structure tree or a true `/MarkInfo /Marked`.
- Levels 1 to 5 preserve a tagged input: tree shape, MCIDs, `/Alt`, `/ActualText`, and `/Lang` survive, and a tagged PDF 2.0 source keeps its header block with the binary marker. Level 0 and the `pdfimage` command refuse a tagged input with `Error: /tagged in RewritePDF` or `/tagged in ImagePDF` instead of silently dropping the tree.
- `internal/pdfa` adds `ReadUA2`, `UA2Write`, `UA2XMP`, `UA2ExtraObjects`, `UA2Catalog`, and `PreflightUA2`. The machine checks are `ua2-marked`, `ua2-structtree`, `ua2-document`, `ua2-lang`, `ua2-displaydoctitle`, `ua2-pdfuaid`, `ua2-title`, `ua2-rolemap`, and `ua2-mcid`, reported as `Error: /ua2-<rule> in PDFUA`. A `pdfuaid` claim is kept or added only after a passing preflight; it is never written from nothing.
- Tag generation, reading order, and role assignment stay out, and the docs say preflight only. `validate` does not call `PreflightUA2` yet. `make pdfua2-check` runs veraPDF over the PDFs under `sampledata/pdfua2/`, excludes `negative/`, and veraPDF 1.30.2 reports both samples valid.

### Public API

- New functions: `MeasureInkAmount`, `WritePostScript`, `ExtractText`, and `Document.Tagged`. New types: `PostScriptOptions` and `PDFAMode` with `PDFANone`, `PDFA4`, and `PDFA4F`. New field: `RewriteOptions.PDFA`. No existing signature changed.
- `WritePostScript` follows `RewritePDF` on nil documents, nil contexts, and cancelled contexts. `RewritePDF` with a PDF/A mode appends the claim or returns the preflight `JobError`, and a tagged document at level 0 returns the `/tagged` `JobError`.
- `documentation/public-api.md` carries the full contracts.

### Dependencies

- `github.com/mrjoshuak/go-jpeg2000 v1.5.12` becomes a direct requirement for `/JPXDecode`. The module is Apache-2.0, pure Go, and has no transitive requirements. The reason is on the `go.mod` line.
- `golang.org/x/sys v0.48.0` and `golang.org/x/text v0.42.0` become indirect requirements through `x/image/font/sfnt` and `x/image/vector`.
- No cgo, no `os/exec`, and no Ghostscript linkage. A `CGO_ENABLED=0 go build ./...` and a `go list -deps` run with no `runtime/cgo` are recorded in the phase 3 rows.

### Docs and ledgers

- New `documentation/fonts.md` and `documentation/gs-argv-grammar.md`.
- Refreshed `cli.md`, `devices.md`, `features.md`, `covered-and-not-covered.md`, `public-api.md`, `test.md`, `language.md`, `folder-structure.md`, `copyright-and-rewrite.md`, `ghostscript-baseline.md`, and `gs-argv-mapping.md`. The wording stays "profile preflight" and "preflight only"; no doc calls the output compliant or certified.
- `plans/v0.0.1/10-deferred.md` moves the ten landed rows to 10.4 and leaves Type 1 and PDF/UA-2 tag generation as the open text rows; the printer languages stay out of product.
- New samples: `sampledata/pdfa/path-a4.pdf` written by `rewrite -pdfa 4`, and `sampledata/pdfua2/tagged-ua2.pdf` and `untagged.pdf` written by the checked-in `internal/pdfa/gen_ua2_samples.go`.
- `plans/v0.0.3/` holds the ten phase files, the program ledger, and `v0.0.3-closure.md`. The closure records `make lint` and `make test` exit 0 on the merged tree on 2026-09-25, after the phase 9 merge at `7c8819c`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | Levels 3 through 5 now decode CCITT and JPEG2000 streams and re-encode them as DCT, and level 2 re-encodes CCITT as Flate RGB. v0.0.2 copied both stream types through, so those inputs now pay decode, an optional resample, and encode (`internal/pdfout/levels.go`, `documentation/devices.md:111-120`). `Do` decodes each `/XObject` name once per painted page (`internal/pdf/do.go:34-51`). Text painting loads an outline and rasterizes a coverage mask for every shown glyph, with no outline cache (`internal/pdf/text.go:448-476`, `internal/pdf/font.go:552-571`). `ink_cov` scans the finished pixmap once, the same as `inkcov`. No measured baseline. |
| **Memory** | One decoded image per `/XObject` name stays resident for the painted page (`internal/pdf/do.go:34-51`), and the rewrite level loops hold one image at a time (`internal/pdfout/levels.go:167-177`, `:234-245`). The decoded-stream cap is 32 MiB per image, checked before allocation for CCITT and from the JPX header (`internal/pdf/filter.go:15`, `internal/pdf/ccitt.go:41`, `internal/pdf/jpx.go:34-45`). A glyph mask caps at 20,000 pixels a side and 40,000,000 pixels and is released after the glyph (`internal/pdf/text.go:23-25`, `:467-469`). The only per-font cache is the name-to-GID map, built once per embedded program (`internal/pdf/font.go:419-436`); glyph outlines are not cached. |
| **Behavior / correctness** | Level 0 still refuses an image page with `undefined in Do`, but `raster`, `validate`, and `compare raster` now paint RGB and gray image XObjects, including Spectre's own `pdfimage` output (`documentation/devices.md:87-93`, `:142-148`). Text operators execute instead of returning `undefined`; a standard 14 font with no outline program returns `invalidfont` on paint while advances and extraction still work (`documentation/fonts.md:49-65`). A tagged document is refused by `rewrite -level 0` with `/tagged in RewritePDF` and by `pdfimage` with `/tagged in ImagePDF`, while levels 1 through 5 keep the tree (`documentation/cli.md:70`, `:129`, `documentation/features.md:46`). The PDF/A-4 claim and the PDF/UA-2 checks are profile preflight, not certificates (`documentation/devices.md:124-132`, `:191-195`). |
| **API / CLI** | New public symbols: `MeasureInkAmount`, `PDFAMode` with `PDFANone`, `PDFA4`, and `PDFA4F`, `RewriteOptions.PDFA`, `PostScriptOptions`, `(*Document).Tagged`, `(*Instance).ExtractText`, and `(*Instance).WritePostScript` (`documentation/public-api.md`, `spectreps/options.go:20-56`). New commands: `ink_cov`, `ps`, `text`, and `gs` (`internal/cli/run.go:50-76`). New flags: `rewrite -pdfa 4|4f` (`internal/cli/run.go:321`), `raster -format ppm|png|jpeg|tiff` (`internal/cli/run.go:130`, `internal/cli/raster_format.go`), and the `gs` switch allowlist in `documentation/gs-argv-grammar.md`. No existing signature changed. `RewriteOptions` gained one field. |
| **Dependencies** | New direct module `github.com/mrjoshuak/go-jpeg2000` v1.5.12, Apache-2.0, pure Go, with no transitive module requirements (`go.mod:8-9`, `plans/v0.0.3/3-jpeg2000-decode.md:23`). `golang.org/x/sys` v0.48.0 and `golang.org/x/text` v0.42.0 enter as indirect requirements through `golang.org/x/image/font/sfnt` and `x/image/vector` (`go.mod:11-15`). `golang.org/x/image` v0.46.0 was already required, and `x/image/ccitt` ships inside it. No cgo, no `os/exec`, no Ghostscript. |
| **Binary size / build time** | More packages link. `go build -trimpath ./cmd/spectreps` under go1.26.4 is 4,639,903 bytes and 122 dependency packages on master, and 6,207,997 bytes and 144 dependency packages on this branch, a rise of 1,568,094 bytes (+34%) and 22 packages. `make build` still writes `bin/spectreps`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Tagged documents are refused by generated writers. `rewrite -level 0` exits 1 with `Error: /tagged in RewritePDF`, and `pdfimage` exits 1 with `Error: /tagged in ImagePDF`. Both used to write a file with the structure tree dropped (`documentation/cli.md:70`, `:129`, `plans/v0.0.3/10-pdfua2.md:62`). | Use `rewrite -level 1` through `-level 5` to keep the tree. No `pdfimage` path keeps tags, so that job must not run on a tagged input. |
| PDF text operators run. `Tj`, `TJ`, `'`, and `"` returned `undefined` in v0.0.2, and PostScript `show` was out of the subset (`git show master:documentation/features.md:12`, `git show master:documentation/language.md:123`). They now paint and extract. A standard 14 font with no outline program fails a paint with `invalidfont`, where the old failure was `undefined` (`documentation/features.md:18-23`, `documentation/language.md:77-79`). | A job that treated `/undefined in Tj` as the unsupported-input signal must handle success, embedded outlines that now paint, or `/invalidfont`. Update expected error text. |
| CCITT and JPEG2000 streams decode. Level 2 re-encodes CCITT as Flate RGB, and levels 3 through 5 re-encode CCITT and JPEG2000 as DCT. v0.0.2 copied those streams through unchanged (`plans/v0.0.3/2-ccitt-decode.md:35`, `plans/v0.0.3/3-jpeg2000-decode.md:39`, `documentation/devices.md:111-120`). | Output bytes change for inputs with those streams at levels 2 through 5. Two runs on the same input still return equal bytes, so a v0.0.2 output is not a byte oracle. |
| Image PDFs rasterize. `raster`, `validate`, and `compare raster` on a PDF with an RGB or gray image used to exit 1 with `undefined in Do` (`git show master:documentation/features.md:12`, `:35`, `git show master:documentation/devices.md:77`). They now paint the page (`documentation/devices.md:87-93`). | No code change. A test that asserted the old failure must be updated. |
| `RewriteOptions` gained the `PDFA` field (`spectreps/options.go:42-46`, `documentation/public-api.md`). | Keyed literals compile unchanged. An unkeyed literal must add the third value. The zero value `PDFANone` leaves the claim off. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make build`
- [x] `go vet ./...`
- [x] `CGO_ENABLED=0 go build ./...`
- [x] `make pdfa-check` (veraPDF 1.30.2)
- [x] `make pdfua2-check` (veraPDF 1.30.2)

### Commands

```sh
make lint
make test
make build
go vet ./...
CGO_ENABLED=0 go build ./...
go test -count=1 -p 24 ./...
go test -count=1 -v ./spectreps -run TestExtractTextGolden
go test -count=1 ./internal/cli -run TestRewriteSamples
make pdfa-check
make pdfua2-check
```

Every command exited 0 on 2026-09-25 on `feature/003-deferred-work` at `4e6b2cc`. `make lint` ran `gofmt` with no output, `golangci-lint run ./...`, and `size-check`, which reported `clean (0 over-limit files)`. `make test` ran `go test -p 24 ./...`, and a fresh `go test -count=1 -p 24 ./...` passed with every package `ok`. `cmd/spectreps` and `internal/engine` have no test files. `make build` wrote `bin/spectreps`. With veraPDF 1.30.2 on PATH, `make pdfa-check` exited 0 with `compliant="2" nonCompliant="0"` and `make pdfua2-check` exited 0 with 1727 passed rules and 0 failed rules per file; both checks exclude a `negative/` folder, and both still print a skip when veraPDF is absent.

The phase rows carry dated outcomes, all 2026-09-25. Of the 72 rows with a literal `Proof:` field, 70 are checked and pass. Row 4-1.2 stays open by design with measured numbers: `TestRewriteSamples` passes, but levels 1 and 2 output 610,034 bytes against the 596,341-byte input, so the not-larger-than-input guard cannot be added. Row 3.6, Type 1 charstrings, is the deferred row and never ran. Phase 7 records its nine proof outcomes inline with the same date, written as `Proof (2026-09-25):` rather than `Proof:`.

---

## Screenshots / sample output

Measured on 2026-09-25 on `feature/003-deferred-work` at `4e6b2cc`. Output files went to `/tmp/opencode/pr-v003/`.

### Stable byte output

| Command | Input | Two runs | Bytes | Header |
| --- | --- | --- | ---: | --- |
| `rewrite -level 2` | `sampledata/compress/whatisthis.pdf` (596,341 bytes) | `cmp` exit 0 | 610,034 | `%PDF-1.4` |
| `rewrite -pdfa 4` | `sampledata/compress/path.pdf` (8,450 bytes) | `cmp` exit 0 | 16,404 | `%PDF-2.0` |
| `rewrite -pdfa 4` | `sampledata/compress/whatisthis.pdf` | `cmp` exit 0 | 617,860 | `%PDF-2.0` |
| `ps` | `sampledata/compress/path.pdf` | `cmp` exit 0 | 22,205 | `%!PS-Adobe-3.0` |

`file` reads the level 2 output as PDF 1.4 with 8 pages, the same page count as the input. Both PDF/A-4 outputs carry the `%PDF-2.0` header and the binary marker, and `file` reads them as PDF 2.0. The PostScript output is one date-free `%!PS-Adobe-3.0` program.

### CLI transcript

```sh
$ bin/spectreps version
0.0.2
$ bin/spectreps ink_cov -w 20 -h 20 sampledata/compress/path.pdf
Page 1
0.00000 0.00000 0.00000 RGB
$ bin/spectreps ink_cov -w 20 -h 20 /tmp/opencode/pr-v003/gs_img.pdf
Page 1
0.00000 100.00000 100.00000 RGB
Page 2
100.00000 0.00000 100.00000 RGB
$ bin/spectreps gs -sDEVICE=pdfwrite -sOutputFile=/tmp/opencode/pr-v003/gs-out.pdf sampledata/fixtures/gs-argv-input.pdf
$ bin/spectreps validate /tmp/opencode/pr-v003/gs-out.pdf
$ echo $?
0
$ bin/spectreps pdfimage -o /tmp/opencode/pr-v003/path_img.pdf sampledata/compress/path.pdf
$ bin/spectreps raster -o /tmp/opencode/pr-v003/path_img.ppm /tmp/opencode/pr-v003/path_img.pdf
$ echo $?
0
$ bin/spectreps pdfimage -w 20 -h 20 -r 72 -o /tmp/opencode/pr-v003/gs_img.pdf sampledata/fixtures/gs-argv-input.pdf
$ bin/spectreps compare raster -w 20 -h 20 -r 72 sampledata/fixtures/gs-argv-input.pdf /tmp/opencode/pr-v003/gs_img.pdf
$ echo $?
0
$ go test -count=1 -v ./spectreps -run TestExtractTextGolden
=== RUN   TestExtractTextGolden
--- PASS: TestExtractTextGolden (0.00s)
PASS
ok  	github.com/chinmay-sawant/spectrePS/spectreps	0.006s
$ make pdfa-check
compliant="2" nonCompliant="0" failedJobs="0"
$ make pdfua2-check
{"report":{"jobs":[{"itemDetails":{"name":".../compliant-ua2.pdf"...},"validationResult":[{"compliant":true,"passedRules":1727,"failedRules":0}...]},
          {"itemDetails":{"name":".../tagged-ua2.pdf"...},"validationResult":[{"compliant":true,"passedRules":1727,"failedRules":0}...]}]}}
```

`sampledata/compress/path.pdf` is a 200 by 200 point page whose content repeats one 0.5 pt red stroke at y=100 400 times. At `-w 20 -h 20` the mark falls outside the 20 by 20 point device, so `ink_cov` reports a blank page and exits 0. The nonzero witness is the image PDF written from `sampledata/fixtures/gs-argv-input.pdf`: `pdfimage` wrote 1,295 bytes, `raster` on that file exited 0, and `compare raster` against the source exited 0 with no output. The direct and round-trip PPMs are byte-equal for both pages (`cmp` exit 0); page 1 has 400 red pixels and page 2 has 400 green pixels on a 20 by 20 point page. This is the `Do` path against Spectre's own image PDF, which v0.0.2 could not rasterize.

`bin/spectreps gs -sDEVICE=pdfwrite` wrote a 901-byte, 2-page PDF that `validate` accepts with no output and exit 0. `bin/spectreps pdfimage -o` on `path.pdf` wrote a 2,254-byte PDF that `raster` accepts with exit 0; the page is white, matching the direct raster of `path.pdf`. `bin/spectreps version` prints `0.0.2`, the same string `documentation/cli.md` states.

---

## Related issues

- No GitHub issue exists for this work. `gh issue list --state all` returns nothing, so there are no IDs to close or relate.
- Parent ledger: `plans/v0.0.1/10-deferred.md`, which holds the rows this tag consumes.
- Program ledger: `plans/v0.0.3/00-program.md`. Closure: `plans/v0.0.3/v0.0.3-closure.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-v0.0.3-deferred-work.md` when process-gated

---

## Follow-ups (out of scope)

- Writer cleanup row 1.2 stays open. The guard that levels 1 and 2 are not larger than the input cannot pass: on `sampledata/compress/whatisthis.pdf` the input is 596,341 bytes, the classic output is 610,034 after the container skip (614,343 before it), and the optional packed writer reaches 596,491, still 150 bytes over. `plans/v0.0.3/4-writer-cleanup.md`.
- Type 1 charstrings and `seac`. Phase 9 row 3.6 is dropped to `plans/v0.0.1/10-deferred.md` 10.1.
- Tag generation for PDF/UA-2 waits on reading order and role assignment. `plans/v0.0.1/10-deferred.md` 10.2, `plans/v0.0.3/10-pdfua2.md`.
- The external verdicts are recorded: veraPDF 1.30.2 reports the checked-in PDF/A-4 and PDF/UA-2 samples valid. The deliberate UA-2 negative fixture lives under `sampledata/pdfua2/negative/` and is excluded from the make run. `plans/v0.0.3/8-pdfa4.md` row 5.1, `plans/v0.0.3/10-pdfua2.md` row 5.2.
- The content interpreter has no `W` clipping operator, so painting a PDF that uses clipping (for example `sampledata/compress/whatisthis.pdf`) returns `undefined in W` even though rewrite copies the content through. No v0.0.3 row covers it; noted for a later tag.
- `spectreps version` and `spectreps.Version()` still print `0.0.2`, because no v0.0.3 row asked for a bump. `spectreps/instance.go:7`, `documentation/cli.md:28`.

---

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [ ] No unrelated changes in diff
- [ ] Public API / CLI changes documented
- [ ] New rules have fixture coverage when applicable
- [ ] PR has assignee and labels
- [ ] Related issues use correct Closes/Relates keywords
- [ ] No secrets or generated artifacts committed
- [ ] Diff-stat-by-extension table pasted at the bottom

---

## Diff stat by extension

Generated with `bash scripts/pr-diff-stat.sh master` after the veraPDF commit at `51d2286`, before this last table refresh, so the refresh's own `.md` lines are not in the table.

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.bin` | 2 | Binary | Binary |
| `.go` | 102 | 18459 | 236 |
| `.j2k` | 1 | Binary | Binary |
| `.jp2` | 1 | Binary | Binary |
| `.md` | 31 | 1785 | 92 |
| `.mod` | 1 | 9 | 0 |
| `.pdf` | 6 | Binary | Binary |
| `.ppm` | 2 | Binary | Binary |
| `.py` | 1 | 59 | 0 |
| `.sum` | 1 | 6 | 0 |
| No extension | 1 | 33 | 1 |
| **Total** | **149** | **20351** | **329** |
