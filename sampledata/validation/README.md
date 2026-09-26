# Validation corpus

Real PDF and PostScript files for the v0.0.4 validation suite. Every corpus
file has one row in `manifest.tsv` with its source, pinned commit, license,
SHA-256, feature, and expected verdict; `README.md` files carry the prose.
Tests read the manifest through `internal/validation` and are named
`TestValidation<Area>`, so
`go test -count=1 ./... -run TestValidation` runs the group.

The folders follow the feature areas: `compatibility/` for version and known
feature gaps, `postscript/` for interpreter programs,
`paths/` for path and paint operators, `structural/` for xref and object
streams, `images/` for Flate, DCT, CCITT, and JPEG2000 decode, `text/` for
fonts and extraction, `tagged/` and `pdfa/` for the profile preflights,
`rewrite/` for the writers, `gs-argv/` for the bounded `gs` mode, and `refs/`
for the external Ghostscript reference proofs in Phase 11.

## Tiers

The committed tier is checked in and every file is at or under 1 MiB. Whole
suites and larger files live under `sampledata/validation/external/`, which is
gitignored and fetched by the same generator with `-fetch-external`. Tests
skip the external tier when it is absent, and no test needs the network.

## Manifest

`manifest.tsv` is tab-separated with the header
`path`, `sha256`, `bytes`, `source`, `license`, `feature`, `expect`. Paths are
relative to `sampledata/validation` and use forward slashes. `source` is the
pinned `raw.githubusercontent.com` URL for a fetched file, or `repo-authored`
for a file that already lives in this tree. `expect` is one of:

- `paint`: the corpus job rasterizes the file without an error.
- `struct`: the file opens and its page structure is the claim; painting is
  not asserted. Text extraction, tags, preflight, and rewrite cases use this.
- `refuse:<JobError.Msg>`: the corpus job must return an error whose message is
  the exact text after `refuse:`, and must write no output.

Tests read the table with `validation.Load()`, then call
`validation.CheckFiles(rows, validation.CorpusDir())`. `Row.External()`,
`Row.Refused()`, and `Row.RefuseError()` classify a row. `CheckLicenses` and
`CheckExcluded` enforce the gate below.

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

Every row, sorted by path. The source label names the repository and the
upstream path; the commit is the full commit the URL pins.
The PDF Association licenses the PDF files in `pdf-differences` under CC BY 4.0.
Its Apache 2.0 license covers source code, not those PDFs.

