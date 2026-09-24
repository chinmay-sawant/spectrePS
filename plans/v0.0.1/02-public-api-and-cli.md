# v0.0.1 - Public API and CLI

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Public library is `spectreps/`. Private code is `internal/`. Lint and test passed on 2026-09-24.
> **Estimated effort:** 2 days

---

## Overview

Tag 0.0.1 code. Export the API from `documentation/public-api.md`, implement `New`, `Close`, `Version`, and `CompareFiles`, and stub every interpreter method with `ErrNotImplemented`. The CLI maps those results to the exit codes in `documentation/cli.md`.

Tests live in `spectreps/api_test.go` as `package spectreps_test`. Command tests live in `internal/cli` and call `Run`. `cmd/spectreps/main.go` only starts that function.

## Executive summary

This phase is what makes a later library possible. The interpreter is still absent. File byte compare is real, because it has no parser and it is the compare job Ghostscript leaves to an outside tool.

## Phase 2: Public API and CLI

### 2.1 Instance

- [x] `spectreps/instance.go` implements `New` and `Close` as specified in `documentation/public-api.md`. The session state lives in `internal/engine`. Two `Close` calls on one instance both return nil. Proof: `go test -count=1 -run TestNewClose ./spectreps` exited 0 on 2026-09-24.

### 2.2 Errors

- [x] `spectreps/errors.go` defines `ErrNotImplemented` and `JobError` with the one-line text form from `documentation/public-api.md`. The sentinel value is owned by `internal/engine` and re-exported. Proof: `go test -count=1 -run TestJobErrorText ./spectreps` exited 0 on 2026-09-24.

### 2.3 Stubs

- [x] `RunPostScript`, `OpenPDF`, `RasterizePage`, and `RewritePDF` return `ErrNotImplemented`. `CompareRaster` panics with `ErrNotImplemented` because its signature returns `CompareResult`. A nil context panics with `spectreps: nil context`. A cancelled context returns `ctx.Err()`. Proof: `go test -count=1 -run TestNotImplemented ./spectreps` exited 0 on 2026-09-24.

### 2.4 File byte compare

- [x] `CompareFiles` follows the equal, mismatch, prefix, and empty rules in `documentation/public-api.md`. The compare lives in `internal/engine`. Proof: `go test -count=1 -run TestCompareFiles ./spectreps` exited 0 on 2026-09-24.

### 2.5 CLI

- [x] `internal/cli` implements the commands in `documentation/cli.md`. `cmd/spectreps/main.go` calls `cli.Run`. `version` exits 0 and prints the `Version` constant. A missing file or unknown flag exits 2. `compare bytes` exits 0 for equal files and 1 for a mismatch, printing `mismatch byte N` on stdout. `run` on a readable file exits 1 and prints `spectreps: not implemented` on stderr. `raster` without `-o` exits 2. Proof: `go test -count=1 ./internal/cli` exited 0 on 2026-09-24.

### 2.6 Import boundary

- [x] `cmd/spectreps` imports `internal/cli` and `os`. `internal/cli` imports `github.com/chinmay-sawant/spectrePS/spectreps` and does not import `internal/engine`. Package `spectreps` imports `internal/engine`. No `.go` file imports `os/exec` or uses cgo. Proof on 2026-09-24: `rg -n 'os/exec|import "C"' --glob '*.go'` printed no lines. `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./cmd/spectreps ./internal/cli ./spectreps` printed `cmd/spectreps` importing `internal/cli` and `os`, `internal/cli` importing `spectreps`, and `spectreps` importing `internal/engine`.

### 2.7 Closure

- [x] `make lint` passes. Outcome on 2026-09-24, after `golangci-lint run ./...` replaced `go vet` in the target: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24, after that move: exit 0. Both transcripts are in `plans/v0.0.1/09-release-records.md`.

## Dependencies

Phase 01. `internal/engine` holds the session and byte compare. `internal/cli` holds flags and exit codes. `spectreps/` is the public library.
