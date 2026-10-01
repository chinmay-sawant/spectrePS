## Summary

Record the v0.0.5 release note, bump the reported version to `0.0.5`, align the documentation, and add the release-note skill. This change does not create a git tag; the tag is cut from the merge commit.

---

## Motivation / context

- Plans: `plans/v0.0.5/00-program.md`
- Release note: `plans/v0.0.5/PR/release-v0.0.5.md`
- Issues: see **Related issues**

---

## Changes

### Release note

- `plans/v0.0.5/PR/release-v0.0.5.md` is the v0.0.5 release note, in the shape of `plans/v0.0.4/PR/release-v0.0.4.md`. It covers the eight ledger files and the closure, the install, CLI, and library examples, the diff against v0.0.4, every landed workstream with its measured numbers, the batch2 recovery, the open and deferred rows, and the documentation drift found during the scan.

### Version bump

- `spectreps.Version()` returns `0.0.5`. The PostScript `version` operator mirrors it through `internal/ps/op_info.go`, as that file's comment requires. `spectreps/api_test.go` and `internal/cli/run_test.go` pin the new string.

### Docs alignment

- `README.md`, `documentation/README.md`, `documentation/features.md`, `documentation/cli.md`, and `documentation/test.md` name v0.0.5 as the latest tag. `README.md` and `documentation/features.md` gain the v0.0.5 sentence in the release history, the ledger list gains `plans/v0.0.5/`, and `plans/v0.0.5/00-program.md` records the release state.

### Release-note skill

- `skills/release-note/SKILL.md` records the process this note was produced with: ground the tag range, fan out one scan agent per workstream, consolidate in the previous note's shape, verify the counts, and leave the version bump to the release pull request.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None in this change. The v0.0.5 benchmark record and its first-reading caveat are in `documentation/benchmark.md`. |
| **Memory** | None. |
| **Behavior / correctness** | `spectreps version` prints `0.0.5` instead of `0.0.4`, and the PostScript `version` operator returns `0.0.5`. The behavior changes of the v0.0.5 window landed in pull requests #16 through #23. |
| **API / CLI** | `Version()` returns `0.0.5`. No other signature or default changes. |
| **Dependencies** | None. The v0.0.5 phases added no module requirement. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Version string | Scripts that compare `spectreps version` to `0.0.4` should accept `0.0.5`. |
| Encrypted PDFs with an empty user or owner password | They now open instead of returning `invalidaccess`. Non-empty passwords and public-key handlers still refuse. The change is in #20. |
| Manifest schema, 7 columns to 11 | Corpus consumers add `area`, `probe`, `basis`, and `pages`. The change is in #20. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make build`

### Commands

```sh
make lint
make test
make build
./bin/spectreps version
./bin/spectreps info sampledata/compress/path.pdf
./bin/spectreps info sampledata/validation/compatibility/versions/PDF-versions1.pdf
```

Outcomes on 2026-10-01 on `chore/release-v0.0.5` with the version bump applied: `make lint` exit 0, with `gofmt -l .` printing nothing, `golangci-lint run ./...` exit 0, and `size-check: clean (0 over-limit files)`. `make test` exit 0, every package ok, `internal/cli` in 24.497s, `internal/validation` in 5.447s, `internal/ps` in 3.437s, and `spectreps` in 1.982s. `make build` wrote `bin/spectreps`, and `version` printed `0.0.5`. `info` on the PDF Association incremental-update fixture printed `PDF version: 1.6` from its 1.4 header.

---

## Screenshots / sample output

```
$ ./bin/spectreps version
0.0.5
$ ./bin/spectreps info sampledata/compress/path.pdf
PDF version: 1.4
Pages: 1
Page 1: 200 x 200
Tagged: false
Fonts: none
Images: 0
$ ./bin/spectreps info sampledata/validation/compatibility/versions/PDF-versions1.pdf
PDF version: 1.6
Pages: 1
Page 1: 100 x 100
Tagged: false
Fonts: none
Images: 0
```

---

## Related issues

No GitHub issue exists for this note. The ledger file is `plans/v0.0.5/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-release-v0.0.5.md` when process-gated

---

## Follow-ups (out of scope)

- The `v0.0.5` git tag is cut from the merge commit after this pull request merges.
- The documentation drift listed under "Known limitations" in the release note is not fixed here.
- The 28 open corpus rows and deferred row 4.3 stay open; the note names each one.

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

Generated with `scripts/pr-diff-stat.sh`, with its one diff line pointed at the staged tree (`git diff --cached master --numstat`) so the table covers the whole pull request.

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 4 | 5 | 5 |
| `.md` | 9 | 503 | 11 |
| **Total** | **13** | **508** | **16** |
