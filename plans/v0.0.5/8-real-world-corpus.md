# v0.0.5 - Real-world corpus

> **Parent:** `plans/v0.0.5/00-program.md` - benchmark coverage and PDF compatibility
> **Status:** open. Every row below is `[ ]`. Nothing here has been implemented.
> **Estimated effort:** phase 1 and 2 about a week. Phase 3 is open-ended and gated on a decision this file does not make.

---

## Overview

The committed corpus under `sampledata/validation/` is 128 files, of which 84 rows are in `manifest.tsv`. Most were written by this project to prove a feature that was built first. That ordering is backwards for measuring coverage, and it is the reason the corpus cannot answer "what does Spectre fail on".

This file records where several thousand real files come from, what they cost to fetch, what they are licensed under, and what will happen the first time they run. It also defines the label axes, because a single `feature` column cannot carry a per-feature pass rate.

Every source below was verified against its primary source, not a write-up about it. Where a claim could not be verified it is in the last section rather than softened into a maybe.

## The finding that reorders the work

The dominant image encoding in real scanned PDFs is not CCITT and not JPEG. It is JBIG2 plus JPEG2000.

Measured on `grenfelloflabrad0000john`, 264 pages, 11.5 MB, by counting filter tokens in the file:

```
/Subtype /Image   792     (3 image XObjects per page)
/SMask            264
JBIG2Decode       264
JPXDecode         528
DCTDecode           0
CCITTFaxDecode      0
```

Each page is a JPX background, a second JPX carrying `/SMask`, and a 1-bit JBIG2 mask. This is Internet Archive's standard pipeline and it is documented as such: the `archive-pdf-tools` README says the mask "is losslessly compressed (typically using either JBIG2 or CCITT)" and that it produced over 6 million PDFs in 2021 alone. The `pdf_module_version` metadata field records which generator made a given file.

`golang.org/x/image` v0.46.0 has `ccitt`, `tiff`, `webp`, `vp8`, `vp8l`. It has no JBIG2 and no JPEG2000 decoder. I checked the module proxy for `gen2brain/go-jpeg2000` and `dslipak/jbig2`; both 404. Whether a usable pure-Go JP2 decoder exists anywhere is unverified.

So if the live tier is pointed at Internet Archive or Wikimedia scan corpora, the pass rate measures the refusal path, not the renderer. That is not useless, it is a different product claim, and the honest form of it is two numbers per file: "renders correctly" and "detects the unsupported filter and reports it". Row 3.1 exists because of this.

## The live folder already exists

There is no new mechanism to add. The getting path is already built:

- `sampledata/validation/manifest.tsv`, seven columns: `path`, `sha256`, `bytes`, `source`, `license`, `feature`, `expect`
- `sampledata/validation/external/`, gitignored, for files that are fetched rather than committed
- `go run internal/validation/gen.go -fetch-external`, which fetches and verifies SHA-256
- `internal/validation/manifest.go` with `AdmittedLicense` and `CheckFiles`
- tests that skip the external tier when it is absent, and no test that opens a socket

Two files already use it: `sampledata/validation/external/fax-decode-parms.pdf` and `external/pdf20examples/simple-pdf-2.0.pdf`.

This file extends that shape. It does not add a second fetch tool, a second manifest, or a second cache. The `gen.go` CLI grows flags, and `manifest.tsv` grows columns. That is the whole change.

## Feature labels

One `feature` string cannot produce a per-feature pass rate, because a file carries at least three independent axes: which construct is present, which conformance flavour it targets, and what the expected verdict is. veraPDF's own corpus separates them into a directory tree, a filename, and a `-pass-`/`-fail-` marker.

The manifest grows four columns, each of which deletes existing code rather than adding a concept:

| Column | Job | Replaces |
| --- | --- | --- |
| `area` | controlled key, the pass-rate group | the seven hardcoded folder lists in `TestValidationCorpus*` |
| `probe` | `raster`, `info`, `rewrite`, `text`, `gs-paint`, or `gs-write` | path-prefix inference in `corpusRunRow` |
| `basis` | `spec`, `gs`, or `baseline` | nothing. This is the important one, see below |
| `pages` | expected page count, `0` means do not assert | nothing |

`expect` keeps its current vocabulary: `paint`, `struct`, or `refuse:<JobError.Msg>`.

The feature taxonomy itself is large, around 300 IDs derived from the ISO 32000-2:2020 clause structure and from the labels real corpora already use. Grouping it by clause chapter:

| Prefix | Area | Examples that matter to Spectre |
| --- | --- | --- |
| `A` | File syntax and structure | classic xref, xref stream, hybrid, object streams, incremental update, linearization |
| `B` | Filters | Flate, LZW, ASCII85, ASCIIHex, RunLength, CCITT, DCT, JBIG2, JPX, predictors, unknown filter |
| `C` | Encryption and signatures | RC4, AES-128, AES-256, crypt filters, PKCS#7, DocMDP |
| `D` | Document structure | catalog, page tree, resource inheritance, functions |
| `E` | Graphics state and paths | CTM, line cap and join, dash pattern, clipping, degenerate paths |
| `F` | Colour | all ten colour space families, rendering intents, shadings types 1 to 7, patterns, separations |
| `G` | Transparency | blend modes, groups, soft masks |
| `H` | Images and XObjects | decode arrays, image masks, colour-key masks, inline images, Form and Group XObjects |
| `I` | Optional content | OCG, OCMD, configuration dictionaries |
| `J` | Text and fonts | Type 1, TrueType, Type 0 and CID, Type 3, standard 14, subsets, ToUnicode, vertical writing |
| `K` | Interactive features | annotations, actions, AcroForm, XFA |
| `L` | Multimedia | sound, movie, 3D |
| `M` | Interchange and tagging | metadata streams, structure tree, role maps, artifacts, table and list structure, prepress |
| `N` | XFA | Annex K |

`J` is where Spectre's known gaps live: `J05` Type 1, `J06` standard 14, `J08` TrueType, `J09` Type 3, `J12` composite, `J13` CID, `J20` embedded programs.

## Sources

Counts are verified by tree enumeration unless marked otherwise. Tier is `committed` for bytes that may live in this repository, `fetch` for bytes that may not.

### Committable, small, feature-labeled

