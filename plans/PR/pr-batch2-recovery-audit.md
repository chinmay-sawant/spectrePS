## Summary

This branch lands the batch2 recovery work and the fetched live validation corpus. The PDF reader recovers damaged xref tables, page trees, and streams that Ghostscript opens, and it opens encrypted files whose empty user or owner password authenticates. The corpus gains area, probe, and basis labels, a `survive` expectation, and a per-area pass-rate report.

The branch is 41 commits, 44 files, +10,392/-436 against `master` at 8aea333. Head 7d971d8, merged as 07508d6.

## Motivation / context

- Plans: `plans/v0.0.5/8-real-world-corpus.md` (new), `plans/v0.0.5/00-program.md` phase 8, `plans/v0.0.1/10-deferred.md` 10.2.
- Product decision 2026-09-29: Spectre PS opens what Ghostscript opens. The xref recoveries and empty-password decryption follow that decision.
- Issues: no issue ID was supplied.

## Changes

### Xref and trailer recovery

New `internal/pdf/xrefrecover.go` (700 lines) and the matching routes in `internal/pdf/file.go`:

- Rebuild the xref from `num gen obj` headers when `startxref` names nothing. A candidate has to parse end to end, so object-like bytes inside a valid stream are not indexed. Measured 274 to 357 of the 664 original refusals at that commit (d3eaf32).
- Merge every xref stream the file carries, newest section winning per object number, and index compressed objects from object streams. Rows that name the wrong bytes fall back to the header scan, then to the object streams. Bucket b07 went 0 to 6 of 27 (a022bca).
- Accept five table shapes: a missing or unreadable `startxref`, an offset that names no section, a section that parses to zero rows, a short subsection count, and a missing `endobj` after a complete value. Bucket b01 went 13 of 30, was 0 (1902769).
- Rebuild when `startxref` is absent or unreadable, not only when it names nothing. Re-read a classic section against the specification's 20-byte row grid, so one damaged row no longer shifts every row after it. An absent `/Length` scans to `endstream`. Bucket b02 info went 0 to 25 of 37, raster 0 to 11 (89bec0d).
- Fall through to the header rebuild when `startxref` names a damaged classic table. A `/Prev` that points past the end, names nothing, or names a damaged older section is a broken link, not a damaged newest table. A generation mismatch resolves through headers only on a rebuilt table, so a table the file wrote keeps its strict check. Bucket b03 info went 0 to 10 of 35, raster 0 to 3; 436 to 449 of 664 (1c04f5a).
- Find the last dictionary carrying `/Root` when the `trailer` keyword is damaged (`lastRootDict`). Read a classic table whose keyword sits within 256 bytes before the offset (`recoverNearTable`). Measured 585 to 588 of 664 (fc42b2a).
- Synthesise a root from the newest `/Type /Catalog` object when no trailer names one (`catalogTrailer`). Measured 588 to 602 of 664. The row `structural/parser_rebuildxref_error_notrailer.pdf` now reaches raster and refuses `invalidfont in Tj` (38646ab).
- Read a dictionary key whose leading slash is missing (`+AF` as key `AF`). Measured 602 to 603 of 664 (17bb0ac).
- Scan page objects by number when a rebuilt table cannot reach the page tree. Measured 603 to 605 of 664 (7326f16).
- Read a reference whose generation was dropped, and skip a lone `+` in an array element position. Measured 605 to 606 of 664. The byte test in `peekBareRef` keeps the level-2 rewrite allocation gate at 715 (3bb537d).
- Scan inside a stream span only when its direct `/Length` disagrees with the parsed stream span, keeping objects outside the span ahead of nested candidates (34e683b).
- CI review fixes and a null initialisation for the recovery candidate (5e1bfc9, 636da99, 7d971d8).

Refusals kept on purpose: a dead row the file does not carry anywhere stays `syntaxerror in xref` (`TestReferencedDeadXrefRowStillFails`), a `/Prev` cycle or a chain over 64 stays refused (`bug_xrefv4_loop.pdf`), a table the file wrote keeps strict generation checks (`TestValidTableGenerationStaysStrict`), and a file with no `/Root` in a trailer and no `/Catalog` object still refuses.

### Encryption

New `internal/pdf/crypt.go` (723 lines) and `internal/pdf/cryptvalue.go` (180 lines):

