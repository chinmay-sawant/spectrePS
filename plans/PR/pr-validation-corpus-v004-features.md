## Summary

Phase 12 of the v0.0.4 validation ledger closed on `0 unowned` in the traceability table, and the number was true while the table was blind. `documentation/test.md` had sections for v0.0.1 through v0.0.3 only, so when v0.0.4 landed font subsetting, PDF/UA-2 tag generation, and the Type 1 program, the Go tests came with the features and no document grew to match. No row meant nothing to work, so the corpus never covered the three and the traceability check had nothing to miss. They had 13, 40, and 5 test functions between them and zero rows.

The gap was the ledger, not the corpus. Every fixture the three cases need was already committed, so this adds no network fetch.

---

## Motivation / context

- Plans: `plans/v0.0.4/5-validation.md` phase 13, `plans/v0.0.5/v0.0.5-closure.md` phase 2
- Issues: see **Related issues**

---

## Changes

### Corpus rows

- `sampledata/validation/text/subset-text.pdf` and `text/type1-text.pdf` are copies of `sampledata/fixtures/subset-text.pdf` and `sampledata/fixtures/type1-text.pdf`, byte-identical by digest. They are the two checked-in programs that carry an embedded font outline, so the corpus can drive all three features without a fetch.
- Each carries a manifest row with a SHA-256, a byte count, a license field, and an `expect` value, plus a golden under `text/expected/`.
- The originals stay in `sampledata/fixtures/` on purpose: `internal/cli` reads them there to hold its import boundary, and `internal/pdf` writes the subset one from `TestGenSubsetFixture`. Both copies are listed separately so a change to either has to be made in two places deliberately.
- `traceability.tsv` gains 25 rows across the three sections. Every row names a test that exists.

### Tests

- `internal/cli/validation_corpus_font_test.go` adds `TestValidationCorpusSubset`, `TestValidationCorpusTagGeneration`, and `TestValidationCorpusType1`. 19 subtests.
- The rows are named rather than discovered. A row earns a place when it carries the structure the case needs: a TrueType `/FontFile2` program for the subsetter, an untagged page for the tag generator, a symbolic `/FontFile` Type 1 program for the Type 1 machine. The manifest digest and `expect` value are the input contract.
- `documentation/test.md` gains `Type 1 programs`, `Font subsetting`, and `PDF/UA-2 tag generation` sections, and the `TestValidation` roster names the three new functions so `-run TestValidation` still selects the whole group.

### Ledgers

- `plans/v0.0.4/5-validation.md` gains phase 13 with seven closed rows, each carrying its command and its outcome, plus two rows on what the phase will not do.
- `plans/v0.0.5/v0.0.5-closure.md` records the merged-tree run at `76a95d9` for rows 2.1 to 2.3, keeps the development-tree run in phase 1 rather than overwriting it, and leaves row 2.4 open. Row 2.2 said `eleven packages` while naming twelve, so the count is corrected to twelve in both places it appears.

### Three verdicts that are narrower than they look

- **Subset.** The invariant is the retag, not the size. The writer prepends a six-letter tag derived from the subset digest to the original `/BaseFont`, and a name the source already carried passes through unchanged, so the test asserts at least one name was rewritten rather than all of them. The original producer tag survives inside the embedded program's name table, which is why the test reads `/BaseFont` rather than scanning the payload. Subsetting an already-subsetted program can grow the file, so no case claims the subset write is smaller than the input. Only `text/subset-text.pdf` carries no `/ToUnicode`, so the synthesis case is named on that row alone and skips if the source ever gains one.
- **Tag generation.** Three untagged rows are accepted and four are refused, each a distinct contract: a clip is `undefined in W` through the level 0 path recorder, an image with no `/Alt` is `alt in Tag`, a tagged input is `tagged in RewritePDF` before either, and `-tags` with `-pdfa` is `unsupported in RewritePDF`. Every refusal writes no output file. Reading order, role assignment, and the `O(n^2)` step in `tag.DerivePlan` stay a unit-test claim in `internal/tag`.
- **Type 1.** The row is `struct`, not `paint`, and that is the measured verdict rather than a shortfall. The page shows three codes and the synthetic program carries two, so `spectreps text` returns `ABZ` with the code-point fallback for the third while `spectreps raster` refuses `invalidfont in Tj` and writes nothing. The refusal is the no-outline policy in `documentation/fonts.md` applied to a code the program does not carry. A blank page would have been the wrong answer. Painting `seac` and flex glyphs therefore stays a unit-test claim, recorded as such so the corpus row is not read as more than it is.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None. No production file changes; the only Go file is a `_test.go`. The three new tests add 0.34s to `internal/cli`. |
| **Memory** | None at runtime. Each test writes to `t.TempDir()` and the corpus adds 2,953 bytes of PDF. |
| **Behavior / correctness** | No shipped behavior changes. The corpus now pins three refusal messages and the `ABZ` extraction, so a change to any of them fails a test. |
| **API / CLI** | No signature or flag changes. The tests drive `-subset-fonts`, `-tags`, `-claim`, `-tag-title`, `-tag-lang`, and `text` as they already exist. |
| **Dependencies** | None. No `go.mod` change, no third-party module, no new tool. |
| **Binary size / build time** | Unchanged. `make build` still produces the same 7577005 byte `bin/spectreps` reporting `0.0.4`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | No API, CLI, or file format change. The two new corpus PDFs are additive. |

