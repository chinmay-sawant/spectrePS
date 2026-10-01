# Validation corpus

Real PDF and PostScript files for the validation suite. Every corpus file has
one row in `manifest.tsv` with its pinned source, license, SHA-256, byte count,
the feature it covers, the expected verdict, and four label columns;
`README.md` files carry the prose. Tests read the manifest through
`internal/validation` and are named `TestValidation<Area>`, so
`go test -count=1 ./... -run TestValidation` runs the group.

The folders follow the feature areas: `compatibility/` for version and known
feature gaps, `postscript/` for interpreter programs, `handbuilt/` for
`paths/` for path and paint operators,
`structural/` for xref and object streams, `images/` for Flate, DCT, CCITT, and
JPEG2000 decode, `text/` for fonts and extraction, `tagged/` and `pdfa/` for the
profile preflights, `rewrite/` for the writers, `gs-argv/` for the bounded `gs`
mode, and `refs/` for the external Ghostscript reference proofs. The `area`
column, not the folder name, is what the report groups on.

## Tiers

The committed tier is checked in and every file is at or under 1 MiB. Whole
suites, larger files, and files whose licence does not permit committing live
under `sampledata/validation/external/`, which is gitignored apart from its own
README and is fetched with `make validation-fetch`. Tests skip the external tier
when it is absent, and no test needs the network.

The manifest rows that pin the external tier are committed. The bytes are not.
The reviewable record of what the tier holds therefore lives in git even though
the files do not, and `external/README.md` describes the tier in the tree.

## Manifest

`manifest.tsv` is tab-separated with eleven columns:

```
path  sha256  bytes  source  license  feature  expect  area  probe  basis  pages
```

Paths are relative to `sampledata/validation` and use forward slashes. `source`
is the pinned `raw.githubusercontent.com` URL for a fetched file, or
`repo-authored` for a file that already lives in this tree.

`expect` is one of:

- `paint`: the corpus job rasterizes the file without an error.
- `struct`: the file opens and its page structure is the claim; painting is
  not asserted. Text extraction, tags, preflight, and rewrite cases use this.
- `survive`: the file is defective, or carries a feature the reader does not
  implement, so the claim is robustness rather than one fixed outcome. The job
  must terminate inside the language caps, and any failure must be a named
  `JobError` rather than a Go panic, a runtime fault, or a hang. Recovering the
  page and exiting 0 is also a pass. Use this where a conforming reader may
  legitimately refuse, repair, or give up, because pinning one of those would
  be pinning an implementation choice.
- `refuse:<JobError.Msg>`: the corpus job must return an error whose message
  contains the text after `refuse:`, and must write no output.

The last four columns are the label axes, added by
`plans/v0.0.5/8-real-world-corpus.md`:

- `area` is a closed set, `KnownAreas` in `internal/validation`. It is the
  pass-rate group, so a report joins on it without parsing the feature prose.
- `probe` selects the job, from `KnownProbes`. It replaced the path-prefix
  inference the runner used to do, so a new corpus folder needs a manifest row
  and nothing in Go.
- `basis` records where `expect` came from, from `KnownBasis`. `spec` means a
  clause of ISO 32000 or the PostScript Language Reference fixes it. `gs` means
  a measured Ghostscript run fixes it. `baseline` means the outcome was
  recorded from this build rather than derived, which is reported and never
  gated. **The baseline count is the debt.** Every baseline row has to be
  promoted to `spec` or `gs` with a written reason, or deleted.
- `pages` is the expected page count, or `0` to assert nothing. It is `0`
  everywhere today, because a page count taken from our own `info` output would
  be a circular assertion. Filling it needs an independent oracle.

A label outside its closed set fails the manifest parse, so a typo cannot
silently create a new report group.

Tests read the table with `validation.Load()`, then call
`validation.CheckFiles(rows, validation.CorpusDir())`. `Row.External()`,
`Row.Survived()`, `Row.Refused()`, `Row.RefuseError()`, and `Row.Gated()`
classify a row. `CheckLicenses` and `CheckExcluded` enforce the gate below.

`make validation-report` writes a pass rate per area and per basis to
`profiles/corpus-report.tsv`. A report whose baseline share grows is a report
saying the corpus has stopped being a specification.