- ISO 32000-1 Algorithm 2 key derivation with the user and owner checks for revisions 2 to 4, and ISO 32000-2 Algorithm 2.B with the AESV3 key path.
- RC4-40, RC4-128, AES-128-CBC, and AES-256-CBC. RC4 is implemented in package because `crypto/rc4` is deprecated.
- Decryption applies at every string and stream read, not only page content. The `/Encrypt` handler is resolved before the crypt state is installed, so its own `/O` and `/U` are never decrypted. A per-stream `/Crypt` filter is removed from the decode chain once it selects the cipher. `RawObject` refuses every object while a crypt state is installed, so a copy writer never emits ciphertext.
- 129 of the 135 encrypted bucket files now open; extracted text and page counts (56, 1, 127) match pypdf. The merged count went 449 to 578 of the 664 original refusals (97f6f83, 0bcc76a).
- Still refused: non-`/Standard` handlers (public key), `/R` outside 2 to 6, short `/O` or `/U`, a non-empty user password, unknown `/CFM`, a named default filter absent from `/CF`, and corrupted `/O`, `/U`, or `/ID`. In that bucket, four AESV2 files whose empty password does not authenticate and two files that fail on xref stay refused.
- Refusal tests pin every handler shape across revisions 2 to 6: literal and hex `/O` `/U`, both `EncryptMetadata` values, inline and indirect handlers, unresolvable references, and malformed trailers (8f3154f, da7ab3e, 6aace77). The b08 measurement corrects an earlier count: 58 of 59 files derive an empty user password, and b08 plus b09 hold 118 encrypted files.

### Page tree, content, and infos

- Read the kind of a `/Contents` array after resolving an indirect reference. 93 of the 664 refusals opened, and another 50 share the cause (f9b4385).
- Keep the decoded head of a damaged Flate stream instead of refusing, and skip an in-use xref row whose object the file does not carry in the font and image surveys. Bucket g01 opened 48 of 78, was 0; bucket g12 opened 26 of 65 (719c8e5, fe65e2f).
- Resolve ISO 32000-1 filter abbreviations (`/A85`, `/AHx`, `/LZW`, `/Fl`, `/RL`). Ignore a catalog `/Version` that is not a name and keep the header version, which pdfTeX and luaTeX need. Treat a content stream whose decoder is missing, or whose expansion hits the 32 MiB cap, as a page-local loss. Splice nested `/Contents` arrays with a depth cap of 8, and resolve an indirect `/Filter` or `/DecodeParms` on a copy. Bucket b06 opened 8 of 29, was 0; 407 to 415 of 664 (a2c6833).
- Track repeated and indirect `/Kids`, handle a childless `/Pages` as an empty subtree, and skip a `/Kids` entry that resolves to no object in the page, content-number, and size walks. The page-node resolve was extracted from `walkPageSizes` when complexity hit 13 against a limit of 10; the extraction is neutral on 47 of 65 tail files (2b39263).
- Keep page sizes through `recoverPageSizes`, `scanPageSizes`, and `walkDamagedPageSizes` when a rebuilt table or a page tree holding a stream cannot be walked.

### Graphics fill bound

`Pixmap.Fill` now computes the path's bounding box and tests only pixels inside it. A 6,638-point path on a 612 by 792 page was 484,704 pixels times 6,638 points, about 3.2 billion cross tests; the file read as a hang but was finite. GHOSTSCRIPT-692620-1.pdf finishes in 16 seconds instead of never, and its remaining failure is the documented `invalidfont in Tj` (91352dc). A synthetic path measures 0.009s bounded against 17.3s unbounded. Tests pin the bounding-box correctness invariant and a 5-second wall-clock guard (8914def).

### Live validation corpus

- Manifest schema 7 to 11 columns: `area`, `probe`, `basis`, and `pages` added. `expect` gains `survive` (exit 0, no error). Closed vocabularies for 15 areas, 6 probes, and 3 bases. `baseline` rows are report-only debt and never gate; `spec` and `gs` rows gate. AGPL-3.0 and CC-BY-SA-4.0 are admitted only in the external tier.
- `internal/validation/gen.go` rewritten with a content-addressed sha256 cache (`os.UserCacheDir()/spectreps/validation`, `-cache` or `SPECTREPS_VALIDATION_CACHE`), `-verify-only`, `-dry-run`, `-area`, and `-workers`. Per-host budgets, five retries with jitter, and retryable 408/429/5xx responses.
- `sampledata/validation/manifest.tsv` grew from 83 rows to 3,123. Measured basis: 2,889 `gs`, 180 `spec`, 54 `baseline`. Probes: 3,032 `info`, 38 `ps`, 31 `raster`, 11 `text`, 8 `rewrite`, 3 `gs`.
- New repo-authored `sampledata/validation/postscript/standard14-sweep.ps` (groff `-Tps` output naming 13 standard 14 faces). Its expected outcome is `refuse:undefined in matrix`, tracked in 10-deferred 10.1.
- New `scripts/validation-report.sh` joins `go test -json` subtests on area and basis and writes `profiles/corpus-report.tsv`. `Makefile` gains `validation-fetch`, `validation-verify`, and `validation-report`. `.gitignore` keeps the external tier except its README and ignores `/.notes/`.
- The corpus report at that commit measured 3,123 rows, 3,123 pass, 0 fail, 0 skip.