| Source | Files | Licence | What it gives |
| --- | --- | --- | --- |
| `pdf-association/pdf-differences` @ `907fe96e52b73e491489eee545c47b119bf9989b` | 37 | CC BY 4.0 for the PDFs | One directory per interoperability topic, each citing its ISO clause. Best label density available. |
| `openpreserve/format-corpus` `pdf-handbuilt-test-corpus` | 89 | CC0 (README) | 500 to 640 bytes each, one well-formedness violation each, 88 cases. `T03_*` is 12 cross-reference tests, `T04_*` is 18 trailer and EOF tests. |
| `openpreserve/format-corpus` `jhove-errors` + `govdocs1-error-pdfs` | 153 | CC0 README, GovDocs terms on the second | Real-world files that defeated a preservation tool. |
| `openpreserve/format-corpus` `pdfCabinetOfHorrors` | 24 | CC0 README | Encryption, embedded video, JS, font embedding variants, one-byte corruption. |
| `pdf-association/techniques-for-accessible-pdf` @ `3e31095d8aeea941eccbbb766a23369374c2e9a6` | 82 | CC BY 4.0 | One directory per tagging technique with matched correct and incorrect variants. |
| `pdf-association/pdf20examples` @ `c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa` | 7 | CC BY-SA 4.0 | The only public corpus isolating PDF 2.0-only constructs. |
| `OpenPrinting/cups` `examples/` at or after `f1ac9f5889e6` | 6 | Apache-2.0, self-describing header | `testfile.ps` is the single best standard-14 PostScript file in existence. The four Scribus files carry 56 to 66 `setcachedevice` calls and `VM`. |
| `qpdf/qpdf` `qpdf/qtest/qpdf` | 628 | Apache-2.0 | The only expected-outcome oracle in existence. See below. |
| `pdfminer/pdfminer.six` `samples/` | 56+ | MIT | Cleanest licence of any PDF corpus. Filenames encode the malformation class. |

### Committable at volume, or fetch-only at volume

| Source | Volume | Pin quality | Notes |
| --- | --- | --- | --- |
| CourtListener S3 `harvard_pdf/` | 9,000,001 verified floor, about 9.3M estimated | S3 key plus SHA-256, objects immutable in practice | PDF 1.3, no fonts, CCITT G4 page images. Public bucket, no signature, no auth. The best large image-only corpus. |
| GovInfo Federal Register | 1,019,844 for 1994 to 2026 | Weak. No version identifier, and GPO re-signs packages. Pin by hash. | GPO certification signatures, AcroForm widgets, `/SMask`, mixed standard 14 and embedded CFF. ~1,800 to 2,300 files per month. |
| SEC EDGAR `ARS` filings | 2,930 in calendar 2025, verified lower bound | Excellent. Accession numbers are permanent. | 14 subset TrueType programs, Type0 Identity-H with CIDFontType2, symbol fonts through `/CIDToGIDMap /Identity`, `/ToUnicode` throughout. Requires a declared contact User-Agent or sec.gov returns 403. Limit 10 requests per second. |
| EUR-Lex CELEX | Not verified | ELI URIs | The best tagged PDFs found: `/StructTreeRoot`, Type0 Identity-H, CIDFontType2, linearized. 24 languages per act. `Crawl-delay: 10` makes bulk collection impractical. |
| IRS forms | Not verified | Poor. `fw4.pdf` is silently replaced each tax year. | The only source found with xref streams and object streams. 45 `/ObjStm`, 3 `/XRef`, tagged, AcroForm, CID-keyed CFF. Vendor the bytes. |
| `labs.pdfa.org/stressful-corpus/` | 32,679 PDFs, 31 GB, as published | Six `.tgz` batches, each with a `.sha512` sidecar, batched by source project | This is where the SafeDocs issue-tracker corpus moved after `corpora.tika.apache.org` went dead in January 2025. Versioned by crawl date; older crawls are never mutated. |
| `veraPDF/veraPDF-corpus` branch `staging` @ `bb75f4f0073d9350dfd058c0162a367e6fadf25e` | 2,906 | SHA pins bytes | Three-level ISO clause tree. Filenames carry clause, test number, and expected verdict. PDF/A-4 is 485 files, PDF/UA-2 is 138. |
| `veraPDF/veraPDF-regression-tests` @ `8aa0d444dc2b798dc7e8fdd4f59738e982689281` | 597 | SHA pins bytes | CC0 1.0, the cleanest licence of any PDF corpus. |
| `mozilla/pdf.js` `test/pdfs/` @ `d52fdf411a6e4d338180687456e0df019e28475e` | 984 tracked, 982 under `test/pdfs` | SHA pins bytes | Best colour space, font, and out-of-bounds soft mask coverage anywhere. No expected-error data; the mapping is inline in `api_spec.js` as exception classes. |
| `chromium/pdfium` `testing/resources` | 301 PDFs, 508 `.in` templates | SHA pins bytes | BSD-3. The `.in` files are templates expanded by `testing/tools/fixup_pdf_template.py`, so the corpus is mechanically regenerable. Note the `.in` files encode correct offsets, so they cannot express a lying xref. |
| `CC-MAIN-2021-31-PDF-UNTRUNCATED` | about 8,000,000 | Digest-addressed | The largest unbiased web sample. The right substrate for measuring how often each font type actually appears. No published study answers that question; you have to measure it. |
| Internet Archive, Wikimedia Commons IA scans | 46M and 1,877,704 | IA: item identifier plus per-file MD5. Commons: file revision plus SHA-1 | Both are the same bytes by two doors, and both are JBIG2 and JPX. Commons median is 163 MB per file, p90 679 MB, max 2.07 GB. |

### Sources that do not work

- `corpora.tika.apache.org`, the original SafeDocs host, is NXDOMAIN. Any plan that says "sweep the Tika corpus" is dead. Use `labs.pdfa.org` instead.
- `cgit.ghostscript.com`, the Artifex test corpus, is behind an anti-bot wall.
- `data.uspto.gov` serves an AWS WAF challenge. `bulkdata.uspto.gov` and `patft.uspto.gov` fail DNS.
- `legislation.gov.uk` and EUR-Lex metadata pages return HTTP 202 bot interstitials.
- The CourtListener REST API is membership-gated. The S3 bucket is not, but the posture is worth a decision.
- HathiTrust has no PDFs. Its Data API v2 serves EBM only, behind a per-item flag.
- Project Gutenberg is effectively PDF-free. Of 60 sampled ebook IDs, 1 had a PDF, and it was LaTeX output with no images.
- Wikimedia Commons has zero PostScript files. EPS is converted to SVG on upload.
- The PDF Association's own corpus index contains no publisher-remediated tagged-PDF corpus, no reading-order ground truth, no XFA set, and no signature corpus. Those are real gaps, not search failures.

