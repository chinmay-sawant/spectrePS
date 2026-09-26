# v0.0.4 - Fonts, tags, and PDF coverage

> **Parent:** `plans/v0.0.3/00-program.md` - the previous tag
> **Status:** implemented. Phases 1 to 6 landed. `tiffsep`, bare CFF, and performance rows 5.4 and 5.6 are deferred to `plans/v0.0.1/10-deferred.md` 10.1, 10.2, and 10.5. `make lint` and `make test` pass on the merged tree.
> **Estimated effort:** days for phase 1, weeks for phases 2 to 4, about two weeks for phase 5, about a week for phase 6.

---

## Overview

v0.0.3 is released. This folder takes the open items the v0.0.3 release note recorded: the writer size ceiling, Type 1 fonts, PDF/UA-2 tag generation, and the remaining PDF coverage. The printer languages stay explicitly deferred.

The order follows dependency and risk. The writer ceiling closes the only open v0.0.3 row first. Type 1 fonts extend the font machine that already reads metrics and extracts text. Tag generation needs the text machine and the structure parse, both complete. PDF coverage is the largest phase and starts with the content graphics gate that transparency and separations need.

## Executive summary

Phase 1 accepts the level 1 and 2 output sizes as a 610,034-byte ceiling and adds the assertion. Phase 2 reads Type 1 `/FontFile` programs, interprets charstrings, composes `seac`, and paints through the existing coverage path, with bare CFF gated last. Phase 3 fixes marked-content reading, which is broken today, then builds the tag recorder, structure authoring, the parent tree, the claim, and reading order. Phase 4 adds clip, Form XObjects, `/ExtGState`, transparency, separations, font embedding and subsetting, filters, and optional content. Phase 5 proves the landed v0.0.3 surface with code-based tests and a checked-in corpus under `sampledata/validation/`, and it does not depend on phases 1 to 4. Phase 6 builds the benchmark harness, profiles the CLI application and the library, and lands only the fixes the profiles confirm.

## Phase index

| File | Order | Job |
| --- | --- | --- |
| `1-writer-ceiling.md` | 1 | Accept the levels 1 and 2 output size as a recorded ceiling. |
| `2-type1-fonts.md` | 2 | Type 1 charstrings and `seac`, with bare CFF gated. |
| `3-pdfua2-tags.md` | 3 | Marked content reading and PDF/UA-2 tag generation. |
| `4-pdf-coverage.md` | 4 | Content graphics, transparency, separations, font embedding, filters, optional content, and a read-only `spectreps info`. |
| `5-validation.md` | 5 | Prove each landed v0.0.3 feature with code tests and a checked-in corpus. |
| `6-performance-profiling.md` | 6 | Benchmark and profile the CLI and the library, then fix what the profiles confirm. |
| `v0.0.4-closure.md` | gate | `make lint` and `make test` transcripts for the merged tree. |

## What is not in v0.0.4

| Item | Why |
| --- | --- |
| PCL, PXL, XPS, and the printer device list | Out of product. Explicitly deferred on 2026-09-25 in `plans/v0.0.1/10-deferred.md` 10.3. |
| Encryption | Needs key derivation, crypt filters, and decryption at every read. Its own plan. |
| Linearization, PCLm, and output encryption | Out of the current tags. |
| Full color management | `/Alternate` only; a pure-Go profile transform is its own decision. |
| JPEG2000 encoding, font hinting, OCR | Not needed for the planned jobs. |

## Dependencies

Phase 1 depends on the v0.0.3 writer and `TestRewriteSamples`. Phase 2 depends on the v0.0.3 font and text machine. Phase 3 depends on the v0.0.3 structure parse, preflight, and metadata, and uses the local gowkhtmltopdf implementation as a design reference only; no code is copied. Phase 4 depends on the v0.0.3 painter and writer, and its transparency and separation phases depend on its content graphics phase. Phase 5 depends on the landed v0.0.3 tree and its closure record, and on the case list in `documentation/test.md`; it does not depend on phases 1 to 4. Phase 6 depends on the corpus from phase 5 for accepted inputs, and it does not depend on phases 1 to 4. The closure waits for the phases it records and does not depend on any single one.
