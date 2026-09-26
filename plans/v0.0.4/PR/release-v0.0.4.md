## v0.0.4

Fourth release of Spectre PS. `spectreps version` prints `0.0.4`. This note covers `master` through `8be9947` plus the version bump in `b90e999`, and the tag is cut from the master head that carries both release pull requests.

The `plans/v0.0.4/` ledger split the release into seven files. Four are feature phases, one is a validation phase, one is a performance phase, and one is a writer-size acceptance gate. Those numbers are build steps inside the release, not product releases.

This is the release that closes most of what v0.0.3 listed as out of scope. Type 1 fonts paint. PDF/UA-2 tags are generated, not just preserved. Fonts subset. Transparency, separations, and optional content paint. The stream filters beyond Flate decode. The PostScript Level 2 operators the CUPS corpus needs run. What is left is the bare-CFF container, the standard 14 outlines, the `tiffsep` plates, encryption, and the printer languages.

Spectre PS is a Go library and a `spectreps` command for a small slice of the jobs Ghostscript is used for. It reads a PostScript subset and a PDF subset, paints pages to RGB pixels, writes new PDFs and PostScript, stops on the first error, measures a page, extracts text, and compares bytes or pixels. It does not link Ghostscript and it does not start `gs`. The `spectreps gs` mode scans an allowlisted argv itself.