`scripts/pdfa-profiles.tsv` maps the PDF/A files to their veraPDF profiles and
expected compliance verdicts. Run `make pdfa-corpus-check` to check all 11
profiles plus three noncompliant controls. A manifest `struct` verdict only
means Spectre opened the page structure. The PDF/A verdict comes from
veraPDF's JSON report.

## Expected text

`text/expected/` holds one golden extraction per `text/` row, named after the
input without its extension plus `.txt`. The text corpus run writes them with
`UPDATE_FIXTURES=1` and compares them on every later run:

```sh
UPDATE_FIXTURES=1 go test -count=1 ./internal/cli -run TestValidationCorpusText
```

The files carry the CRLF line ends of the extraction verbatim, so
`.gitattributes` marks the folder `-text`. They are test output, not corpus
inputs, so they carry no manifest row. A row in the `corpusPending` table in
`internal/cli/validation_corpus_run_test.go` is skipped until its owning fix
lands; its golden is written when the integrator removes the row and reruns the
update.

## Provenance

`manifest.tsv` is the record. It carries one row per file with the full pinned
URL, the byte count, the SHA-256, the licence, the feature, the verdict, and the
four label columns, sorted by path. A second copy of that table in prose cannot
be kept honest at this size, so there is not one.

What follows is the per-source summary. `Commit` is the immutable revision the
URL pins.

| Source | Commit | Rows | Licence | What it covers |
| --- | --- | ---: | --- | --- |
| `openpreserve/format-corpus` `pdf-handbuilt-test-corpus` | `366f068` | 89 | CC-BY-SA-4.0 | ISO 32000-1:2008 well-formedness cases, one defect each. Live tier, because the dataset's rights record is share-alike. See `external/handbuilt/README.md`. |
| `openpreserve/format-corpus` `pdfCabinetOfHorrors` | `366f068` | 23 | CC0-1.0 | Real-world files with an archival-hostile feature. External only. |
| `OpenPrinting/cups` `examples/` | `e72b702` | 6 | Apache-2.0 | Real producer PostScript, and the standard 14 blind spot. Three rows external for size. |
| `pdf-association/pdf-differences` | `907fe96` | 5 | CC-BY-4.0 | One directory per interoperability topic, each citing its ISO clause. |
| `pdf-association/techniques-for-accessible-pdf` | `3e31095` | 2 | CC-BY-4.0 | One directory per tagging technique, correct and incorrect variants. |
| `pdf-association/pdf20examples` | `c20f2c1` | 1 | CC-BY-SA-4.0 | The PDF 2.0 container. External only, because the licence is share-alike. |
| `qpdf/qpdf` `qpdf/qtest/qpdf` | `54d6053` | 5 | Apache-2.0 | Container, xref, and stream fixtures. |
| `mozilla/pdf.js` `test/pdfs` | `d52fdf4` | 20 | Apache-2.0 | Images, fonts, tagged structure, and version edge cases. |
| `chromium/pdfium` `testing/resources` | `a843234` | 2 | BSD-3-Clause | xref loop and rebuilt-xref regressions. |
| `veraPDF/veraPDF-corpus` `staging` | `bb75f4f` | 2,712 | CC-BY-4.0 | The conformance corpus: 2,691 fetched rows labelled three levels deep by ISO 32000 clause, plus the 21 committed samples. |
| `veraPDF/veraPDF-regression-tests` | `6eb6c68` | 1 | CC0-1.0 | The PDF/A-3a sample. |
| `cvfile/cv` | `539b12d` | 1 | Apache-2.0 | The PDF/A-3u sample, from an upstream `.cv` file. |
| repo-authored | n/a | 22 | repo-authored | Hand-written PostScript, PDF, and gs-argv fixtures. |

The PDF Association licenses the PDF files in `pdf-differences` under CC BY 4.0.
Its Apache 2.0 `LICENSE` file covers source code, not those PDFs, and the README
is the only place the split is written down. A manifest built from the
`LICENSE` badge would mislabel every one of them.

## Fetch and verify

```sh
make validation-fetch         # fetch the live tier
make validation-fetch BULK=1  # add the batch2 bulk tier, a 4.5 GB download
make validation-verify        # check the cache, no network
make validation-report        # per-area and per-basis pass rate
go test -count=1 ./internal/validation -run TestValidation
```

