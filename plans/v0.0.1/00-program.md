# v0.0.1 - Spectre PS program

> **Parent:** `skills/phase-wise-checklist/SKILLS.md` - checklist rules
> **Status:** tags 0.0.1, 0.0.2, and 0.0.3 are checked. The next open phase is 07.
> **Estimated effort:** 0.0.1 is a few days of skeleton work. The full ledger through validate is many weeks.

---

## Overview

Spectre PS is a Ghostscript-class tool in Go. The CLI and a future library call package `spectreps`. This folder is the only active ledger. Contracts live in `documentation/`. This file does not repeat their checklists.

Tag cuts:

| Tag | Done when these phase files have no open `[ ]` rows |
| --- | --- |
| 0.0.1 | `01-repository-baseline.md`, `02-public-api-and-cli.md` |
| 0.0.2 | `03-postscript-subset.md`, `04-raster.md`, `05-compare-raster.md` |
| 0.0.3 | `06-pdf-open.md` |
| 0.0.4 | `07-rewrite.md` |
| 0.0.5 | `08-validate.md` |

`09-release-records.md` stores the `make lint` and `make test` transcripts for each tag. `10-deferred.md` is the only home for work this ledger will not do yet.

## Executive summary

Ghostscript 9.55.0 on this machine interprets PostScript and PDF, rasterizes through devices such as `png16m` and `ppmraw`, and rewrites through `pdfwrite` and `ps2write`. It does not ship a compare device. Byte compare and pixel compare are Spectre commands. Compression in the first rewrite tag is Flate on the new PDF's streams, which is the `pdfwrite` path, not a raster of the input.

The public API is fixed in `documentation/public-api.md` before the interpreter exists, so later phases fill methods instead of inventing a second entry point. `cmd/spectreps` imports `internal/cli` only. The command calls package `spectreps`.

v0.0.1 ships `Version`, `New`, `Close`, `CompareFiles`, and a CLI that returns `ErrNotImplemented` for every job that needs an interpreter. That is intentional. A PostScript interpreter is tag 0.0.2.

## Phase index

| File | Tag | Job |
| --- | --- | --- |
| `01-repository-baseline.md` | 0.0.1 | Module, make, docs, git remote. |
| `02-public-api-and-cli.md` | 0.0.1 | Exported API, CLI, file byte compare. |
| `03-postscript-subset.md` | 0.0.2 | Scanner, stacks, operators in `documentation/language.md`. |
| `04-raster.md` | 0.0.2 | RGB pixmap, y flip, PPM raw, PNG encode. |
| `05-compare-raster.md` | 0.0.2 | `CompareRaster` and `spectreps compare raster`. |
| `06-pdf-open.md` | 0.0.3 | PDF subset, Flate decode, shared path engine. |
| `07-rewrite.md` | 0.0.4 | New PDF, Flate encode, stable bytes. |
| `08-validate.md` | 0.0.5 | Stop on first error. |
| `09-release-records.md` | each tag | Lint and test transcripts. |
| `10-deferred.md` | later | PDF/A, ps2write, text, bbox, ink, images, printers. |

## Dependencies

Phase 02 depends on phase 01. Phase 03 depends on phase 02. Phase 04 depends on phase 03. Phase 05 depends on phase 04. Phase 06 depends on phase 04, because PDF paint uses the pixmap. Phase 07 depends on phase 06. Phase 08 depends on phases 03 and 06, because both front ends must return `JobError`. Deferred rows name the phase they wait on.

System Ghostscript is documentation input. No phase may shell out to it.
