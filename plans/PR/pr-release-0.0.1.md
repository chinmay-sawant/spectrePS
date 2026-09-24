## Summary

Record the v0.0.1 release note. That is the first product release. The ledger phases 0.0.1 through 0.0.5 all ship in it. `spectreps version` prints `0.0.1`. This change does not create a git tag.

---

## Motivation / context

- Plans: `plans/v0.0.1/00-program.md`
- Release note: `plans/v0.0.1/PR/release-v0.0.1.md`
- Issues: see **Related issues**

---

## Changes

### Release note

- `plans/v0.0.1/PR/release-v0.0.1.md` is the v0.0.1 release note. It lists the five ledger phases, the merged pull requests, the build, and the deferred rows.
- There is no previous product release. Ledger numbers 0.0.2 through 0.0.5 are build steps inside v0.0.1.

### README

- The README no longer says raster, PDF, and rewrite are later tags. Those jobs are on `master`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None. Documentation only. |
| **Memory** | None. |
| **Behavior / correctness** | None. The command and library are unchanged. |
| **API / CLI** | `spectreps version` still prints `0.0.1`. |
| **Dependencies** | None. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | No code change. |

---

## Test plan

- [x] Documentation-only change. `make test` and `make lint` are not required.

### Commands

```sh
git diff master -- README.md plans/v0.0.1/PR/release-v0.0.1.md
```

---

## Screenshots / sample output

```
plans/v0.0.1/PR/release-v0.0.1.md
```

---

## Related issues

No GitHub issue exists for this note. The ledger file is `plans/v0.0.1/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-release-0.0.1.md` when process-gated

---

## Follow-ups (out of scope)

- Do not push a `v0.0.1` git tag until asked. The GitHub Release body is the note in this pull request.
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
| `.md` | 3 | 271 | 1 |
| **Total** | **3** | **271** | **1** |