`bulk.tsv` pins whole tarballs. Its only row is the batch2 stressful-corpus
archive: 5,613 files, 6,063,564,131 bytes extracted, and a 4.5 GB download from
`labs.pdfa.org`. `make validation-fetch BULK=1` checks the archive SHA-512,
extracts to a staging directory, checks every member against
`bulk/batch2.members.tsv`, and only then moves a file into the tree, so the
tree only ever sees bytes whose digest is pinned. `make validation-verify
BULK=1` checks the extracted files without touching the network. The bulk tier
is audit material: no manifest row names its files, and `make test` never
reads them.

The generator downloads every selected row into a content-addressed cache,
checks its SHA-256 and byte count, and only places files into the tree after
every row has passed. A mismatch exits non-zero and writes nothing. The bytes
carry no timestamp, so a second run leaves
`git diff --exit-code -- sampledata/validation` clean. Repo-authored rows are
skipped: their bytes are already in the tree and `TestValidationManifest` checks
their digests.

A second run costs no network, because the cache is keyed by digest. Override it
with `-cache DIR` or `SPECTREPS_VALIDATION_CACHE`, which is how a CI cache step
points at a restored directory. Per-host concurrency and rate limits are a table
in `gen.go` rather than a flag, so the numbers are reviewable in a diff, and
every request carries a descriptive User-Agent because sec.gov and Wikimedia
both return 403 without one.

One trap worth recording, because it looks like a wrong pin rather than a bug in
the fetcher: do not set `Accept-Encoding` on the request. `net/http` adds gzip
itself and decompresses the response transparently, but only when the header is
absent. Setting it by hand makes the transport hand back the compressed bytes,
so the digest is taken over the gzip stream. Small files come back uncompressed
and pass, so the failure appears only on the rows big enough for the server to
compress, which reads exactly like a handful of stale pins.

## License gate

The committed tier admits `Apache-2.0`, `MIT`, `BSD-3-Clause`, `CC0-1.0`,
`CC-BY-4.0`, `US-public-domain`, and `repo-authored`. A `CC BY-SA` file is
external-only, `AGPL-3.0` is external-only, and the gate rejects any other license
name. Both of the external-only cases are fetch-and-run, never commit: putting
share-alike or copyleft bytes in this repository would impose the licence on
everything around them.

The gate reads the `license` column, so it can only tell that a string is
admitted, not that the string is true. That is a real limit, and it is how the
hand-built set was once committed under a licence its source does not support.
Recording the URL the licence text was actually read at is row 1.5, and it is
open.

The `openpreserve/format-corpus` rows split by folder, and the split is the
point. `pdfCabinetOfHorrors` carries its own grant in its folder README, "All
files in this folder: Creative Commons CC0: Public Domain Dedication", so those
23 rows are `CC0-1.0`. `pdf-handbuilt-test-corpus` carries no folder-level
grant, only provenance pointing at the iPres 2017 dataset, and that dataset's
rights record at <https://doi.org/10.22000/53> reads "This work is licensed
under CC BY-SA 4.0" and names Michelle Lindlar as rights holder. Those 89 rows
are therefore `CC-BY-SA-4.0` and live in the fetched tier, because
`AdmittedLicense` admits a share-alike row there only.

That distinction was found the hard way. The hand-built set was first committed
with its licence recorded as `CC0-1.0`, on the strength of the repository's root
README alone, and the error was invisible because a wrong licence string
satisfies a gate that reads the string rather than the source. A folder-level
grant is a grant; a root-level blanket claim that a folder then redirects away
from is not. See `external/handbuilt/README.md`.

These source families stay out of the committed tier even where a license
would pass: the Isartor test files, the GhostPDL and MuPDF examples, the iText
resources, the PDFBox testfiles, and the pdf.js third-party-named files
`tracemonkey.pdf`, `TAMReview.pdf`, `firefox_logo.pdf`, `pdfjs_wikipedia.pdf`,
`22060_A1_01_Plans.pdf`, and `openoffice.pdf`. The veraPDF corpus is used
without its `Isartor test files/` folder. `TestValidationLicenses` locks the
gate and the exclusion list.

## Embedded font review