### Docs and audit tooling

- `documentation/features.md` and `documentation/test.md` record the label axes, the live tier, and the report commands. `documentation/test.md` adds the handbuilt, archival, survive-verdict, and fault-signal tests.
- `plans/v0.0.5/8-real-world-corpus.md` is the new phase 8 plan; `plans/v0.0.1/10-deferred.md` records five PostScript operators (`matrix`, `setpacking`, `ashow`, `widthshow`, `awidthshow`), twelve operators from `ArtifexSoftware/tests`, and the `scan` rejection policy.
- New `scripts/audit-batch2-render.py` resumes the 664-file audit in short batches with a 25-second child cap. It records Info results, Spectre page-one raster results, and Ghostscript PPM page output. Result: 581 of 664 files emitted at least one nonempty PPM page at 12 dpi, Info passed on 612, and 564 pass both. Three named files now pass Info: GHOSTSCRIPT-701877-0 (1 page), GHOSTSCRIPT-695619-0 (269 pages), TIKA-3224-1 (12 pages); Ghostscript emitted 1, 299, and 24 pages.
- `sampledata/validation/traceability.tsv` gains 13 rows for the new tests and commands.

## Commits on the branch

Oldest first. The commit bodies carry the measured numbers for each step.

```text
ab168c5 Add a fetched live tier to the validation corpus and report a pass rate
719c8e5 fix(pdf): keep damaged flate streams and duplicated page-tree kids
8f3154f Lock the encrypted-PDF refusal against a real security handler
da7ab3e Lock the /Encrypt refusal against the shapes real files carry
fe65e2f Step over dead xref rows in the font and image surveys
f9b4385 fix(pdf): read the /Contents kind after resolving the reference
ed90ca6 Merge branch 'fix/batch2-g10' into fix/batch2-integrated
e89ee73 Merge branch 'fix/batch2-g06' into fix/batch2-integrated
4a77138 Merge branch 'fix/batch2-g04' into fix/batch2-integrated
345de41 Merge branch 'fix/batch2-g05' into fix/batch2-integrated
2b39263 Extract the page-node resolve from walkPageSizes
d3eaf32 Rebuild an xref table from object headers when startxref names nothing
6aace77 Lock the /Encrypt refusal against the b08 handler shapes
a022bca Recover the xref at the /Prev, stream, and row level
ef36f4e Merge branch 'fix/batch2-b07' into fix/batch2-integrated
1902769 Open five xref refusal shapes without a table swap
3020b2e Merge b01 into b07's xref recovery, keeping both mechanisms
89bec0d Recover four xref shapes the batch2 xref buckets still refused
12186e2 Take b02's xref recovery whole over the b01 and b07 union
a2c6833 Open eight tail refusals from filter, version, and page shapes
29aa937 Merge branch 'fix/batch2-b06' into fix/batch2-integrated
1c04f5a Open damaged xref tables and broken /Prev links from the headers
ae056a1 Label the UnknownFilter tolerance as ours, not the oracle's
97f6f83 Open encrypted files whose empty user password authenticates
0bcc76a Merge b09: open encrypted files whose empty user password authenticates
fc42b2a Read the root dictionary when the trailer keyword is damaged
91352dc Bound the fill scan by the path's bounding box
8914def Pin the new recoveries and the fill bound with tests that bite
38646ab Synthesise a root from the catalog when no trailer names one
17bb0ac Read a dictionary key whose leading slash is missing
7326f16 Find pages by scanning when a rebuilt table cannot reach them
3bb537d Read a reference whose generation was left out, and a stray sign in an array
34e683b fix(pdf): recover damaged batch2 structures
1e33c65 docs(pr): add batch2 recovery audit details
cb25771 docs(pr): sync batch2 diff table
55bde8e docs(pr): record batch2 review metadata
5e1bfc9 fix: address batch2 CI review findings
320cee1 docs(pr): refresh final batch2 diff table
636da99 fix: clear remaining CI lint findings
8a1fcf8 docs(pr): update final test and diff results
7d971d8 fix: initialize recovery candidate with null value
```

## Impact