## Licence traps

These are the ones that would produce a wrong `license` column or a legal problem.

**`pdf-differences` LICENSE is Apache-2.0 and the PDFs are CC BY 4.0.** The root LICENSE is plain Apache 2.0. The README says "PDF files are copyright by the PDF Association and distributed under a Creative Common Attribution 4.0 International license. Any source code in this repository is licensed under Apache License." There is no per-directory LICENSE inside any of the 19 fixture directories. A manifest built from the root LICENSE mislabels every one of the 37 PDFs. The README also says implementations at fault "will not be named or identified", so a fixture must not be annotated with the name of a product you believe renders it wrong.

**URW base 35 is not redistributable.** `ArtifexSoftware/urw-base35-fonts` is AGPL-3.0 with an exception that grants permission "to include these font programs in a Postscript or PDF file that consists of a document". That is an embedding grant, not a redistribution grant. The CTAN `urw` package is GPL-2 with no exception at all, which is worse. This is the source every project reaches for first and it is the wrong one.

**Adobe Core 14 is fetch-only.** The grant permits embedding, not redistribution. Ghostscript's own LICENSE carries the identical carve-out. `afdl.adobe.com` no longer resolves and `adobe-type-tools/afdl` is 404.

**`qpdf/performance-test-files` has no licence.** The only public qpdf PDF repository contains the first 100 pages of the ISO PDF specification with no licence file. No licence means no permission.

**`openpreserve/format-corpus` has no LICENSE file.** The CC0 claim is a README assertion. CC0 is a public-domain dedication so the claim is probably right, but it is weaker provenance than veraPDF's CC BY grant, and `govdocs1-error-pdfs` says "otherwise stated" in its own README.

**Chromium `third_party/test_fonts` declares eight licences in one directory**, including CC0 dedications, a bespoke `test_fonts` grant, and AhemFont. Never commit a `third_party` subtree as a unit.

**`poppler/test` is GPL-2.0** and the main repo's README warns submitters against publishing PDFs whose copyright does not allow redistribution. Do not mine tracker attachments.

**Internet Archive grants no redistribution licence.** Its terms restrict access to scholarship and research, require self-certified fair use, and state that content may be removed without notice. Project Gutenberg is worse for this purpose: the licence requires distributing verbatim copies while stripping all Project Gutenberg references, and you cannot strip a page from a PDF without changing the bytes you are trying to test.

**`govdocs1-error-pdfs` and `0ca/corpus_pdfs` must not be committed.** The first carries GovDocs terms rather than CC0. The second has no LICENSE file at all.

Two structural rules follow, and both should be encoded in the manifest schema rather than remembered:

Split `embedding_grant` from `redistribution_grant`. URW and the Core 14 are exactly the case where the first is yes and the second is no, and a single boolean cannot represent it. A test PDF that embeds a Core 14 font is committable; the extracted `.pfb` is not.

Record the URL you read the licence text at, not a licence name. Four sources have a repo-level licence that misdescribes the fixture bytes.

## Expected outcomes

The hard problem is that for 5,000 real files nobody knows in advance which should pass.

The published evidence is one-sided. Six parsers disagree on 502 of 1,572 curated files, and 43.5% of a 6,281-file real-world control, on page count, JavaScript visibility, encryption status, or AcroForm presence. That is not a parser bug, it is an underspecified specification, and it reproduces. So an expected outcome must not come from Spectre.

`basis` is the rule:

- `spec` means a clause of ISO 32000-1, ISO 32000-2, or the PostScript Language Reference fixes the outcome, and the manifest names the clause. Gating.
- `gs` means `documentation/ghostscript-baseline.md` or a measured Ghostscript run on the same file fixes the outcome. Gating. These are the most valuable rows available, because the project already has a baseline discipline and 5,000 real files give 5,000 real comparisons.
- `baseline` means we ran it and recorded what happened. Not gating. Reported, and a debt.

The rule that keeps this honest: the `baseline` count must go down, not up. A corpus refresh that adds 800 rows of which 780 are `baseline` made the suite worse, and the report says so. Every `baseline` row carries an obligation to be re-examined within a release or two and either promoted to `spec` or `gs` with a written reason, or deleted. When a `baseline` row turns out to be a real bug, file it under the `[~]` convention in `plans/v0.0.1/10-deferred.md`, keep the row as `baseline` so the delta stays visible, and do not promote it to a passing expectation.

Two portable formats exist and neither is used by the projects that invented them. KillerPDF publishes a fixed five-column CSV per source, `File,QpdfExitCode,SHA256,Bytes,Source`, produced by running `qpdf --check` on every file. The design property is stated in their README: "qpdf results showing which files are already damaged before KillerPDF touches them." The expected outcome comes from an implementation with no stake in the result. pqpdf publishes JSONL with resource-budget fields worth stealing, notably `grant_mb`, `incomplete` as a recorded status rather than an error, and `last_stage` which localises a hang to a specific analysis pass.

qpdf's own convention is the one to copy. `qpdf/qtest/qpdf` pairs each malformed input with a sibling `.out` file holding the expected stderr, and `error-condition.test` carries a 40-entry list. Two expectations per file: strict mode and recovery mode, because "must fail" and "must fail and then recover to a stated degree" are different contracts. Both are extractable with two regexes over two Perl files. Note the `.out` files embed the input filename, so normalise it before diffing or every assertion breaks on rename.

qpdf's fuzz corpus has a better idea still. `fuzz/qtest/fuzz.test` keys every input by SHA-1 and asserts the count and that each run exits clean. The oracle is the hash, with no message brittleness.

## What the first run will look like

Two published accounts, both current, both from commercial products with a marketing incentive, so read the numbers and discount the framing.

A forensic scanner against about 7,850 real PDFs: 1 decompression bomb out of 400 live samples, 216 stalls out of 6,281 (3.4%), and parser disagreement on 32% of curated and 43.5% of real files.

A PDF tool against 47,024 files, first baseline: **zero crashes and zero timeouts**, including 80 deliberately damaged inputs. The failure mass was skips, not crashes: 8.4% skipped and 1.7% save-failed on the regression set, 14.5% and 5.5% on the stress set.

Three things to plan around.

