## Summary

Tag 0.0.5 makes `spectreps validate` stop on the first error. A subset PostScript program or a phase 06 PDF exits 0. A `stackunderflow`, a bad xref, an encrypted file, or `deletefile` exits 1.

---

## Motivation / context

- Plans: `plans/v0.0.1/08-validate.md`
- Issues: see **Related issues**

---

## Changes

### Command

- PostScript input runs through `RunPostScript`. The first `JobError` exits 1 and the error line goes to stderr.
- `add` with an empty stack reports `Error: /stackunderflow in add`. The operator name is the one the program called.
- PDF input opens the file and rasterizes every page. The first open or paint error stops the command. A truncated xref and an encrypted file fail. `Tj` fails the page.
- `validate` writes no output file and no PDF/A metadata.

### Banned file operator

- `deletefile` still returns `invalidaccess` and does not read the path.
- A file created next to the input is still there after the command.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. Validate rasterizes each PDF page and discards the pixmap. |
| **Memory** | One page pixmap at the default 612 by 792, 72 dpi. |
| **Behavior / correctness** | Validate exits 0 only when the subset runs or the PDF pages paint. |
| **API / CLI** | `Document.PageCount` reports the page leaves. `validate` exit codes are locked. |
| **Dependencies** | No third-party Go modules. |
| **Binary size / build time** | `make build` still produces `bin/spectreps`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | Existing job signatures are unchanged. `PageCount` is a new method. |

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
go test -count=1 ./internal/cli -run TestValidatePS
go test -count=1 ./internal/cli -run TestValidatePDF
go test -count=1 ./internal/cli -run TestValidateBanned
```

Each proof command exited 0 on 2026-09-25. `make build` was run because the command behavior changed.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/cli
ok  github.com/chinmay-sawant/spectrePS/internal/ps
ok  github.com/chinmay-sawant/spectrePS/spectreps
```

---

## Related issues

No GitHub issue exists for this tag. The ledger file is `plans/v0.0.1/08-validate.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-validate.md` when process-gated

---

## Follow-ups (out of scope)

- Rows in `plans/v0.0.1/10-deferred.md` stay deferred. They need a new plan file before any of that work starts.
- Source positions on `JobError` are still omitted. The scanner does not record a line and column.

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
| `.go` | 4 | 132 | 3 |
| `.md` | 5 | 42 | 12 |
| **Total** | **9** | **174** | **15** |
