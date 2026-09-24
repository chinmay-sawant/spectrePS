## Summary

Tag 0.0.3 opens a PDF subset and rasterizes its path operators. `OpenPDF` walks the page tree. `RasterizePage` paints one page through the same pixmap as PostScript. `RewritePDF` still returns `ErrNotImplemented`.

---

## Motivation / context

- Plans: `plans/v0.0.1/06-pdf-open.md`
- Issues: see **Related issues**

---

## Changes

### File open

- A header must contain `%PDF-`.
- Classic xref tables and xref streams both open. Object streams supply the objects a type 2 xref row names.
- Content streams use Flate through `compress/zlib`. An unknown filter returns `undefined`. A trailer `/Encrypt` returns `invalidaccess`.
- Page count is the number of page leaves. `/Count` is not the answer when the tree disagrees.

### Paint

- `m l c h re S s f f* n q Q w RG rg g G` paint through `internal/graphics`.
- Curves follow the PostScript subset: three straight segments, not a flattened Bezier.
- `Tj`, `TJ`, `'`, `"`, and `Do` return `undefined` with that operator name.

### Public methods and CLI

- `OpenPDF` and `RasterizePage` do this work. A bad page index is `rangecheck`.
- `spectreps raster -o out.ppm in.pdf` writes a P6 file for page 0.
- `spectreps rewrite` still exits 1 with `spectreps: not implemented` after a PDF opens.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. PDF open is new and unbenchmarked. |
| **Memory** | One Flate stream stops at 32 MiB decoded. |
| **Behavior / correctness** | Path PDFs rasterize. `Tj`, `Do`, encryption, and unknown filters fail the page. |
| **API / CLI** | `OpenPDF` and `RasterizePage` do real work. `RewritePDF` still returns `ErrNotImplemented`. |
| **Dependencies** | No third-party Go modules. Flate uses the standard library. |
| **Binary size / build time** | `make build` still produces `bin/spectreps`. The command now links the PDF reader. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | The public signatures are unchanged. |

---

## Test plan

- [x] `make test`
- [x] `make lint`
- [x] `make build` when the change adds or edits Go code under `cmd/spectreps`

### Commands

```sh
make lint
make test
make build
go test -count=1 ./internal/pdf -run TestClassicXref
go test -count=1 ./internal/pdf -run TestXrefStream
go test -count=1 ./spectreps -run TestPDFPathMatchesPS
go test -count=1 ./internal/pdf -run TestPDFReject
go test -count=1 -run 'TestRasterizePage|TestNotImplemented' ./spectreps
go test -count=1 ./internal/cli -run TestRasterPDF
```

Each proof command exited 0 on 2026-09-24. `cmd/spectreps/main.go` is unchanged. `make build` was run because the command now links `internal/pdf`.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/pdf
ok  github.com/chinmay-sawant/spectrePS/spectreps
ok  github.com/chinmay-sawant/spectrePS/internal/cli
```

---

## Related issues

No GitHub issue exists for this tag. The ledger file is `plans/v0.0.1/06-pdf-open.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-pdf-open.md` when process-gated

---

## Follow-ups (out of scope)

- Tag 0.0.4 writes a new PDF in `plans/v0.0.1/07-rewrite.md`.

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
| `.go` | 24 | 4702 | 18 |
| `.md` | 6 | 172 | 14 |
| **Total** | **30** | **4874** | **32** |
