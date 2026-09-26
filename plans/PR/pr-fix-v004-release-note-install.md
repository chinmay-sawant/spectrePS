## Summary

The v0.0.4 release note told a reader to check out `feature/004-fonts-tags-and-coverage`. Pull request #13 merged with `--delete-branch`, so that branch no longer exists and the first command in the note fails on a fresh clone. This change points the note at the tag it ships as.

---

## Motivation / context

- Release note: `plans/v0.0.4/PR/release-v0.0.4.md`
- Follows: #13
- Issues: see **Related issues**

---

## Changes

### Install instructions

- `git checkout feature/004-fonts-tags-and-coverage` becomes `git checkout v0.0.4`. The deleted branch is not a thing a reader can fetch.

### Header metadata

- The opening line now reads the way the v0.0.3 note reads: `master` through `8be9947` plus the version bump in `b90e999`, with the tag on the merge commit `413aa16`. It previously said "plus the version bump in this release branch" and named no commit.
- A `Commit:` line names `413aa16`, the merge of #13.
- A `Pull requests:` line names #13 for the seven `plans/v0.0.4/` ledger files and #11 for the ten `plans/v0.0.3/` phases that carried the baseline. The v0.0.3 note has both lines; this note had neither.

### Diff line

- `291 files, 36,290 insertions, 991 deletions` becomes `294 files, 36,765 insertions, 1,002 deletions`. The old numbers were the feature branch before the release commit, so they left out the release note and the version bump. The new numbers are what the merge carried, and they match the total in #13's body.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | None. |
| **Memory** | None. |
| **Behavior / correctness** | None. Documentation only. |
| **API / CLI** | None. |
| **Dependencies** | None. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | Documentation only. |

---

## Test plan

- [ ] `make test` (not run: documentation only, and AGENTS.md skips both gates for a docs-only change)
- [ ] `make lint` (not run: documentation only)
- [x] `make build` was already green on `413aa16`, and this change touches no Go file

### Commands

```sh
git diff --name-only 413aa16..HEAD   # one .md file
make build && ./bin/spectreps version # 0.0.4, unchanged by this PR
```

---

## Screenshots / sample output

Documentation only, so there is none. The corrected block reads:

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
git checkout v0.0.4
make build
```

---

## Related issues

No GitHub issue. This fixes a defect in the note landed by #13, caught while publishing the release.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/` when process-gated

---

## Follow-ups (out of scope)

- The tag `v0.0.4` and the GitHub release are created from `master` after this merges, so the tag points at the commit the note names.

---

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [x] No unrelated changes in diff
- [ ] Public API / CLI changes documented
- [ ] New rules have fixture coverage when applicable
- [ ] PR has assignee and labels
- [ ] Related issues use correct Closes/Relates keywords
- [x] No secrets or generated artifacts committed
- [ ] Diff-stat-by-extension table pasted at the bottom

---

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.md` | 2 | 127 | 4 |
| **Total** | **2** | **127** | **4** |

Two files: the release note at 6 insertions and 4 deletions, and this body at 121 insertions.