**Conformance files fail more than real files.** Only 58.1% of the 649 standards-and-colour files saved, against 90% on real regression files. Building a pass-rate denominator out of veraPDF's synthetic corpus publishes a much worse and much less meaningful number than reality. Keep the denominators separate per collection, as KillerPDF does with one summary row each.

**Expect mass false positives in a specific, nameable set.** Ten real-world structures each fired on a large fraction of real PDFs before being corrected. Two are directly ours: linearized `/Size` non-monotonic by offset fired on **67% of real PDFs**, and the `setpagedevice` operator is "standard PostScript page-setup op in every Distiller/Ghostscript PDF", flagged as an RCE pass-through. Both are false alarms with named fixes. Budget for the triage before the first run, not after.

**The corpus converges and that is fine.** KillerPDF gained 1,485 successful saves between two releases and then reported identical totals for three consecutive releases after that. A 47k-file corpus reaches a fixed set of files that no amount of work will move. Decide in advance that those are recorded, not chased.

Two things nobody has published, so budget for discovering them. There is no account of a Go PDF parser's first run at volume, and no triage-time-in-hours figure. The closest reference is a 14-bug annotated historic corpus that took about six months to construct, verified only at search-snippet level.

## veraPDF as a CI gate

Measured on the local binary, `./verapdf/verapdf`, version 1.30.2, built 2026-06-03. Not read from tutorials.

`--flavour` takes `[0, 1a, 1b, 2a, 2b, 2u, 3a, 3b, 3u, 4, 4f, 4e, ua1, ua2, wt1r, wt1a]`. `--format` takes `[raw, xml, text, html, json]`, default `xml`. `mrr` is deprecated and is not a valid value in 1.30.2, despite appearing in most blog snippets.

```sh
./verapdf/verapdf --flavour 4 --format json build/pdfs
./verapdf/verapdf --flavour ua2 --format json build/pdfs
```

Always pass an explicit flavour. `-f 0` is the default and means detect from the file's own XMP, with `--defaultflavour` falling back to `1b`. One file can then yield several profile results, because a PDF/A-4 file that also declares PDF/UA-2 gets judged against both.

Exit codes, measured: 0 compliant, 1 non-compliant, 2 bad argument or bad profile path, 4 no files found, 7 unparseable.

Three traps.

**The batch exit code is not a bitmask.** A batch of one non-compliant file plus one unparseable file exits 7, not 1, and the JSON still says `nonCompliantPdfaCount=1`. A gate that runs a whole corpus in one invocation and checks `== 1` reports a tool error while a genuine non-compliance sits unreported. Parse the JSON.

**Two silent-success footguns.** `verapdf` with no file arguments prints usage and exits 0. `verapdf --nonsense file.pdf` treats `--nonsense` as a filename, logs a SEVERE line to stderr, validates the real file anyway, and exits 0. A gate that only asks "was it zero" can pass a build that validated nothing.

**There is no third "not applicable" outcome.** Only pass and fail, plus a structurally distinct `taskException` for "could not check". The `-undefined-` marker in veraPDF-corpus is a corpus-side convention for a 10-file `Undefined/` directory. Running 1.30.2 over those 10 gives 7 pass and 3 fail, so a gate that scores all 2,906 files from filename-derived expectations reports 3 spurious failures unless `Undefined/` is excluded.

The JSON shape for 1.30.2, from execution. `jobs` is a list, not a dict. `validationResult` is an array. The field is `compliant`, not `isCompliant`. The often-copied `report['jobs']['validationResult']['compliant']` is wrong.

- verdict: `report.jobs[i].validationResult[j].compliant`, a boolean
- tool failure: `report.jobs[i]` has `taskException` and **no `validationResult` key at all**
- rollup: `report.batchSummary.validationSummary.{compliantPdfaCount, nonCompliantPdfaCount, failedJobCount, totalJobCount, successfulJobCount}`

Scale. JVM startup is 0.63 to 0.71 s and marginal cost is about 0.07 s per file. Ten files in one invocation took 1.38 s; ten invocations took 9.62 s. A 5,000-file corpus is roughly 54 minutes of pure JVM startup if invoked per file, against single-digit minutes batched. veraPDF's own docs corroborate: 1,526 files in 1 m 40 s in one `--recurse` run. So: one invocation per flavour, over a whole directory, with `--processes` set to the runner's core count, parsing the single JSON document. Never shell out per file. There is no project or manifest mode; inputs are files, directories, or ZIPs, and ZIPs are scanned recursively.

Distribution. There is no `veraPDF/veraPDF` repository and `veraPDF-apps` has no GitHub releases. Binaries come from `software.verapdf.org/releases/`, GPG-signed, key `78B17FE7`. Licence is GPLv3+ or MPLv2+ per the source headers; the grant covering the prebuilt installer zip is unverified. A container image exists and is pinnable: `verapdf/cli`, tags include `v1.30.2`, and Docker Hub's web page showing only `latest` is misleading. 1.30.2 fixes parsing vulnerabilities in Rich Text values, XFA forms, and PostScript syntax in `/ToUnicode` CMaps and PS Type1 fonts, so run the gate on untrusted corpus files in a sandbox.

**This repository has the bug already.** `scripts/` and the Makefile `pdfa-check` target use `--flavour 4 $$base || status=1`, which conflates exit 1 (a real non-compliance) with 2, 4 and 7 (the gate itself is broken), and cannot distinguish "validated nothing" from "everything passed". `pdfua2-check` ends on a bare veraPDF invocation and inherits the same problem. Row 5.1 fixes both.

## Fuzzing

Go 1.26.4 has no fuzzing today. `testing.F` arrived in 1.18 and nothing relevant changed in 1.24, 1.25 or 1.26, so the target of 1.26.4 needs nothing new.

Supported argument types are `[]byte`, `string`, `bool`, `byte`, `rune`, the float and int families. `[]byte` is right for both PDF and PostScript. The target must not return a value, and the `tests` vet analyzer fails the build if it does, so a parser that errors is not a failure: return early on error and assert invariants on success.

Verified API facts that will otherwise cost a day:

