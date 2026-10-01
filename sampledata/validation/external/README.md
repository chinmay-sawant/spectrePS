# The live corpus tier

This folder holds the fetched half of the validation corpus. The files here are
not committed. The rows in `../manifest.tsv` that pin them are committed, so the
reviewable record of what this tier contains lives in git even though the bytes
do not.

Nothing in `make test` or `make lint` needs this folder. A test that names a row
whose file is absent skips with the path it expected, so a fresh clone runs the
whole gate without a network.

## Filling it

```sh
make validation-fetch
```

That runs `go run internal/validation/gen.go -fetch-external`, which downloads
every external row, checks its SHA-256 and byte count, and only then places the
files. A digest mismatch exits non-zero and writes nothing, so a partial fetch
never looks like a complete one.

Confirm a machine is provisioned without touching the network:

```sh
make validation-verify
```

## The bulk tier

`make validation-fetch BULK=1` adds `_bulk/`, the batch2 tarball from
`labs.pdfa.org/stressful-corpus/pdfs_202011/`: 5,613 files, 6,063,564,131 bytes
extracted, and a 4.5 GB download. It is opt-in because of the size; a plain
`make validation-fetch` prints the cost and fetches none of it. The committed
`../bulk.tsv` pins the archive SHA-512 and byte count, and
`../bulk/batch2.members.tsv` pins every member SHA-256, so extraction verifies
each file before it reaches the tree. Upstream states no redistribution grant
for the set, so the tier is fetched and never committed. No manifest row names
these files and `make test` never reads them; the tier exists for the batch2
audit.

## How the fetch behaves

Every row is fetched into a content-addressed cache first, at
`os.UserCacheDir()/spectreps/validation`, and then hardlinked into this folder.
Three things follow. A killed run never leaves a half-written file, because each
blob is written to a temp name and renamed. A second run costs no network, which
is why `make validation-fetch` is instant once the cache is warm. And the tree
costs no extra bytes, because the files are links.

Override the cache with `-cache DIR` or the `SPECTREPS_VALIDATION_CACHE`
environment variable, which is how a CI cache step points at a restored
directory.

Per-host concurrency and rate limits are a table in `gen.go`, not a flag, so the
numbers are reviewable in a diff. Every request carries a descriptive User-Agent,
which sec.gov and Wikimedia both require and both enforce with a 403.

## What is here now

| Folder | Rows | Licence | What it is |
| --- | --- | --- | --- |
| `_bulk/` | 5,613 | none stated upstream | The batch2 stressful-corpus tarball: GHOSTSCRIPT and TIKA issue-tracker files from `labs.pdfa.org`. Fetched only with `make validation-fetch BULK=1`, because the download is 4.5 GB. Audit material, not a `make test` input. |
| `artifex/` | 233 | AGPL-3.0 | The Artifex public test files: 79 PDF/X prepress suites, 29 JBIG2, 13 JPEG2000, the PDF Association safedocs subset, 21 PostScript programs, 6 encrypted, and 4 CMap cases. Fetched, never committed. Three files are excluded for third-party content. |
| `verapdf/` | 2,691 | CC-BY-4.0 | The veraPDF conformance corpus, three levels deep by ISO 32000 clause. Every expected outcome is a measured Ghostscript run rather than a recording of our own reader. The upstream directory structure is preserved on disk. |
| `handbuilt/` | 89 | CC-BY-SA-4.0 | ISO 32000-1 well-formedness cases, one defect each. Fetched rather than committed because the dataset's rights record is share-alike, not CC0. See `handbuilt/README.md`. |
| `pdfCabinetOfHorrors/` | 23 | CC0-1.0 | Real-world files carrying a feature an archival tool rejects: four encrypted files, an embedded video, a JavaScript action, a JPX image, non-embedded fonts, and three deliberately damaged variants. |
| `cups/` | 3 | Apache-2.0 | Scribus 1.4 PostScript output with embedded subset fonts, `setcachedevice` charstrings, and `VM` operators. The best `setcachedevice` coverage in any permissively licensed source. |
| `fax-decode-parms.pdf` | 1 | Apache-2.0 | A 1.5 MiB CCITT fax file with a non-embedded standard 14 text layer. |
| `pdf20examples/` | 1 | CC-BY-SA-4.0 | The PDF 2.0 container. |

`pdfCabinetOfHorrors/digitally_signed_3D_Portfolio.pdf` is deliberately absent.
Its folder README says it was "kindly provided by Adobe", so the repository's
blanket CC0 statement does not clearly reach it and the file is not pinned.

## What is not here, and why

`govdocs1-error-pdfs` from the same upstream is 54 real-world files that
defeated a preservation tool, which makes it the most valuable group found. Its
own README says the files are "copied from Govdocs1" and quotes GovDocs' claim
of free redistribution for research, which is not the same as the CC0 the parent
folder claims. It stays out until someone decides that is close enough, because
a wrong licence column is worse than a missing row.

The Artifex public test files are AGPL-labelled and some carry proprietary
third-party content, so they can be fetched and run but never committed. They
are not pinned yet. The token list in `internal/validation/manifest.go` refuses
them in the committed tier.