The text samples come from different producers, so every committed text PDF was
opened and its embedded font programs read before its bytes were committed. A
file whose font program cannot be redistributed is excluded, and the
replacement is recorded here. The review table:

| File | Embedded program | Notice | Decision |
| --- | --- | --- | --- |
| `text/Embedded_font.pdf` | `AAAAAA+NotoSansTC-DemiLight`, `/FontFile3` CFF | Copyright 2014, 2015 Adobe Systems Incorporated; Noto is a trademark of Google Inc. Noto is under the SIL Open Font License 1.1 | kept |
| `text/complex_ttf_font.pdf` | `ZCCVRA+font0000000013f5eeab`, `LSUISA+font0000000013f5eeab`, `/FontFile2` | one name record each (the PostScript name), no copyright or license notice | kept, synthetic subset from the pdf.js test suite |
| `text/mixedfonts.pdf` | `DejaVuSans`, `/FontFile2` | Copyright (c) 2003 by Bitstream, Inc., Bitstream Vera license | kept |
| `text/UA1_Tpdf-G5_03.pdf` | `MYJKXS+FreeSans`, `/FontFile2` | the subsetter stripped the name table; GNU FreeFont is GPLv3+ with the font embedding exception | kept |
| `text/standard_fonts.pdf` | none, standard 14 | not embedded | kept |
| `text/IdentityToUnicodeMap_charCodeOf.pdf` | none | not embedded | kept |
| `text/nonembedded_type1_tounicode.pdf` | none | not embedded | kept |
| `text/simpletype3font.pdf` | Type 3, no program | no font file | kept |
| `text/repo-tagged-text.pdf` | none, standard 14 | not embedded | kept |
| `text/subset-text.pdf` | `Synth`, `/FontFile2` | one name record, no copyright or license notice | kept, synthetic program from `internal/truetypesynth` |
| `text/type1-text.pdf` | `SynthType1`, `/FontFile` | one name record, no copyright or license notice | kept, synthetic program from `internal/type1synth` |
| `paths/whatisthis.pdf`, `rewrite/whatisthis.pdf` | `Fira-Sans`, `Fira-Sans-Light`, `Fira-Sans-Bold`, `Fira-Sans-Light-Italic`, `/FontFile2` | Digitized data copyright 2012-2016, The Mozilla Foundation and Telefonica S.A., SIL Open Font License 1.1 | kept |
| `text/arial_unicode_en_cidfont.pdf` | `ArialUnicodeMS`, `/FontFile2` | Monotype/Microsoft commercial font, no redistribution right | excluded, CID TrueType and ToUnicode coverage moved to `text/mixedfonts.pdf` |
| `text/UA1_Tpdf-G2_01.pdf` and its rewrite copy | `Dutch801SWM`, `/FontFile2` | Bitstream Dutch 801, commercial Times clone | excluded, the CC BY 4.0 row moved to `text/UA1_Tpdf-G5_03.pdf` |
| `text/UA1_Tpdf-G2_04.pdf` | `SegoeUISymbol` and `Calibri`, `/FontFile2` | Microsoft commercial fonts | excluded, no replacement |

The `tagged/` and `pdfa/` rows come from the veraPDF corpus, which the
veraPDF Consortium publishes as a set under CC BY 4.0, so the gate treats the
set under that license. The programs those files embed are ArialMT, Times New
Roman, KozMinPro-Bold, IDAutomationHC39M, and AboriginalSerif. The stricter
per-program rule above applies to `text/`, where the files come from many
producers and the pdf.js suite mixes in fonts from unrelated products.

## Substitutions and decisions

- The plan's `text/arial_unicode_en_cidfont.pdf`, `UA1_Tpdf-G2_01.pdf`, and
  `UA1_Tpdf-G2_04.pdf` were excluded by the font review, as listed above.
- `text/mixedfonts.pdf` substitutes for the Arial Unicode MS file: it is a CID
  TrueType page with `/ToUnicode` and a redistributable DejaVu program.
- `text/UA1_Tpdf-G5_03.pdf` and `rewrite/UA1_Tpdf-G5_03.pdf` carry the CC BY
  4.0 coverage: one FreeSans program, visually separated tagged content.
