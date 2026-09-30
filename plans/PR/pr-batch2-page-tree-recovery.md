# fix(pdf): preserve damaged page tree occurrences

## Summary

Damaged PDF page trees now keep repeated `/Kids` references as separate page occurrences while stopping cycles. Recovery uses the catalog-selected tree when possible, keeping page counts, sizes, and content references aligned instead of counting unrelated page objects.

## Motivation / context

- Follow-up to the merged batch2 recovery work in PR #20.
- The selected tree reported 269 pages for GHOSTSCRIPT-695619-0.pdf and 12 for TIKA-3224-1.pdf, while Ghostscript rendered 299 and 24.

## Changes

### Damaged page trees

- Traverse shared `/Kids` references per occurrence with a visit cap and current-branch cycle check.
- Prefer the selected page tree after xref rebuilding. Skip missing references and resource streams, count recovered untyped children as blank pages, and cap results at the root `/Count`.
- Keep recovered page sizes and content references in the same order as the recovered page leaves.

### Regression coverage

- Add inline fixtures for a shared subtree with malformed children under readable and rebuilt xref tables.
- Check `PageCount`, `Info`, and `PageContentNums` together.
- The two batch2 PDFs now report 299 and 24 pages through Spectre `Info`, matching the Ghostscript PPM audit.

## Impact

| Area | Impact |
|---|---|
| Performance | Normal page trees do not allocate recovery metadata. Damaged tree walks stop after twice the xref entry count. |
| Memory | Recovery metadata is stored only when the page tree needs fallback handling. |
| Behavior / correctness | Damaged trees retain repeated page occurrences and ignore unrelated pages outside the selected tree. |
| API / CLI | No public API or CLI changes. |
| Dependencies | No dependency changes. |
| Binary size / build time | Not measured. |

## Breaking changes / migration

| Item | Migration |
|---|---|
| None | - |

## Test plan

- [x] `timeout 30s make test` (passed in 19.5 seconds)
- [ ] `make lint` (not run locally at the user's request; CI should run it)
- [x] Targeted PDF recovery and allocation checks passed.

### Commands

```sh
timeout 30s make test
```

## Screenshots / sample output

```text
GHOSTSCRIPT-695619-0.pdf: Spectre Info 299 pages; Ghostscript PPM 299 pages
TIKA-3224-1.pdf: Spectre Info 24 pages; Ghostscript PPM 24 pages
```

## Related issues

- Relates to #20, the merged batch2 recovery pull request.

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [x] Related work linked above
- [x] PR body saved under `plans/PR/`

## Follow-ups (out of scope)

- None.

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [x] No unrelated source changes in this diff
- [x] No public API or CLI changes
- [x] New recovery rule has inline fixture coverage
- [x] PR has assignee and labels
- [x] Related work points to PR #20

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 4 | 260 | 57 |
| `.md` | 1 | 92 | 0 |
| **Total** | **5** | **352** | **57** |
