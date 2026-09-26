# v0.0.4 - PDF/UA-2 tag generation

> **Parent:** `plans/v0.0.4/00-program.md` - program ledger
> **Status:** implemented.
> **Estimated effort:** six to nine weeks across six phases. Phases 1 to 4 build the machine. Phase 5 adds meaning and can be cut.

---

## Overview

PDF/UA-2 is ISO 14289-2. `plans/v0.0.3/10-pdfua2.md` preserves and preflights an existing structure tree and refuses to generate one. This ledger generates tags for a parsed PDF and preflights the result. It never certifies, and the claim wording is "generate and preflight".

The phase order starts with a defect. `internal/pdf/content.go` handles path, paint, state, text, and `Do`, and every other operator returns `undefined` with its name. `BMC`, `BDC`, `EMC`, `MP`, and `DP` are not handled, so a tagged page fails `RasterizePage` and `ExtractText` with `undefined in BDC`. Spectre cannot record marked content before it can read it and cannot author a tree before it can record.

The design reference is the local gowkhtmltopdf tree at `5eb1173`, MIT licensed, copyright (c) 2026 Chinmay Sawant. No code is copied. Its derivation layer reads an HTML DOM and a laid-out box tree, and Spectre has neither, so the shapes and failure modes are the reference and the Go is new against ISO 32000-2 and ISO 14289-2.

## Executive summary

Reading comes first: the marked content operators, an event seam, a recorder, and an MCID allocator. The authoring model and parent tree follow, then the catalog and the claim, then reading order and roles, then validation and docs.

`pdfuaid` is written only when the built bytes pass `pdfa.PreflightUA2`. A refusal after the first pass keeps the tree and writes no claim.

Phases 1 to 4 are proven with fixtures and synthetic events and do not derive meaning from content. Phase 5 adds `/P` reading order, H1 to H6, lists, tables, figures with `/Alt`, and `/ActualText`, and it is the section to cut if the ledger must ship early. Open decision 6 records whether a cut that cannot prove `TestTagTable` may write the claim at all. veraPDF stays a proof tool. The open decisions are recorded at the end.

## Phase 1: Marked content reading

The interpreter learns the five marked content operators before anything writes one. This phase changes no pixel and no extracted glyph.

### 1.1 Read the operators
- [x] `BMC`, `BDC`, `EMC`, `MP`, and `DP` parse and push or pop operands. Nesting caps at 64, an unmatched `EMC` is `syntaxerror in content`, and a page with marked content rasterizes and extracts as if the markers were absent. `sampledata/pdfua2/tagged-ua2.pdf`, which fails with `undefined in BDC` today, passes. Proof: `go test -count=1 ./internal/pdf -run TestMarkedContentRead` and `TestRasterizeTaggedContent` exit 0.

### 1.2 Event seam
- [x] `PaintOptions` carries an optional marked-content sink. `BeginMarkedContent` receives the tag name, the properties value, and the nesting depth, `EndMarkedContent` fires at `EMC`, events pair in stream order, and a nil sink keeps the old behavior. Proof: `go test -count=1 ./internal/pdf -run TestMarkedContentEvents` exits 0.

## Phase 2: Tag recorder and MCID allocator

### 2.1 Recorder
- [x] A new package `internal/tag` holds a recorder that implements `graphics.Marker` the way `internal/pdfout/device.go` and `internal/psout/device.go` do, plus the marked-content sink, and records events in stream order. `internal/pdfout.EmitPage` is not reused: it records no marked content, refuses text by design, and refuses images. Proof: `go test -count=1 ./internal/tag -run TestTagRecorder` exits 0.

### 2.2 Text and image seams
- [x] Text run events carry the font resource name, size, text and line matrices, rise, spacing, and the shown bytes, so the recorder can re-emit `Tf`, `Tm`, `Td`, `Tc`, `Tw`, `Tz`, `Ts`, `Tj`, and `TJ`. Image events carry the XObject name and dictionary before decode through an optional interface modeled on `glyphMarker`, and a marker without it keeps the current refusal. Proof: `go test -count=1 ./internal/pdf -run TestTagTextRun` and `TestTagImageName` exit 0.

