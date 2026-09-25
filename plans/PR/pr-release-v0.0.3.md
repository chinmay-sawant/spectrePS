## Summary

Record the v0.0.3 release note and the release plumbing. `spectreps version` prints `0.0.3`. This change does not create a git tag.

---

## Motivation / context

- Plans: `plans/v0.0.3/00-program.md`
- Release note: `plans/v0.0.3/PR/release-v0.0.3.md`
- Issues: see **Related issues**

---

## Changes

### Release note

- `plans/v0.0.3/PR/release-v0.0.3.md` is the v0.0.3 release note. It lists the ten phases, the merged pull request, the build, the new commands and API, the measured sizes, the veraPDF verdicts, and the open rows.

### Version bump

- `spectreps.Version()` returns `0.0.3`, and the version tests and docs follow. `spectreps version` prints `0.0.3`.

### Docs alignment and samples

- `README.md`, `documentation/cli.md`, `documentation/features.md`, `documentation/test.md`, `documentation/public-api.md`, `documentation/folder-structure.md`, `documentation/development.md`, `documentation/copyright-and-rewrite.md`, and `documentation/gs-argv-mapping.md` are aligned with the v0.0.3 surface: the command table, the `Do` sentence, the `raster -format` flag, the `gs` mode sentences, and the dependency and ledger rows.
- `sampledata/fixtures/text.pdf` is a checked-in two-line Helvetica PDF for `spectreps text`, recorded in `sampledata/fixtures/README.md`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None. |
| **Memory** | None. |
| **Behavior / correctness** | `spectreps version` prints `0.0.3` instead of `0.0.2`. |
| **API / CLI** | `Version()` returns `0.0.3`. |
| **Dependencies** | None in this change. The `go-jpeg2000` requirement landed earlier in the release. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Version string | Scripts that compare `spectreps version` to `0.0.2` should accept `0.0.3`. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make build`

### Commands

```sh
make lint
make test
go test -count=1 -p 24 ./...
make build
./bin/spectreps version
./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
```

Outcomes on 2026-09-25 on `chore/release-0.0.3`: `make lint` exit 0, `go test -count=1 -p 24 ./...` exit 0 for every package, `make build` wrote `bin/spectreps`, `version` printed `0.0.3`, and `text` printed `Hello` and `World` with CRLF line endings.

---

## Screenshots / sample output

```
$ ./bin/spectreps version
0.0.3
$ ./bin/spectreps text -pages 1 sampledata/fixtures/text.pdf
Hello
World
```

---

## Related issues

No GitHub issue exists for this note. The ledger file is `plans/v0.0.3/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-release-v0.0.3.md` when process-gated

---

## Follow-ups (out of scope)

- Do not push a `v0.0.3` git tag or create the GitHub release until the release pull request is merged; the merge commit is the tag target.
- Deferred rows stay in `plans/v0.0.1/10-deferred.md`.

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

Generated with `bash scripts/pr-diff-stat.sh master` on the branch before this PR note was committed.

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 3 | 4 | 4 |
| `.md` | 12 | 285 | 23 |
| `.pdf` | 1 | Binary | Binary |
| **Total** | **16** | **289** | **27** |
