## Summary

Tag 0.0.4 writes a new PDF for a document this module can rasterize. The page content is the path subset from phase 06, and the default streams are Flate. Two rewrites of the same input return the same bytes.

---

## Motivation / context

- Plans: `plans/v0.0.1/07-rewrite.md`
- Issues: see **Related issues**

---

## Changes

### Writer

- `internal/pdfout` records device marks and emits `m`, `l`, `S`, `f`, and `f*`, plus color and line width.
- Curves stay the three straight segments phase 06 already paints.
- The file is a classic PDF 1.4. `CompressStreams` wraps each content stream with zlib and `/Filter /FlateDecode`. The zero option leaves the stream uncompressed.
- The trailer `/ID` is a SHA-256 of the stored stream bytes. The file has no `/Info`, `/CreationDate`, or `/ModDate`.

### Public method and CLI

- `RewritePDF` walks every page, emits operators, and returns the new PDF. A nil document is `rangecheck`.
- `spectreps rewrite -o out.pdf in.pdf` writes that file and exits 0. Missing `-o` exits 2. `-compress=false` selects the uncompressed option.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. Rewrite is new and unbenchmarked. |
| **Memory** | One decoded content stream still stops at 32 MiB. The writer holds the new file in memory. |
| **Behavior / correctness** | A path PDF rewrites to a PDF whose page 0 raster matches the input. `Tj` still fails the job. |
| **API / CLI** | `RewritePDF` does this work. `spectreps rewrite` writes the output file. |
| **Dependencies** | No third-party Go modules. Flate uses `compress/zlib`. |
| **Binary size / build time** | `make build` still produces `bin/spectreps`. The command now links the PDF writer. |

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
go test -count=1 ./spectreps -run TestRewritePixels
go test -count=1 ./internal/pdfout -run TestFlate
go test -count=1 ./spectreps -run TestRewriteStable
go test -count=1 ./internal/cli -run TestRewriteCLI
```

Each proof command exited 0 on 2026-09-25. `make build` was run because `internal/cli` now writes the rewrite output.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/pdfout
ok  github.com/chinmay-sawant/spectrePS/spectreps
ok  github.com/chinmay-sawant/spectrePS/internal/cli
```

---

## Related issues

No GitHub issue exists for this tag. The ledger file is `plans/v0.0.1/07-rewrite.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-pdf-rewrite.md` when process-gated

---

## Follow-ups (out of scope)

- Tag 0.0.5 stops on the first error in `plans/v0.0.1/08-validate.md`.
- Image downsample, DCT, CCITT, font subsetting, and PDF/A stay in `plans/v0.0.1/10-deferred.md`.

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
| `.go` | 17 | 1008 | 36 |
| `.md` | 5 | 43 | 13 |
| **Total** | **22** | **1051** | **49** |
