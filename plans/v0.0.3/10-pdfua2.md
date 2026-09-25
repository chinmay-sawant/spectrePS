# v0.0.3 - PDF/UA-2 preservation and preflight

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** not started.
> **Estimated effort:** weeks for the phases below. Tag generation is not scoped here.

---

## Overview

PDF/UA-2 is ISO 14289-2, a companion to PDF 2.0 and ISO/TS 32005. The maintainer named it as the latest accessibility compliance. Spectre today has no structure tree, no tagged content, no font model, and no metadata writer. The pass-through writer copies tagged objects verbatim because it never parses them, so an already-tagged file can survive levels 1 to 5.

This file is preserve and preflight, not create and not certify.

## Executive summary

An already-conforming PDF/UA-2 file survives a level 1 to 5 rewrite with its tree, MCIDs, `/Alt`, `/ActualText`, `/Lang`, and XMP intact. Spectre preflights the machine-checkable subset and says "preflight only". Generated files, including level 0, `ImagePDF`, and `WriteImages`, refuse a tagged input instead of stripping tags. Tag generation needs the font and text machine from phase 9, then reading order and role assignment, and it is out of this ledger.

## Phase 1: Target and claim

### 1.1 Scope decision

- [x] The phase row states preserve and preflight, the interface changes (`validate` gains the checks, `rewrite` refuses to drop tags, `Document` gains tagged and structure accessors), and the claim wording. Proof: the decision is recorded below, 2026-09-25.

**Scope decision, 2026-09-25.** Preserve and preflight, not create and not certify. Spectre keeps an existing structure tree through a pass-through rewrite and checks the machine-checkable subset. It does not write tags, does not guess reading order, and does not claim PDF/UA-2 or any other conformance. The claim wording is "preflight only". "Compliant" and "certified" stay out of the CLI, the docs, and the API text.

Interface changes:

- `validate` gains the PDF/UA-2 machine checks in phase 5 through `internal/pdfa`. The checks read the typed structure model; they do not rasterize.
- `rewrite` refuses to drop tags. Level 0 builds a new path-only file, so a tagged input returns `JobError` `/tagged in RewritePDF`. Levels 1 through 5 use the pass-through writer, which copies the structure objects, and a tagged input keeps its source header version so a PDF 2.0 file does not leave as a 1.4 shell.
- `pdfimage` refuses a tagged PDF with `/tagged in ImagePDF`, because an image PDF has no tags to keep and the command would otherwise drop them silently.
- `Document` gains `Tagged()`, backed by `File.HasStructTree()`. The typed structure model lives on `File` (`StructTree`, `MarkInfo`, `Header`), where `internal/pdfa` reads it. The public structure accessor waits for phase 5, when the preflight result shape is known.

`Write` and `WriteImages` take page and image values and never see a source document. The refusal sits at the source-aware entry points: `RewritePDF` level 0 and the `pdfimage` command. Tag generation stays out of this ledger.

## Phase 2: Structure tree model

### 2.1 Parse the tree

- [x] `internal/pdf` parses `/MarkInfo`, `/StructTreeRoot`, `/K`, `/S`, `/P`, `/Pg`, `/MCID`, `/Alt`, `/ActualText`, `/Lang`, `/Namespaces`, `/RoleMap`, `/RoleMapNS`, and `/ParentTree` into typed values with cycle and depth caps. The model is `internal/pdf/structtree.go`; `/StructTreeRoot` without `/ParentTree` leaves the lookup empty, and a cycle or a depth past 64 is `limitcheck`. Proof: `go test -count=1 ./internal/pdf -run TestStructTreeParse` exited 0 on 2026-09-25.

### 2.2 Parent tree

- [x] MCID-to-StructElem lookup works both ways through `/ParentTree`, and a missing parent is a `JobError`. `Lookup` reads `(page, MCID)` and `Key` reads the element; a `/ParentTree` entry keyed by the page object number or by the page `/StructParents` value both resolve. A `/K` claim with no agreeing entry is `undefined in ParentTree`. Proof: `go test -count=1 ./internal/pdf -run TestParentTree` exited 0 on 2026-09-25.

