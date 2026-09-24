# v0.0.1 - Public API and CLI

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented, lint and test passed on 2026-09-24
> **Estimated effort:** 2 days

---

## Overview

Tag 0.0.1 code. Export the API from `documentation/public-api.md`, implement `New`, `Close`, `Version`, and `CompareFiles`, and stub every interpreter method with `ErrNotImplemented`. The CLI maps those results to the exit codes in `documentation/cli.md`.

Tests live in `api_test.go` as `package spectreps_test`, plus `cmd/spectreps` tests that run the binary.

## Executive summary

This phase is what makes a later library possible. The interpreter is still absent. File byte compare is real, because it has no parser and it is the compare job Ghostscript leaves to an outside tool.

## Phase 2: Public API and CLI

### 2.1 Instance

- [x] `instance.go` implements `New` and `Close` as specified in `documentation/public-api.md`. Two `Close` calls on one instance both return nil. Proof: `go test -count=1 -run TestNewClose .` exited 0 on 2026-09-24.

### 2.2 Errors

- [x] `errors.go` defines `ErrNotImplemented` and `JobError` with the one-line text form from `documentation/public-api.md`. Proof: `go test -count=1 -run TestJobErrorText .` exited 0 on 2026-09-24.

### 2.3 Stubs

- [x] `RunPostScript`, `OpenPDF`, `RasterizePage`, and `RewritePDF` return `ErrNotImplemented`. `CompareRaster` panics with `ErrNotImplemented` because its signature returns `CompareResult`. A nil context panics with `spectreps: nil context`. A cancelled context returns `ctx.Err()`. Proof: `go test -count=1 -run TestNotImplemented .` exited 0 on 2026-09-24.

### 2.4 File byte compare

- [x] `CompareFiles` follows the equal, mismatch, prefix, and empty rules in `documentation/public-api.md`. Proof: `go test -count=1 -run TestCompareFiles .` exited 0 on 2026-09-24.

### 2.5 CLI

- [x] `cmd/spectreps/main.go` implements the commands in `documentation/cli.md`. `version` exits 0 and prints the `Version` constant. A missing file or unknown flag exits 2. `compare bytes` exits 0 for equal files and 1 for a mismatch, printing `mismatch byte N` on stdout. `run` on a readable file exits 1 and prints `spectreps: not implemented` on stderr. `raster` without `-o` exits 2. Proof: `go test -count=1 ./cmd/spectreps` exited 0 on 2026-09-24.

### 2.6 Import boundary

- [x] `cmd/spectreps` imports only `github.com/chinmay-sawant/spectrePS`. No `.go` file imports `os/exec` or uses cgo. Proof: `rg -n 'os/exec|import "C"' --glob '*.go'` printed no lines on 2026-09-24.

### 2.7 Closure

- [x] `make lint` passes. Outcome on 2026-09-24: exit 0. `gofmt -l .` printed nothing. `go vet ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24: exit 0. Transcript is in `plans/v0.0.1/09-release-records.md`.

## Dependencies

Phase 01. No `internal/` package in this phase.
