## Summary

Fetch the two external corpus rows and record what they actually do, and add the CI workflow that runs the documented gate. One of the two rows carried a verdict that had never been measured, and measuring it changed the value: `external/fax-decode-parms.pdf` was recorded `paint` and refuses with `invalidfont in Tj`.

---

## Motivation / context

- Plan: `plans/v0.0.4/5-validation.md` phase 13, rows 13.7 to 13.9. Parent phase: PR #17, which closed the traceability gap for the three v0.0.4 features.
- A corpus row only runs when its file is present, and `sampledata/validation/external/` is gitignored. Both external rows were therefore recorded and never executed. The `expect` column is documented as the measured verdict, and for one row it was not measured at all.
- The repository had no CI, so `make lint` and `make test` ran only when a human typed them.

---

## Changes

### A recorded verdict corrected by measurement

- `external/fax-decode-parms.pdf` is a 10-page qpdf scan with 8 images and an OCR text layer. `spectreps info` reports three fonts, `Helvetica-Bold`, `Times-Italic`, and `Times-Roman`, each with `embedded=false`.
- Measured, it refuses: `Error: /invalidfont in Tj`. The `expect` value moves from `paint` to `refuse:invalidfont in Tj`, which is the same policy already recorded as `refuse:invalidfont` on `postscript/cups-testfile.ps`.
- **The refusal is not a CCITT defect.** The message names `Tj`, a text operator, so the page decoded all 8 images before the text layer failed. The committed row `images/ccitt_EndOfBlock_false.pdf` still carries the painting CCITT claim and still passes.
- The two rows are external for different reasons, recorded in the corpus README: the fax file is 1,565,966 bytes, over the 1 MiB committed-tier limit, and its Apache-2.0 license is admitted; the PDF 2.0 file is 5,211 bytes and its CC-BY-SA-4.0 license is what keeps it out, because `AdmittedLicense` admits CC BY-SA in the external tier only.
- `external/pdf20examples/simple-pdf-2.0.pdf` passes as `struct` unchanged.

### CI

- `.github/workflows/ci.yml`, two jobs on every push and pull request.
- The `test` job runs `make test NPROC=16`. `NPROC` is a simple assignment in the Makefile, so a command-line value overrides `nproc`; `make -n test NPROC=16` prints `go test -p 16 ./...`, which is the proof the override reaches the tool.
- The `lint` job runs `make lint` with golangci-lint pinned to `v1.64.8`, the version `documentation/development.md` records.
- No step fetches the external tier, so CI is hermetic and those two rows skip rather than passing quietly. No test opens a network connection, and the gate does not depend on one.

### Ledger

- `plans/v0.0.4/5-validation.md` rows 13.7, 13.8, and 13.9, replacing the single phase-13 closure row.
- `sampledata/validation/README.md` gains an external-tier section with the fetch date, both digests, the two different reasons, the measured refusal, and the root-cause argument.

---

## Impact