| Path | Source | Commit | License | SHA-256 | Feature | Expect |
| --- | --- | --- | --- | --- | --- | --- |
| `compatibility/encryption/encrypted-40-bit-R3.pdf` | qpdf/qpdf qpdf/qtest/qpdf/encrypted-40-bit-R3.pdf | `54d6053` | Apache-2.0 | `221b17165d2c807500165c7890f6f8f355b6cf62ecdb430d9bae71cec17b8369` | PDF 1.4 encrypted input, reader refuses at open | `refuse:invalidaccess in Encrypt` |
| `compatibility/fonts/Type3Test.pdf` | pdf-association/pdf-differences Type3WordSpacing/Type3Test.pdf | `907fe96` | CC-BY-4.0 | `8ac40dc49d40af8b5a50b442ced56b8db962c961a779a0284cd3a6207adc0bb7` | PDF 1.7 Type 3 font, reader opens but painter refuses | `refuse:invalidfont in Tj` |
| `compatibility/versions/PDF-versions1.pdf` | pdf-association/pdf-differences PDF-version/PDF-versions1.pdf | `907fe96` | CC-BY-4.0 | `94b949fc0fcd6e81f19a262f5d49be684e6db135ee4bac7b67f22625edac31b7` | PDF 1.4 header upgraded to 1.6 by incremental catalog generation | `struct` |
| `compatibility/versions/PDF-versions2.pdf` | pdf-association/pdf-differences PDF-version/PDF-versions2.pdf | `907fe96` | CC-BY-4.0 | `e03ffc599c0b47c66b98650742eec25f5f36ee37bc53ac9c08fb5e15f5d3d65e` | PDF 1.4 header upgraded to 1.6 by replacement catalog | `struct` |
| `compatibility/versions/PDF-versions3.pdf` | pdf-association/pdf-differences PDF-version/PDF-versions3.pdf | `907fe96` | CC-BY-4.0 | `690ec13b9c407aa351eb863ffe712bf42432ecb47e0e9b27f322e65ae3d4e287` | PDF 1.6 effective version with a PDF 2.0-only graphics-state feature | `refuse:undefined in gs` |
| `compatibility/versions/asciihexdecode.pdf` | mozilla/pdf.js test/pdfs/asciihexdecode.pdf | `d52fdf4` | Apache-2.0 | `17cc92aa58ec48f75b01c6b6e7f8c8bade0a36f7295d0f833a16bafae854dc79` | PDF 1.0 header and ASCIIHexDecode page | `struct` |
| `compatibility/versions/poppler-67295-0.pdf` | mozilla/pdf.js test/pdfs/poppler-67295-0.pdf | `d52fdf4` | Apache-2.0 | `f513f3b4b4a1cb223622e91f03701c0f396e6eed47121fb8cba5c0e515357d3b` | PDF 1.2 header and page structure | `struct` |
| `external/fax-decode-parms.pdf` | qpdf/qpdf qpdf/qtest/qpdf/fax-decode-parms.pdf | `54d6053` | Apache-2.0 | `6cf3a42a79698eb9a6649d51a78647dccb05dd88dbc0b384b843618963f8755e` | CCITT fax DecodeParms, 1.5 MiB | `refuse:invalidfont in Tj` |
| `external/pdf20examples/simple-pdf-2.0.pdf` | pdf-association/pdf20examples Simple PDF 2.0 file.pdf | `c20f2c1` | CC-BY-SA-4.0 | `296d2a0b2ce19b606f29265694f194a754fbc61783982b5b8d730e8637482236` | PDF 2.0 container | `struct` |
| `gs-argv/gs-argv-input.pdf` | repo-authored | `repo` | repo-authored | `24a87a42435a40f8852dd0693a9eba916e9f1f0306418c494330c3be2329c822` | gs argv two-page red and green input | `paint` |
| `gs-argv/page.ps` | repo-authored | `repo` | repo-authored | `7a22578e24030868d84d0316bd8e1915a048918e142426c77e29c3600cd3e1b3` | gs argv PostScript input | `paint` |
| `gs-argv/path.pdf` | repo-authored | `repo` | repo-authored | `9ade703482ba3755932d5bec7e0dfdb737fb37146e0abfc691281ba7491a0bfd` | gs argv PDF input | `paint` |
| `images/UnknownFilter-xrefstm.pdf` | pdf-association/pdf-differences UnknownFilter/UnknownFilter-xrefstm.pdf | `907fe96` | CC-BY-4.0 | `2c153eb1a00e2583d03e939b116b623fe78b70be18e4308347cfcf5f2c4c3c81` | xref stream with an unknown filter | `refuse:undefined in XXXDecode` |
| `images/bug_jpx.pdf` | mozilla/pdf.js test/pdfs/bug_jpx.pdf | `d52fdf4` | Apache-2.0 | `4010d808557a72278368c05518367735ed186cbdd04974b8506275f9ac40f999` | JPEG2000 image in an object stream | `paint` |
| `images/ccitt_EndOfBlock_false.pdf` | mozilla/pdf.js test/pdfs/ccitt_EndOfBlock_false.pdf | `d52fdf4` | Apache-2.0 | `f2289b94e7e3f05f9e37754c399d3f525edf9444fb32c3279959cf64b575fe62` | CCITT G4 image with EndOfBlock false | `paint` |
| `images/cmykjpeg.pdf` | mozilla/pdf.js test/pdfs/cmykjpeg.pdf | `d52fdf4` | Apache-2.0 | `659d6b19912f63db988b0b26b9bde0e6d8100667ef162051a4a84ea8e5b90272` | CMYK DCT image inside marked content | `paint` |
| `images/jp2k-resetprob.pdf` | mozilla/pdf.js test/pdfs/jp2k-resetprob.pdf | `d52fdf4` | Apache-2.0 | `582da92f1cad4fa47639e5df5c29ebba6e4ae432ba5e9283297ee6e5e88d7efa` | JPEG2000 reset probability case | `paint` |
| `images/jpx_lzw.pdf` | chromium/pdfium testing/resources/jpx_lzw.pdf | `a843234` | BSD-3-Clause | `ba14897a4139944e56674fefe176a56f29326e69ff2baf3e07ba096b520def78` | JPX image behind an LZW filter chain | `refuse:undefined in Do` |
| `images/jpx_smaskindata.pdf` | mozilla/pdf.js test/pdfs/jpx_smaskindata.pdf | `d52fdf4` | Apache-2.0 | `a82813559c74f074b0caf92c80875fc018acb737269a2af5508c7e083b2a5fd3` | JPEG2000 image with SMask data | `paint` |
| `paths/path.pdf` | repo-authored | `repo` | repo-authored | `9ade703482ba3755932d5bec7e0dfdb737fb37146e0abfc691281ba7491a0bfd` | PDF path operators, repeated strokes | `paint` |
| `paths/whatisthis.pdf` | repo-authored | `repo` | repo-authored | `2c9359dfd8402ee9b78b4f08a6c9d2ce4bb309d8dc41b2c8b4c040433929dfe2` | PDF 1.7 text, DCT image, object containers | `struct` |
| `paths/xobject-image.pdf` | mozilla/pdf.js test/pdfs/xobject-image.pdf | `d52fdf4` | Apache-2.0 | `9bd4a6f17bac7b6d218ae0f8d28ea80f7025948499fa3686e46d6950cac6ab9a` | PDF image XObject through Do | `paint` |
| `pdfa/4f-6-7-3-t01-pass-a.pdf` | veraPDF/veraPDF-corpus PDF_A-4f/6.7 Metadata/6.7.3 Version identification/veraPDF test suite 6-7-3-t01-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `8c6e0bfcbeaaf831723a6b981ea57b1bc04f5a86da3fd912552fb3f6f5686e1d` | PDF/A-4f version identification, pass | `struct` |
| `pdfa/negative/4-6-1-8-t01-fail-a.pdf` | veraPDF/veraPDF-corpus PDF_A-4/6.1 File structure/6.1.8 Indirect objects/veraPDF test suite 6-1-8-t01-fail-a.pdf | `bb75f4f` | CC-BY-4.0 | `950c46606fa7532198ca5f208936da17f54a4c680ef11edc8152089484baa084` | PDF/A-4 indirect objects, fail a | `struct` |
| `pdfa/negative/4-6-1-8-t01-fail-b.pdf` | veraPDF/veraPDF-corpus PDF_A-4/6.1 File structure/6.1.8 Indirect objects/veraPDF test suite 6-1-8-t01-fail-b.pdf | `bb75f4f` | CC-BY-4.0 | `5485315331a04633bb2fc0a3297610ddfb197bf2b8811507222e3d9a73f1a8f4` | PDF/A-4 indirect objects, fail b | `struct` |
| `pdfa/negative/4f-6-7-3-t01-fail-a.pdf` | veraPDF/veraPDF-corpus PDF_A-4f/6.7 Metadata/6.7.3 Version identification/veraPDF test suite 6-7-3-t01-fail-a.pdf | `bb75f4f` | CC-BY-4.0 | `ab9e5e4244ced310c6168465c8e2725b2c03007e15b23f406be5d9cf30c614da` | PDF/A-4f version identification, fail a | `struct` |
| `pdfa/profiles/1a.pdf` | veraPDF/veraPDF-corpus PDF_A-1a/6.8 Logical structure/6.8.2 Tagged PDF/6.8.2.2 Mark information dictionary/veraPDF test suite 6-8-2-2-t01-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `b5d194c0e6d91119f0f99354f3d1078877406b695cecf75fb0b5cb51b18c14fd` | PDF/A-1a compliant sample; veraPDF test suite 6-8-2-2-t01-pass-a.pdf | `struct` |
| `pdfa/profiles/1b.pdf` | veraPDF/veraPDF-corpus PDF_A-1b/6.2 Graphics/6.2.3.3 Uncalibrated color space/veraPDF test suite 6-2-3-3-t03-pass-d.pdf | `bb75f4f` | CC-BY-4.0 | `2a5ba3a4c85ace9bbece0543634ba5447eb51276572424c1a9715d5b758a12a2` | PDF/A-1b compliant sample; veraPDF test suite 6-2-3-3-t03-pass-d.pdf | `struct` |
| `pdfa/profiles/2a.pdf` | veraPDF/veraPDF-corpus PDF_A-2a/6.7 Logical structure/6.7.3 Artefacts/6.7.3.4 Structure types/veraPDF test suite 6-7-3-4-t01-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `e715e6cd3061bf660d4b15043ea045637c13f3470bf03fff693e721656b63b0f` | PDF/A-2a compliant sample; veraPDF test suite 6-7-3-4-t01-pass-a.pdf | `struct` |
| `pdfa/profiles/2b.pdf` | veraPDF/veraPDF-corpus PDF_A-2b/6.1 File structure/6.1.13 Implementation limits/veraPDF test suite 6-1-13-t09-pass-b.pdf | `bb75f4f` | CC-BY-4.0 | `5297b152a1a0cbc348084a58c90ceb6b24d6ac20093708a0b87769758fc379ae` | PDF/A-2b compliant sample; veraPDF test suite 6-1-13-t09-pass-b.pdf | `struct` |
| `pdfa/profiles/2u.pdf` | veraPDF/veraPDF-corpus PDF_A-2u/6.2 Graphics/6.2.11 Fonts/6.2.11.7 Unicode character maps/6.2.11.7.2 Level A and Level U conformance/veraPDF test suite 6-2-11-7-2-t01-pass-f.pdf | `bb75f4f` | CC-BY-4.0 | `c77ba0dd71e43823e1fb6a087889a1774ae8c3ea127a8e6861b7b0180eaf2c94` | PDF/A-2u compliant sample; veraPDF test suite 6-2-11-7-2-t01-pass-f.pdf | `struct` |
| `pdfa/profiles/3a.pdf` | veraPDF/veraPDF-regression-tests PDF_A-3a/1224/test-fixed.pdf | `6eb6c68` | CC0-1.0 | `4f2f7f75e453c0dd8de0c372f151eb414317990658a378c0c0baf95231bd45c2` | PDF/A-3a compliant regression sample; embedded Latin Modern font uses GFL | `struct` |
| `pdfa/profiles/3b.pdf` | veraPDF/veraPDF-corpus PDF_A-3b/6.8 Embedded files/veraPDF test suite 6-8-t02-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `5b8fdee7090f0a0fb7266d10dd0f6f1f8a37d286e3c06b1aa82e01d6d4f0a8a4` | PDF/A-3b compliant sample; veraPDF test suite 6-8-t02-pass-a.pdf | `struct` |
| `pdfa/profiles/3u.pdf` | cvfile/cv integrations/tests/fixtures/unicode.cv | `539b12d` | Apache-2.0 | `fe6328e694d37c0b3005d61e40ec6edf826dbc9acda00b7467de0df6469ca1e4` | PDF/A-3u compliant sample; upstream .cv is a PDF file | `struct` |
| `pdfa/profiles/4.pdf` | veraPDF/veraPDF-corpus PDF_A-4/6.2 Graphics/6.2.4 Colour spaces/6.2.4.3 Uncalibrated -Device colour spaces/veraPDF test suite 6-2-4-3-t01-pass-e.pdf | `bb75f4f` | CC-BY-4.0 | `b2b40e60a6a8e2a52cedd2ad520357057a7b48565ed98b566d08bf2d55d8eb46` | PDF/A-4 compliant sample; veraPDF test suite 6-2-4-3-t01-pass-e.pdf | `struct` |
| `pdfa/profiles/4e.pdf` | veraPDF/veraPDF-corpus PDF_A-4e/6.7 Metadata/6.7.3 Version identification/veraPDF test suite 6-7-3-t01-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `6b1c9c0fc488abd2fd6a34daf846dd882c45350cf9ee165c6313dc5b7d57a08e` | PDF/A-4e compliant sample; veraPDF test suite 6-7-3-t01-pass-a.pdf | `struct` |
| `postscript/cups-smiley.ps` | OpenPrinting/cups data/smiley.ps | `e72b702` | Apache-2.0 | `1d3bf1f1f3f6426591e696660ff1cbf2f70d1307721e0cb999926cc80c977bb9` | CUPS smiley page, arc and rectstroke | `paint` |
| `postscript/cups-testfile.ps` | OpenPrinting/cups examples/testfile.ps | `e72b702` | Apache-2.0 | `858d4c9ac31128ae7ef634d3d8b4a870d2ba34d76ca9357e9104c85bc5f99523` | CUPS test page, graphics; standard 14 text needs an outline program | `refuse:invalidfont` |
| `postscript/curve.ps` | repo-authored | `repo` | repo-authored | `62c8bde9c35c2f91f42743fcd916d136db4b5b9761b096ec0032108b9eb0d763` | PostScript cubic curve | `paint` |
| `postscript/eofill.ps` | repo-authored | `repo` | repo-authored | `43b8821a29d0d45034084111abf110b04e399f105801ff4f67e1a67ecdb454b1` | PostScript even-odd fill | `paint` |
| `postscript/gray.ps` | repo-authored | `repo` | repo-authored | `a477fb71d1ea8be88711eddf9801bb2c12ff0f4a544132a1d7524e7c2ef9e7a2` | PostScript setgray levels | `paint` |
| `postscript/limits.ps` | repo-authored | `repo` | repo-authored | `4ec5844223065829436390aafbe12834fe20d5db2c6fbec53d6fe378d315b911` | PostScript path point cap | `refuse:limitcheck` |
| `postscript/line.ps` | repo-authored | `repo` | repo-authored | `97e4d30badf862f62d7cad6305daa9988dfb35922ef702952101f8d3b7d57f49` | PostScript axis-aligned strokes | `paint` |
| `postscript/rect.ps` | repo-authored | `repo` | repo-authored | `2ff2bbbf59e05e4c19d3d86dd4d25618d7be2a61bbb168007877d584a09cc2c8` | PostScript axis-aligned fill | `paint` |
| `postscript/twopage.ps` | repo-authored | `repo` | repo-authored | `e87502e58e09a397f2d83871bd65203cc743869798a7039742bb9bf3f1fc0527` | PostScript two pages | `paint` |
| `refs/curve.ps` | repo-authored | `repo` | repo-authored | `62c8bde9c35c2f91f42743fcd916d136db4b5b9761b096ec0032108b9eb0d763` | gs reference, cubic curve | `paint` |
| `refs/diag.ps` | repo-authored | `repo` | repo-authored | `14829addcd3ce21bfe5b945d7385309207a5f80726e34097b555c2db91dd810d` | gs reference, diagonal stroke | `paint` |
| `refs/line.ps` | repo-authored | `repo` | repo-authored | `97e4d30badf862f62d7cad6305daa9988dfb35922ef702952101f8d3b7d57f49` | gs reference, axis-aligned stroke | `paint` |
| `refs/rect.ps` | repo-authored | `repo` | repo-authored | `2ff2bbbf59e05e4c19d3d86dd4d25618d7be2a61bbb168007877d584a09cc2c8` | gs reference, axis-aligned fill | `paint` |
| `rewrite/UA1_Tpdf-G5_03.pdf` | pdf-association/techniques-for-accessible-pdf fundamentals/5-appropriate-semantics/G5_03-Visually-separated-content-appropriately-tagged/UA1_Tpdf-G5_03.pdf | `3e31095` | CC-BY-4.0 | `071680cd1480e1ebe83ccab3b822b6520a0681d630fdb0975737c37c716147b7` | Rewrite a CC BY 4.0 tagged file | `struct` |
| `rewrite/bug_jpx.pdf` | mozilla/pdf.js test/pdfs/bug_jpx.pdf | `d52fdf4` | Apache-2.0 | `4010d808557a72278368c05518367735ed186cbdd04974b8506275f9ac40f999` | Rewrite a JPEG2000 image page | `struct` |
| `rewrite/ccitt_EndOfBlock_false.pdf` | mozilla/pdf.js test/pdfs/ccitt_EndOfBlock_false.pdf | `d52fdf4` | Apache-2.0 | `f2289b94e7e3f05f9e37754c399d3f525edf9444fb32c3279959cf64b575fe62` | Rewrite a CCITT image page | `struct` |
| `rewrite/object-stream.pdf` | qpdf/qpdf qpdf/qtest/qpdf/object-stream.pdf | `54d6053` | Apache-2.0 | `7e895514a76bff9c9bb699a14275735c9f5c61415fda7acc1e3e80d3003bc730` | Rewrite a container PDF | `struct` |
| `rewrite/path.pdf` | repo-authored | `repo` | repo-authored | `9ade703482ba3755932d5bec7e0dfdb737fb37146e0abfc691281ba7491a0bfd` | Rewrite levels 0 to 5, no images | `struct` |
| `rewrite/structure_simple.pdf` | mozilla/pdf.js test/pdfs/structure_simple.pdf | `d52fdf4` | Apache-2.0 | `5b5510bb5f91c86aa0bbd4882842d4583c814e0e69a0cfbff0e70dc250a8d0f3` | Rewrite a tagged structure sample | `struct` |
| `rewrite/whatisthis.pdf` | repo-authored | `repo` | repo-authored | `2c9359dfd8402ee9b78b4f08a6c9d2ce4bb309d8dc41b2c8b4c040433929dfe2` | Rewrite levels 1 to 5, text and DCT image | `struct` |
| `rewrite/xobject-image.pdf` | mozilla/pdf.js test/pdfs/xobject-image.pdf | `d52fdf4` | Apache-2.0 | `9bd4a6f17bac7b6d218ae0f8d28ea80f7025948499fa3686e46d6950cac6ab9a` | Rewrite an image XObject page | `struct` |
| `structural/bad-xref.pdf` | qpdf/qpdf qpdf/qtest/qpdf/bad-xref.pdf | `54d6053` | Apache-2.0 | `5895dfedf978f600b12e29643617dbb111869b60526d957878e349571e1467d1` | Broken classic xref | `refuse:syntaxerror in xref` |
| `structural/bug_xrefv4_loop.pdf` | chromium/pdfium testing/resources/bug_xrefv4_loop.pdf | `a843234` | BSD-3-Clause | `fd7c7174ed00dd18dec996dfffeacbb3524632c02f406c05d8fc560cb87c2c28` | xref v4 loop regression | `refuse:syntaxerror in xref` |
| `structural/compress-objstm-xref.pdf` | qpdf/qpdf qpdf/qtest/qpdf/compress-objstm-xref.pdf | `54d6053` | Apache-2.0 | `7aa4aea24328616ce247cc4132efa3b8f53d2844179d17b51383f8771d2de358` | Compressed object stream with xref stream | `struct` |
| `structural/object-stream.pdf` | qpdf/qpdf qpdf/qtest/qpdf/object-stream.pdf | `54d6053` | Apache-2.0 | `7e895514a76bff9c9bb699a14275735c9f5c61415fda7acc1e3e80d3003bc730` | xref stream and object stream | `struct` |
| `structural/page-with-no-resources.pdf` | qpdf/qpdf qpdf/qtest/qpdf/page-with-no-resources.pdf | `54d6053` | Apache-2.0 | `cc6b72d3232f27a5704d8c4d07ebe78b3e55afe6be11af4214b633eb2a2b5581` | Page with no /Resources | `paint` |
| `structural/parser_rebuildxref_error_notrailer.pdf` | chromium/pdfium testing/resources/parser_rebuildxref_error_notrailer.pdf | `a843234` | BSD-3-Clause | `d47175f63d3734205e652c3640a42214b956062c299ce761d4c0353e69bac1f8` | Rebuilt xref with no trailer | `refuse:syntaxerror in xref` |
| `structural/xref-range.pdf` | qpdf/qpdf qpdf/qtest/qpdf/xref-range.pdf | `54d6053` | Apache-2.0 | `2a17e8c9dceb4e204173c6233ab39b506af651f69b8be2f58ed60e79516b42eb` | xref stream with /Index ranges | `struct` |
| `tagged/8.4.5.8-t01-pass-a.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t01-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `acf85833e0580624ba647b9ff3c57d9dc8ba3a17911ecdbe3adbe350c43431e9` | PDF/UA-2 Unicode character maps, t01 pass a | `struct` |
| `tagged/8.4.5.8-t01-pass-b.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t01-pass-b.pdf | `bb75f4f` | CC-BY-4.0 | `64ee7d5bf187f79b244128baecf1b55afa5f05861f0deaccef414b1d538f73c6` | PDF/UA-2 Unicode character maps, t01 pass b | `struct` |
| `tagged/8.4.5.8-t01-pass-c.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t01-pass-c.pdf | `bb75f4f` | CC-BY-4.0 | `2f4c00947a0b33bf828781423cf8a5d0614962def11cb66b8b45ee0de3777c52` | PDF/UA-2 Unicode character maps, t01 pass c | `struct` |
| `tagged/8.4.5.8-t02-pass-a.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t02-pass-a.pdf | `bb75f4f` | CC-BY-4.0 | `f1a0fdd214f15acf172914004bfadc41c437b0acb927c7b37494017dd0db8a0b` | PDF/UA-2 Unicode character maps, t02 pass a | `struct` |
| `tagged/negative/8.4.5.8-t01-fail-a.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t01-fail-a.pdf | `bb75f4f` | CC-BY-4.0 | `ac490e87b5c392dfcc5a899427849f1a2a2ee2e77f0bc6344f0ef5c82bf01656` | PDF/UA-2 Unicode character maps, t01 fail a | `struct` |
| `tagged/negative/8.4.5.8-t02-fail-a.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t02-fail-a.pdf | `bb75f4f` | CC-BY-4.0 | `2ef138c3eb0395bdc3a2cf81959ce19d0f2fae8e3ebcabb23bef4b0b91fad453` | PDF/UA-2 Unicode character maps, t02 fail a | `struct` |
| `tagged/negative/8.4.5.8-t02-fail-b.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t02-fail-b.pdf | `bb75f4f` | CC-BY-4.0 | `0e4e72d4911bf13eb1077d8d20058c57763a3b66905232c625357727a5eebfa3` | PDF/UA-2 Unicode character maps, t02 fail b | `struct` |
| `tagged/negative/8.4.5.8-t02-fail-c.pdf` | veraPDF/veraPDF-corpus PDF_UA-2/8.4 Text representation for content/8.4.5 Fonts/8.4.5.8 Unicode character maps/8.4.5.8-t02-fail-c.pdf | `bb75f4f` | CC-BY-4.0 | `d9f9859bc58e62320918398209aed9d63ca4a7f3866394768a62bb92885a38ff` | PDF/UA-2 Unicode character maps, t02 fail c | `struct` |
| `text/Embedded_font.pdf` | mozilla/pdf.js test/pdfs/Embedded_font.pdf | `d52fdf4` | Apache-2.0 | `4b7588e070cd37c7cfa2ce0a493ba9ec6d57fb36a2ca3c6ab50c74a7818a2f9a` | OpenType FontFile3, Noto Sans TC | `struct` |
| `text/IdentityToUnicodeMap_charCodeOf.pdf` | mozilla/pdf.js test/pdfs/IdentityToUnicodeMap_charCodeOf.pdf | `d52fdf4` | Apache-2.0 | `81243ff4cca5f7898688008224247d88534cd03e76ac3cbbdc38f925cfbee18f` | Identity ToUnicode map extraction | `struct` |
| `text/UA1_Tpdf-G5_03.pdf` | pdf-association/techniques-for-accessible-pdf fundamentals/5-appropriate-semantics/G5_03-Visually-separated-content-appropriately-tagged/UA1_Tpdf-G5_03.pdf | `3e31095` | CC-BY-4.0 | `071680cd1480e1ebe83ccab3b822b6520a0681d630fdb0975737c37c716147b7` | Tagged text with GNU FreeSans | `struct` |
| `text/complex_ttf_font.pdf` | mozilla/pdf.js test/pdfs/complex_ttf_font.pdf | `d52fdf4` | Apache-2.0 | `4b205d3fd11e7ad2b9b86cc446561909d48dd9e0b38327cfd69d0fdeb6c28e14` | Synthetic TTF text with clipping | `struct` |
| `text/mixedfonts.pdf` | mozilla/pdf.js test/pdfs/mixedfonts.pdf | `d52fdf4` | Apache-2.0 | `04f17b84112d8130d75601f8f9c2027481a19df595aacb753ca3f9f3b08b2350` | CID TrueType DejaVu and ToUnicode | `struct` |
| `text/nonembedded_type1_tounicode.pdf` | mozilla/pdf.js test/pdfs/nonembedded_type1_tounicode.pdf | `d52fdf4` | Apache-2.0 | `95ef7cc7e6c80b3ce6e884407b0b695898a6f47a30b339d7472657406f041a82` | Non-embedded Type 1 with ToUnicode | `struct` |
| `text/repo-tagged-text.pdf` | repo-authored | `repo` | repo-authored | `e4f5500fe4b6c8a50f847e22082fa33e43820e28692bcb265faf22e8f00d0e95` | Repo-authored tagged text, Differences and ToUnicode | `struct` |
| `text/simpletype3font.pdf` | mozilla/pdf.js test/pdfs/simpletype3font.pdf | `d52fdf4` | Apache-2.0 | `a5697cbc51c19816944658b91e3a41f86e98ae63fbd1007798994b073385aef1` | Type 3 font with no program | `struct` |
| `text/standard_fonts.pdf` | mozilla/pdf.js test/pdfs/standard_fonts.pdf | `d52fdf4` | Apache-2.0 | `4d48fc12e619a823bf70b3a4d8e9d916a85683ecb176d138af66a57351bd21f9` | Standard 14 metrics and extraction | `struct` |
| `text/subset-text.pdf` | repo-authored | `repo` | repo-authored | `14b0bc415bbdafcc4177deb34c74ca7e555c4a2e1de114134857f25709220ceb` | Synthetic TrueType FontFile2 for subset validation | `struct` |
| `text/type1-text.pdf` | repo-authored | `repo` | repo-authored | `23749b69f83db2b1a646757edbcebb9e79e878adb25b8b8b446abdea569b089e` | Symbolic Type 1 program with seac and flex glyphs | `struct` |

## Fetch and verify

```sh
go run internal/validation/gen.go                 # committed tier only
go run internal/validation/gen.go -fetch-external # plus the external tier
go test -count=1 ./internal/validation -run TestValidation
```

The generator downloads every selected row, checks its SHA-256 and byte count,
and only writes files after every row has passed. A mismatch exits non-zero and
writes nothing. The bytes carry no timestamp, so a second run leaves
`git diff --exit-code -- sampledata/validation` clean. Repo-authored rows are
skipped: their bytes are already in the tree and `TestValidationManifest`
checks their digests.

## License gate

The committed tier admits `Apache-2.0`, `MIT`, `BSD-3-Clause`, `CC0-1.0`,
`CC-BY-4.0`, `US-public-domain`, and `repo-authored`. A `CC BY-SA` file is
external-only: `external/pdf20examples/simple-pdf-2.0.pdf` is the one such row.
The gate rejects AGPL and any other license name.

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
for every committed row, and `go test -count=1 ./... -run TestValidation`
asserts the same facts in Go. The state after the v0.0.4 validation work:

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

The external tier (`external/`) holds fetched, non-committed files and is
skipped cleanly when absent. The `Text` goldens under `text/expected/` are
written by `UPDATE_FIXTURES=1 go test ./internal/cli -run
TestValidationCorpusText` and checked in.

## The external tier, measured

Both external rows were fetched on 2026-09-26 with
`go run internal/validation/gen.go -fetch-external`, and both digests matched.
They are external for two different reasons:

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

With the tier present the corpus is 66 pass, 0 fail, 0 skipped. With it absent,
64 pass, 0 fail, 2 skipped, and `make test` stays green either way. No test
opens a network connection, so CI runs without this tier and treats those two
rows as skipped rather than passed.