### 2.3 MCIDs
- [x] Every generated marked-content sequence gets an MCID unique on its page, numbers follow paint order, and the page table resolves MCID to element both ways. Proof: `go test -count=1 ./internal/tag -run TestTagMCIDCoverage` exits 0.

## Phase 3: Structure authoring and the parent tree

### 3.1 Tree objects
- [x] A typed builder produces `pdf.Value` for `/StructTreeRoot`, each `/StructElem`, the structure namespace dictionary, and any `/RoleMap`, with object numbers allocated from `ObjectCount()+1` in a fixed element order. `pdf.SerializeValue` sorts keys, so equal input gives equal bytes. Proof: `go test -count=1 ./internal/tag -run TestBuildTreeObjects` and `TestTagStable` exit 0 with equal bytes on two runs.

### 3.2 Parent tree and leaf kids
- [x] The parent tree is one flat `/Nums` array keyed by a fresh `/StructParents` value per page, every `/K` claim resolves through `StructTree.Lookup` and `StructTree.Key` both ways, and no MCID is orphaned on either side. A childless element serializes its MCIDs bare only when its content sits on one page; content that spans pages serializes as `<< /Type /MCR /Pg <page> /MCID n >>`. Proof: `go test -count=1 ./internal/tag -run TestTagParentTree` and `TestTagContentMCID` and `TestTagMCR` exit 0.

### 3.3 Role namespace
- [x] Open decision 8 picks one: every generated element carries `/NS`, or `newElem` inherits `/NS` from an ancestor. Either way every `/S` name is standard in the PDF 2.0 structure namespace or has a `/RoleMap` entry, and `TOC`/`TOCI` spelling matches ISO 32000-2 and round-trips through `internal/pdf`. Proof: `go test -count=1 ./internal/tag -run TestTagRoleNamespace` exits 0.

### 3.4 Writer seams
- [x] `File.PageObjectNums()` names page objects in `PageCount` order, a font dictionary accessor lets `TaggedFontOK` read a loaded resource, and `pdfout.CopyOptions` gains a tag switch so a generated file from a PDF 1.x source gets the PDF 2.0 header and its binary marker. Proof: `go test -count=1 ./internal/pdf -run TestPageObjectNums` and `go test -count=1 ./internal/pdfout -run TestTaggedHeaderOption` exit 0.

## Phase 4: Metadata, API, and the claim

Metadata and preflight already exist in `internal/pdfa`. This phase wires them to the generation path.

### 4.1 Catalog
- [x] The catalog override carries `/StructTreeRoot`, `/MarkInfo << /Marked true >>`, `/Lang`, `/ViewerPreferences << /DisplayDocTitle true >>`, and `/Metadata`, reusing `pdfa.UA2Catalog` and `pdfa.UA2ExtraObjects`. Proof: `go test -count=1 ./internal/tag -run TestTagCatalog` and `go test -count=1 ./internal/pdfa -run TestUA2Metadata` exit 0.

### 4.2 Public API and CLI
- [x] `RewriteOptions` gains the tag switch, `RewritePDF` builds the tagged write, and the CLI exposes the flag. `Document.Tagged` stays the input predicate, and the tag switch on a tagged input is a named refusal. Proof: `go test -count=1 ./spectreps -run TestRewriteTags` and `go test -count=1 ./internal/cli -run TestTagCommand` exit 0.

### 4.3 Claim opt-in
- [x] The builder runs `pdf.Open` plus `pdfa.PreflightUA2` on its own bytes, and only then does `pdfa.UA2Write` add `pdfuaid:part 2` and `pdfuaid:rev 2024` when the caller opted in. The claim needs a non-empty `dc:title`, from the caller or the source catalog; without one `ua2-title` fails. A refusal keeps the tree and writes no claim. Proof: `go test -count=1 ./internal/tag -run TestTagClaim` and `TestTagTitleRequired` exit 0, with one positive and one negative case.