| Area | Impact |
|------|--------|
| **Performance** | Fill work is proportional to the path's bounding box: 0.009s bounded against 17.3s unbounded on the synthetic guard, and GHOSTSCRIPT-692620-1.pdf went from past 30 seconds to 16 seconds. Xref rebuilds and header scans run only on damaged files and cache per `File`. |
| **Memory** | Header-scan and packed-object caches per `File`. The audit renders into a temporary directory that is removed after counting pages. |
| **Behavior / correctness** | Damaged xref tables, damaged page trees, and empty-password encrypted files now open. Dead rows, `/Prev` cycles, no `/Root` plus no `/Catalog`, malformed streams, unknown filters, and non-empty passwords still refuse. The manifest schema changed. |
| **API / CLI** | No public Go API or CLI command changes. New make targets and corpus files. |
| **Dependencies** | None added. MD5, RC4, and AES uses carry `nolint` with the ISO algorithm step that requires them. RC4 is local because `crypto/rc4` is deprecated. |
| **Binary size / build time** | No new modules. Binary size grows with the crypt and recovery code. |

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Manifest schema is 7 to 11 columns with closed vocabularies for `area`, `probe`, and `basis`. | Add the four columns to any consumer and use the vocabulary values. Readers that only need `path` still parse rows positionally. |
| Encrypted PDFs with an empty user or owner password now open instead of returning `invalidaccess`. | Callers that relied on the old refusal need to handle readable content. Non-empty passwords and public-key handlers still refuse. |

## Test plan

- [x] `make build`
- [x] `make test` (all 14 packages; fetched corpus rows skip when the external tier is absent, which is how CI runs)
- [x] `make lint` (CI findings cleared in 5e1bfc9 and 636da99)
- [x] `make validation-fetch`, `make validation-verify`, `make validation-report` (report measured 3,123 rows, 3,123 pass, 0 fail, 0 skip)
- [x] `python3 scripts/audit-batch2-render.py summary` (needs the gitignored `.notes/wt/refuse664.tsv` and `sampledata/validation/external/_bulk/batch2`)

### Commands

```sh
make build
make test
make lint
make validation-fetch
make validation-verify
make validation-report
python3 scripts/audit-batch2-render.py summary
```

## Screenshots / sample output

```text
batch2 audit     GS pages emitted 581/664; Info passed 612/664; both 564
                 remaining Info refusals: 12 syntaxerror in xref, 4 undefined in pdf,
                 1 syntaxerror in FlateDecode
Info recoveries  GHOSTSCRIPT-701877-0   1 page    (GS 1)
                 GHOSTSCRIPT-695619-0   269 pages (GS 299)
                 TIKA-3224-1           12 pages  (GS 24)
corpus report    3,123 rows, 3,123 pass, 0 fail, 0 skip
                 basis: 2,889 gs, 180 spec, 54 baseline
```

## Related issues

- No issue ID was supplied.
- Plans: `plans/v0.0.5/8-real-world-corpus.md`, `plans/v0.0.1/10-deferred.md` 10.1 and 10.2.

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [x] Related issue status stated above
- [x] Filled body committed under `plans/PR/pr-batch2-recovery-audit.md`

## Follow-ups (out of scope)

- 17 Ghostscript-rendered files still fail Info: 12 `syntaxerror in xref`, 4 `undefined in pdf`, 1 `syntaxerror in FlateDecode`.
- Four AESV2 encrypted files whose empty password does not authenticate, plus public-key handlers and files with a real password. No password prompt exists.
- Documentation debt: `documentation/features.md` (lines 16, 45, 62, 102) and `documentation/test.md` (lines 156, 171, 173, 265) still say an encrypted file is refused. The merged code opens empty-password files.
- `internal/pdf/xrefrecover.go` (comment at lines 28-34) still says `structural/parser_rebuildxref_error_notrailer.pdf` refuses, but catalog synthesis now opens it through to raster `invalidfont in Tj`.
- `internal/pdf/encrypt_test.go` header comment still says no key derivation exists.
- The 664-file audit depends on the gitignored `.notes/wt/refuse664.tsv` and `external/_bulk/batch2`, so it cannot be rerun from a clean checkout.
- Deferred PostScript operators: `matrix`, `setpacking`, `ashow`, `widthshow`, `awidthshow` (10.1), twelve operators from `ArtifexSoftware/tests` (10.6), and the `scan` rejection policy.

## Reviewer checklist

- [x] Behavior matches summary and test plan
- [x] No unrelated changes in diff (two workstreams: batch2 recovery and live corpus, both described)
- [x] Public API / CLI changes documented (none; manifest schema change described)
- [x] New recovery rules have fixture coverage
- [x] PR has assignee and labels
- [x] Related issues use correct Closes/Relates keywords (none supplied)
- [x] No secrets or generated artifacts committed (external tier gitignored except its README, `/.notes/` ignored)
- [x] Diff-stat-by-extension table pasted below

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 29 | 5681 | 209 |
| `.md` | 8 | 947 | 140 |
| `.ps` | 1 | 279 | 0 |
| `.py` | 1 | 176 | 0 |
| `.sh` | 1 | 139 | 0 |
| `.tsv` | 2 | 3137 | 84 |
| No extension | 2 | 33 | 3 |
| **Total** | **44** | **10392** | **436** |