---

## Test plan

- [x] `make test`
- [x] `make lint`
- [x] `make build`

### Commands

```sh
make lint
go clean -testcache && go test -p 4 -count=1 ./...
make build
go test -count=1 ./internal/cli -run 'TestValidationCorpusSubset|TestValidationCorpusTagGeneration|TestValidationCorpusType1' -v
bash scripts/check-traceability.sh
bash scripts/validation-run.sh
```

All exited 0 on 2026-09-26 with go1.26.4 on linux/amd64:

- `make lint`: gofmt clean, `golangci-lint run ./...` 0 findings, size-check 0 over-limit files.
- `go test -p 4 -count=1 ./...`: 12 packages ok, 3 with no test files, 0 failures. The test cache was cleared first, because 9 cached packages is weak evidence for a gate.
- The three new tests: 19 subtests, 0 failures, 0 skips.
- `scripts/check-traceability.sh`: `270 cases: 244 live, 0 pending, 26 proof, 0 unowned, 0 failing`.
- `scripts/validation-run.sh`: `64 pass, 0 fail, 2 skipped`.

---

## Screenshots / sample output

```
=== RUN   TestValidationCorpusSubset
--- PASS: TestValidationCorpusSubset (0.31s)
=== RUN   TestValidationCorpusTagGeneration
--- PASS: TestValidationCorpusTagGeneration (0.02s)
=== RUN   TestValidationCorpusType1
--- PASS: TestValidationCorpusType1 (0.01s)
PASS
```

```
270 cases: 244 live, 0 pending, 26 proof, 0 unowned, 0 failing
64 pass, 0 fail, 2 skipped
```

The `Z` in the Type 1 golden is the code-point fallback, and the reason the same file refuses to paint:

```
$ od -c sampledata/validation/text/expected/type1-text.txt
0000000   A   B   Z  \r  \n
```

---

## Related issues

- Relates to the phase 13 rows in `plans/v0.0.4/5-validation.md`
- Relates to rows 2.1 to 2.4 in `plans/v0.0.5/v0.0.5-closure.md`

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [x] Related issues filled
- [x] Filled body committed under `plans/PR/pr-validation-corpus-v004-features.md`

---

## Follow-ups (out of scope)

- Row 2.4 of `plans/v0.0.5/v0.0.5-closure.md` wants `make bench` on the merged tree, about six minutes. It is work rather than a decision and is left open.
- Deferred row 4.3 of `plans/v0.0.5/5-baseline-and-budget.md` wants a `-count=10` re-capture before any timing number is treated as a baseline. It needs a decision from the integrator, not more code.
- A corpus folder for font subsetting or tag generation. Both need a row with a particular structure rather than a row of their own, so the two tests name the committed rows they drive.
- A real third-party Type 1 program with `flex`. The synthetic program in `internal/type1synth` covers the mechanism, and a fetched font would need a license-gate entry and a redistribution review for no extra coverage.

---

## Reviewer checklist

- [x] Behavior matches summary and test plan
- [x] No unrelated changes in diff
- [x] Public API / CLI changes documented
- [x] New rules have fixture coverage
- [x] PR has assignee and labels
- [x] Related issues use correct Closes/Relates keywords
- [x] No secrets or generated artifacts committed
- [x] Diff-stat-by-extension table pasted at the bottom

---

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 1 | 406 | 0 |
| `.md` | 5 | 255 | 10 |
| `.pdf` | 2 | Binary | Binary |
| `.tsv` | 2 | 27 | 0 |
| `.txt` | 2 | 2 | 0 |
| **Total** | **12** | **690** | **10** |