| Area | Impact |
|------|--------|
| **Behavior / correctness** | No production code changed. The one behavior change is to a recorded claim: a corpus row that said `paint` now says `refuse:invalidfont in Tj`. Anyone who read the manifest believed that file rasterized; it does not, and did not. |
| **Corpus** | With the tier present, 66 pass, 0 fail, 0 skipped. With it absent, 64 pass, 0 fail, 2 skipped. `make test` is green in both configurations. |
| **API / CLI** | None. No exported symbol, flag, or exit code changed. |
| **CI** | First workflow in the repository. `make lint` and `make test` now run on every push and pull request instead of on demand. `concurrency` cancels superseded runs on the same ref. |
| **Dependencies** | None added to `go.mod`. The workflow uses `actions/checkout@v4`, `actions/setup-go@v5`, and `golangci/golangci-lint-action@v6`, and reads the Go version from `go.mod` rather than pinning one. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make lint` and `make test` with the external tier absent, the configuration CI sees
- [x] `bash scripts/validation-run.sh`
- [x] `bash scripts/check-traceability.sh`
- [x] `make pdfa-check`
- [x] `make pdfua2-check`
- [x] `make -n test NPROC=16`
- [x] `go run internal/validation/gen.go` re-fetched every pinned row and left the tree unchanged
- [x] The workflow parses as valid YAML with both jobs present

### Commands

```sh
make lint
make test
bash scripts/validation-run.sh
bash scripts/check-traceability.sh
make pdfa-check
make pdfua2-check
make -n test NPROC=16
```

`make lint` exits 0 with gofmt, golangci-lint, and `size-check` reporting `clean (0 over-limit files)`. `make test` passes 12 packages with a cleared test cache, both with the external tier present and with it absent. `scripts/validation-run.sh` reports `66 pass, 0 fail, 0 skipped` with the tier present. `scripts/check-traceability.sh` reports `270 cases: 244 live, 0 pending, 26 proof, 0 unowned, 0 failing`, unchanged from PR #17. `make pdfa-check` reports `compliant="2" nonCompliant="0"` and `compliant="1" nonCompliant="0"`, and `make pdfua2-check` reports 7 files each with `failedRules: 0`, on the locally installed veraPDF 1.30.2. The generator re-fetched every pinned row and left `sampledata/validation` unchanged.

### The failing row this surfaced

Fetching the tier turned the gate red before the manifest was corrected:

```
--- FAIL: TestValidationCorpusImages/external/fax-decode-parms.pdf
    validation_corpus_run_test.go: exit 1: Error: /invalidfont in Tj
```

That failure is the finding. It is recorded here rather than only in the phase file because a reviewer should see the red run, not just the corrected value.

---

## Screenshots / sample output

```sh
$ bin/spectreps info sampledata/validation/external/fax-decode-parms.pdf
PDF version: 1.4
Pages: 10
Tagged: false
Fonts:
  Helvetica-Bold embedded=false
  Times-Italic embedded=false
  Times-Roman embedded=false
Images: 8

$ bin/spectreps raster -o /tmp/fax.ppm sampledata/validation/external/fax-decode-parms.pdf
Error: /invalidfont in Tj

$ bin/spectreps raster -o /tmp/ccitt.ppm sampledata/validation/images/ccitt_EndOfBlock_false.pdf
$ echo $?
0

$ make -n test NPROC=16
go test -p 16 ./...

$ make -n test
go test -p 24 ./...

$ bash scripts/validation-run.sh | tail -1
66 pass, 0 fail, 0 skipped
```

`spectreps info` is what separates the two questions. The 8 images are present and the fonts are the problem. A raster refusal naming `Tj` rather than `Do` means the images decoded and the text layer is what failed.

---

## Related issues

- No GitHub issue exists for this work.
- Parent pull request: #17, `chore/validation-corpus-v004-features`.
- Program ledger: `plans/v0.0.1/00-program.md`. Phase file: `plans/v0.0.4/5-validation.md` phase 13.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/` when process-gated

---

## Follow-ups (out of scope)

- `scripts/check-traceability.sh` verifies that a row's test function exists, not that the function asserts the row's case. Repointing `compare files first byte` from `TestCompareFiles` to the real but unrelated `TestValidatePS` still reports `270 cases, 0 failing`. Closing it means giving each test a case identifier the script greps for in the function body, which changes test style across the suite.
- No CI job runs `make validation-run`, `make refs-gs-check`, or the veraPDF targets. veraPDF is a proof tool and stays out of the gate, so its verdicts remain a manual act.
- The external tier is fetched by a deliberate local `go run internal/validation/gen.go -fetch-external`. No automated job runs it, which is why this PR had to do it by hand.
- The no-outline policy is unchanged: standard 14 text does not render. Fully documented and green, and a user only discovers it by trying.

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

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.md` | 2 | 41 | 2 |
| `.tsv` | 1 | 1 | 1 |
| `.yml` | 1 | 92 | 0 |
| **Total** | **4** | **134** | **3** |