- `text/repo-tagged-text.pdf` is repo-authored. It is a deterministic classic
  xref file, no dates, with a `Document` tree, a `Figure` with `/Alt`, MCIDs, a
  `/Differences` encoding, and a `/ToUnicode` CMap, all over standard 14
  Helvetica. It substitutes for the second accessible-PDF file, because no
  other file in that corpus passes the font review.
- `paths/path.pdf`, `paths/whatisthis.pdf`, `rewrite/path.pdf`,
  `rewrite/whatisthis.pdf`, `gs-argv/path.pdf`, and `gs-argv/gs-argv-input.pdf`
  are copies of files already in this repository. `whatisthis.pdf` was added in
  commit `24ac57d` and its producer is WeasyPrint 68.1; `path.pdf` is a
  hand-written 1.4 file; `gs-argv-input.pdf` is the two-page red and green
  fixture from `sampledata/fixtures/README.md`.
- The existing `sampledata/pdfa/` and `sampledata/pdfua2/` files stay where
  they are. They are not in this manifest: the verifier walks only
  `sampledata/validation`, and a row that points outside that tree would break
  the unlisted-file check and the fetcher. Phase 9 tests read those folders by
  their recorded paths.
- `structural/object-stream.pdf`, `paths/xobject-image.pdf`,
  `images/ccitt_EndOfBlock_false.pdf`, and `images/bug_jpx.pdf` are copied
  into `rewrite/` so each row keeps one canonical path inside the corpus.
- `text/subset-text.pdf` and `text/type1-text.pdf` are copies of
  `sampledata/fixtures/subset-text.pdf` and
  `sampledata/fixtures/type1-text.pdf`. They are the two checked-in programs
  that carry an embedded font outline, so the corpus can drive font subsetting
  and the Type 1 machine without a network fetch. `sampledata/fixtures/`
  keeps the originals, because `internal/cli` reads them there to hold its
  import boundary and `internal/pdf` writes the subset one from
  `TestGenSubsetFixture`. Both copies are listed separately so a change to
  either file has to be made in two places on purpose.
- The 1.5 MiB qpdf `fax-decode-parms.pdf` and the CC BY-SA 4.0 pdf20examples
  file are external-only rows.

## Measured gaps

The `expect` column is the measured verdict. `make validation-run` asserts it
for every committed row, `go test -count=1 ./... -run TestValidation` asserts
the same facts in Go, and `make validation-report` prints the rate per area and
per basis. The current state, measured with the live tier present:

```
manifest rows           2890
rows in the report      2890
pass                    2890
fail                       0
skip                       0
rows run more than once  10   a feature test on top of its area test
```

With the live tier absent, 189 pass and 10 skip, and `make test` stays green
either way. No test opens a network connection, so CI runs without the tier and
treats those rows as skipped rather than passed.

By basis: 2,689 `gs`, 175 `spec`, 26 `baseline`. The 2,689 is the number that
matters. Before the conformance tier landed it was 7, which means almost every
expected outcome in the corpus was being asserted against our own reader. An
expectation derived from the implementation under test is a change detector, not a
specification. Each of the 2,689 is a measured Ghostscript run on the same file, so
it is an independent claim.

The 26 remaining `baseline` rows are the debt: the 23 `pdfCabinetOfHorrors` rows and
the 3 fetched CUPS rows, whose verdicts were recorded from this build rather than
derived from a clause or an oracle.

The earlier state, kept because the deviations it recorded are still current:

- Every committed row passes. `postscript/cups-smiley.ps` paints through the
  new `arc`, `rectstroke`, and `setlinecap` operators. `paths/bug_jpx.pdf`,
  `rewrite/bug_jpx.pdf`, `rewrite/UA1_Tpdf-G5_03.pdf`, and
  `text/UA1_Tpdf-G5_03.pdf` open through the xref `/Prev` chain.
  `paths/xobject-image.pdf` and its rewrite paint through the two-name
  `[/ASCIIHexDecode /DCTDecode]` image chain.
  `images/ccitt_EndOfBlock_false.pdf` paints through the Group 3 row decoder
  and `/EndOfBlock false`. `images/cmykjpeg.pdf` paints. The tagged and
  `simpletype3font` streams read through an indirect `/Length`. `text/dash`
  content is a documented no-op.
