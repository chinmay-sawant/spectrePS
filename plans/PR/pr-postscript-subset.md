## Summary

Tag 0.0.2 runs the PostScript subset, paints pages, and compares those pixels. `RunPostScript` returns one `PageImage` per page. `spectreps raster` writes PPM or PNG. `spectreps compare raster` reports the first pixel mismatch.

---

## Motivation / context

- Plans: `plans/v0.0.1/03-postscript-subset.md`, `plans/v0.0.1/04-raster.md`, `plans/v0.0.1/05-compare-raster.md`
- Issues: see **Related issues**

---

## Changes

### Scanner and reader

- Integers, reals, names, comments, parenthesis strings, and hex strings scan as specified.
- An integer outside int32 is `rangecheck`. Unmatched braces are `syntaxerror`.
- `{ { 1 2 add } }` is one executable array whose only element is an executable array.

### Execution

- Top-level names run immediately. A procedure object is pushed. `exec`, `if`, `ifelse`, `repeat`, `for`, `loop`, and `forall` call procedures.
- A name inside a procedure is looked up when the procedure runs.
- `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` return `invalidaccess`. `show` stays `undefined`.
- Operand, execution, dictionary, and procedure-nesting caps return `stackoverflow` or `limitcheck`. A cancelled context returns `ctx.Err()`.
- Path and matrix operators record device-space points. `currentpoint` stays in user space.

### Raster and compare

- `internal/graphics` owns the default matrix, gsave cap, and the pixmap device. Row 0 is the top of the page.
- `RunPostScript` returns images for a subset program. `show` returns `JobError`. A page past the pixel cap returns `limitcheck` and allocates no pixmap.
- `spectreps raster` writes P6 PPM, or PNG when the path ends in `.png`. More than one page without `%d` exits 2.
- `CompareRaster` checks width, then height, then RGB bytes, and ignores stride padding.
- The first fixture is `testdata/line-bottom.ppm`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. The interpreter is new and unbenchmarked. |
| **Memory** | Operand stack cap is 8192 objects. Dictionaries grow with `def`. |
| **Behavior / correctness** | Subset programs paint. `run` on a subset program exits 0. `show` exits 1 with `JobError`. |
| **API / CLI** | `RunPostScript` and `CompareRaster` do real work. `OpenPDF`, `RasterizePage`, and `RewritePDF` still return `ErrNotImplemented`. |
| **Dependencies** | No third-party Go modules. |
| **Binary size / build time** | `make build` still produces `bin/spectreps`. The command now links the interpreter. |

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
go test -count=1 ./internal/ps -run TestScan
go test -count=1 ./internal/ps -run TestProcedure
go test -count=1 ./internal/ps -run TestExecRule
go test -count=1 ./internal/ps -run TestLateLookup
go test -count=1 ./internal/ps -run 'TestStack|TestMath|TestDict|TestControl'
go test -count=1 ./internal/ps -run TestBanned
go test -count=1 ./internal/ps -run TestLimits
go test -count=1 ./internal/graphics -run TestMatrix
go test -count=1 ./spectreps -run 'TestYFlip|TestPaint|TestPixelCap|TestRunPostScript|TestCompareRaster'
go test -count=1 ./internal/cli -run 'TestRasterFiles|TestCompareRasterCLI|TestCompareSameOptions'
```

Each proof command exited 0 on 2026-09-24. `make build` was not required for a new `cmd` file. `cmd/spectreps/main.go` is unchanged.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/ps
```

---

## Related issues

No GitHub issue exists for this tag. The ledger files are `plans/v0.0.1/03-postscript-subset.md`, `plans/v0.0.1/04-raster.md`, and `plans/v0.0.1/05-compare-raster.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-postscript-subset.md` when process-gated

---

## Follow-ups (out of scope)

- Tag 0.0.3 opens a small PDF in `plans/v0.0.1/06-pdf-open.md`.

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
|-----------|-------|------------|-----------|
| `.go` | 30 | 5908 | 98 |
| `.md` | 7 | 193 | 29 |
| `.ppm` | 1 | 4 | 0 |
| **Total** | 38 | 6105 | 127 |
