## Summary

Tag 0.0.2 starts with a PostScript subset under `internal/ps`. Programs can scan, build procedures, and run the stack, math, dictionary, and control operators from `documentation/language.md`. `RunPostScript` still returns `ErrNotImplemented` until a pixmap device exists.

---

## Motivation / context

- Plans: `plans/v0.0.1/03-postscript-subset.md`
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
- Path and matrix operators record device-space points through a fake device. `currentpoint` stays in user space.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. The interpreter is new and unbenchmarked. |
| **Memory** | Operand stack cap is 8192 objects. Dictionaries grow with `def`. |
| **Behavior / correctness** | `internal/ps` runs the subset. The public `RunPostScript` method is unchanged. |
| **API / CLI** | No new exported signature. The command still reports `spectreps: not implemented` for `run`. |
| **Dependencies** | No third-party Go modules. |
| **Binary size / build time** | `cmd/spectreps` does not import `internal/ps`, so the binary does not grow from this package yet. |

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
```

Each proof command exited 0 on 2026-09-24. `make build` was not required. This change does not edit `cmd/spectreps`.

---

## Screenshots / sample output

```
ok  github.com/chinmay-sawant/spectrePS/internal/ps
```

---

## Related issues

No GitHub issue exists for this phase. The ledger is `plans/v0.0.1/03-postscript-subset.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-postscript-subset.md` when process-gated

---

## Follow-ups (out of scope)

- Phase 04 binds the same path operators to the pixmap. `RunPostScript` stays `ErrNotImplemented` until that device exists.

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
| `.go` | 17 | 4706 | 0 |
| `.md` | 3 | 136 | 11 |
| **Total** | 20 | 4842 | 11 |