- Deviations from the plan's target wording, both recorded:
  - `postscript/cups-testfile.ps` is `refuse:invalidfont`, not `paint`. Its
    text uses the standard 14 fonts, and `documentation/language.md` and
    `documentation/fonts.md` refuse a standard 14 `show` on a device because
    the tree ships no substitute outlines. Every operator the file uses runs;
    an outline program is the one remaining gate.
  - `images/UnknownFilter-xrefstm.pdf` refuses `undefined in XXXDecode`, not
    `undefined in Predictor`. The newest xref stream chains through `/Prev` to
    an older xref stream whose `/Filter` is the unknown `/XXXDecode`, and the
    refusal names that filter, the same contract the stream chain uses.
- `text/type1-text.pdf` is `struct`, not `paint`, and that is the measured
  verdict rather than a shortfall. The page shows three codes and the
  synthetic program carries two of them, so `spectreps text` returns `ABZ`
  with the code-point fallback for the third while `spectreps raster` refuses
  `invalidfont in Tj` and writes no output. The refusal is the no-outline
  policy in `documentation/fonts.md`, applied to a code the program does not
  carry. `TestValidationCorpusType1` asserts both halves.
- Font subsetting and PDF/UA-2 tag generation have no dedicated corpus folder.
  Both are driven over the committed rows by `TestValidationCorpusSubset` and
  `TestValidationCorpusTagGeneration` in `internal/cli`, because each needs a
  row with a particular structure rather than a row of its own: a `/FontFile2`
  program for the subsetter, an untagged page for the tag generator. The rows
  are named in those two tests, and the refusals they assert are the measured
  ones: a clip is `undefined in W`, an image with no `/Alt` is `alt in Tag`,
  and a tagged input is `tagged in RewritePDF`.

Two findings came out of adding the 89 hand-built cases, and both are
recorded rather than fixed.

`external/handbuilt/hello_world.pdf` was first added with `expect: struct`, on the
reasoning that the name and the contents both suggest a valid minimal PDF. That
was wrong. Its cross-reference table gives object 5 an offset of 402, and byte
402 is the middle of the previous object's `endstream`. Ghostscript 9.55.0
reports the same file as damaged. The row is now `survive`, and the feature
column records that the damage is upstream's rather than ours. It is the first
time a corpus row was wrong rather than the reader, and it is why
`plans/v0.0.5/8-real-world-corpus.md` says to classify before fetching.

The 89 hand-built rows all pass the survive contract, so no input panics, hangs,
or faults. The refusals are not spread across the parser, though. 81 of the 89
report `syntaxerror in obj`, 4 report `syntaxerror in pdf`, 3 report
`syntaxerror in xref`, and 1 reports `syntaxerror in endobj`. That uniformity is
a finding rather than a pass: the reader stops at the first structural oddity it
meets, so it does not distinguish a damaged catalog from a damaged page tree
from a damaged cross-reference table, and the 89 cases measure one code path
rather than 89. Ghostscript repairs and continues on the same files. Whether
Spectre stays strict is a policy decision that `documentation/language.md`
already makes, so closing the gap means choosing a recovery policy rather than
patching a parser. It is a deferred row.

### The conformance tier, and why its expectations are not ours

`external/verapdf/` holds 2,691 of the 2,906 files in `veraPDF/veraPDF-corpus`
at commit `bb75f4f`, 158,276,020 bytes. Two folders are left out.
`Isartor test files/` (205) is already refused by the exclusion list in
`internal/validation`. `Undefined/` (10) is left out because the filenames carry
no determinate verdict: veraPDF 1.30.2 fails 3 of those 10, so including them
would report three spurious failures against an expectation nothing defines.

Each row's expected outcome came from running Ghostscript 9.55.0 on the same
file, not from running Spectre:

- `gs -q -dNOPAUSE -dBATCH -dSAFER -sDEVICE=nullpage` on every file
- Ghostscript opened 2,691 of 2,691
- Spectre opened 2,682 of 2,691
- the 9 disagreements are recorded as `survive` with `basis=spec`, because each is
  a documented refusal: three encrypted files, a malformed stream, a malformed
  `/Info` dictionary, a malformed object, a malformed file header, and one wrong
  filter name, which is exactly what the 6.1.6.2 Filters fail case tests

