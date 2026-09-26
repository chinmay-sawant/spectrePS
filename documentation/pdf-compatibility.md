# PDF versions and compatibility

Spectre reads the PDF files its parser and painter can handle. It has no
allowlist of two PDF versions. The validation corpus has ordinary files with
headers from PDF 1.0 through PDF 2.0, plus an intentionally invalid 3.5
header. Opening one file at a given version does not establish support for
every feature that version defines.

## Version reported by `info`

`spectreps info` reports the effective PDF version. It starts with the
`%PDF-x.y` header and takes a higher `/Version` name from the current document
catalog. An incremental update can replace the catalog and raise the version.
The three files in `sampledata/validation/compatibility/versions/` each have
a 1.4 header and a current catalog version of 1.6. `info` reports 1.6 for all
three. This follows the [PDF Association version examples](https://github.com/pdf-association/pdf-differences/tree/907fe96e52b73e491489eee545c47b119bf9989b/PDF-version).

The report describes a declared version. It does not certify that the objects
inside the file obey the PDF specification. The third version example
declares 1.6 but contains `/UseBlackPtComp`, a PDF 2.0 feature. Spectre's
painter refuses its graphics state with `undefined in gs`; `info` can still
read its structure and report 1.6.

## Corpus cases

| File | Header | `info` | Paint or `validate` |
| --- | --- | --- | --- |
| `compatibility/versions/asciihexdecode.pdf` | 1.0 | 1.0 | Page structure opens. |
| `compatibility/versions/poppler-67295-0.pdf` | 1.2 | 1.2 | Page structure opens. |
| `compatibility/versions/PDF-versions1.pdf` | 1.4 | 1.6 | Passes. |
| `compatibility/versions/PDF-versions2.pdf` | 1.4 | 1.6 | Passes. |
| `compatibility/versions/PDF-versions3.pdf` | 1.4 | 1.6 | `undefined in gs`. |
| `compatibility/fonts/Type3Test.pdf` | 1.7 | 1.7 | `invalidfont in Tj`. |
| `compatibility/encryption/encrypted-40-bit-R3.pdf` | 1.4 | Refused. | `invalidaccess in Encrypt`. |

The Type 3 case shows that a readable page tree does not mean the text can
paint. The encrypted case cannot open at all. The manifest pins each source,
license, byte count, digest, and expected outcome. The PDF Association files
are CC BY 4.0; the qpdf encrypted file is Apache 2.0. The corpus tests never
start Ghostscript, and this table makes no claim about a Ghostscript result
for these exact files.

The version test opens corpus files with every published header version from
1.0 through 1.7, plus 2.0. It checks the reported version for each file, not
complete rendering or conformance for each PDF version.

## PDF/A profiles

PDF/A-1a and PDF/A-1b are profiles of PDF 1.4. PDF/A-2 and PDF/A-3 add A,
B, and U levels based on PDF 1.7. PDF/A-4 uses PDF 2.0 and has base, E,
and F profiles. There is no PDF/A-1c profile. These names describe archival
requirements, not extra PDF header versions.

The committed corpus has one veraPDF-compliant file for each of the 11
profiles: 1a, 1b, 2a, 2b, 2u, 3a, 3b, 3u, 4, 4e, and 4f. The 3u source uses
the `.cv` extension but starts with `%PDF-1.7`; the committed copy has a
`.pdf` extension so both Spectre and veraPDF can read it. Three existing
PDF/A-4 and 4f files provide noncompliant controls. All 14 profile verdicts
come from veraPDF 1.30.2 with an explicit `--flavour` for each file. The
checker reads its JSON compliance result.

The manifest records Spectre's separate `info` outcome for each file. A
`struct` result means Spectre opened the page tree. It says nothing about
archival compliance, accurate painting, or support for writing that profile.
The PDF/A-3a sample embeds Latin Modern Roman. That font is distributed under
the GUST Font License. The other new profile samples contain no embedded font
files.

| Profile | veraPDF | Spectre `validate` on the compliant sample |
| --- | --- | --- |
| 1a | Compliant. | `ua2-document in PDFUA`. |
| 1b | Compliant. | Passes. |
| 2a | Compliant. | `ua2-document in PDFUA`. |
| 2b | Compliant. | Passes. |
| 2u | Compliant. | `invalidfont in Tj`. |
| 3a | Compliant. | `invalidfont in TJ`. |
| 3b | Compliant. | Passes. |
| 3u | Compliant. | Passes. |
| 4 | Compliant. | Passes. |
| 4e | Compliant. | Passes. |
| 4f | Compliant. | Passes. |

Spectre `validate` also passes all three PDF/A-4 and 4f controls that
veraPDF marks noncompliant. A `validate` pass is therefore no PDF/A
compliance verdict. For tagged PDF/A-1a and 2a, the PDF/UA-2 preflight is
the first rule that refuses the sample.

Run the profile checks with:

```sh
make pdfa-corpus-check
```

The profile map lives in `scripts/pdfa-profiles.tsv`. The command requires a
veraPDF executable. Set `VERAPDF` to its path if it is outside the repository
and not on `PATH`.

## Compliance checks

`spectreps validate` stops at the first parser or painter error. It is not a
general PDF specification validator. For a tagged input, it also runs the
PDF/UA-2 machine preflight described in `devices.md`. A PDF/A-4 rewrite runs
the separate PDF/A-4 profile preflight before it writes a claim. These
preflights check named rules; they do not certify a file. The optional
`make pdfa-check` and `make pdfua2-check` targets use veraPDF on sampled
profile files when it is installed.

Run the compatibility corpus and version report checks with:

```sh
go test -count=1 ./internal/cli -run 'TestValidationCorpusPDF|TestValidationPDFVersionCorpus'
```

Run the full committed corpus with `make test`. The source files can be
restored from their pinned URLs with `go run internal/validation/gen.go`.
