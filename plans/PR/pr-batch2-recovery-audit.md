## Summary

This change adds regression coverage for three remaining batch2 recovery shapes and completes a full renderer audit. It replaces the `nullpage` count with a PPM render count over the same 664 files.

## Motivation / context

- Plans: batch2 validation work supplied in the issue description
- Issues: no issue ID was supplied

## Changes

### PDF recovery

- Scan inside a stream only when its direct `/Length` disagrees with the parsed stream span. Keep valid objects outside that span ahead of any nested candidates.
- Keep page sizes available when a rebuilt table or a page tree containing a stream cannot be walked.
- Add inline hex fixtures for a bad stream length, a broken second page-tree branch, a stream in `/Kids`, a malformed object body, and a valid stream containing object-like bytes.

### Batch2 audit

- Add `scripts/audit-batch2-render.py` to resume the 664-file audit in short batches. It records Info results, Spectre page-one raster results, and Ghostscript PPM page output. Each child process is capped at 25 seconds.
- The fresh audit found 581 of 664 files emitted at least one nonempty PPM page at 12 dpi. Ghostscript returned zero without output for 77 files, returned an error for 3, and timed out for 3.
- The current build passes Info on 612 files. Of those, 564 also emitted Ghostscript PPM pages. Seventeen Ghostscript-rendered files still fail Info: 12 with `syntaxerror in xref`, 4 with `undefined in pdf`, and 1 with `syntaxerror in FlateDecode`.
- The three named files now pass Info: GHOSTSCRIPT-701877-0 (1 page), GHOSTSCRIPT-695619-0 (269 pages), and TIKA-3224-1 (12 pages). Ghostscript emitted 1, 299, and 24 pages for those files respectively.

## Impact

| Area | Impact |
|------|--------|
| **Performance** | Stream scanning adds work only when a direct stream length is wrong. Page fallback runs only after a damaged page-tree walk fails. |
| **Memory** | The audit writes each render to a temporary directory and removes it after counting pages. |
| **Behavior / correctness** | The three named Info failures now open. The refreshed audit also identifies 17 other renderer-valid Info refusals for follow-up. |
| **API / CLI** | No public API or command changes. |
| **Dependencies** | None. |
| **Binary size / build time** | No dependency or command changes. |

## Breaking changes / migration

| Item | Migration |
|-----------|-----------|
| None | - |

## Test plan

- [x] `make test` (passed in 16.5 seconds)
- [ ] `make lint` (not run locally at the user's request; CI remains the lint gate)
- [x] `make build`

### Commands

```sh
make build
make test
python3 scripts/audit-batch2-render.py summary
```

## Screenshots / sample output

```text
Ghostscript emitted pages: 581
Info passed: 612; info passed and GS emitted pages: 564
```

## Related issues

- No issue ID was supplied.

## PR metadata checklist (author)

- [ ] Self-assigned (`--assignee @me`)
- [ ] Labels applied
- [x] Related issue status stated above
- [x] Filled body committed under `plans/PR/pr-batch2-recovery-audit.md`

## Follow-ups (out of scope)

- Review the 17 renderer-valid Info refusals found by the full audit.
- Local `make lint` was not run at the user's request. CI must pass before merge.

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [ ] No unrelated changes in diff
- [ ] Public API / CLI changes documented
- [x] New recovery rules have inline fixture coverage
- [ ] PR has assignee and labels
- [ ] Related issues use correct Closes/Relates keywords
- [ ] No secrets or generated artifacts committed
- [ ] Diff-stat-by-extension table pasted below

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
|-----------|-------|------------|-----------|
| pending generator output | | | |
| **Total** | | | |
