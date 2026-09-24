# v0.0.1 - Public API and CLI

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** not started
> **Estimated effort:** 2 days

---

## Overview

Tag 0.0.1 code. Export the API from `documentation/public-api.md`, implement `New`, `Close`, `Version`, and `CompareFiles`, and stub every interpreter method with `ErrNotImplemented`. The CLI maps those results to the exit codes in `documentation/cli.md`.

Tests live in `api_test.go` as `package spectreps_test`, plus `cmd/spectreps` tests that run the binary.

## Executive summary

This phase is what makes a later library possible. The interpreter is still absent. File byte compare is real, because it has no parser and it is the compare job Ghostscript leaves to an outside tool.

## Phase 2: Public API and CLI

### 2.1 Instance

- [ ] `instance.go` implements `New` and `Close` as specified in `documentation/public-api.md`. Two `Close` calls on one instance both return nil. Proof: `go test -count=1 -run TestNewClose .`

### 2.2 Errors

- [ ] `errors.go` defines `ErrNotImplemented` and `JobError` with the one-line text form from `documentation/public-api.md`. Proof: `go test -count=1 -run TestJobErrorText .`

### 2.3 Stubs

- [ ] `RunPostScript`, `OpenPDF`, `RasterizePage`, `RewritePDF`, and `CompareRaster` return `ErrNotImplemented`. Proof: `go test -count=1 -run TestNotImplemented .`

### 2.4 File byte compare

- [ ] `CompareFiles` follows the equal, mismatch, prefix, and empty rules in `documentation/public-api.md`. Proof: `go test -count=1 -run TestCompareFiles .`

### 2.5 CLI

- [ ] `cmd/spectreps/main.go` implements the commands in `documentation/cli.md`. `version` exits 0 and prints the `Version` constant. A missing file or unknown flag exits 2. `compare bytes` exits 0 for equal files and 1 for a mismatch, printing `mismatch byte N` on stdout. `run` on a readable file exits 1 and prints `spectreps: not implemented` on stderr. `raster` without `-o` exits 2. Proof: `go test -count=1 ./cmd/spectreps`

### 2.6 Import boundary

- [ ] `cmd/spectreps` imports only `github.com/chinmay-sawant/spectrePS`. No `.go` file imports `os/exec` or uses cgo. Proof: search the tree for `os/exec` and `import "C"` and record an empty result next to this row.

### 2.7 Closure

- [ ] `make lint` passes. Record the outcome here.
- [ ] `make test` passes. Record the outcome here.

## Dependencies

Phase 01. No `internal/` package in this phase.