- Seeds live in `testdata/fuzz/<FuzzName>/` and in `f.Add` calls. The directory name is the exact function name.
- A seed-only CI gate is `go test -run 'Fuzz' ./...`. It is not `-run '^$'`: that combination prints "no tests to run" and silently gates nothing.
- Crash artifacts are named by the **first 16 hex characters** of the SHA-256, not 64. The official docs show 64-character examples and are stale. Trust the `To re-run:` line the tool prints.
- Raw binaries cannot be dropped into `testdata/fuzz/`. They need the `go test fuzz v1` encoding, or `golang.org/x/tools/cmd/file2fuzz`. A stray `README.md` in that directory is parsed as a corpus entry and fails.
- A read-only `testdata/fuzz/` loses the crasher entirely. The `pkg.go.dev` claim that it falls back to the build cache is not what 1.26.4 does.
- A hang and an OOM report identically, and under a container memory limit an OOM can produce a non-reproducible green nightly. Set `GOMEMLIMIT` in the target and cap the runner's memory.
- `-fuzz` refuses to match more than one target, and a **no-match is a warning that exits 0**, so a typo in a nightly job gives a green build. Guard with `go test -list`.
- `-fuzz` rejects `-coverprofile` and any profile flag. The fuzz cache is unbounded; `go clean -fuzzcache` at the end of nightly jobs.
- Go has no dictionary support. `-dict` does not exist. `google/fuzzing/dictionaries/pdf.dict` (1,466 tokens) and `ps.dict` (701 tokens, and referenced by nothing in OSS-Fuzz, so free value) have to become `f.Add` seeds or generated skeletons. qpdf's `pdf.dict` is byte-identical to the google one.

Seed volume. Moonshine distilled 42,056 PDFs to 664 and 5,666 TTFs to 27 while preserving coverage. Go's own tree has 39 `Fuzz*` functions and 25 files under `testdata/fuzz/`, spread across four directories. The team that wrote the engine does not commit large corpora. So: about 150 to 250 named seeds per target, pruned on coverage with `GODEBUG=fuzzdebug=1`, and the generated corpus stays in `$GOCACHE`.

## PostScript

The state here is worse than PDF and worth stating plainly, because the corpus is not the only gap.

Adding up every source verified, with no double counting: about 90 `.ps` and 134 `.eps`, roughly 225 files, of which **28 are the only real interpreter test corpus in existence**. Only about 8 are committable under a permissive licence. Most `.eps` are vector figures that test paths and clipping rather than the interpreter. There is no public corpus of dvips output, because dvips output is a build artifact and nobody archives it. The largest permissively licensed file is `cups/examples/testfile.ps` at 17 KB, and the four CUPS Scribus files carry the best `setcachedevice` and `VM` coverage in any permissive source.

The standard-14 blind spot is worse. Across every corpus obtained, the union of font names referenced by `findfont`, `%%DocumentFonts`, `%%DocumentNeededResources` and `/FontName` is **four faces out of fourteen**: Helvetica, Helvetica-Bold, Times-Italic, Times-Roman. The best single file references three.

That is fixable, cheaply and legally, and the recipe was run.

`groff` is GPL-3.0, but its PostScript output is not a derivative work of groff. The output is a data file of glyph placements and advances computed from AFM metrics, with no groff source text copied into it. The same reasoning that lets you commit LaTeX output of an LPPL tool applies: the tool is GPL, the output is yours.

groff's `font/devps/` maps its short names onto the PostScript standard 35 by name, and calls `findfont` without embedding, which is exactly Spectre's blind spot:

```
CR -> Courier          HR  -> Helvetica          TR  -> Times-Roman
CB -> Courier-Bold     HB  -> Helvetica-Bold     TB  -> Times-Bold
CI -> Courier-Oblique  HI  -> Helvetica-Oblique  TI  -> Times-Italic
CBI-> Courier-BoldOblique  HBI -> Helvetica-BoldOblique  TBI -> Times-BoldItalic
S  -> Symbol            ZD  -> ZapfDingbats
```

Determinism, measured. Default groff embeds wall-clock time, so two runs a second apart differ only on `%%CreationDate`. With `SOURCE_DATE_EPOCH=0` three runs a second apart were byte-identical, and the header becomes `%%CreationDate: Thu Jan  1 00:00:00 1970`. Without this the corpus cannot be hash-pinned.

Coverage, measured. Selecting the reachable faces produces 13 of 14 named and not embedded, in 7,767 bytes, and the body switches fonts with `/F1 10/Courier@0 SF` and uses kerning, so it exercises `findfont`, `makefont`, `selectfont`, `show`, `stringwidth`, kerning and per-font advance measurement. ZapfDingbats is the fourteenth and did not work: `font/devps/text.enc` covers "the standard PS text fonts (excluding special fonts)" and has no `ZD` entry, so selection fell back to Symbol. Thirteen is what was run.

To paint any of it, Liberation (OFL 1.1, metric-compatible with Helvetica, Times and Courier) and TeX Gyre (GUST/LPPL, the remaining standard 35) are both committable. That is the same substitution the reference implementation uses, and it is the same substitution the spec permits. ISO 32000-2 clause 9.6.2.2 requires "these fonts, or their font metrics and suitable substitution fonts", and the wording was accepted into PDF 2.0.

This closes the gate on the standard-14 row in `plans/v0.0.1/10-deferred.md` 10.1, and supplies the CFF outline data that the bare-CFF row in the same section also needs. TeX Gyre is 33 CFF OTFs at 2.8 MB, and it is a real-world regression for the bare CFF reader rather than a synthetic one.

One gotcha when building the fixture. A roff control line takes at most one argument, so `.\f[CR]Sample ABC` fails, because troff reads `Sample` as a macro name. Start the line with the font escape and no leading dot.

## What this slice landed

The committed half of the plan is in the tree. The proof commands and their
outcomes are on the rows.

| Proof | Outcome |
| --- | --- |
| `make lint` | clean, including `size-check` |
| `make test` | all 14 packages ok |
| `make test` with the live tier removed | all packages ok, 10 rows skipped |
| `make validation-fetch` | 177 rows fetched and verified, 14.1 MiB |
| `make validation-fetch` a second time | 177 rows, 0.08 s, no network |
| `make validation-verify` | 177 rows present and verified |
| `make validation-report` | 199 rows, 199 pass, 0 fail, 0 skip |
| `bash scripts/check-traceability.sh` | 284 cases, 254 live, 30 proof, 0 pending, 0 failing |
| `make test` with the live tier removed | all packages ok, every fetched row skipped |
| `make validation-report` after the conformance tier | 3,123 rows, 3,123 pass, 0 fail, 0 skip |

## What the corpus found

Four things, all recorded rather than fixed. Each one is a row.

