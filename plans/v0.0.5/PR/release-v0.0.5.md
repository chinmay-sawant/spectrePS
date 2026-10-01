## v0.0.5

Fifth release of Spectre PS. `spectreps version` prints `0.0.5`. This note covers `master` through `303eb51` plus the version bump and documentation alignment in this pull request, and the tag is cut from the master head that carries this note.

The `plans/v0.0.5/` ledger splits the release into eight phase files and a closure. Five are benchmark phases, two are PDF compatibility phases, and one is the real-world corpus. The batch2 recovery is not a ledger file; it arrived as pull requests #20, #21, and #23, driven by the corpus the ledger built.

This is the release that measures the tree v0.0.4 skipped, and it is the release that starts opening damaged files. The benchmark extension adds 32 functions, takes the suite from 35 functions in six packages to 67 in eleven, and records nine hot paths that had never been timed. The corpus extension gives the validation set four label axes, a per-area pass-rate report, a fetched tier, and a pinned 5,613-file bulk tier. The recovery work reopens xref tables, page trees, damaged streams, and encrypted files whose empty password authenticates. The product decision recorded on 2026-09-29 is that Spectre PS opens what Ghostscript opens. What is left is the font substitution plan, the veraPDF gate, fuzzing, and the corpus rows that need a licence or a decision.

Spectre PS is a Go library and a `spectreps` command for a small slice of the jobs Ghostscript is used for. It reads a PostScript subset and a PDF subset, paints pages to RGB pixels, writes new PDFs and PostScript, stops on the first error, measures a page, extracts text, and compares bytes or pixels. It does not link Ghostscript and it does not start `gs`. The `spectreps gs` mode scans an allowlisted argv itself.

