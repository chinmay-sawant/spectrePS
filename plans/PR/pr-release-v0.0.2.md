## Summary

Record the v0.0.2 release note and the release plumbing. `spectreps version` prints `0.0.2`. This change does not create a git tag.

---

## Motivation / context

- Plans: `plans/v0.0.2/00-program.md`
- Release note: `plans/v0.0.2/PR/release-v0.0.2.md`
- Issues: see **Related issues**

---

## Changes

### Release note

- `plans/v0.0.2/PR/release-v0.0.2.md` is the v0.0.2 release note. It lists the five phases, the merged pull requests, the build, the compression level table, the measured sample sizes, and the deferred rows.
- The v0.0.3 plan folder was folded into `plans/v0.0.2/` as phases 4 and 5, because no tag existed between them.

### Version bump

- `spectreps.Version()` returns `0.0.2`, and the version tests and docs follow. `spectreps version` prints `0.0.2`.

### README and docs

- `README.md`, `documentation/cli.md`, `documentation/test.md`, `documentation/features.md`, and `documentation/public-api.md` state the new version and the current surface.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None. |
| **Memory** | None. |
| **Behavior / correctness** | `spectreps version` prints `0.0.2` instead of `0.0.1`. |
| **API / CLI** | `Version()` returns `0.0.2`. |
| **Dependencies** | None in this change. `golang.org/x/image` landed earlier in the release. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| Version string | Scripts that compare `spectreps version` to `0.0.1` should accept `0.0.2`. |

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
```

All exited 0 on 2026-09-25 on `chore/release-0.0.2`. `spectreps version` printed `0.0.2`.

---

## Screenshots / sample output

```
$ ./bin/spectreps version
0.0.2
```

---

## Related issues

No GitHub issue exists for this note. The ledger file is `plans/v0.0.2/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-release-v0.0.2.md` when process-gated

---

## Follow-ups (out of scope)

- Do not push a `v0.0.2` git tag or create the GitHub release until asked. The release body is the note in this pull request.
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

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 3 | 4 | 4 |
| `.md` | 7 | 348 | 6 |
| **Total** | **10** | **352** | **10** |
