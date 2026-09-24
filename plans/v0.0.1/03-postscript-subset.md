# v0.0.2 - PostScript subset

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Lint and test passed on 2026-09-24. `RunPostScript` still returns `ErrNotImplemented`.
> **Estimated effort:** 1 to 2 weeks

---

## Overview

Implement `documentation/language.md` under `internal/ps/`. `RunPostScript` still returns `ErrNotImplemented` until phase 04 connects a device. This phase proves the stacks with a test device that records operator calls, or with operand-stack dumps where no paint is required.

Package `spectreps` is the only caller of `internal/ps` from outside that tree. `internal/cli` stays on the public API.

## Executive summary

The reader scans `{ ... }` into an executable array and does not run it. Names are looked up when they execute. Banned file operators exist and return `invalidaccess`. Those three rules are the ones a PostScript subset usually gets wrong, so each has its own proof.

## Phase 3: PostScript subset

### 3.1 Scanner

- [x] `internal/ps` scans integers, reals, executable names, literal names, comments, parenthesis strings with the escapes listed in `documentation/language.md`, and hex strings. An int token outside int32 is `rangecheck` at scan time. Proof: `go test -count=1 ./internal/ps -run TestScan` exited 0 on 2026-09-24.

### 3.2 Procedures

- [x] The reader builds nested executable arrays for `{ { 1 2 add } }`. The outer array has one element, and that element is an executable array. Braces that do not match return `syntaxerror`. Proof: `go test -count=1 ./internal/ps -run TestProcedure` exited 0 on 2026-09-24.

### 3.3 Execution rule

- [x] Top-level `1 2 add` leaves 3. `{ 1 2 add }` leaves a procedure. `{ 1 2 add } exec` leaves 3. `{ { 1 2 add } } exec` leaves the inner procedure and does not run `add`. Proof: `go test -count=1 ./internal/ps -run TestExecRule` exited 0 on 2026-09-24.

### 3.4 Late lookup

- [x] A procedure built while `/test` has one value, then redefined, runs the new value. Lookup is not frozen at scan time. Proof: `go test -count=1 ./internal/ps -run TestLateLookup` exited 0 on 2026-09-24.

### 3.5 Stack, math, dict, control

- [x] Stack, math, compare, array, dictionary, and control operators from `documentation/language.md` match the error names in that file. `div` pushes a real. Division by zero is `undefinedresult`. Integer overflow is `rangecheck`. `copy` is the count form only. Proof: `go test -count=1 ./internal/ps -run 'TestStack|TestMath|TestDict|TestControl'` exited 0 on 2026-09-24.

### 3.6 Banned operators

- [x] `file`, `run`, `deletefile`, `renamefile`, `filenameforall` are defined and return `invalidaccess`. An unknown name such as `show` returns `undefined`. Proof: `go test -count=1 ./internal/ps -run TestBanned` exited 0 on 2026-09-24.

### 3.7 Limits

- [x] Operand stack 8192, execution stack 500, dictionary stack 20, and procedure nesting 128 return `limitcheck` or `stackoverflow` as named in the language file. A cancelled context returns `ctx.Err()`. Proof: `go test -count=1 ./internal/ps -run TestLimits` exited 0 on 2026-09-24.

### 3.8 Closure

- [x] `make lint` passes. Outcome on 2026-09-24: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24: exit 0. `go test ./...` passed `internal/cli`, `internal/ps`, and `spectreps`. `internal/ps` took 0.009s.

## Dependencies

Phase 02, for `JobError` and the public method that will eventually call this package. Graphics operators may return `undefined` until phase 04 defines them, or they may record into a fake device in this phase. The language file lists them. If they land here, phase 04 only binds them to the pixmap. Do not implement both a fake and a pixmap with different coordinates. Pick the fake device in this phase, and move the same calls onto the pixmap in phase 04.