**A licence in the manifest was wrong, and nothing caught it.** This is the one
that mattered most, because it is a defect in the project's own gate rather than
in the corpus. The 89 hand-built cases were committed with `CC0-1.0` recorded,
taken from a repository README with no `LICENSE` file behind it. The dataset they
come from states CC BY-SA 4.0 and names its rights holder. The manifest is
committed, so the wrong string was in the tree. It passed
`TestValidationLicenses` because that test reads the string, not the source, and
a gate that reads a string cannot tell a true string from a convenient one. The
files are fetched now. The generalisable lesson is in row 1.5, which is still
open: a `license_url` column recording where the text was actually read is what
would have caught this.

**A row was wrong before the reader was.** `handbuilt/hello_world.pdf` was
committed with `expect: struct`, on the reasoning that the name and the contents
both suggest a valid minimal PDF. It is damaged: its cross-reference table gives
object 5 an offset of 402, and byte 402 is the middle of the previous object's
`endstream`. Ghostscript 9.55.0 reports the same damage. The row is now
`survive`, and its feature column says the damage is upstream's. This is the
first time a corpus row was wrong rather than the reader, and it is the
strongest argument for classifying before fetching.

**The reader stops at the first structural oddity.** All 89 hand-built rows
pass the survive contract, so nothing panics, hangs, or faults. The refusals
are not spread across the parser: 81 report `syntaxerror in obj`, 4 report
`syntaxerror in pdf`, 3 report `syntaxerror in xref`, and 1 reports
`syntaxerror in endobj`. The 89 cases are measuring one code path rather than
89, because the scanner bails rather than distinguishing a damaged catalog from
a damaged page tree from a damaged cross-reference table. Ghostscript repairs
and continues on the same files. Whether Spectre stays strict is a policy
decision `documentation/language.md` already makes, so closing the gap means
choosing a recovery policy, not patching a parser. That is now a deferred row, in 10.7.

**The generated PostScript sweep found a missing operator.**
`postscript/standard14-sweep.ps` is `groff -Tps` output naming 13 standard 14
faces, and it refuses with `undefined in matrix`. The `matrix` operator is
PostScript Level 1 clause 7.1.2.1 and it is outside the documented subset in
`documentation/language.md`, which lists `translate`, `scale`, `rotate`,
`concat`, `setmatrix`, `currentmatrix`, and `dtransform` but not `matrix`. The
sweep also uses `setpacking`, `ashow`, `widthshow`, and `awidthshow`. The row
is `refuse:undefined in matrix` with `basis=spec`, because the language
contract says an operator outside the subset returns a named error and this is
the measured one. Adding the operators is a deferred row, in 10.7.

## Rows

### Phase 1: label axes and the report

- [x] 1.1 Add `area`, `probe`, `basis`, `pages` to `manifest.tsv` and to `ParseManifest`. Every existing row gets an `area` from its folder, a `probe` matching its old runner, a `basis`, and `pages` 0. Existing `expect` values are unchanged. `go test ./internal/validation` passes on 199 rows.
- [x] 1.2 Dispatch `corpusRunRow` on `row.Probe`. `corpusProbeArgs` builds the argv for all six probes, so the runner reads its job from a column.
- [x] 1.3 Collapse the hardcoded folder lists in `TestValidationCorpus*` into one function selecting on `row.Area`. `corpusRowsByArea` now backs pdfa, tagged, structural, images, malformed, and prepress. The last path-based selector, `TestValidationCorpusImages`, was hiding `external/artifex/jbig2/t89-halftone.pdf`, which the report caught as a row no test touched.
- [x] 1.4 Add `make validation-report`, running `go test -json -shuffle=on` and joining on `area` and `basis`, writing to `profiles/`. It counts distinct rows per group and reports the rows a feature test runs twice, so the duplication is visible rather than inflating the rate. `make validation-report` writes `profiles/corpus-report.tsv`: 199 rows, 199 pass, 0 fail.
- [ ] 1.5 Add `embedding_grant` and `redistribution_grant` to the manifest schema, and a `license_url` column. Not done. No row needs a stand-in string for the split: the earlier `CC-BY-4.0-cover` label was used by zero rows and has been removed from the schema, so the five PDF Association `pdf-differences` rows carry `CC-BY-4.0`, the grant their README states, and `sampledata/validation/README.md` records why the repository's Apache 2.0 `LICENSE` is the wrong source for it. What is still missing is the two columns and a `license_url`, plus a per-file audit; one directory's README is currently the only evidence.

### Phase 2: the committed additions

- [x] 2.2a `veraPDF/veraPDF-corpus` `staging` at `bb75f4f`, 2,691 of 2,906 files, 158,276,020 bytes, CC-BY-4.0, in the fetched tier under `external/verapdf/`. `Isartor test files/` (205) is excluded by the token list and `Undefined/` (10) is excluded because veraPDF 1.30.2 fails 3 of its 10, so a filename-derived expectation would report three spurious failures. Every expected outcome is a measured Ghostscript 9.55.0 run on the same file, not a recording of our own reader: Ghostscript opened 2,691 of 2,691, Spectre opened 2,682, and the 9 disagreements are documented refusals recorded at `basis=spec`. This lifted `basis=gs` from 7 rows to 2,689.
- [x] 2.2b `ArtifexSoftware/tests` at `f7d5087`, 233 of 236 files, 169,647,889 bytes, AGPL-3.0, in the fetched tier under `external/artifex/`. Three PostScript files are excluded for third-party content that is not the repository's to grant: `ps/image-qa.ps` carries a proprietary Artifex licence, `ps/gray-to-cmyk-with-transfer.ps` embeds Corel copyright three times, and `ps/puzzleware.ps` says "free puzzleware", which nobody defines. This required widening the licence gate, which refused AGPL outright, to admit it in the external tier only. It carries 79 PDF/X prepress files, 29 JBIG2, 25 from the PDF Association safedocs subset, 21 PostScript programs, 13 JPEG2000, 6 encrypted, and 4 CMap, so it reaches four deferred gates at once. It also found the measured operator list now in `plans/v0.0.1/10-deferred.md` 10.6.
- [x] 2.1 `openpreserve/format-corpus` `pdf-handbuilt-test-corpus`, 89 files, 372 KB, pinned at `366f068cec399d0cdfd61fa473de3ab6dc858098`. `TestValidationCorpusHandbuilt` runs all 89 through the survive contract. **Fetched, not committed.** The licence is CC BY-SA 4.0, so the files live in `sampledata/validation/external/handbuilt/`. They were first committed with the licence recorded as `CC0-1.0`, which was wrong, and the error was invisible because a wrong licence string satisfies a gate that reads the string rather than the source. See row 7.3.
- [ ] 2.2c `pdf-association/pdf-differences`, 37 files, CC BY 4.0, one row per directory. Five of its files are already in the manifest; the other 32 are not added.
- [ ] 2.3 `qpdf/qpdf` `qpdf/qtest/qpdf` malformed inputs with their sibling `.out` expected-stderr files.
- [ ] 2.4 `openpreserve/format-corpus` `pdfCabinetOfHorrors`, 23 files, committed as live-tier rows and fetched. `jhove-errors` (99) and `govdocs1-error-pdfs` (54) are not added: the first was not staged, and the second is held back on its licence, which quotes GovDocs rather than the CC0 the parent folder claims.
- [ ] 2.6 `pdf-association/techniques-for-accessible-pdf`, 82 files. One is already in the manifest.
- [ ] 2.7 `mozilla/pdf.js` malformed subset by name.
- [ ] 2.8 `pdfminer/pdfminer.six` `samples/`, 56+ files, MIT.
- [ ] 2.9 `veraPDF/veraPDF-regression-tests`, 597 files, CC0, 248.6 MiB. The cleanest licence available and the next cheapest addition.

