# PDF version compatibility corpus

> **Status:** phase complete on the development tree. This file does not cut a release or change `spectreps version`.
> **Parent:** `plans/v0.0.5/00-program.md`.

This phase records a version-reporting correction and real inputs that show
where PDF opening, painting, and profile checks differ. It does not add a
general PDF conformance validator.

The PDF/A profile corpus is in `7-pdfa-profiles.md`.

## Phase 1: Corpus and version report

- [x] 1.1 Stored five pinned PDF Association and qpdf files under `sampledata/validation/compatibility/`. The manifest records source, license, SHA-256, byte count, and outcome. `TestValidationManifest` and `TestValidationLicenses` passed under `make test NPROC=4`.
- [x] 1.2 `info` reads the current catalog `/Version` when it raises the header version. `TestValidationPDFVersionCorpus` passed: all three PDF Association incremental-update files report 1.6 from a 1.4 header.
- [x] 1.3 `TestValidationCorpusPDF` passed over the five inputs. `make validation-run` reported 71 pass, 0 fail, 0 skipped, including the named graphics-state, Type 3, and encryption refusals.
- [x] 1.4 `documentation/pdf-compatibility.md`, `documentation/cli.md`, and `documentation/public-api.md` distinguish a version report, painting, the PDF/A-4 and PDF/UA-2 preflights, and certification.

## Closure

- [x] `make lint`. Outcome on 2026-09-26: exit 0; golangci-lint passed and size-check found 0 over-limit files.
- [x] `make test NPROC=4`. Outcome on 2026-09-26: exit 0; the CLI, PDF, validation, and other tested packages passed.
- [x] `make validation-run`. Outcome on 2026-09-26: 71 pass, 0 fail, 0 skipped.
- [x] `make pdfa-check` and `make pdfua2-check` with veraPDF 1.30.2. Outcome on 2026-09-26: exit 0 for both. PDF/A reported 2 compliant base samples and 1 compliant 4f sample. PDF/UA-2 reported 7 compliant samples and 0 noncompliant; veraPDF also printed two ToUnicode CMap parser warnings. These are sample profile checks, not general PDF certification.