The alternative, recording what our own reader did, would have produced 2,691
`baseline` rows and a pass rate that measured nothing.

Two defects were found while building this and are fixed rather than papered
over. The first staging pass flattened each path to its numeric clause parts,
which collided, because `PDF_A-1b/6.1 File structure/6.1.2 File header/x.pdf`
and `PDF_A-2b/6.1 File structure/6.1.2 File header/x.pdf` reduce to the same
name. 115 names collided, 136 files were overwritten, and a recorded digest
could belong to a different URL than its row claimed. The upstream directory
structure is now preserved on disk, which also puts the ISO clause tree where a
reader can see it.

The report script joined subtest names against manifest paths verbatim, and the
testing package rewrites spaces to underscores, so all 2,691 rows with a space in
their path were dropped and the report claimed 199 rows out of 2,890. It joins on
the mangled form now.

The `Text` goldens under `text/expected/` are written by
`UPDATE_FIXTURES=1 go test ./internal/cli -run TestValidationCorpusText` and
checked in.

## The external tier, measured

The live tier is 2,868 rows, which is every manifest row carrying a pinned URL
minus the 22 repo-authored ones. `make validation-fetch` verifies all of them. The two original rows were fetched on 2026-09-26 with
`go run internal/validation/gen.go -fetch-external`, and both digests matched.
Those two are external for two different reasons:

- `external/fax-decode-parms.pdf` is 1,565,966 bytes, 1.49 MiB, over the 1 MiB
  committed-tier limit. Its license, Apache-2.0, is admitted to the committed
  tier; only its size keeps it out.
- `external/pdf20examples/simple-pdf-2.0.pdf` is 5,211 bytes, well under the
  limit. Its license, CC-BY-SA-4.0, is the reason: `AdmittedLicense` admits a
  CC BY-SA row in the external tier only.

Fetching them changed a recorded verdict. `external/fax-decode-parms.pdf` was
recorded as `paint`, and that value had never been measured, because the row
only runs when the tier is present. Measured, it refuses:

```
Error: /invalidfont in Tj
```

The file is a 10-page scan with an OCR text layer over three non-embedded
standard 14 fonts, `Helvetica-Bold`, `Times-Italic`, and `Times-Roman`, and
`spectreps info` reports `embedded=false` for all three. The refusal is the
no-outline policy in `documentation/fonts.md`, the same one already recorded as
`refuse:invalidfont` on `postscript/cups-testfile.ps`, and not a CCITT defect.
The error itself proves the CCITT path is sound: the refusal names `Tj`, a text
operator, so the page got past all 8 images before the text layer failed. The
`expect` column is now `refuse:invalidfont in Tj`, and the committed row
`images/ccitt_EndOfBlock_false.pdf` still carries the painting CCITT claim.

The tier now also holds `pdfCabinetOfHorrors` (23 real-world files) and `cups`
(3 Scribus PostScript programs). Measured, the 23 real-world files split 17
opened cleanly and 5 refused with a correct named error: four
`invalidaccess in Encrypt` for the encrypted files, which is the documented
refusal, and one `syntaxerror in obj` for the one-byte corruption. One of them,
`test_fontArialNotEmbedded.pdf`, is a 56-page document. Both survive outcomes
occur, so the robustness check is not passing everything.

The 3 CUPS rows are `baseline`, so their `survive` verdict is recorded rather
than derived. They are the best `setcachedevice` and `VM` coverage in any
permissively licensed source, and they are the natural next candidates to
promote to `spec` once someone reads what they actually exercise.

The CUPS rows are pinned at commit `e72b702`, the same revision the two
committed CUPS rows use, not to `master`. An earlier attempt pinned them to
`master` and the digest check failed on all five CUPS rows, because CUPS has
changed those files since `e72b702`. That is the pin working: a wrong URL
produces a mismatch rather than a silently different corpus file.

`govdocs1-error-pdfs` from the same upstream is 54 real-world files that
defeated a preservation tool, which makes it the most valuable group found. It
is not pinned, because its own README says the files are copied from GovDocs1
and quotes a free-redistribution claim for research, which is not the CC0 the
parent folder asserts. A wrong licence column is worse than a missing row.
