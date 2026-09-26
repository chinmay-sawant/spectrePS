## Summary

The v0.0.4 release note named `413aa16` as the commit its tag points at. That was true when written and stopped being true when #14 merged the correction to that same note. This change removes the circular reference so the note is correct as published.

---

## Motivation / context

- Release note: `plans/v0.0.4/PR/release-v0.0.4.md`
- Follows: #13, #14
- Issues: see **Related issues**

---

## Changes

### Remove the self-reference

- The opening line no longer says the tag points at a named commit. It says the note covers `master` through `8be9947` plus the version bump in `b90e999`, and that the tag is cut from the master head carrying both release pull requests.
- `**Commit:**` becomes `**Commits:**` and names `8be9947` for the last phase commit and `b90e999` for the version bump, then says the tag is cut from the master head above them.
- The `Pull requests:` line gains #14 alongside #13 and #11.

### Why this is the last correction

Naming the tag's own commit from inside the file the tag ships cannot converge. Fixing the sha merges a new commit, which changes the sha, which makes the note wrong again. With the sha gone there is nothing left to go stale, so the note needs no further edit after the tag exists.

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
- [x] Verified the note names no commit that this change or the tag can invalidate

### Commands

```sh
grep -n '413aa16' plans/v0.0.4/PR/release-v0.0.4.md   # no hits
grep -n -e '8be9947' -e 'b90e999' plans/v0.0.4/PR/release-v0.0.4.md
```

---

## Screenshots / sample output

Documentation only. The corrected header lines read:

```
Fourth release of Spectre PS. `spectreps version` prints `0.0.4`. This note covers `master`
through `8be9947` plus the version bump in `b90e999`, and the tag is cut from the master head
that carries both release pull requests.

- **Commits:** `8be9947` is the last phase commit and `b90e999` is the version bump. The tag is
  cut from the master head above them.
```

---

## Related issues

No GitHub issue. Second correction to the note landed by #13, found while publishing the release.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/` when process-gated

---

## Follow-ups (out of scope)

- The `v0.0.4` tag and the GitHub release are cut from `master` once this merges.

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
| `.md` | 2 | 124 | 3 |
| **Total** | **2** | **124** | **3** |

Two files: this body at 121 insertions, and the release note at 3 insertions and 3 deletions.