- **License:** [MIT](https://github.com/chinmay-sawant/spectrePS/blob/master/LICENSE). Copyright (c) 2026 Chinmay Sawant.
- **Module:** `github.com/chinmay-sawant/spectrePS`, Go 1.26.4. Two direct dependencies, `golang.org/x/image` v0.46.0 and `github.com/mrjoshuak/go-jpeg2000` v1.5.12, plus the indirect `golang.org/x/sys` v0.48.0 and `golang.org/x/text` v0.42.0. No new module in this release.
- **Library:** `github.com/chinmay-sawant/spectrePS/spectreps`.
- **Ledger:** `plans/v0.0.4/00-program.md`.
- **Closure:** `plans/v0.0.4/v0.0.4-closure.md`, lint and test exit 0 on 2026-09-26.
- **Commits:** `8be9947` is the last phase commit and `b90e999` is the version bump. The tag is cut from the master head above them.
- **Pull requests:** [#13](https://github.com/chinmay-sawant/spectrePS/pull/13) the seven ledger files of `plans/v0.0.4/` and this note, [#14](https://github.com/chinmay-sawant/spectrePS/pull/14) the correction to this note's install block, and [#11](https://github.com/chinmay-sawant/spectrePS/pull/11) the ten phases of `plans/v0.0.3/` that carried the v0.0.3 baseline.
- **Diff:** 294 files, 36,765 insertions, 1,002 deletions against v0.0.3, measured against the merged tree.

---

### Highlights

| Phase file inside v0.0.4 | What shipped |
| --- | --- |
| **`1-writer-ceiling.md`** | Levels 1 and 2 on `whatisthis.pdf` are held to a recorded 610,034-byte ceiling. The writer itself did not change. |
| **`2-type1-fonts.md`** | The Type 1 charstring decoder, `seac`, flex, hint replacement, and the built-in encoding. A Type 1 `A` paints the same pixels as the synthetic TrueType `A`. |
| **`3-pdfua2-tags.md`** | Marked content reads, then tag generation. `rewrite -tags` writes a PDF/UA-2 structure tree with reading order and roles derived from device geometry. |
| **`4-pdf-coverage.md`** | Clip, Form XObjects, `/ExtGState`, twelve separable blend modes, transparency groups, soft masks, image masks, `/Decode`, separations to an RGB preview, the color operators, LZW/ASCII85/ASCIIHex/RunLength and predictors, optional content, font subsetting, and `spectreps info`. |
| **`5-validation.md`** | 58 `TestValidation` cases over a checked-in corpus of 64 files, a manifest that pins every digest, and a 245-row traceability index. |
| **`6-performance-profiling.md`** | 35 benchmarks, an allocation budget that gates `make test`, four measured fixes, and the recorded baseline in `documentation/performance.md`. |
| **`v0.0.4-closure.md`** | `make lint` and `make test` exit 0 on the merged tree. |

Of 212 ledger rows, 206 are checked and 6 are `[~]`. There is no unchecked row.

---

### Install / build

From source, on the release tag:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout v0.0.4
make build
```

`make build` writes `bin/spectreps`. `make test` is `go test -p $(nproc) ./...`. `make lint` is `gofmt`, `golangci-lint`, and the 2,000-line Go file check. `make pdfa-check`, `make pdfua2-check`, `make refs-gs-check`, `make bench`, `make bench-profile`, and `make bench-check` are proof tools. None of them runs inside `make test`, and each prints a skip when its external tool is absent.

The validation corpus is checked in under `sampledata/validation/`, one subfolder per feature area, with `manifest.tsv` recording the pinned source, license, SHA-256, byte count, feature, and expected verdict for every file.

```sh
./bin/spectreps version
./bin/spectreps info sampledata/compress/path.pdf
./bin/spectreps raster -format png -o page.png sampledata/compress/path.pdf
./bin/spectreps rewrite -subset-fonts -o small.pdf sampledata/compress/path.pdf
./bin/spectreps rewrite -tags -claim -tag-title "Report" -tag-lang en -o tagged.pdf sampledata/compress/path.pdf
./bin/spectreps validate sampledata/pdfua2/tagged-ua2.pdf
./bin/spectreps ps -o page.ps sampledata/compress/path.pdf
./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
```

Library. The new symbols are `Instance.PreflightUA2`, `Document.Info`, `PDFInfo`, `PDFPageSize`, and `PDFFontInfo`, and `RewriteOptions` gained five fields:

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

    info, err := doc.Info()
    if err != nil {
        panic(err)
    }
    fmt.Printf("version=%s pages=%d tagged=%v images=%d\n",
        info.Version, info.Pages, info.Tagged, info.Images)
    for _, f := range info.Fonts {
        fmt.Printf("font %s embedded=%t\n", f.Name, f.Embedded)
    }

    opt := spectreps.DefaultRewriteOptions()
    opt.Level = 2
    opt.SubsetFonts = true
    opt.Tag = true
    opt.Claim = true
    opt.Title = "Report"
    opt.Lang = "en"
    out, err := in.RewritePDF(ctx, doc, opt)
    if err != nil {
        var job spectreps.JobError
        if errors.As(err, &job) && job.Op == "PDFUA" {
            // The tree was written. Only the pdfuaid claim was withheld.
            fmt.Printf("tagged but unclaimed: /%s\n", job.Msg)
        } else {
            panic(err)
        }
    }
    if err := os.WriteFile("tagged.pdf", out, 0o600); err != nil {
        panic(err)
    }
}
```

---

### What landed in v0.0.4

#### Type 1 fonts

- New `font.LoadType1(data []byte, lengths [3]int) (*font.Type1Font, error)` reads a PFA or PFB `/FontFile`. The eexec seed is 55665, the charstring seed is 4330, `/lenIV` accepts 0 and 4, and `RD` byte runs and hex strings are decoded.
- Charstrings interpret the moveto, lineto, and curveto families, `hsbw`, `sbw`, `closepath`, `callsubr` and `return` with direct and negative indexes, `div`, `endchar`, `seac`, and flex. OtherSubrs 0, 1, and 2 are the flex curves; OtherSubr 3 is the hint replacement Subr; 14 through 18 are the Multiple Master blend operators. Hint operators are parsed and dropped, never applied to output.
- A Type 1 glyph now feeds the same `x/image/vector` coverage path as TrueType, mapped through the `/FontMatrix` and flipped on Y at 64 ppem. A Type 1 `A` and the synthetic TrueType `A` are pixel-equal under `CompareRaster`, locked by `sampledata/fixtures/type1-tj.ppm`.
- **This reverses v0.0.3.** A Type 1 `/FontFile` used to paint `invalidfont`. `invalidfont` is now the answer for a standard 14 with no program, `/MMType1`, `/CIDFontType0`, `/CIDFontType0C`, Type 3, and a glyph name missing from CharStrings. Advances and extraction work for all of them.
- Width order is fixed: `/Widths`, then standard 14 metrics, then the charstring `hsbw` or `sbw` width, then `/MissingWidth`, then 0. A symbolic Type 1 with no `/Encoding` now starts from the program's own built-in encoding with `/BaseEncoding` and `/Differences` layered on top, so extraction no longer falls to a bare code point. `TestType1Extract` locks `ABZ\r\n` for a symbolic fixture.
- Charstring caps: 48 operands, call depth 10, 100,000 points, 65,535 subrs, 65,535 glyphs.
- The gated phase 7, the bare CFF container and the Type 2 charstring interpreter, did not run. The tag budget ran out. Those three rows are `[~]` and point at `plans/v0.0.1/10-deferred.md` 10.1.

#### PDF/UA-2 tag generation

- `BMC`, `BDC`, `EMC`, `MP`, and `DP` now parse, nesting capped at 64. An unmatched `EMC` is `syntaxerror in content` and the 65th open sequence is `limitcheck`. **A tagged page used to fail `RasterizePage` and `ExtractText` with `undefined in BDC`.** It now paints and extracts as if the markers were absent.
- New `internal/tag`. A `tag.Recorder` implements four seams at once, carries no page resources, cannot paint, and records events in stream order. `tag.DerivePlan` reads geometry off those events; `tag.Build` writes the file.
- Reading order and roles are derived from device geometry, not from a fact in the file, because an untagged PDF stores no semantics. Every threshold is a named constant and the whole table is in `documentation/devices.md:198-209`: 0.5 line-box heights for one baseline, 3 line heights for one visual segment, 1.35 times the smaller line height for one paragraph, 1.2 times the body size for a heading, a bullet or a number of up to three digits for a list, and 2 or more cells across 2 or more rows with column starts agreeing within 4 points for a table.
- Roles emitted: `Document`, `P`, `H1` through `H6`, `L` with `/ListNumbering` plus `LI`, `Lbl`, `LBody`, `Table` with `TR`, `TH` and `/A << /O /Table /Scope /Column >>`, `TD`, `Figure` with `/Alt`, `Span`, and `Artifact`. Every event either joins an element, becomes a `Figure`, or is wrapped in `/Artifact BMC`, so coverage is total.
- MCIDs are numbered one per structure sequence in paint order from 0, so the parent-tree array index is the paint-order position. Artifacts share the paint order but get no MCID and no parent-tree entry. The parent tree is one flat `/Nums` array with a key base placed past the highest page object number.
- `/ActualText` is deliberately narrow: a run carries it only for a ligature code point or a code with no Unicode mapping.
- `Claim` runs `pdfa.PreflightUA2` on the bytes the builder just produced. The claim `pdfuaid:part 2` and `pdfuaid:rev 2024` is written on a second pass only when that preflight passes. On a refusal the tree is returned with no claim **and** a `JobError`, which is the one path in the package that returns a payload alongside an error.
- Refusals: `-tags` on a tagged input is `/tagged in RewritePDF`, `-tags` with `-pdfa 4` or `4f` is `/unsupported in RewritePDF`, an image with no `/Alt` source is `/alt in Tag`, and a `Claim` with no title is `/ua2-title in PDFUA`. `-claim`, `-tag-title`, and `-tag-lang` without `-tags` exit 2 with `spectreps: -claim, -tag-title, and -tag-lang need -tags`.
- The claim is "generate and preflight", never certification. veraPDF 1.30.2 reported 1727 passed rules and 0 failed for `generated/report.pdf`, `tagged-ua2.pdf`, and `compliant-ua2.pdf` on 2026-09-26.

#### Content graphics, transparency, and color

- `W` and `W*` intersect a clip applied to later marks, `W*` uses the even-odd rule, and `q`/`Q` save and restore the clip list. Fills, strokes, glyphs, images, and group composites all go through the clipped variants.
- `Do` on `/Subtype /Form` runs the form content with `/Matrix` concatenated, `/BBox` as a clip, the form's own or the inherited `/Resources`, and an implicit `q`/`Q`. Nesting stops at depth 32 with `limitcheck in Do`. A reversed `/BBox` still clips.
- `B`, `B*`, `b`, and `b*` fill and stroke. `BX` skips content through the matching `EX` and a stray `EX` is ignored. `J`, `j`, `M`, `i`, `ri`, `d`, and `Tr` are accepted and ignored, which is a recorded deviation from ISO 32000-2.
- `gs` resolves `/ExtGState` by name and honors `/LW`, `/CA`, `ca`, `/BM`, and `/SMask`. Every entry is validated before anything changes, so a refusal leaves the previous state intact. A non-numeric `/LW`, `/CA`, or `ca` is `typecheck in gs`; an unknown name, an unsupported entry, a `/BM` array, or a non-separable mode name is `undefined in gs`.
- All twelve ISO 32000-1 separable blend modes composite with the standard formulas. Hue, Saturation, Color, and Luminosity have no constant and are refused by name.
- A Form XObject with `/Group << /S /Transparency >>` renders into a scratch pixmap and composites once, honoring group alpha, blend, and `/CS`. `/I` and `/K` are read and validated but every group uses the isolated shape, a recorded deviation. Any `/Group` key outside the six known ones refuses rather than being ignored.
- `/ExtGState /SMask` with `/S /Alpha` or `/S /Luminosity` builds a per-pixel coverage plane over a `/G` Form XObject, with the ISO 32000-1 luminosity weights 0.30, 0.59, and 0.11. `/TR /Identity` only.
- The new device seam is four optional interfaces beside the unchanged three-method `graphics.Marker`: `ClipMarker`, `AlphaMarker`, `SoftMaskMarker`, and `GroupMarker`. A device that cannot honor a feature keeps refusing by name, which is how the level 0 rewrite recorder still refuses `W`, `W*`, and `Do`.
- `K`, `k`, `CS`, `cs`, `SC`, `sc`, `SCN`, and `scn` run. A new color resolver handles `DeviceRGB`, `DeviceGray`, `DeviceCMYK`, `Indexed`, `ICCBased` through `/Alternate`, and `CalRGB` and `CalGray` as RGB and gray. `/Separation` and `/DeviceN` rasterize to their RGB preview using `r = (1-C)*(1-K)`. Type 2 sampled and type 4 PostScript-calculator tint transforms evaluate; type 0 and type 3 are `undefined`.
- Optional content reads `/OCProperties /D`. Content under an `OFF` group is skipped, `/OC` on XObjects and property dictionaries is honored, and the four `/P` policies are implemented. Alternate configurations, the `/AS` map, and the visibility flag stay out.

#### Image masks, soft masks, and `/Decode`

- **This reverses v0.0.3, where `Do` silently ignored `/Mask` and `/Decode` while the documentation said both were refused.** An `/SMask` stream now decodes to an alpha plane, a color-key `/Mask` array builds coverage from its sample ranges, a stencil `/Mask` stream builds a bilevel plane, and `/ImageMask true` decodes at one bit per sample with the most significant bit first.
- `/Decode` remaps gray, RGB, and CMYK samples before the color conversion. An Indexed space refuses a `/Decode` array because its sample is a table index. A CCITT or JPX image refuses `/Decode` outright because the shared sample path does not run for them.
- `/Matte` is accepted and ignored, so the matte color is not removed before compositing. That is the one documented deviation in this area.
- `Pixmap.DrawImage` now composites the source alpha. It previously ignored it.

#### Stream filters and predictors

- `internal/pdf/filter.go` went from 82 lines to 667. `LZWDecode`, `ASCII85Decode`, `ASCIIHexDecode`, and `RunLengthDecode` all decode now, and `Decode` covers Flate, LZW, ASCII85, ASCIIHex, and RunLength with the filter name carried into the error.
- Predictor 2 is TIFF horizontal differencing and 10 through 15 are the PNG set. Every predicted row begins with an algorithm tag byte and that tag selects the row's algorithm regardless of what `/Predictor` says, so `/Predictor 12` and `/Predictor 15` read the same tags and may mix algorithms. Xref streams, object streams, content streams, and image streams all share the decoder, so a predicted xref stream with Flate plus a PNG predictor decodes for the first time.
- A multi-stage `/Filter` chain decodes in order with its parallel `/DecodeParms`. A non-name chain item is `undefined in Image`, and a second unknown name refuses with that name.
- A `/Predictor` outside 1, 2, and 10 through 15 is `undefined in Predictor`. A non-integer `/Predictor`, a `/Predictor` under 1, a `/Colors` outside 1 to 64, a `/BitsPerComponent` not in 1, 2, 4, 8, or 16, a bad `/EarlyChange`, and a row that does not divide the stride are all `syntaxerror in Predictor`.
- One decoded stream is still capped at 32 MiB, and the cap reports `limitcheck` with the filter name. CCITT and JPX check `Columns * Rows` and the declared sample count against the same cap before allocating.

#### CCITT Group 3 and JPEG2000 reach

- `/K == 0` without end-of-line markers and `/K > 0` mixed 1-D and 2-D both used to be `undefined in Image`. A new in-package decoder, 831 lines with generated code tables and a 13-bit lookahead, runs both shapes.
- `/EndOfBlock` is read, validated, and then deliberately dropped: both values stop the decoded rows after exactly `/Rows`, so `/EndOfBlock false` decodes without a closing pattern and `/EndOfBlock true` does not change the pixels.
- `/BlackIs1` and `/EncodedByteAlign` now apply in both the library and in-package paths.

#### xref, object streams, and stream lengths

- A trailer `/Prev` chain is walked newest section first. The newest section wins per object number, an older section fills the gaps, and `/Prev` and `/Size` are never supplied by an older section. A cycle or a chain longer than 64 sections is `syntaxerror in xref`.
- An indirect `/Length` resolves through the xref. When the declared span and the `endstream` keyword disagree, the reader rewinds and scans, and `endstreamX` inside the data does not terminate the body. This is how a page body that literally contains the bytes `endstream` reads correctly.
- A Type0 font with an indirect `/DescendantFonts` array now loads. A missing `/DescendantFonts` used to be `invalidfont`; it now loads with no outline source, and a `CIDFontType0` descendant still reports advances because `/W` and `/DW` load before the program lookup.
- A `/Kids` cycle is now `limitcheck in pdf` and a catalog with no `/Pages` is `undefined in pdf`. Both used to be `syntaxerror in pdf`.

#### The `spectreps info` command

- `spectreps info file.pdf` takes one positional and no flags, so any flag exits 2. It writes to stdout only, creates no file, and never paints a page.
- Output is key/value lines: `PDF version:`, `Pages:`, one `Page N: W x H` per page in points, `Tagged:`, then either `Fonts: none` or `Fonts:` with a two-space-indented `  Name embedded=bool` per font, then `Images:`.
- `Document.Info()` returns `PDFInfo{Version, Pages, PageSizes, Tagged, Fonts, Images}`. The two slices are always non-nil, so `len()` is safe on the nil-document path. A nil document is `PDFInfo{}` with `JobError{Op: "Info", Msg: "rangecheck"}`, and a malformed `/MediaBox` is `/syntaxerror in Info`.
- A font is `embedded=true` when its descriptor carries `/FontFile`, `/FontFile2`, or `/FontFile3`, when it is Type 3, or when every descendant of a Type0 font carries a program. A missing `/BaseFont` prints the placeholder `(none)`. CIDFont descendants are excluded from the list.
- The command is read-only on purpose. The writers omit `/Info` because dates would break the stable-byte promise.

#### Font subsetting

- `RewriteOptions.SubsetFonts` and `spectreps rewrite -subset-fonts`, off by default, so bytes change only when the caller opts in. Levels 1 through 5 apply it. Level 0 ignores it and still refuses text with `undefined in Tj`.
- A subsetter rebuilds `glyf` with zero-length entries for unused glyphs, follows composite component references transitively, always keeps glyph 0, and rebuilds `loca` including a promotion to long form when needed. **Glyph indices never change**, so content streams, `/Widths`, `/Differences`, `/Encoding`, and `/CIDToGIDMap` stay valid with no page re-encode.
- A synthesized `/ToUnicode` CMap is appended, `bfchar` for isolated codes and `bfrange` for consecutive codes, in sections of at most 100 entries. The subset tag is six uppercase letters from `SHA-256(program)`, and the tagged `/BaseFont` is `<tag>+<name>`.
- An OpenType `/FontFile3` program and a Type 1 `/FontFile` are copied whole. So is a font with no program, an unused font, a font whose used codes reach the 4096 collector cap, and a font with no subsettable `glyf` outlines. A font copied whole still trips the PDF/A `font-not-embedded` rule, so the claim and the subsetter do not fight.
- Two calls with the flag on return equal bytes. The subsetting also applies alongside a PDF/A claim, and a tagged input keeps its tree, MCIDs, `/Alt`, `/ActualText`, and `/Lang` through a subsetting rewrite.
- `CopyOptions.PackObjects` was an open question and is now answered: internal only, not on `RewriteOptions`, so no public call selects the packed output.

#### `validate` and the PDF/UA-2 preflight

- **A previously exit-0 `validate` on a tagged file can now exit 1.** For a tagged input, one whose catalog carries `/StructTreeRoot` or a true `/MarkInfo /Marked`, `validate` runs `Instance.PreflightUA2` after the pages paint. An untagged PDF is not a UA-2 request and is untouched.
- The rule list went from nine to ten. `ua2-content` is new and proves MCID coverage in both directions, decoding every page with its own content scanner so a stray or unclosed `EMC` fails there.
- The other nine are unchanged: `ua2-marked`, `ua2-structtree`, `ua2-document`, `ua2-lang`, `ua2-displaydoctitle`, `ua2-pdfuaid`, `ua2-title`, `ua2-rolemap`, `ua2-mcid`. A refusal is `Error: /ua2-<rule> in PDFUA`, exit 1.
- The PDF/A preflight keeps its nine rules. `cmyk-without-profile` got wider: it now fires on a `/Separation` or `/DeviceN` value nested inside a page `/ColorSpace` resource, through a name, an array, or a reference, and on an ICCBased `/Alternate` naming `DeviceCMYK`. The `/BM` refusal for anything but `Normal` stays, even though blend modes now paint.

#### The PostScript Level 2 set

- New operators, all driven by the CUPS corpus files: `bind`, `clip`, `initclip`, `clippath`, `pathbbox`, `arc`, `arcn`, `rectfill`, `rectstroke`, `setlinecap`, `stringwidth`, `dtransform`, and the identification operators `languagelevel`, `version`, `revision`, `product`, and `serialnumber`.
- `cups-smiley.ps` paints through the new `arc`, `rectstroke`, and `setlinecap`, which is the direct evidence that they are new.
- `clip` intersects the current path with the nonzero rule and shares the `graphics.Clip` list the PDF `W` and `W*` use. `initclip` resets to the page. `clippath` returns the stored clip subpaths or the page rectangle.
- **The page box is no longer a hardcoded letter page.** `gstate` carries the device page in pixels, `UsePixmap` records the real pixmap geometry, and `pageRect` converts back into interpreter space. Before this, a 20 by 20 pixmap or a 5100 by 6600 one at 600 dpi still reported 612 by 792 from `initclip` and `clippath`.
- `setlinecap` accepts 0, 1, and 2, and `rangecheck` outside that. The device always draws a capsule, so 0 and 2 carry the recorded deviation.
- `arc` is a chord approximation at 5-degree steps and a negative radius is `rangecheck`.
- The same fields make the retained-page-byte budget correct for a non-letter page, because the budget multiplies pixels.

#### Interpreter bounds

- Four new caps in `internal/ps`. A runaway loop now stops instead of running forever.
- Executed objects and procedure entries cap at 67,108,864, reported as `limitcheck` on `exec`. The step counter counts procedure entries as well as objects, because an empty body `{ } loop` has no object to count and would otherwise spin with the counter standing still. The interpreter retires about 22 million objects a second, so the cap is roughly three seconds of runaway execution. The heaviest corpus file uses 53,000 objects, so the headroom is over a thousand.
- Retained page bytes for one run cap at 1,073,741,824, reported as `limitcheck` on `showpage`. The budget is in bytes rather than pages because the page count that fills it depends on the caller's geometry. A letter page at 72 dpi is 1,454,208 bytes, so the budget allows 738 pages.
- Array elements cap at 1,048,576 and string bytes at 33,554,432, both reported as `rangecheck` and not `limitcheck`, because the requested size is an operand rather than a resource the interpreter accumulated. Without the array cap, `2147483647 array` asks the Go runtime for roughly 171 GB and crashes instead of reporting a PostScript error.

#### The PostScript writer stops dropping images

- **A behavior flip.** An image page used to leave as a silently blank program while `documentation/devices.md` said the recorder refused images. It is now `Error: /undefined in Do`, exit 1, no output file, locked by `TestValidationPSOutImage`. The audit found the code contradicted its own documentation and the code changed to match.
- The `%%BoundingBox` now comes from the first page's resolved `/MediaBox` instead of a fixed 612 by 792, floored at the minimum and ceiled at the maximum because DSC wants integers. An A4 document now emits `%%BoundingBox: 0 0 595 842`. A page with no resolvable box falls back per axis.
- Everything else is unchanged: the same mark set, the same prolog, one `%%Page` and one `showpage` per page, no creation date, two calls return equal bytes, and the round trip through `RunPostScript` still matches under `CompareRaster`.

#### Validation corpus and traceability

- 64 files across ten subfolders, 62 committed and 2 in the external tier. Every committed file is at or under 1 MiB and every one has a manifest row with a pinned upstream commit, a license, a SHA-256, a byte count, a feature, and an expected verdict.
- The `expect` column is one of `paint`, `struct`, or `refuse:<JobError.Msg>`. There are 7 refusal rows and each names its exact error text.
- License totals for the committed tier: 26 Apache-2.0, 19 repo-authored, 14 CC-BY-4.0, 3 BSD-3-Clause. Upstream families are pdf.js 17, qpdf 7, veraPDF 12, pdfium 3, CUPS 2, techniques-for-accessible-pdf 2, pdf-differences 1, plus 19 repo-authored. The gate admits `Apache-2.0`, `MIT`, `BSD-3-Clause`, `CC0-1.0`, `CC-BY-4.0`, `US-public-domain`, and `repo-authored`, and keeps CC BY-SA external-only. Isartor, GhostPDL, MuPDF, iText, PDFBox, and six named pdf.js files are excluded by name.
- An embedded-font review runs as a second gate. Files carrying a font with no redistribution right were excluded and the decision is written in `sampledata/validation/README.md`: `arial_unicode_en_cidfont.pdf` for a Monotype font, `UA1_Tpdf-G2_01.pdf` for Bitstream Dutch 801, and `UA1_Tpdf-G2_04.pdf` for SegoeUISymbol and Calibri.
- 58 `TestValidation` functions across 9 packages, plus 7 end-to-end `TestValidationCorpus*` runs that drive the real CLI over the manifest. A blank success fails. A refusal that exits 0 fails.
- `sampledata/validation/traceability.tsv` is a 245-row index: 90 rows for the `plans/v0.0.4/5-validation.md` phases and 155 for the `documentation/test.md` cases, each naming the test that proves it. 219 rows are `live` and 26 are `proof`. **No row is pending.** `scripts/check-traceability.sh` runs `go test -list '.*' ./...` and exits 1 on a live row whose function is missing. Its guard was proved by editing one test name and watching the script fail with the case name.
- 92 test files changed, 14,399 insertions. The suite is 143 test files with 402 test functions, 67 `TestValidation` functions, and 35 benchmarks.
- `sampledata/validation/refs/` holds four repo-authored PostScript programs for the Ghostscript cross-check, and nothing under the corpus was produced by Ghostscript.

#### Benchmarks and allocation budgets

- 35 benchmarks across `spectreps`, `internal/cli`, `internal/pdf`, `internal/pdfout`, `internal/graphics`, and `internal/ps`. None runs inside `make test`.
- `TestPerformanceAllocs` does gate, because allocation counts are deterministic and wall-clock is not. `MeasureBox`, `MeasureInk`, `MeasureInkAmount`, `CompareRaster`, and `CompareFiles` are locked at 0 allocations. The `internal/cli` level 2 rewrite is locked at 715 allocations, with the collector paused so the zlib writer pool stays populated; the isolated perf branch measured 701 and the integrated tree adds one allocation in the image decode path, one bool and one flag entry for `-subset-fonts`, and the four tag flags. The guard was proved by lowering the ceiling by one and watching the test fail.
- Four fixes landed, each with a before and after and each naming the behavior test that proves it. Removing the per-stroke slice took `BenchmarkStroke` from 9,728 B/op and 1 alloc to 0 and 0, and `BenchmarkRunPostScript` from 893,220 to 701,185 B/op. A zlib writer pool took `BenchmarkRewriteLevel0` from 990,326 to 158,982 B/op and `BenchmarkImagePDF` from 818,204 to 4,565 B/op. A tight-stride `bytes.Equal` path in `CompareRaster` took it from 85,676 to 2,067 ns/op.
- The recorded baseline is in `documentation/performance.md`, taken on 2026-09-26 on a 13th Gen Intel Core i7-13700HX with 24 threads, 7.6 GiB RAM, and Go 1.26.4. Level 3 is the slowest rewrite at 512,952,881 ns because the medium cap keeps the longest side at 1754 pixels at quality 80.
- The fixed per-process cost is about 2.6 ms, measured by `version`. The engine job for a small program adds under 1 ms.

#### Reference proofs

- `documentation/reference-proofs.md` records the tool matrix, the exact commands, the normalization rule, the geometry rule, and the measured diffs. Ghostscript 9.55.0 and veraPDF 1.30.2, measured 2026-09-26.
- The normalization rule is that the pixel body starts after the `255` max-value line, because gs writes a comment line the Spectre PPM does not. The geometry rule is that Spectre takes the page size from `-w` and `-h` and gs takes it from the `/MediaBox`, so the PostScript cases pin gs with `-g612x792`. No anti-aliasing, gamma, or color transform is applied to either side.
- Body of 1,454,112 bytes. An axis-aligned stroke and an axis-aligned fill match Ghostscript byte for byte, and the script requires it. A diagonal stroke differs in 1,506 bytes and a cubic curve in 5,934. Those two counts are recorded, not gated. A level 2 rewrite of two files matches gs in 0 bytes of 120,000.
- veraPDF 1.30.2 reported `compliant="2" nonCompliant="0" failedJobs="0"` for the PDF/A-4 base files and `compliant="1" nonCompliant="0"` for the 4f file, and 7 successful UA-2 jobs with 0 failures.

#### Documentation corrections

Five claims in the tree were wrong and are now fixed. Each was found by the validation audit, and each is worth naming because the release note would otherwise repeat them.

- The 600 dpi pixel-cap claim was false. A letter page at 600 dpi is 5,100 by 6,600, which is 33,660,000 pixels against the 40,000,000 cap, so it passes. The area cap is crossed at 655 dpi. The cap did not move; the documentation was wrong and now names the real boundary, locked by `TestValidationPixelCap`.
- `covered-and-not-covered.md` said `Do` refuses `/SMask` and `/Decode`. The code ignored both and painted wrong pixels. The reader honors them now and the doc describes support.
- `cli.md` named `ErrNotImplemented` as a live exit-1 cause. No method returns it. The exit table no longer names it, and the sentinel's doc comment says plainly that it is legacy and returned by nothing.
- The stale "Phase 02 behavior" section in `cli.md`, which described commands that had shipped two releases earlier, is gone.
- The `mismatch width` example in `cli.md` was unreachable from the CLI, because `compare raster` gives one `RunOptions` to both files. The example is replaced with prose; `CompareRaster` can still report `width` or `height` to a library caller.

Two more doc claims were corrected in the other direction. `gs-argv-mapping.md` said `%stdout`, `%pipe%`, and a trailing `-` were literal path text; they have been rejected since v0.0.3. And the 300-to-600 dpi performance gap of 9x recorded on 2026-09-25 did not reproduce: on the merged tree it is 3.9x for 4x the pixels, attributed to the per-stroke allocation that fix 5.1 removed plus a page buffer that grows past the 30 MB L3.

New documentation: `documentation/performance.md`, `documentation/reference-proofs.md`, and `sampledata/validation/README.md`.

---

### What this release does not do

These rows stay out of this tag: `plans/v0.0.1/10-deferred.md`, the out-of-scope tables in the `plans/v0.0.4/` files, and the limits in `documentation/language.md` and `documentation/covered-and-not-covered.md`. Each one needs a new plan file before work starts.

- Bare CFF and CID-keyed CFF. The CFF container, the Type 2 charstring interpreter, and `/FontFile3 /Subtype /Type1C` are all `[~]` in `plans/v0.0.4/2-type1-fonts.md` phase 7. The tag budget ran out.
- Standard 14 outline programs. They have metrics and extraction but no outlines, so `show` on a device is `invalidfont`. The CUPS `cups-testfile.ps` corpus row is a recorded `refuse:invalidfont` because of this gate.
- Spot-color plate output. The reader resolves `/Separation` and `/DeviceN` to the RGB preview, but there is no plate accumulator and no `tiffsep` device. That row was the plan's declared first cut.
- Encryption, linearization, and output encryption. An encrypted trailer still refuses with `invalidaccess`.
- Combined PDF/A-4 plus UA-2 output. `-tags` with `-pdfa` is `/unsupported in RewritePDF`.
- Inline images, `sh` shading, patterns, and dash patterns. Each is a new painting model, not a decode row.
- The four non-separable blend modes, alternate OCG configurations, the `/AS` usage map, and the OCG visibility flag.
- CFF subsetting, Type 1 program writing, Type 3 fonts, vertical writing, color fonts, variable fonts, and multiple master instance selection.
- Annotation structure, MathML, PostScript tag generation, and anything past the machine rules toward WCAG.
- Full color management. `/Alternate` only; profile transforms, rendering intents, black point compensation, and CMYK ICC are out.
- JPEG2000 encoding, font hinting, and OCR.
- PostScript `save` and `restore`. `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` return `invalidaccess`.
- PCL, PXL, XPS, and the printer device list from `gs -h`. Out of product.
- Two performance follow-ups: a font parse cache and the 300-to-600 dpi scaling row.

Known limitations:

- Levels 1 and 2 grow an already-packed file. The input is 596,341 bytes, the classic output is 610,034 after the container skip, 614,343 before it, and the packed writer reaches 596,491, which is 150 over. The source packs 78 non-stream objects, so a not-larger-than-input guard cannot pass. The recorded ceiling is 610,034 bytes and the assertion is `whatisthisLevel12Ceiling`.
- A level 0 rewrite still refuses `W`, `W*`, and `Do`, so it refuses a clipping or form page even though `RasterizePage` paints one. The recorder implements no clip, form, alpha, blend, soft mask, or group seam.
- A standard 14 font, `/MMType1`, a bare CFF, a `/CIDFontType0` descendant, and a Type 3 font paint `invalidfont`. Advances and extraction still work.
- A tagged document is refused by `rewrite -level 0` and by `pdfimage`, because neither writer can keep the tree.
- The stroke model is a capsule with round caps and joins and no dash pattern, so `J`, `j`, `M`, `i`, `ri`, `d`, and `Tr` are accepted and ignored. That is a deviation from ISO 32000-2.
- `/Matte` is accepted and ignored, `/I false` and `/K true` are read and validated but not given a different composite, and a soft-mask coverage plane is sampled on the device grid the group rendered with. Each is a recorded deviation.
- Anti-aliasing is off. There is no `TextAlphaBits` equivalent, and turning it on would change pixels and invalidate fixtures.
- Text pixels never byte-match Ghostscript, because hinting and antialiasing differ. The oracle is text and geometry.
- The PDF/A-4 claim and the PDF/UA-2 checks are profile preflight, never certification.
- `graphics.State` is still exported with no production caller. It is the one finding from the audit that was neither fixed nor written down as deliberate.
- `documentation/test.md` names 58 `TestValidation` functions and 67 exist, and `documentation/development.md` lists 9 private packages and 13 exist. Both lists drifted as the last rows landed.
- `documentation/performance.md` records the `internal/cli` level 2 allocation ceiling as 701 and the code locks 715. The code comment reconciles them; the doc was not refreshed.
- `sampledata/validation/README.md` still refers to a `corpusPending` table that no longer exists, and its provenance table says one image refuses `undefined in Predictor` where the manifest says `undefined in XXXDecode`.
- `copyright-and-rewrite.md` still lists PDF info and font subsetting as not covered, and both landed in this release.

Rewritten PDF bytes are not compared with Ghostscript. Pixel tests compare Spectre with Spectre. The only external pixel gate is `make refs-gs-check`, which is exact for axis-aligned marks and recorded elsewhere.