### 2.3 Role map

- [x] A custom type resolves to a standard type through `/RoleMap` and `/RoleMapNS`; same-namespace mappings, cycles, and unmapped custom types are errors. A chain resolves hop by hop, a target in the source namespace or an identity default mapping is `syntaxerror in RoleMap`, a revisited pair or a chain past 32 is `limitcheck in RoleMap`, and a type with no mapping is `undefined in RoleMap`. Proof: `go test -count=1 ./internal/pdf -run TestRoleMap` exited 0 on 2026-09-25.

### 2.4 Font check

- [x] The font check is dictionary-level only: `/ToUnicode`, a standard encoding for simple fonts, or `/ActualText` covering the run. No glyph decoding. `TaggedFontOK` reads the font dictionary, and `TaggedTextOK` walks `/ActualText` on the element and its ancestors. Proof: `go test -count=1 ./internal/pdf -run TestTaggedFontCheck` exited 0 on 2026-09-25.

## Phase 3: Pass-through and refusal

### 3.1 Levels 1 to 5 preserve tags

- [x] A tagged PDF 2.0 fixture round-trips through levels 1 to 5 with tree shape, MCIDs, and `/Alt` intact. The fixture is built in the test: one page, a Document, a Figure with `/Alt`, one MCID, and a parent tree. Proof: `go test -count=1 ./internal/pdfout -run TestLevelsPreserveTags` exited 0 on 2026-09-25.

### 3.2 Generated writers refuse tagged input

- [ ] Level 0, `Write`, `WriteImages`, and `ImagePDF` return a clear `JobError` on a tagged input instead of stripping tags. A silent downgrade stays a bug. Proof: `go test -count=1 ./spectreps -run TestRewriteRefusesTagged`.

### 3.3 Container version

- [ ] A PDF 2.0 input does not leave as a 1.4 shell when tags are present. Proof: `go test -count=1 ./internal/pdfout -run TestTaggedHeader`.

## Phase 4: Metadata

### 4.1 Read and write metadata

- [ ] Catalog `/Metadata`, `pdfuaid`, `dc:title`, `/Lang`, `/MarkInfo`, and `/ViewerPreferences` are read and written. A `pdfuaid` claim is never written unless the source carried it or an explicit opt-in follows a passing preflight. Proof: `go test -count=1 ./internal/pdfa -run TestUA2Metadata`.

## Phase 5: Validation, docs, and closure

### 5.1 Machine checks

- [ ] Checks cover `/Marked true`, `/StructTreeRoot`, one `Document` in the pdf2 namespace, `/Lang` syntax, `DisplayDocTitle`, `pdfuaid` values, `dc:title`, role-map resolution, and MCID coverage. Proof: `go test -count=1 ./internal/pdfa -run TestUA2Preflight`, with one conforming and one failing fixture per check.

### 5.2 veraPDF outcomes

- [ ] `verapdf --flavour ua2 --format json` outcomes are recorded on the rows for the fixtures. veraPDF is a proof tool, never a build or runtime dependency. Proof: the recorded command and outcome.

### 5.3 Docs and closure

- [ ] `documentation/features.md`, `covered-and-not-covered.md`, `devices.md`, and `cli.md` state the preserve-and-preflight scope and the claim wording, and the new deferred row moves to 10.4. Proof: `grep -n 'PDF/UA-2' documentation/features.md`.
- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

The pass-through writer from v0.0.2, the metadata writer from phase 8, and phase 9 for tag generation. `pdf.Value` object access is already present.

## Not in this plan

- Tag generation, reading order, role assignment, lists and tables, figure alt text, annotation links, structure destinations, and MathML.
- WCAG certification, PAC reliance, and font embedding heuristics.
- A `Document` wrapper for PDF/UA-1 inputs.