## Phase 5: Reading order and richer roles

An untagged PDF stores no semantics. Reading order comes from the extraction geometry plus column grouping and is an approximation, not a fact. This phase is the one to cut if the ledger must ship early.

### 5.1 Reading order and paragraphs
- [x] Text runs group into blocks by baseline and box geometry, blocks order top to bottom then left to right, and each block becomes a `/P`. Proof: `go test -count=1 ./internal/tag -run TestTagReadingOrder` and `TestTagGenerateParagraphs` exit 0 on a two-column fixture.

### 5.2 Headings
- [x] A run whose size is a clear step above the body size becomes `/H1` through `/H6`, and the thresholds are recorded in `documentation/devices.md`. Proof: `go test -count=1 ./internal/tag -run TestTagHeadings` exits 0.

### 5.3 Lists
- [x] Bullet and number prefixes detected in text runs produce `/L`, `/LI`, `/Lbl`, and `/LBody`. Proof: `go test -count=1 ./internal/tag -run TestTagLists` exits 0.

### 5.4 Tables
- [x] A grid inferred from aligned text runs emits `/Table`, `/TR`, `/TH`, and `/TD` with `/Scope` on header cells. Proof: `go test -count=1 ./internal/tag -run TestTagTable` exits 0.

### 5.5 Figures and alt text
- [x] An image XObject inside marked content becomes `/Figure`. `/Alt` comes from the source fixed in open decision 4, and an image with no alt source returns a named `JobError` instead of an empty `/Alt`. Proof: `go test -count=1 ./internal/tag -run TestTagFigureAlt` and `TestTagImageOnlyPage` exit 0.

### 5.6 ActualText
- [x] A text run whose glyphs map to ligature code points or have no Unicode mapping carries `/ActualText` per the policy in open decision 9, and `TaggedTextOK` accepts the result. Proof: `go test -count=1 ./internal/tag -run TestTagActualText` exits 0.

### 5.7 Artifacts
- [x] Content outside the reading order, such as rules, page numbers, and repeated furniture, is wrapped in `/Artifact`, or the decision records why not. Proof: `go test -count=1 ./internal/tag -run TestTagArtifacts` exits 0.

## Phase 6: Validation and docs

### 6.1 Content-side coverage
- [x] `PreflightUA2` reports an unpaired or uncovered marked-content item as a named rule, so a preserved or generated file is checked against its content, not only its parent tree. Proof: `go test -count=1 ./internal/pdfa -run TestUA2ContentCoverage` exits 0 with a positive and a negative fixture.

### 6.2 veraPDF outcomes
- [x] Generated samples live under `sampledata/pdfua2/generated/`, and the run records the veraPDF version, the pass and fail counts, and every failed clause. veraPDF stays a proof tool, never a build or runtime dependency. Proof: `make pdfua2-check` runs `verapdf --flavour ua2 --format json` over the folder and exits 0; the version and counts are recorded on the row.

### 6.3 Round trip
- [x] A generated file reopens in Spectre, rasterizes with marked content skipped, extracts its text, and returns equal bytes across two generation runs. Proof: `go test -count=1 ./spectreps -run TestTagRoundTrip` and `go test -count=1 ./internal/tag -run TestTagStable` exit 0.

### 6.4 Validate wiring
- [x] Open decision 11 decides whether `validate` calls `PreflightUA2`, and `documentation/cli.md` matches the answer. Proof: `go test -count=1 ./internal/cli -run TestValidateUA2` exits 0, or the decision records the deferral.

### 6.5 Docs and closure
- [x] `documentation/features.md`, `devices.md`, `covered-and-not-covered.md`, `cli.md`, and `public-api.md` state the generate-and-preflight scope, the refusal matrix, and the claim wording, and `make lint` and `make test` pass with outcomes recorded on the day. Proof: `grep -n 'generate and preflight' documentation/features.md documentation/devices.md` exits 0, `make lint` exits 0, and `make test` exits 0. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Open decisions

