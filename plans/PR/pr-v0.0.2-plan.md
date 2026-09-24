## Summary

Add the v0.0.2 ledger for the three shortest deferred jobs, and make `make test` run package tests with `-p`.

---

## Motivation / context

- Plans: `plans/v0.0.2/00-program.md`
- Issues: see **Related issues**

---

## Changes

### Plan

- `1-bbox-inkcov.md` measures the existing pixmap. The box is in points. Coverage is RGB occupancy, not CMYK.
- `2-jpeg-raster.md` adds `.jpg` and `.jpeg` through `image/jpeg`. TIFF stays deferred.
- `3-pdfimage.md` wraps each painted page in a 24-bit RGB PDF. `Do` and DCT stay deferred.
- Text, PDF/A, PostScript rewrite, `gs` flags, and printer languages stay in `plans/v0.0.1/10-deferred.md`.

### Tests

- `make test` runs `go test -p $(nproc) ./...`. `-p` is the number of test binaries at once. The default was already `GOMAXPROCS`. The Makefile now sets it from `nproc`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | Package tests can run together, capped by the CPU count. |
| **Memory** | Several test binaries may be live at once. |
| **Behavior / correctness** | No product behavior change. Bbox, JPEG, and bitmap PDF are planned, not implemented. |
| **API / CLI** | None yet. |
| **Dependencies** | None. |
| **Binary size / build time** | None. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | `make test` still runs the same tests. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [ ] `make build` not required. No Go code under `cmd/spectreps` changed.

### Commands

```sh
make lint
make test
```

Both exited 0 on 2026-09-25. `make test` printed `go test -p 24 ./...`.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/cli
ok  github.com/chinmay-sawant/spectrePS/spectreps
```

---

## Related issues

No GitHub issue exists for this plan. The parent ledger is `plans/v0.0.1/10-deferred.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-v0.0.2-plan.md` when process-gated

---

## Follow-ups (out of scope)

- Phases 1 to 3 are unchecked. Do not start them until asked.
- TIFF, text, PDF/A, `ps2write`, `gs` argv, and printer devices stay deferred.

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
| `.md` | 7 | 213 | 6 |
| No extension | 1 | 5 | 2 |
| **Total** | **8** | **218** | **8** |