- **License:** [MIT](https://github.com/chinmay-sawant/spectrePS/blob/master/LICENSE). Copyright (c) 2026 Chinmay Sawant.
- **Module:** `github.com/chinmay-sawant/spectrePS`, Go 1.26.4. Two direct dependencies, `golang.org/x/image` v0.46.0 and `github.com/mrjoshuak/go-jpeg2000` v1.5.12, plus the indirect `golang.org/x/sys` v0.48.0 and `golang.org/x/text` v0.42.0. No new module in this release. CI pins `golangci-lint` v1.64.8.
- **Library:** `github.com/chinmay-sawant/spectrePS/spectreps`.
- **Ledger:** `plans/v0.0.5/00-program.md`.
- **Closure:** `plans/v0.0.5/v0.0.5-closure.md`. Phase 1 lint, test, build, and bench exit 0 on the development tree on 2026-09-26. Phase 2 lint, test, and build exit 0 at `76a95d9` on 2026-09-26; `make bench` on the merged tree did not run. Phases 1 and 2 are partially closed, and no timing number is a baseline while deferred row 4.3 is open.
- **Commits:** 69 sit between the tag and `303eb51`, and this pull request adds the version bump and the documentation alignment above them.
- **Pull requests:** [#16](https://github.com/chinmay-sawant/spectrePS/pull/16) the 32 benchmark functions and the two performance documents, [#17](https://github.com/chinmay-sawant/spectrePS/pull/17) the corpus coverage for the v0.0.4 features, [#18](https://github.com/chinmay-sawant/spectrePS/pull/18) the first CI workflow and the fetched tier, [#19](https://github.com/chinmay-sawant/spectrePS/pull/19) the PDF version and PDF/A compatibility corpus, [#20](https://github.com/chinmay-sawant/spectrePS/pull/20) the batch2 recovery and the live corpus, [#21](https://github.com/chinmay-sawant/spectrePS/pull/21) the damaged page tree recovery, [#22](https://github.com/chinmay-sawant/spectrePS/pull/22) the sync of the merged PR #20 body, [#23](https://github.com/chinmay-sawant/spectrePS/pull/23) the opt-in bulk tier, and [#24](https://github.com/chinmay-sawant/spectrePS/pull/24) the release note, the version bump, the documentation alignment, and the release-note skill.
- **Diff:** 115 files, 21,190 insertions, 460 deletions against v0.0.4, measured against the merged tree. Pull request #20 alone is 41 commits, 44 files, +10,392/−436.

Of 118 rows across the `plans/v0.0.5/` ledger, 88 are checked, 29 are open, and 1 is deferred. The 28 open rows in `8-real-world-corpus.md` are the corpus work that needs a licence, a decision, or a later tag. The status header of that file still says every row is `[ ]`, which was true when it was written and is not true now.

---

### Highlights

| Workstream inside v0.0.5 | What shipped |
| --- | --- |
| **`1-graphics-device.md`** | Fill at path complexity on a 200 by 200 page, the four clip paths with their snapshot allocation, glyph blending, and `ShowPage`. The fill scan costs 0.97 ms at 4 points and 26.5 ms at 128. No production change. |
| **`2-encoders.md`** | PPM, PNG, JPEG, and TIFF encoders and the RGB, gray, and CMYK packers, plus an isolated Flate decode benchmark. `EncodeFlateRGB` allocates once per pixel, 532,160 allocations for a 532,144-pixel image. |
| **`3-font-and-metadata.md`** | First benchmarks in `internal/font`, `internal/pdfa`, and `internal/engine`. The Type 1 program parse is 42 times the glyph interpret. `pdfa.Preflight` is the most expensive single call outside a rewrite at 1.65 ms. |
| **`4-reading-and-writing.md`** | First benchmarks for `Info`, `WritePostScript`, `PreflightUA2`, and the tagged and PDF/A rewrites. `tag.DerivePlan` is superlinear, 74,667 ns at 50 runs and 2,448,927 ns at 800. |
| **`5-baseline-and-budget.md`** | `BENCH_PKGS` goes from six packages to eleven, `documentation/performance.md` gets a coverage extension, seven allocation ceilings land, and the `-count=10` capture is deferred. |
| **`6-pdf-compatibility.md`** | `info` reports the effective PDF version, the catalog `/Version` can raise the header, and seven pinned compatibility files land with a 12-case test. |
| **`7-pdfa-profiles.md`** | One compliant sample for all 11 PDF/A profiles, three negative controls, a JSON-parsing checker, and `make pdfa-corpus-check` with 14 pass. |
| **`8-real-world-corpus.md`** | An 11-column manifest, six probes, three basis values, a per-area report, a fetched tier, and 3,123 rows. The conformance tier lifts `basis=gs` from 7 rows to 2,889. |
| **`v0.0.5-closure.md`** | Development and merged tree transcripts: lint, test, build, and bench on the first; lint, test, and build on the second. `make bench` on the merged tree is row 2.4 and is unrun. |
| **batch2 recovery, #20, #21, #23** | 578 of the audit's 664 refusals open at the #20 merge, 129 of 135 encrypted files open, and two damaged page trees report their full page counts. The bulk tier pins the 5,613-file corpus behind `BULK=1`. |

---

### Install / build

From source, on the release tag:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout v0.0.5
make build
```

`make build` writes `bin/spectreps`. `make test` is `go test -p $(nproc) ./...`. `make lint` is `gofmt`, `golangci-lint`, and the 2,000-line Go file check. `make pdfa-check`, `make pdfa-corpus-check`, `make pdfua2-check`, `make refs-gs-check`, `make bench`, `make bench-profile`, and `make bench-check` are proof tools. None of them runs inside `make test`, and each prints a skip when its external tool is absent. `make bench` covers eleven packages now, not six.

The validation corpus has a committed tier and a fetched tier. `make validation-fetch` fetches the 3,101 fetchable rows through a content-addressed cache; a second run costs no network. `make validation-fetch BULK=1` adds the batch2 tarball, 5,613 files and a 4.5 GB download. `make validation-verify` checks the tree without network. `make validation-report` writes the per-area pass rate to `profiles/`. `make validation-run` still runs the committed rows. Fetched rows skip when absent, which is how CI runs.

```sh
./bin/spectreps version
./bin/spectreps info sampledata/compress/path.pdf
./bin/spectreps info sampledata/validation/compatibility/versions/PDF-versions1.pdf
./bin/spectreps raster -format png -o page.png sampledata/compress/path.pdf
./bin/spectreps rewrite -subset-fonts -o small.pdf sampledata/compress/path.pdf
./bin/spectreps rewrite -tags -claim -tag-title "Report" -tag-lang en -o tagged.pdf sampledata/compress/path.pdf
./bin/spectreps validate sampledata/pdfua2/tagged-ua2.pdf
./bin/spectreps ps -o page.ps sampledata/compress/path.pdf
./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
```

Library. No exported symbol, method, signature, field, or default changed in this release. The one behavior change is `PDFInfo.Version`: `Document.Info()` now reports the higher of the PDF header version and the current catalog's `/Version` name, so the three `PDF-versions` fixtures read 1.6 from a 1.4 header.

```go
package main

import (
    "context"
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
    fmt.Printf("effective pdf %s, %d pages, tagged=%v\n",
        info.Version, info.Pages, info.Tagged)
}
```

---

### What landed in v0.0.5

#### Benchmark coverage

- 32 benchmark functions were added across ten benchmark files, and the suite went from 35 functions producing 40 result rows to 67 functions producing 86 rows. Five packages had no `bench_test.go` at all: `internal/engine`, `internal/font`, `internal/pdfa`, `internal/psout`, and `internal/tag`. `BENCH_PKGS` in the Makefile went from six packages to eleven.
- The benchmark phases changed no production file. The one production change in this area came from the batch2 audit, which hit a path that made the fill scan hang and bounded it by the path's bounding box. See the recovery section.
- `internal/graphics/bench_test.go` gained seven functions. The fill benchmark is a scaling table: a closed star at 4, 32, and 128 points costs 968,770, 7,689,271, and 26,492,447 ns on a 200 by 200 page, so 8 times the points cost 7.9 times the time. A path of 32 separate squares costs 21,577,169 ns, because a pixel outside a segment's scanline is rejected cheaply. `StrokeClipped` costs 10,995,205 ns and 1,900,792 bytes over 6 allocations, and `FillClipped` costs 10,318,785 ns over 8, which is the two snapshot slices per call. `CompositeGroupClipped` costs 890,755 ns against 37,632 ns for the same group unclipped, so the whole-page snapshot is 23.7 times the composite. `DrawGlyph` on a 64 by 64 mask costs 29,528 ns at full coverage and 9,706 ns at zero, all 0 allocations, so the early return still walks the whole mask. `ShowPage` copies the 1,454,400-byte pixel plane and reports 1,458,197 B/op.
- `internal/cli/bench_test.go` gained the four raster encoders on one painted 200 by 200 page. PPM costs 54,055 ns and 246,052 bytes, which is the body copied twice, once for the header prepend. PNG costs 709,869 ns and 1,016,390 bytes over 34 allocations, JPEG 504,297 ns and 169,776 bytes over 11, TIFF 164,556 ns at `none` against 540,593 ns at `deflate`, or 3.3 times the time for a 3.0 times larger file.
- `internal/pdfout/bench_test.go` covers the three `packSamples` branches and the Flate RGB encoder. RGB costs 221,533 ns, gray 202,610 ns, CMYK 640,379 ns, so CMYK is 2.9 times RGB. **`EncodeFlateRGB` allocates once per pixel**: 14,909,755 ns, 3,193,154 bytes, and 532,160 allocations for a 532,144-pixel image. That is the largest allocation count in the tree, and it was invisible before this release. The heap profile follows it: `image.(*RGBA).At` and `image.(*YCbCr).At` are 70.24 percent of every object the writer package allocates.
- `internal/pdf/bench_test.go` isolates `Decode` from `Open` with `BenchmarkDecodeFlate`: 1,728,564 ns at 606.62 MB/s, and the decode allocates 5.25 times its 1 MiB output through the 4,096-byte append path in `readLimited`.
- `internal/font/bench_test.go` covers the standard 14 lookup (379.6 ns, 0 allocations), all 256 codes of three encodings through `GlyphName` and back (52,444 ns for 768 pairs), the AGL map (49.76 ns), the Type 1 load (20,970 ns and 163 allocations to parse, 497.0 ns to interpret one glyph, so the parse is 42 times the interpret), and TrueType subsetting (2,354 ns and 27 allocations), with a subset tag at 284.5 ns.
- `internal/pdfa/bench_test.go` covers `Preflight` at 1,653,991 ns and 12 allocations, `PreflightUA2` at 371,776 ns and 4,512 allocations, and the XMP packets: the PDF/A-4 packet is a constant string at 0.1534 ns, and the UA-2 packet is built per call at 2,596 ns and 13 allocations. `internal/engine/bench_test.go` is the package's first test file and measures the byte compare at about 4,400 MB/s on 1 MiB buffers, 0 allocations.
- `spectreps/bench_test.go` added the five jobs that had no benchmark. `Document.Info` costs 1,359 ns and 7 allocations, `WritePostScript` 495,039 ns and 5,628 allocations, the tagged rewrite 1,505,441 ns, 1,440,061 bytes, and 12,095 allocations, and the PDF/A-4 rewrite 81,110 ns and 147 allocations, so the tagged writer is 18.6 times the archival writer on the same input.
- `internal/tag/bench_test.go` records the `orderFlow` slope: 74,667, 379,228, and 2,448,927 ns at 50, 200, and 800 text runs, so four times the runs costs 5.1 times from 50 to 200 and 6.5 times from 200 to 800. `internal/psout/bench_test.go` emits a 2,000-segment content stream at 2,323,015 ns and 25,979 allocations, 13 per segment, and frames ten pages at 438,270 ns.

#### Allocation budgets and the performance record

- `TestJobAllocs` and `TestEncoderAllocs` are new ordinary tests, so they gate `make test`. Seven exact ceilings landed: `DocumentInfo` 7, `PreflightUA2` 4,512, `WritePostScript` 5,628, `RewritePDFTagged` 12,090, the PDF/A-4 rewrite 147, PNG encode 34, and TIFF uncompressed 27. The guards were proved by lowering each by one and watching it fail.
- The tagged ceiling is 12,090 and the `-benchmem` row reads 12,095. `AllocsPerRun` warms the zlib writer pool before it measures and the benchmark does not. `spectreps/alloc_test.go` and `documentation/performance.md` both say so, and the guard proof prints the exact mismatch.
- The `internal/cli` level 2 rewrite ceiling moved from 701 to 715. The isolated perf branch measured 701 and the merged tree adds one allocation in the image decode path, one bool and one flag entry for `-subset-fonts`, and the four tag flags; the code comment records why the budget is against the integrated tree.
- `documentation/performance.md` keeps the v0.0.4 tables and adds a `Coverage extension` section above them. The new section records one findings row per hot path and names `graphics.inside`, `snapshotRect`, `EncodeFlateRGB`, `readLimited`, `packedCMYK`, `encodePPM`, `orderFlow`, `pdfa.Preflight`, and `ua2ContentRule`. None of the nine was fixed; they are recorded.
- `documentation/benchmark.md` is new. `profiles/` is gitignored, so it is the durable copy of the capture: 324 lines, 258 benchmark lines, 85 distinct names, 86 rows. **The timing numbers are a first reading, not a baseline.** `make bench-check` compares `-count=3` against `-count=5`, which is below the six samples benchstat needs for a confidence interval. All 107 rows it called significant sat at exactly `p=0.036`, the Mann-Whitney U floor at that sample size, on a tree where no production file had changed, and 44 rows came back `± ∞`. Deferred row 4.3 raises both sides to `-count=10`, needs a decision on the 20 minutes of machine time, and has to run before any timing number is called a baseline. The allocation columns are unaffected and exact.
- The three profile leaders are `compress/flate` in `pdfout` at 9.04 percent flat and 16.64 percent cumulative, `pdf.(*File).ObjectCount` in `pdfa` at 7.38 percent flat and 22.14 percent cumulative, and `tag.runText` in `tag` at 5.71 percent flat and 15.22 percent cumulative.

#### PDF version reporting and the compatibility corpus

- **`info` now reports the effective PDF version, not the header.** `File.Info` calls `version()`, which reads the current catalog's `/Version` and reports the higher name. A `/Version` that is not a name is ignored and the header stands, which is the pdfTeX and luaTeX shape. A name that does not parse is `syntaxerror in Info`. The three PDF Association incremental-update files report 1.6 from a 1.4 header.
- The new `TestValidationPDFVersionCorpus` covers 12 cases: nine where the effective version equals a header from 1.0 to 2.0, plus the three incremental-update files that read 1.6 from a 1.4 header. The library test `TestRecoverCrossRef` also pins the version over a rebuilt xref.
- Seven pinned files landed under `sampledata/validation/compatibility/`: four PDF Association files, one qpdf file, and two pdf.js files. The recorded outcomes include `struct` for the first two incremental updates and `refuse:undefined in gs` for the third, which carries a PDF 2.0-only graphics state; `refuse:invalidfont in Tj` for the Type 3 font; and `refuse:invalidaccess in Encrypt` for the 40-bit encrypted file.
- `documentation/pdf-compatibility.md` is new and draws the line the release state needs: a version report says nothing about conformance, a `struct` verdict says nothing about archival compliance, and a preflight is never a certification. `documentation/cli.md` and `documentation/public-api.md` now describe the effective-version rule.

#### The PDF/A profile corpus

- Eleven compliant samples now pin one file per PDF/A profile that veraPDF supports, from 1a through 4e plus the committed 4f sample, under `sampledata/validation/pdfa/profiles/`. Three known noncompliant files stay in `pdfa/negative/` as controls.
- `scripts/pdfa-profiles.tsv` and `scripts/check-pdfa-profiles.py` are the new checker. It verifies every sample's byte count and SHA-256 against the manifest, requires all 11 positive profiles to be present, calls veraPDF once per file with an explicit `--flavour`, and parses `report.jobs[i].validationResult[j].compliant` plus `jobEndStatus` instead of reading the process exit code. A tool failure or a parse failure counts as a failure.
- `make pdfa-corpus-check` ran on 2026-09-26 with veraPDF 1.30.2: 14 pass, 0 fail, 14 samples, eleven compliant under their named profiles and three controls noncompliant as expected. The older `make pdfa-check` and `make pdfua2-check` still read the process exit code only, and open row 5.1 of `8-real-world-corpus.md` names that as a live bug.
- The phase closure also records `make validation-run` at 83 pass, 0 fail, 0 skipped, after the profile rows landed.

#### The validation corpus becomes a real-world corpus

- The manifest went from 7 columns to 11. `area` is the pass-rate group and replaced seven hardcoded folder lists in the `TestValidationCorpus*` selectors. `probe` is the job selector and replaced path-prefix inference in `corpusRunRow`. `basis` records where the expected outcome comes from: `spec` and `gs` gate, and `baseline` is reported rather than gated. `pages` is an expected page count, and `0` asserts nothing. The label vocabularies are closed sets and parsed at load time.
- The probes are `info`, `raster`, `rewrite`, `text`, `ps`, and `gs`. The areas are 15 keys, from `pdfa` and `tagged` down to `gs-argv`. The `expect` vocabulary kept its old values and gained `survive`, which passes only when the file exits 0 or reports a named error and never a signal, a panic, a fatal runtime fault, or silent non-zero exit.
- The manifest holds 3,123 rows: 82 committed and 3,041 fetched. By basis, 2,889 rows are `gs`, 180 are `spec`, and 54 are `baseline`; by expectation, 2,926 are `struct`, 163 `survive`, 23 `paint`, and 11 `refuse`. The fetched tier holds the verifier corpus at 2,691 of 2,906 files, which excludes the 205 Isartor files and the 10 `Undefined/` files that veraPDF 1.30.2 fails 3 of, the Artifex corpus at 233 of 236 files, and the 89 hand-built files. The earlier fetch proof recorded 177 rows and 14.1 MiB; the conformance tier later lifted the fetchable count to 3,101.
- `internal/validation/gen.go` gained `-fetch-external`, `-verify-only`, `-dry-run`, `-area`, `-workers`, and `-cache`, plus `SPECTREPS_VALIDATION_CACHE`. Files stream into a content-addressed cache, are verified, and are hardlinked into the tree, so a killed run never leaves a file that looks complete and a second run costs no network. Fetching follows a per-host politeness table, retries 408, 429, 5xx, and transport errors with capped full-jitter backoff, never retries a 404, and treats a digest mismatch as fatal on the first try.
- `make validation-report` runs the corpus shuffled and writes `profiles/corpus-report.tsv` with group, key, rows, pass, fail, skip, and pass rate. It counts rows a feature test runs twice so duplication does not inflate the rate. The merged-tree report measured 3,123 rows, 3,123 pass, 0 fail, 0 skip. `scripts/check-traceability.sh` reports 284 cases, 254 live, 30 proof, and 0 pending.
- The suite now has 156 test files, 448 test functions, 82 `TestValidation` functions, and 67 benchmarks, against 143, 402, 67, and 35 at v0.0.4. The new CLI tests cover font subsetting, tag generation, Type 1 text extraction, the survive verdict, and the PDF version corpus.
- **The corpus found a licence error in this repository's own gate.** The 89 hand-built files were committed with `CC0-1.0`, taken from a repository README with no `LICENSE` file behind it. The dataset's rights record says CC BY-SA 4.0 and names its rights holder. `TestValidationLicenses` passed the whole time because it read the string, not the source. The files moved to the fetched tier, the manifest records `CC-BY-SA-4.0`, and the licence gate now admits share-alike and AGPL content only in the external tier. Open row 1.5 would add a `license_url` column so the next audit can tell.
- **A corpus row was wrong before the reader was.** `handbuilt/hello_world.pdf` was committed as `expect: struct`. Its xref gives object 5 an offset of 402 that does not point at the object, and Ghostscript 9.55.0 reports the same damage. The row is now `survive` with the damage attributed upstream.
- The 89 hand-built files all pass the survive contract, but the refusals are not spread across the parser: 81 report `syntaxerror in obj`, 4 `syntaxerror in pdf`, 3 `syntaxerror in xref`, and 1 `syntaxerror in endobj`. Ghostscript repairs and continues on the same files. Choosing a repair policy is a product decision, and it is deferred.
- The generated PostScript sweep `postscript/standard14-sweep.ps` is 7,767 bytes from `SOURCE_DATE_EPOCH=0 groff -Tps`, byte-identical across three runs a second apart, and names 13 of the 14 standard 14 faces without embedding them. It refuses with `undefined in matrix`, which is the language contract working: `matrix` is outside the documented subset. The five operators real groff output needs, `matrix`, `setpacking`, `ashow`, `widthshow`, and `awidthshow`, are deferred. ZapfDingbats is the fourteenth face and the ZD path does not reach it yet.
- The Artifex corpus added 21 PostScript programs and found twelve more operators that real programs use; eight of the 21 do not run. The `scan` operator also rejects valid input on six programs. Both findings are in the deferred ledger, and the first gate on the operator row is the standard 14 outlines, because a sweep cannot paint its faces until outlines exist.
- The CUPS corpus rows are two committed files, `cups-testfile.ps` and `cups-smiley.ps`, and three live-tier Scribus files at pin `e72b702`, recorded as `baseline` because their survive verdict is measured and not derived.
- Three defects were found while building the fetcher, and all three are fixed. The fetcher set `Accept-Encoding: gzip` by hand, which stops `net/http` from decompressing transparently, so the digest was taken over the gzip stream and only the files large enough to compress failed. Five CUPS rows were first pinned to `master` while the committed rows used `e72b702`, and every digest check failed; that is the pin working. And the hand-built licence error above moved 89 rows to the fetched tier.

#### The bulk tier

- `sampledata/validation/bulk.tsv` pins the batch2 stressful-corpus tarball at its upstream SHA-512 and 4,496,841,248 bytes, and `bulk/batch2.members.tsv` pins all 5,613 member digests and byte counts.
- `gen.go -fetch-bulk` verifies the archive before extracting to staging, verifies every member before moving it into `external/_bulk/`, rejects absolute and `..` members and unlisted files, and leaves the tree untouched on any mismatch. `make validation-fetch BULK=1` is the make path. A plain fetch prints the 4.5 GB cost and fetches none of it.
- The proofs: 5,613 files present and verified; `-verify-only` caught a deleted member by name; `-fetch-bulk` restored it from the cache with no network in 1m53s; `make test` never reads the bulk files and stays green with the tier absent.

#### Damaged PDF recovery

- The product decision behind the largest code change in this window is that Spectre PS opens what Ghostscript opens. The batch2 audit ran 664 real damaged PDFs through `info`, a page-one raster, and Ghostscript's own PPM output. At the #20 merge, 578 of the 664 original refusals opened, and 129 of the 135 encrypted-bucket files opened. The audit summary records Ghostscript emitting pages for 581 of 664 files, `info` passing on 612, and 564 passing both.
- **Recovery only runs when strict reading fails.** `readTable` tries the file's `startxref` and its `/Prev` chain first, and the header rebuild starts only when that fails and ends with a trailer `/Root`. A file-written table keeps its strict generation check; the tolerance applies only to a table the reader rebuilt. Per-object reads fall back to a header scan and packed rows only after the row fails. What still refuses by design: a `/Prev` cycle or a chain longer than 64 sections, a file with neither a trailer `/Root` nor a catalog, malformed streams, and referenced dead rows.
- The xref and trailer fixes, in the order they were measured: rebuild from `num gen obj` headers when `startxref` names nothing (274 to 357 of 664); recover a damaged classic table per field and repair a broken `/Prev` from headers (436 to 449); read the root dictionary when the `trailer` keyword is damaged and read a table whose keyword sits within 256 bytes before the offset (585 to 588); synthesize a root from the newest `/Type /Catalog` when no trailer names one (588 to 602); read a dictionary key whose leading slash is missing (602 to 603); find pages by scanning when a rebuilt table cannot reach the page tree (603 to 605); read a reference whose generation was left out and skip a lone `+` in an array (605 to 606); recover xref streams newest-first with per-object row repair; and open five refusal shapes without a table swap plus four more after that. Encryption opening then took the merged count to 578.
- Stream and content fixes: a missing `/Length` scans to `endstream`; a missing `endobj` is accepted only when the next object header is within 64 bytes; a truncating or Adler-32-failing Flate stream keeps its decoded head; the ISO filter abbreviations `/A85`, `/AHx`, `/LZW`, `/Fl`, and `/RL` decode; a content stream whose decoder is missing or whose expansion hits the 32 MiB cap becomes a page-local loss instead of a whole-file refusal; nested `/Contents` arrays splice to a depth of 8; and an indirect `/Filter` or `/DecodeParms` resolves on a copy.
- **Encryption moved from always-refuse to the empty-password case.** The reader now opens files whose empty user or owner password authenticates, across RC4-40, RC4-128, AES-128-CBC, and AES-256-CBC, using the ISO 32000-1 Algorithm 2 and ISO 32000-2 Algorithm 2.B derivations. Strings and streams decrypt at read time, a leading `/Crypt` filter is dropped once it selects the cipher, and `RawObject` returns false while a crypt state is installed. Extracted text and page counts match pypdf on the five fixtures. Non-empty passwords, public-key handlers, unknown `/CFM` values, and damaged `/O`, `/U`, or `/ID` material still refuse.
- Page tree recovery keeps repeated `/Kids` references as separate page occurrences while stopping cycles. The damaged walk keys on the current branch rather than a global visited set, has a visit cap of twice the xref entry count, and untyped children count as blank pages only after a rebuild. GHOSTSCRIPT-695619-0.pdf reports 299 pages and TIKA-3224-1.pdf reports 24, where the reader reported 269 and 12 before, and both match the Ghostscript PPM audit. The recovery prefers the catalog-selected tree and keeps recovered page sizes and content references in the same order as the leaves.
- **The fill scan is bounded by the path's bounding box.** A corpus file with a 6,638-point path made `Pixmap.Fill` walk 6,638 points for each of 484,704 pixels, about 3.2 billion cross tests, and the file went from past 30 seconds to 16 seconds. `TestValidationFillLargePathTerminates` pins a 5-second budget and the correctness test pins the pixels; a synthetic 6,998-point path went from 17.3 seconds unbounded to 0.009 seconds bounded.
- Dead xref rows no longer veto `Info`: the font and image surveys skip a row whose offset does not hold the object it claims, while every reference path still resolves strictly. `Info` also recovers page sizes by scanning when the second page tree fails.

#### CI and the external tier

- The repository had no CI before this release. `.github/workflows/ci.yml` is the first workflow: two jobs on `ubuntu-latest`, `make test NPROC=16` with a 20-minute timeout and `make lint` through `golangci-lint-action` at v1.64.8, both reading the Go version from `go.mod`. The workflow does not fetch the corpus, so it stays hermetic and the external rows skip.
- `.gitignore` now ignores the fetched tier except its README, and `/notes` and `profiles/` stay out of the tree. The licence gate refuses AGPL and share-alike bytes on committed rows and admits them in the external tier only.
- The `fax-decode-parms.pdf` row was corrected by measurement from `paint` to `refuse:invalidfont in Tj`. All eight of its images decode; the refusal is the standard 14 no-outline policy, and the error names `Tj`.

#### Documentation

- `documentation/benchmark.md` and `documentation/pdf-compatibility.md` are new. `documentation/performance.md` gains the coverage extension, the new findings rows, the new budget rows, and a rewritten `bench-check` section. `documentation/development.md` gains every new make target, the eleven-package bench statement, and the fetch and bulk paragraphs. `documentation/test.md` gains the four corpus label axes, the survive contract, the new `TestValidation` names, and the benchmark gate. `README.md` gains the test and corpus section, and `documentation/features.md`, `documentation/cli.md`, `documentation/public-api.md`, `documentation/folder-structure.md`, and `documentation/README.md` follow the behavior changes above.

---

### What this release does not do

These rows stay out of the tag: the open rows of `plans/v0.0.5/8-real-world-corpus.md`, deferred row 4.3 of `5-baseline-and-budget.md`, and the deferred rows in `plans/v0.0.1/10-deferred.md`. Each needs a licence, a decision, or a later plan.

- **No timing number is a baseline.** `make bench` on the merged tree is closure row 2.4 and did not run, and the `-count=10` re-capture in deferred row 4.3 waits on a decision about 20 minutes of machine time. Read the timing columns as magnitude only.
- **The corpus is half landed.** Open rows cover the remaining 32 `pdf-differences` files, the qpdf malformed inputs with their `.out` expectations, the `pdfCabinetOfHorrors`, the accessible-pdf techniques, the pdf.js malformed subset, the pdfminer.six samples, and the 597-file veraPDF regression set. The SEC EDGAR, GovInfo, and IRS sources are pinned but not fetched.
- **JBIG2 and JPEG2000 are still a decision, not a row.** The dominant encoding in real scans is JBIG2 plus JPEG2000, and a pure-Go decoder for either was not found. A scan corpus would measure the refusal path, so row 3.1 waits on a two-number product claim or a cgo decision that `AGENTS.md` currently forbids.
- **The font substitution plan is not built.** The 33 TeX Gyre CFF OTFs, the Noto symbol fonts, the 14-name substitution table, the Core 14 AFMs, and the ZapfDingbats path are open rows 4.1 through 4.4 and 4.8. Until they land, the standard 14 still paint `invalidfont`, `cups-testfile.ps` is still a recorded refusal, and the groff sweep still stops at 13 faces.
- **The veraPDF gate is not fixed.** Open rows 5.1 through 5.5: this repository's own `pdfa-check` and `pdfua2-check` conflate exit 1 with exits 2, 4, and 7, run one invocation per file group rather than one per flavour over a directory, do not pin the validator, and have no `pdfa-gate`. The new `pdfa-corpus-check` is a separate, JSON-parsing target and does not repair the old one.
- **Fuzzing has not started.** Rows 6.1 through 6.6 cover the xref, filter, font, and PostScript targets, the seed gate, and per-target provenance. There is no `func Fuzz` and no `testdata/fuzz` in the tree.
- **The deferred ledger still carries the old gates.** Bare CFF and CID-keyed CFF, standard 14 outline programs, spot-color plate output, output encryption and linearization, inline images, shading, patterns, the four non-separable blend modes, alternate OCG configurations, full color management, PCL and XPS, and the font parse cache all stay `[~]`. The five operators the groff sweep needs and the twelve the Artifex programs need sit beside them.
- **There is no general repair policy.** The batch2 fixes open the specific shapes the audit named. The 89 well-formedness cases still measure one refusal path, 81 of them through `syntaxerror in obj`, and the choice between strict and repairing behavior is a product decision that has not been made.
- **Encryption is partial.** Empty-password files open. Public-key handlers, real passwords, output encryption, and linearization do not. Four AESV2 fixtures whose empty password does not authenticate and two xref failures in the encrypted bucket stay refused.

---

### Known limitations

- **Recovered files change their exit codes.** A damaged file that v0.0.4 refused with exit 1 can now open with exit 0. Recovery runs only after strict reading fails, and the strict checks still apply to a file that wrote a valid table, but a caller that relied on the old refusal needs to handle readable content for the empty-password encryption case specifically.
- The 17 Ghostscript-rendered files the #20 follow-up names still fail `info`: 12 `syntaxerror in xref`, 4 `undefined in pdf`, and 1 `syntaxerror in FlateDecode`. The audit's persisted `info` table at HEAD holds 52 failures in total, including 8 encrypted and 3 `syntaxerror in Info`.
- The tagged write and the PDF/A write are preflight, never certification. That is unchanged from v0.0.4.
- The painting and writer limits v0.0.4 listed still stand: a level 0 rewrite refuses `W`, `W*`, and `Do`; the stroke model is a capsule with no dash pattern; anti-aliasing is off; text pixels never byte-match Ghostscript; `/Matte` is accepted and ignored; and the standard 14, `/MMType1`, bare CFF, `CIDFontType0`, and Type 3 fonts paint `invalidfont` while advances and extraction work.
- The corpus report measured 3,123 rows at the #20 merge, and the bulk tier adds 5,613 files that no manifest row and no test reads. `make test` never fetches, and the bulk audit still needs a gitignored local input list, so the audit cannot rerun from a clean checkout.
- `documentation/benchmark.md` records about 8.7 million allocations for the level 3 through 5 rewrites against 2,381 in the v0.0.4 table, a difference of four orders of magnitude. `documentation/performance.md` says only that the two sets do not measure the same work, and no document reconciles the delta.
- Documentation drift found during the note scan and not fixed:
  - `documentation/cli.md`, `documentation/features.md`, and `documentation/test.md` still say every encrypted file refuses `invalidaccess in Encrypt`. The reader opens empty-password files now, and the compatibility fixture's `reader refuses at open` feature text is stale for the same reason.
  - `documentation/features.md` and `documentation/test.md` record the basis split as 26 `baseline` rows of 2,890 and 2,689 `gs` rows. The manifest holds 54 `baseline` rows, 2,889 `gs` rows, and 3,123 rows in total.
  - `documentation/test.md` says its `TestValidation` list names every function in the suite. It names 65 of 82.
  - `documentation/performance.md` leaves the old guard proof for the 700 ceiling directly above the current 715 proof, and its prose says the tagged benchmark reads 12,095 where `documentation/benchmark.md` records 12,094.
  - `documentation/benchmark.md` says the two `PreflightUA2` rows "agree" while its own tables read 4,511 and 4,512 allocations.
  - `internal/validation/manifest.go` still says seven columns in its package comment. `sampledata/validation/external/README.md` says the Artifex rows "are not pinned yet" above its own table of 233 pinned rows.
  - `plans/v0.0.5/8-real-world-corpus.md` still says every row is `[ ]`, and its phase 8 text points at a `10.7` deferred section that does not exist. The rows landed in 10.6, and `plans/v0.0.1/10-deferred.md` carries the repair-policy row twice.
  - `plans/v0.0.5/v0.0.5-closure.md` calls the bench suite twelve packages where `BENCH_PKGS` has eleven, and `scripts/validation-run.sh` still says it is for v0.0.4 and names two external rows.

Rewritten PDF bytes are not compared with Ghostscript. Pixel tests compare Spectre with Spectre. The only external pixel gate is `make refs-gs-check`, which is exact for axis-aligned marks and recorded elsewhere.
