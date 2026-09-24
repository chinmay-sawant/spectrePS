# v0.0.5 - Validate

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** not started
> **Estimated effort:** 3 days

---

## Overview

`spectreps validate` is the stop-on-first-error command. It is the sanity check Ghostscript documents with `-dPDFSTOPONERROR`, exposed as its own subcommand so the exit code is the result.

It does not write a repaired file. It does not emit PDF/A metadata. It does not claim conformance.

## Executive summary

PostScript errors already exist as `JobError` after phase 03. PDF rejections exist after phase 06. This phase wires them to the CLI and locks the exit codes. Rendering commands are unchanged.

## Phase 8: Validate

### 8.1 PostScript

- [ ] `spectreps validate good.ps` exits 0 for a subset program. `spectreps validate bad.ps` exits 1 and prints the `JobError` line on stderr for a `stackunderflow`. Proof: `go test -count=1 ./internal/cli -run TestValidatePS`

### 8.2 PDF

- [ ] `spectreps validate good.pdf` exits 0 for a phase 06 fixture. A truncated xref exits 1 with `JobError`. An encrypted file exits 1 with `invalidaccess`. Proof: `go test -count=1 ./internal/cli -run TestValidatePDF`

### 8.3 Banned operator

- [ ] `spectreps validate` on a program that calls `deletefile` exits 1 with `invalidaccess`. The process does not delete a temp file created next to the input. Proof: `go test -count=1 ./internal/cli -run TestValidateBanned`

### 8.4 Closure

- [ ] `make lint` passes. Record the outcome here.
- [ ] `make test` passes. Record the outcome here.
- [ ] Tag 0.0.5 note added to `plans/v0.0.1/09-release-records.md`.

## Dependencies

Phases 03 and 06. Phase 07 is not required for validate, and validate must not start depending on rewrite output.