### Phase 3: the live tier

- [ ] 3.1 **Decision row.** JBIG2 and JPEG2000, or a two-number scanned-document claim. Unchanged and still open. The live tier now holds 23 real-world files, none of which is a scan, so the decision is not yet forced. It will be forced the moment a scan corpus is added.
- [x] 3.2 Extend `gen.go` with `-fetch-external`, `-verify-only`, `-dry-run`, `-area`, `-workers`, and `-cache`, plus `SPECTREPS_VALIDATION_CACHE`. Files are streamed to a content-addressed cache, verified, and hardlinked into the tree, so a killed run never leaves a file that looks complete and a second run costs no network. The previous implementation buffered every file in RAM before touching the tree, which is right for 2 rows and wrong for 5,000. `make validation-fetch` then `make validation-verify` both report 177 rows verified.
- [x] 3.3 Add `make validation-fetch` and `make validation-verify`, keeping corpus work out of `make test` and `make lint` the way `validation-run` already does. `make test` is green with the tier absent.
- [x] 3.4 Per-host politeness as a table in `gen.go`: `raw.githubusercontent.com` 8 concurrent at 20/s, `www.sec.gov` 1 concurrent at 8/s, `archive.org` 4 concurrent with 1 s spacing, default 4 at 5/s. Every request carries a descriptive User-Agent. Retries cover 408, 429, 5xx, and transport errors with capped full-jitter backoff; a 404 is not retried and a digest mismatch is fatal on the first try.
- [x] 3.5 Source the image-only scale set. The source is chosen and pinned, CourtListener S3 `harvard_pdf/`, with a 9,000,001 verified object floor. No files are fetched yet, because the JBIG2 question in row 3.1 decides whether a scan corpus is worth measuring.
- [ ] 3.6 SEC EDGAR `ARS` and GovInfo Federal Register rows.
- [ ] 3.7 Vendor the IRS forms. They remain the only source found with xref streams and object streams.
- [x] 3.8 `SPECTREPS_EXTERNAL=1` and `-shuffle=on` reach the corpus from the outside, and the report runs the corpus tests shuffled so a suite that only passes in one order is visible.
- [x] 3.9 The classifier triage budget is written down before any scale run: the linearized `/Size` class at 67% of real PDFs, and `setpagedevice` on every Distiller and Ghostscript PDF. Both are named false positives with known fixes. The sweep confirmed the second one directly, since groff output reaches `setpagedevice`.
Two rows this phase produced are deferred, and the deferred ledger at `plans/v0.0.1/10-deferred.md` 10.7 holds the single open copy of each: a parser recovery policy, and the five operators real `groff` output needs. The phase file does not carry a second copy.

### Phase 4: fonts and PostScript

- [ ] 4.1 Commit the 33 TeX Gyre CFF OTFs, GUST/LPPL.
- [ ] 4.2 Commit Noto Sans Symbols and Noto Sans Symbols2 for Symbol and ZapfDingbats.
- [ ] 4.3 Wire the 14 base names to the substitution table as one data structure.
- [ ] 4.4 Take Adobe's Core 14 AFMs from `tecnickcom/tc-font-core14-afms`.
- [x] 4.5 `postscript/cups-testfile.ps` and `cups-smiley.ps` were already committed at pin `e72b702`, and the three larger Scribus files are now live-tier rows at the same pin. All six are `Apache-2.0` with a self-describing header.
- [x] 4.6 The four CUPS Scribus files are live-tier rows. They are `baseline`, so their `survive` verdict is recorded rather than derived, and they are the next candidates to promote to `spec`.
- [x] 4.7 Generate the standard-14 sweep with `SOURCE_DATE_EPOCH=0 groff -Tps`, commit it as `postscript/standard14-sweep.ps`, and record the recipe. Three runs a second apart were byte-identical and the header reads `%%CreationDate: Thu Jan  1 00:00:00 1970`. 13 of 14 faces are named and not embedded, in 7,767 bytes. groff's GPL does not attach to its output, so the row is `repo-authored`.
- [ ] 4.8 Fix the `ZD` path so ZapfDingbats is reachable. Until then 13 of 14 are covered and the row says 13.
- [ ] 4.9 Fetch the remainder of the PostScript tier: `ArtifexSoftware/tests`, TeX Live `tlcontrib`, and Burkardt's EPS.

### Phase 5: the gate

- [ ] 5.1 Fix `pdfa-check` and `pdfua2-check`, which conflate exit 1 with 2, 4, and 7. Still a live bug in this repository.
- [ ] 5.2 One veraPDF invocation per flavour over a whole directory.
- [ ] 5.3 Pin the validator to `verapdf/cli:v1.30.2` or the GPG-verified installer.
- [ ] 5.4 Add `make pdfa-gate`, excluding the 10 `Undefined/` files.
- [ ] 5.5 Note in `documentation/development.md` that no published CI recipe exists for veraPDF over a large corpus.

### Phase 6: fuzzing

