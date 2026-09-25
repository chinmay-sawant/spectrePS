# v0.0.3 - PDF/A-4 output

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** complete. 2026-09-25.
> **Estimated effort:** one phase, about two sessions. The ICC profile and the first clean validator run are the long pole.

---

## Overview

The deferred row asked for PDF/A creation. The latest archival part is PDF/A-4. `validate` today stops on the first interpreter error and reads no XMP, output intent, font, filter, or color space, so it is not a conformance check. The pass-through writer cannot append an object, change the header, or add metadata.

The maintainer also named "PDF override". In Ghostscript that is `PDFACompatibilityPolicy`: policy 0 keeps an incompatible feature and keeps the PDF/A claim, policy 1 drops the feature, and policy 2 aborts. Spectre mirrors policy 2 and refuses, because a claim with a known violation is the ambiguity this project already refuses to copy.

## Executive summary

PDF/A-4 base is the default claim, with 4f offered only when the input already carries embedded files. 4e stays deferred. A static XMP packet and a generated sRGB ICC output intent are appended, the header becomes `%PDF-2.0` with a binary marker, and a preflight refuses inputs that need font embedding, LZW, DeviceCMYK, or unsupported image keys. The CLI and docs say "profile preflight", never "compliant" or "certified". veraPDF gives the verdict and is a proof tool, not a dependency.

## Phase 1: Target and policy

### 1.1 Target decision

- [x] The phase row names PDF/A-4 base as the default and the refusal policy, with the claim wording. Proof: the row records the decision and the doc grep in 5.1 shows it. Decision 2026-09-25: `PDFA4` (`-pdfa 4`) is the default claim. `PDFA4F` (`-pdfa 4f`) is offered only when the input already carries `/Names /EmbeddedFiles`. PDF/A-4e and the earlier parts stay out of scope. The policy mirrors Ghostscript `PDFACompatibilityPolicy` 2: a known violation refuses the claim with `Error: /rule in PDFA`. The wording is "profile preflight", never compliant or certified, and the result is not a certificate. The 5.1 grep shows that wording in `documentation/cli.md`, `devices.md`, `covered-and-not-covered.md`, and `features.md`.

## Phase 2: Writer and metadata

### 2.1 Header and marker

- [x] Both writers emit `%PDF-2.0` with a binary marker above byte 127 when the PDF/A option is set, keeping the `/ID` and no `/Encrypt`. Proof: `go test -count=1 ./internal/pdfout -run TestPDFA4Header`. 2026-09-25: pass.

### 2.2 XMP packet

- [x] A static UTF-8 XMP packet with `pdfaid:part=4`, `pdfaid:rev=2020`, and the `F` letter for 4f, with no dates. Proof: `go test -count=1 ./internal/pdfa -run TestXMPPacket`. 2026-09-25: pass.

### 2.3 Output intent and ICC

- [x] `internal/pdfa` generates a minimal D50 sRGB matrix-shaper ICC profile and writes one `/S /GTS_PDFA1` output intent with `DestOutputProfile`, no `DestOutputProfileRef`. Proof: `go test -count=1 ./internal/pdfa -run 'TestICCProfile|TestOutputIntent'`. 2026-09-25: pass. The generated profile also opens in lcms through Pillow with the right color space and description.

### 2.4 Append objects

- [x] `CopyOptions` gains extra object bodies and a catalog override, so `/Metadata` and `/OutputIntents` attach to the copied catalog without changing existing entries. Proof: `go test -count=1 ./internal/pdfout -run TestCopyAppend`. 2026-09-25: pass.

## Phase 3: Preflight

### 3.1 Refusals

- [x] The preflight refuses a missing embedded font, `LZWDecode`, filters outside the ISO 32000-2 table, `DeviceCMYK` without a matching profile, `Alternates` and `OPI` image keys, and unsupported blend modes. Each refusal is a `JobError` with the failed rule. Proof: `go test -count=1 ./internal/pdfa -run 'TestPreflightFonts|TestPreflightFilters|TestPreflightColors'`. 2026-09-25: pass. `TestPreflightEmbeddedFiles` also covers the 4 and 4f embedded-file rules.

## Phase 4: Command and stability

### 4.1 Option and CLI

- [x] `RewriteOptions` gains a PDF/A mode, and `spectreps rewrite -pdfa 4[4f]` refuses non-conforming input with exit 1. Proof: `go test -count=1 ./spectreps -run TestRewritePDFA4` and `go test -count=1 ./internal/cli -run TestRewritePDFA4CLI`. 2026-09-25: both pass. The CLI check also covers exit 2 on `-pdfa 5` and no output file on a refusal.

### 4.2 Stable bytes

- [x] Two runs with the same input and mode return equal bytes, with no dates. Proof: `go test -count=1 ./spectreps -run TestRewritePDFA4Stable`. 2026-09-25: pass.

## Phase 5: Validation, docs, and closure

### 5.1 veraPDF and docs

- [x] `make pdfa-check` runs `verapdf --flavour 4` over `sampledata/pdfa/` and skips when the CLI is absent; the command and outcome are written into the row. `documentation/features.md`, `covered-and-not-covered.md`, `devices.md`, `cli.md`, and `test.md` state the target and the claim wording. Proof: `make pdfa-check` plus `grep -n 'PDF/A-4' documentation/*.md`. Outcomes 2026-09-25 with veraPDF 1.30.2 on PATH: `make pdfa-check` exited 0 with `compliant="2" nonCompliant="0" failedJobs="0"` for `sampledata/pdfa/path-a4.pdf` and `sampledata/pdfa/compliant-a4.pdf`. The first file is a Spectre write; the second is copied from the local gopdfsuit project (`sampledata/wasm-js/compliant.pdf`, SHA-256 `475526f82afe9eb6e36f702910c538fc956fbd8bf658e4183efbdceca74e870c`). The 2026-09-25 earlier run had printed the skip, and the verifier now records real verdicts. veraPDF is available through a local wrapper and stays off the module and the build. The grep lists hits in all five named docs plus `copyright-and-rewrite.md`, `public-api.md`, and `gs-argv-mapping.md`; the wording stays "profile preflight" and no doc makes Spectre's own certification claim. The checked-in sample `sampledata/pdfa/path-a4.pdf` was written by `spectreps rewrite -pdfa 4 -o sampledata/pdfa/path-a4.pdf sampledata/compress/path.pdf`, and `sampledata/pdfa/README.md` records both files.

### 5.2 Closure

- [x] `make lint` and `make test` pass. Outcomes recorded on the day. 2026-09-25: `make lint` exited 0 (gofmt clean, golangci-lint clean, size-check clean, 0 over-limit files). `make test` exited 0 on all eight packages. `go test -count=1 -p 4 ./...` also exited 0.

## Dependencies

The pass-through writer, the image model, and a fixture that already embeds its fonts. The ICC profile is generated in-tree. veraPDF is a proof tool only, and it is Java, so it stays out of `make test`.

## Not in this plan

- PDF/A-1, PDF/A-2, and PDF/A-3, PDF/X, PDF/E, and PDF/UA, which is phase 10.
- Font embedding and subsetting, `ToUnicode` synthesis, color conversion, encryption, and PAdES.
- The policy-0 metadata override. Spectre refuses instead.
