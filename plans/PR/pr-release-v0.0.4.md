## Summary

Record the v0.0.4 release note and bump the reported version to `0.0.4`. This change does not create a git tag.

---

## Motivation / context

- Plans: `plans/v0.0.4/00-program.md`
- Release note: `plans/v0.0.4/PR/release-v0.0.4.md`
- Issues: see **Related issues**

---

## Changes

### Release note

- `plans/v0.0.4/PR/release-v0.0.4.md` is the v0.0.4 release note, in the shape of `plans/v0.0.3/PR/release-v0.0.3.md`. It covers the seven ledger files, the install and library examples, the diff against v0.0.3, the build and library examples, every landed feature with its measured numbers, the five documentation corrections, the open rows, and the known limitations.

### Version bump

- `spectreps.Version()` returns `0.0.4`. The PostScript `version` operator mirrors it through `internal/ps/op_info.go`, as that file's comment requires. The two tests that pin the string follow.

### Docs alignment

- `README.md`, `documentation/README.md`, `documentation/features.md`, `documentation/cli.md`, `documentation/test.md`, `documentation/development.md`, and `documentation/public-api.md` move from "v0.0.4 is merged on master and is not tagged" to "the latest tag is v0.0.4". `README.md` gains the `plans/v0.0.4/` ledger and release-note links, and the v0.0.4 sentence in the release history.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None in this change. The v0.0.4 baseline and the four landed fixes are in `documentation/performance.md`. |
| **Memory** | None. |
| **Behavior / correctness** | `spectreps version` prints `0.0.4` instead of `0.0.3`, and the PostScript `version` operator returns `0.0.4`. |
| **API / CLI** | `Version()` returns `0.0.4`. No other signature or default changes in this PR. |
| **Dependencies** | None. The v0.0.4 phases added no module requirement. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Version string | Scripts that compare `spectreps version` to `0.0.3` should accept `0.0.4`. |
| Unkeyed `RewriteOptions` literals | The v0.0.4 feature phases added `SubsetFonts`, `Tag`, `Claim`, `Title`, and `Lang`. An unkeyed literal must add all five. Keyed literals compile unchanged. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make build`

### Commands

```sh
make lint
make test
go test -count=1 -p 2 ./...
make build
./bin/spectreps version
./bin/spectreps info sampledata/compress/path.pdf
./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
bash scripts/validation-run.sh
bash scripts/check-traceability.sh
bash scripts/check-file-size.sh
```

Outcomes on 2026-09-26 on `feature/004-fonts-tags-and-coverage` with the version bump applied: `make lint` exit 0, with `gofmt -l .` printing nothing, `golangci-lint run ./...` exit 0, and `size-check: clean (0 over-limit files)`. `go test -count=1 -p 4 ./...` exit 0, 11 packages ok and 3 with no test files. `make build` wrote `bin/spectreps`. `version` printed `0.0.4`. `info` printed the seven key/value lines above. `validation-run.sh` reported 62 pass, 0 fail, 2 skipped, the 2 being the external tier. `check-traceability.sh` reported 245 cases, 219 live, 0 pending, 26 proof, 0 unowned, 0 failing.

---

## Screenshots / sample output

```
$ ./bin/spectreps version
0.0.4
$ ./bin/spectreps info sampledata/compress/path.pdf
PDF version: 1.4
Pages: 1
Page 1: 200 x 200
Tagged: false
Fonts: none
Images: 0
```

---

## Related issues

No GitHub issue exists for this note. The ledger file is `plans/v0.0.4/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-release-v0.0.4.md` when process-gated

---

## Follow-ups (out of scope)

- Do not push a `v0.0.4` git tag or create the GitHub release until this pull request is merged; the merge commit is the tag target.
- The six `[~]` rows stay in their phase files and point at `plans/v0.0.1/10-deferred.md`: bare CFF and standard 14 outlines in 10.1, `tiffsep` plates and encryption in 10.2, the printer languages in 10.3, and two performance follow-ups in 10.5.
- The doc drift listed under "Known limitations" in the release note is not fixed here. It needs its own ledger rows.

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

Generated with `scripts/pr-diff-stat.sh`, with its one diff line pointed at the staged tree (`git diff --cached master --numstat`) so the table covers the whole pull request. Run unmodified, the script compares `master...HEAD` and reports 291 files, 36,290 insertions, 991 deletions, which leaves out this note, the release note, and the version bump.

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 174 | 31782 | 811 |
| `.md` | 33 | 2902 | 158 |
| `.pdf` | 52 | Binary | Binary |
| `.ppm` | 1 | Binary | Binary |
| `.ps` | 14 | 757 | 0 |
| `.py` | 1 | 1 | 1 |
| `.sh` | 5 | 613 | 9 |
| `.tsv` | 2 | 319 | 0 |
| `.txt` | 9 | 313 | 0 |
| No extension | 3 | 78 | 23 |
| **Total** | **294** | **36765** | **1002** |