These are recorded now, and later rows cite them by number. Decisions 1 and 7 are fixed by the scope above. Decision 2 keeps the preserve-and-preflight behavior from `plans/v0.0.3/10-pdfua2.md` except where it says open.

1. Path scope. Decided: generation applies to a parsed PDF only. PostScript stays out, and its only output is rasters.
2. Input state. Decided: an untagged PDF is the input, a tagged input keeps preserve and preflight, and the tag switch on a tagged input is a named refusal. Open: whether a partial tree, `/Marked true` with no root or a tree that fails preflight, is refused or rebuilt.
3. Text subset. Open: generation covers the fonts and operators the text machine already reads. Type 1, bare CFF, Type 3, vertical writing, and unsupported operators are either refused or copied untagged.
4. Image-only pages. Open: refuse, tag the image as one `/Figure` with caller alt, or tag the page as artifacts. No in-tree alt source exists, and `dc:title` is document level.
5. Reading order authority. Open: the proposal is the extraction geometry plus column grouping, stated as an approximation. A caller-supplied order is possible but not proposed.
6. Role coverage in the first cut. Open: `Document`, `P`, `H1` to `H6`, and `Figure` is the honest cut. The decision says whether a cut that cannot prove `TestTagTable` may write the claim at all.
7. Claim wording. Decided: "generate and preflight", never certification. `pdfuaid` is written only from built bytes that pass `PreflightUA2`.
8. `/NS` handling. Open: write `/NS` on every element or teach `newElem` to inherit from an ancestor. Writing it everywhere is smaller. Inheriting is closer to ISO 32000-2 and helps preserved files.
9. ActualText policy. Open: narrow, only ligatures and codes with no Unicode mapping, or broad, every text run. The narrow policy matches the current font check.
10. Combined PDF/A-4 plus UA-2. Open: out of this ledger unless it says otherwise. `RewriteOptions.PDFA` and the tag switch interact in the header, the catalog, and the claim writer.
11. `validate` wiring. Open: decide whether `validate` calls `PreflightUA2`. Content-side coverage in phase 6 is the prerequisite.

## Recorded outcomes

The decisions as landed on 2026-09-26: 1 stayed as written; 2 rebuilds a partial tree and refuses the tag switch on a tagged input; 3 covers the text machine's fonts, with Type 1 landed and bare CFF, Type 3, and vertical writing still out; 4 is a `/Figure` with `/Alt` from the BDC property dictionary or the image dictionary, and no source is `Error: /alt in Tag`; 5 is the extraction geometry plus column grouping, an approximation; 6 landed the honest cut plus lists, tables, and artifacts, and the claim writes only after `PreflightUA2` passes; 7 fixed the wording; 8 writes `/NS` on every element; 9 is narrow; 10 refuses Tag with PDFA as `/unsupported in RewritePDF`; 11 makes `validate` call `PreflightUA2` for a tagged input and leaves an untagged file untouched.

## Dependencies

`plans/v0.0.3/9-text-and-fonts.md` (the text and font machine, complete), `plans/v0.0.3/10-pdfua2.md` (the structure parse and preflight, complete), the pass-through writer from v0.0.2 with `Overrides`, `AppendObjects`, and `CatalogOverride`, and the metadata writer and `PreflightUA2` in `internal/pdfa`. `pdf.SerializeValue` and object access are present. veraPDF is a proof tool only. The integrator owns `plans/v0.0.4/00-program.md`, the closure file, and `plans/v0.0.1/10-deferred.md`.

## Not in this plan

- PostScript tag generation, and any HTML or markup input. The semantic input is the parsed PDF.
- Annotation structure: `/OBJR` entries, link tags, structure destinations, and outline `/SE`.
- MathML.
- WCAG certification, PAC reliance, visual checks, and anything past the machine rules.
- Combined PDF/A-4 plus UA-2 output, unless open decision 10 says otherwise.
- Font subsetting, writing fonts, and Type 1, bare CFF, Type 3, or vertical text as tag inputs.
