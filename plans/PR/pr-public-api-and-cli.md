## Summary

Tag 0.0.1 now has a public Go library and a `spectreps` command. Callers import `github.com/chinmay-sawant/spectrePS/spectreps`. The command can print the version, compare file bytes, and reject jobs that still need an interpreter.

---

## Motivation / context

- Plans: `plans/v0.0.1/02-public-api-and-cli.md`
- Issues: see **Related issues**

---

## Changes

### Public library

- Package `spectreps` exports `Version`, `New`, `Close`, `CompareFiles`, `JobError`, and the job methods from `documentation/public-api.md`.
- `CompareFiles` implements the equal, mismatch, prefix, and empty rules.
- Interpreter methods return `ErrNotImplemented`. A cancelled context returns `ctx.Err()`. A nil context panics. `CompareRaster` panics with `ErrNotImplemented` because its signature returns `CompareResult`.

### Command and private packages

- `cmd/spectreps` only calls `internal/cli`.
- `internal/cli` parses flags and maps results to exit codes 0, 1, 2, and 3.
- `internal/engine` holds the session and the byte compare.
- `make lint` runs `gofmt` and `golangci-lint`.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | No measured change. The jobs are stubs or a byte loop. |
| **Memory** | One small session object per `New` call. |
| **Behavior / correctness** | `version` and `compare bytes` work. Other commands return `spectreps: not implemented` after usage checks. |
| **API / CLI** | New import path `github.com/chinmay-sawant/spectrePS/spectreps`. New binary `spectreps`. |
| **Dependencies** | No third-party Go modules. `golangci-lint` is a local tool. |
| **Binary size / build time** | `make build` produces `bin/spectreps`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | First library tag. No prior import path in a release. |

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
./bin/spectreps version
```

`make lint` and `make test` exited 0 on 2026-09-24. `./bin/spectreps version` printed `0.0.1`.

---

## Screenshots / sample output

```
$ ./bin/spectreps version
0.0.1
```

---

## Related issues

No GitHub issue exists for this tag. The ledger is `plans/v0.0.1/02-public-api-and-cli.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-public-api-and-cli.md` when process-gated

---

## Follow-ups (out of scope)

- Tag 0.0.2 starts at `plans/v0.0.1/03-postscript-subset.md`.

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
| `.go` | 14 | 1011 | 0 |
| `.md` | 19 | 270 | 63 |
| `.yml` | 1 | 61 | 0 |
| `(none)` | 1 | 2 | 2 |
| **Total** | 35 | 1344 | 65 |
