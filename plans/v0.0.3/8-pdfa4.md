# v0.0.3 - PDF/A-4 output

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** not started.
> **Estimated effort:** one phase, about two sessions. The ICC profile and the first clean validator run are the long pole.

---

## Overview

The deferred row asked for PDF/A creation. The latest archival part is PDF/A-4. `validate` today stops on the first interpreter error and reads no XMP, output intent, font, filter, or color space, so it is not a conformance check. The pass-through writer cannot append an object, change the header, or add metadata.

The maintainer also named "PDF override". In Ghostscript that is `PDFACompatibilityPolicy`: policy 0 keeps an incompatible feature and keeps the PDF/A claim, policy 1 drops the feature, and policy 2 aborts. Spectre mirrors policy 2 and refuses, because a claim with a known violation is the ambiguity this project already refuses to copy.

## Executive summary

PDF/A-4 base is the default claim, with 4f offered only when the input already carries embedded files. 4e stays deferred. A static XMP packet and a generated sRGB ICC output intent are appended, the header becomes `%PDF-2.0` with a binary marker, and a preflight refuses inputs that need font embedding, LZW, DeviceCMYK, or unsupported image keys. The CLI and docs say "profile preflight", never "compliant" or "certified". veraPDF gives the verdict and is a proof tool, not a dependency.

## Phase 1: Target and policy

### 1.1 Target decision

- [ ] The phase row names PDF/A-4 base as the default and the refusal policy, with the claim wording. Proof: the row records the decision and the doc grep in 5.1 shows it.

## Phase 2: Writer and metadata

### 2.1 Header and marker

- [ ] Both writers emit `%PDF-2.0` with a binary marker above byte 127 when the PDF/A option is set, keeping the `/ID` and no `/Encrypt`. Proof: `go test -count=1 ./internal/pdfout -run TestPDFA4Header`.

### 2.2 XMP packet

- [ ] A static UTF-8 XMP packet with `pdfaid:part=4`, `pdfaid:rev=2020`, and the `F` letter for 4f, with no dates. Proof: `go test -count=1 ./internal/pdfa -run TestXMPPacket`.

### 2.3 Output intent and ICC

- [ ] `internal/pdfa` generates a minimal D50 sRGB matrix-shaper ICC profile and writes one `/S /GTS_PDFA1` output intent with `DestOutputProfile`, no `DestOutputProfileRef`. Proof: `go test -count=1 ./internal/pdfa -run 'TestICCProfile|TestOutputIntent'`.

### 2.4 Append objects

- [ ] `CopyOptions` gains extra object bodies and a catalog override, so `/Metadata` and `/OutputIntents` attach to the copied catalog without changing existing entries. Proof: `go test -count=1 ./internal/pdfout -run TestCopyAppend`.

## Phase 3: Preflight

### 3.1 Refusals

- [ ] The preflight refuses a missing embedded font, `LZWDecode`, filters outside the ISO 32000-2 table, `DeviceCMYK` without a matching profile, `Alternates` and `OPI` image keys, and unsupported blend modes. Each refusal is a `JobError` with the failed rule. Proof: `go test -count=1 ./internal/pdfa -run 'TestPreflightFonts|TestPreflightFilters|TestPreflightColors'`.

## Phase 4: Command and stability

### 4.1 Option and CLI

- [ ] `RewriteOptions` gains a PDF/A mode, and `spectreps rewrite -pdfa 4[4f]` refuses non-conforming input with exit 1. Proof: `go test -count=1 ./spectreps -run TestRewritePDFA4` and `go test -count=1 ./internal/cli -run TestRewritePDFA4CLI`.

### 4.2 Stable bytes

- [ ] Two runs with the same input and mode return equal bytes, with no dates. Proof: `go test -count=1 ./spectreps -run TestRewritePDFA4Stable`.

## Phase 5: Validation, docs, and closure

### 5.1 veraPDF and docs

- [ ] `make pdfa-check` runs `verapdf --flavour 4` over `sampledata/pdfa/` and skips when the CLI is absent; the command and outcome are written into the row. `documentation/features.md`, `covered-and-not-covered.md`, `devices.md`, `cli.md`, and `test.md` state the target and the claim wording. Proof: `make pdfa-check` plus `grep -n 'PDF/A-4' documentation/*.md`.

### 5.2 Closure

- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

The pass-through writer, the image model, and a fixture that already embeds its fonts. The ICC profile is generated in-tree. veraPDF is a proof tool only, and it is Java, so it stays out of `make test`.

## Not in this plan

- PDF/A-1, PDF/A-2, and PDF/A-3, PDF/X, PDF/E, and PDF/UA, which is phase 10.
- Font embedding and subsetting, `ToUnicode` synthesis, color conversion, encryption, and PAdES.
- The policy-0 metadata override. Spectre refuses instead.