- [ ] 6.1 `FuzzXrefParse`, seeded with the 89 hand-built files.
- [ ] 6.2 `FuzzFilters` per filter, seeded from the qpdf filter corpora.
- [ ] 6.3 `FuzzType1Charstrings` and `FuzzTrueType`.
- [ ] 6.4 `FuzzPostScriptScan`, seeded with generated skeletons from `ps.dict`. The sweep in row 4.7 is a better PostScript seed than anything downloadable, because it is real producer output.
- [ ] 6.5 Seed gate is `go test -run 'Fuzz' ./...`; nightly is one job per target, guarded by `go test -list` so a typo is not a green build.
- [ ] 6.6 `testdata/fuzz/<Name>/PROVENANCE.md` per target, enforced by a check.

### Phase 7: the three defects found while building this

- [x] 7.1 The fetcher set `Accept-Encoding: gzip` by hand, which stops `net/http` decompressing transparently, so the digest was taken over the gzip stream. Small files came back uncompressed and passed, so the failure appeared only on the rows big enough for the server to compress, which reads exactly like a handful of stale pins. Fixed by not setting the header, and the reason is a comment in `fetchOnce` so it is not reintroduced.
- [x] 7.2 The CUPS rows were first pinned to `master` while the two committed CUPS rows used `e72b702`, and the digest check failed on all five. CUPS has changed those files since `e72b702`. This is the pin working: a wrong URL produces a mismatch rather than a silently different corpus file. All six rows now use `e72b702`.

- [x] 7.3 The hand-built set was committed under a licence string the source does not support. The `openpreserve/format-corpus` repository has no `LICENSE` file and its root README says "All items are CC0 licenced unless otherwise stated". The `pdf-handbuilt-test-corpus` README states provenance, not a licence, and points at the iPres 2017 dataset, whose rights record at <https://doi.org/10.22000/53> reads "This work is licensed under CC BY-SA 4.0" and names Michelle Lindlar as rights holder. The files are share-alike, so they moved to the fetched tier and the manifest records `CC-BY-SA-4.0`. `pdfCabinetOfHorrors` from the same repository is the counter-example: its folder README carries its own CC0 grant, so those rows stay `CC0-1.0`.

## Dependencies

Phase 1 and 2.1 are in the tree. Everything else is unblocked except where a row
says otherwise.

Phase 3 is gated on row 3.1 for any scan corpus, and unblocked for the sources
already pinned. Phases 4 and 5 are independent of 3 and of each other.

Phase 6 needs no new module requirement while the fuzz targets take `[]byte`.
`golang.org/x/tools/cmd/file2fuzz` is a build-time tool, not a dependency.

Row 5.1 is the cheapest row in this file and still fixes a live bug in this
repository's own gate. The two findings this slice produced are product decisions rather
than code, so they went to the deferred ledger instead of staying here.

## Decisions this file does not make

**cgo.** Adding JBIG2 and JPEG2000 decoders almost certainly means cgo, because no pure-Go JBIG2 decoder was found and the existence of a pure-Go JP2 decoder is unverified. `AGENTS.md` says the module does not use cgo. That rule and row 3.1 are in direct conflict and a human has to break the tie.

**Font shape fidelity.** A TeX Gyre substitution matches a Nimbus-based reference on layout and differs on ink. Every test that compares rendered pixels against Ghostscript for a standard-14 face will fail on shape. Decide now whether those tests assert on layout or on ink, because the answer changes which fixtures are worth having.

**Redistribution posture on the S3 bucket.** The CourtListener REST API is membership-gated and the S3 bucket is not. Depending on an unauthenticated bucket while the official API is metered is a posture decision, not a technical one.

**SEC EDGAR and Rule 10b-5.** The SEC states that information on sec.gov "may be copied or further distributed by users of the web site without the SEC's permission". What is unverified is any SEC statement on republishing filings for a non-regulatory purpose. Filings also embed auditor letterheads from Ernst & Young, KPMG, Deloitte and PwC, which the SEC's reuse statement does not address. This needs a human legal call, not a fetch.

## Unverified

Marked so they are not mistaken for findings.

- The existence of a maintained pure-Go JBIG2 or JPEG2000 decoder. Two candidates 404 on the module proxy and a pkg.go.dev search returned an unparseable shell.
- A published account of a Go PDF parser's first run against a large real-world corpus, and any triage-time-in-hours figure for any language.
- veraPDF exit codes 3, 5, 6, 8, 9, 10, 11 and 12. The table is source-derived from veraPDF-library issue #1562, which is an **open** issue requesting that these be documented. Only 0, 1, 2, 4 and 7 were reproduced by execution. Do not build a gate arm on the others without reproducing it.
- Redistribution terms for the prebuilt veraPDF installer zip. The GPLv3+/MPL2+ grant was read in source-tree headers.
- Per-tag Docker digests for `verapdf/cli`. The tag `v1.30.2` exists; its digest was not fetched.
- The meaning of the trailing letter in veraPDF-corpus filenames. The README documents the directory structure and the general pattern but never defines it.
- EUR-Lex volume, and its reuse licence. Every legal-notice URL returned a bot challenge. Commission Decision 2011/833/EU is the commonly cited instrument and was **not** verified.
- CourtListener's terms of use. `robots.txt`, `/about/` and `/terms/` all returned 403. Whether "free of known copyright restrictions" extends to the PDFs or only to the CSVs is unverified.
- The exact `harvard_pdf/` object count. Verified floor 9,000,001, estimate about 9.3M.
- IRS and USPTO form volume.
- TeX Live repository-level licence. GitHub reports none. The LPPL umbrella is well known but was not confirmed from a primary source, and per-package licences inside those trees vary.
- A maintained public XFA sample repository, a public digital-signature test corpus, and any public reading-order ground truth over a PDF structure tree. ReadingBank is the closest and is explicitly non-redistributable with only about 100 redacted example pages public.
- `ben-milanko/dart-pdf`, `ross-spencer/opf-format-corpus` and `0ca/corpus_pdfs` licences. All three are fetch-only until checked.
- `0ca/corpus_pdfs` has no LICENSE file and aggregates from veraPDF and the PDF/A-2008 suite. Not committable.
- KillerPDF's own internal inconsistency: `README.md` says 17,248 redistributable and `corpus.html` says 17,328, and the per-source counts disagree between `SOURCES.md` and `corpus.html` for at least five sources. Both documents are current. Read both.
- The `setpagedevice` and linearized `/Size` false-positive rates come from one commercial vendor's hardening notes. Verified as published; not independently reproduced.
