# v0.0.4 - Type 1 fonts

> **Parent:** `plans/v0.0.4/00-program.md` - program ledger
> **Status:** implemented (phases 1 to 6). Phase 7 is deferred to `plans/v0.0.1/10-deferred.md` 10.1.
> **Estimated effort:** about two weeks for phases 1 to 6. Phase 7 is a stretch gated on the tag budget.

---

## Overview

A Type 1 simple font already loads as a font resource and contributes advances, encoding names, and extraction. Painting fails: `loadProgram` never reads `/FontFile`, so `out.program` stays nil, `paintSource` is false, and `paintGlyph` returns `invalidfont` from `internal/pdf/text.go:439`. Two silent gaps sit behind that: no advance from the charstring `hsbw` when `/Widths` is missing, and no use of a symbolic font's built-in `/Encoding`. This file reads PFA and PFB programs, decodes the Private dict and CharStrings, interprets Type 1 charstrings, composes `seac`, and feeds the existing `x/image/vector` coverage path. `golang.org/x/image/font/sfnt` parses only SFNT containers, so bare CFF and PFA/PFB need their own reader. This closes deferred row 10.1 and the v0.0.3 row 3.6 drop; the integrator moves the ledger.

## Executive summary

One decoder in `internal/font` and one read path in `internal/pdf`. The eexec and charstring ciphers are fixed by the spec; the risky parts are the Private dict idioms (`RD`, `def`, and the `256 array ... dup code /name put` encoding loop) and OtherSubrs flex. Hint operators are parsed and dropped, matching the hinting exclusion in `documentation/fonts.md`. Outlines convert to the existing `sfnt.Segments` shape at `outlinePPEM`, so `glyphMask`, `rasterizeGlyph`, and `DrawGlyph` do not change. Fixtures are synthetic Go-built programs. Text pixels never byte-match Ghostscript, so paint proofs compare a locked PPM and the synthetic TrueType outline, not `gs` output.

## Phase 1: Written policy

### 1.1 Scope in the docs

- [x] `documentation/fonts.md` replaces the Type 1 deferral in "Type 1 and CFF scope" with the supported subset: PFA and PFB `/FontFile`, eexec seed 55665, charstring seed 4330 and `lenIV`, CharStrings and Subrs, the `hsbw` width, the built-in `/Encoding`, `seac`, and OtherSubrs flex. Hinting, `Type1C`, and `CIDFontType0C` stay out. `documentation/test.md` splits the Type 1 case out of the `invalidfont` bullet. Proof: `grep -n -e 'PFA' -e 'eexec' documentation/fonts.md` exits 0.

## Phase 2: Read the program

### 2.1 PFA and PFB containers

- [x] `font.LoadType1(data []byte, lengths [3]int) (*font.Type1Font, error)` accepts PFA and PFB bytes, prefers the stream's `/Length1`, `/Length2`, and `/Length3` when present, falls back to `eexec` and `cleartomark` or 512-zero markers, accepts hex and binary eexec, and drops the trailer. A truncated or undecryptable program returns an error. Proof: `go test -count=1 ./internal/font -run TestType1Program`.

### 2.2 Private dict and encrypted glyph data

- [x] The tokenizer recovers `/FontName`, `/FontMatrix`, the built-in `/Encoding`, `Private /lenIV`, `Private /Subrs`, and `Private /CharStrings` through the seed 4330 cipher, including `RD` byte runs and hex-string values, `lenIV` 0 and 4, and the `256 array ... dup code /name put` encoding loop. Proof: `go test -count=1 ./internal/font -run 'TestType1Charstring|TestType1Encoding'`.

### 2.3 Charstring paths and widths

- [x] The interpreter supports `hsbw` and `sbw`, the moveto, lineto, and curveto families, `closepath`, `callsubr` and `return` with direct and negative indexes, `div`, `endchar`, and the hint operators parsed and dropped. The first stack-clearing operator yields the advance. Caps bound the operand stack, call depth, subroutine count, and total points; a malformed program returns an error and paints nothing. Proof: `go test -count=1 ./internal/font -run TestType1Charstring`.

### 2.4 seac

- [x] `seac` resolves `bchar` and `achar` through StandardEncoding, takes the base charstring width, offsets the accent by the sidebearing rule checked against fontTools `op_seac`, and appends the accent outline; a base that is itself `seac` is malformed. Proof: `go test -count=1 ./internal/font -run TestType1Seac`.

### 2.5 OtherSubrs

- [x] Flex (OtherSubr 0, 1, and 14 to 18) emits the two flex curves and leaves the final point for the following `pop`; hint replacement (`n 3 callothersubr pop callsubr`) executes the replacement Subr; unknown OtherSubrs pop their arguments and keep the stack balanced. Proof: `go test -count=1 ./internal/font -run TestType1Flex`. As landed, flex is OtherSubrs 0, 1, and 2, and 14 to 18 are the Multiple Master blend operators the Adobe supplement defines.

## Phase 3: PDF font model

### 3.1 /FontFile loads

- [x] `loadProgram` reads `/FontFile` for a simple `/Subtype /Type1` font and `Font` carries the program; a broken or missing program still loads the font with no outline source. Proof: `go test -count=1 ./internal/pdf -run TestType1Glyph`.

### 3.2 Glyph lookup and outline conversion

- [x] A code resolves through the PDF encoding, then the built-in encoding of a symbolic font, to a CharStrings name, and the outline converts to `sfnt.Segments` at `outlinePPEM` with Y down so `glyphMask` and `rasterizeGlyph` stay unchanged. A name missing from CharStrings paints as `invalidfont`. Proof: `go test -count=1 ./internal/pdf -run TestType1Glyph`.

### 3.3 Width fallback

- [x] `/Widths` and standard 14 metrics still win, the charstring `hsbw` width fills a missing `/Widths`, `/MissingWidth` stays last, and `/MMType1` keeps its PDF widths and paints `invalidfont`. Proof: `go test -count=1 ./internal/pdf -run TestType1Widths`.

### 3.4 Built-in encoding and Unicode

- [x] A symbolic font with no PDF `/Encoding` starts from the program's built-in encoding, `/BaseEncoding` and `/Differences` still apply on top, and `Font.Unicode` uses the resolved name before the code-point fallback. Proof: `go test -count=1 ./internal/pdf -run TestType1Unicode`.

## Phase 4: Painting

### 4.1 Pixels through the existing coverage path

- [x] `Tj` on the synthetic Type 1 `A` paints the same pixels as the synthetic TrueType `A` from `synthFont` in `internal/pdf/font_fixture_test.go`, compared with `CompareRaster`. Proof: `go test -count=1 ./internal/pdf -run TestType1Paint`.

### 4.2 Locked fixture

- [x] `sampledata/fixtures/type1-tj.ppm` is the locked page, written by `UPDATE_FIXTURES=1` and read with the recorded SHA-256. Proof: `go test -count=1 ./internal/pdf -run TestType1Paint`.

### 4.3 seac, Subrs, and a corrupt program

- [x] A `seac` glyph and a Subr-drawn glyph paint through the same path, and a corrupt program still returns `invalidfont` and leaves the page white. Proof: `go test -count=1 ./internal/pdf -run TestType1Paint`.

## Phase 5: Extraction

### 5.1 Built-in encoding

- [x] A Type 1 page with no `/ToUnicode` and a symbolic built-in encoding extracts the AGL names, and a code with no name still uses the code-point fallback. Proof: `go test -count=1 ./internal/pdf -run TestType1Extract`.

### 5.2 Public path

- [x] `File.ExtractText` and `spectreps text` on a fixture that embeds the synthetic Type 1 font print the expected lines. Proof: `go test -count=1 ./spectreps -run TestType1Extract`.

## Phase 6: Docs and closure

### 6.1 Feature docs

- [x] `documentation/fonts.md`, `features.md`, `devices.md`, `covered-and-not-covered.md`, `copyright-and-rewrite.md`, `test.md`, and `folder-structure.md` state the Type 1 subset and keep bare CFF, CID-keyed CFF, Type 3, vertical writing, color, and variable fonts out. Proof: `grep -n 'Type 1' documentation/fonts.md documentation/covered-and-not-covered.md` exits 0.

### 6.2 Lint

- [x] `make lint` passes. Outcome recorded on the day. Proof: `make lint`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

### 6.3 Test

- [x] `make test` passes. Outcome recorded on the day. Proof: `make test`.

## Phase 7: Bare CFF, stretch, gated

Start only after phases 1 to 6 land. If the tag budget runs out, the integrator returns this scope to `plans/v0.0.1/10-deferred.md` as its own row.

### 7.1 CFF container

- [~] The reader parses CFF version 1: header, INDEXes, DICTs, charset, and encoding. CFF2 and FDSelect are refused, which keeps `/CIDFontType0C` out. Proof: `go test -count=1 ./internal/font -run TestCFFGlyph`. Returned to `plans/v0.0.1/10-deferred.md` 10.1.

### 7.2 Type 2 charstrings

- [~] The interpreter covers the Type 2 operator set, `hintmask` and `cntrmask`, `callsubr` and `callgsubr` with the 107, 1131, and 32768 bias, the flex operators, and `endchar` seac; width comes from `nominalWidthX`, `defaultWidthX`, and the leading operand. Proof: `go test -count=1 ./internal/font -run TestCFFGlyph`. Returned to `plans/v0.0.1/10-deferred.md` 10.1 as its own item.

### 7.3 Simple Type1C paints

- [~] `/FontFile3 /Subtype /Type1C` loads in a simple font and paints; `/CIDFontType0C` and `/CIDFontType0` stay `invalidfont`. Returned to `plans/v0.0.1/10-deferred.md` 10.1 as its own item; the tag budget ran out.

## Fixtures and licensing

`make test` builds every Type 1 fixture in Go, following `internal/pdf/font_fixture_test.go`: a PFA and a PFB with `/FontName /SynthType1`, `/FontMatrix [0.001 0 0 0.001 0 0]`, an eexec section with `Private /lenIV`, a two-entry `/Subrs`, and CharStrings for `.notdef`, `A`, `B`, a `seac` glyph, and a flex glyph. `A` matches the synthetic TrueType rectangle and advance (100,0 to 600,700, advance 600). Variants cover `lenIV` 0 and 4, hex and binary eexec, and PFA and PFB.

The local `/usr/share/fonts/type1/gsfonts/*.pfb` and `/usr/share/fonts/type1/urw-base35/*.t1` files are development references only. They are GPL-2+ and AGPL-3 with a font exception and must not be checked in. Any real fixture must carry a license permitting redistribution, its license text sits beside it, and `sampledata/fixtures/README.md` records the source, the pinned release, the generation command, and the SHA-256.

## Open questions

- Bare CFF timing: phase 7 is gated and may not run in this tag. If it does not, the integrator returns `/Type1C` to `plans/v0.0.1/10-deferred.md` as its own row.
- PostScript front end: out. `internal/ps` keeps its standard 14 policy, and embedded Type 1 in `findfont` waits for its own tag.
- `/MMType1`: paints `invalidfont` with the PDF widths and encoding, locked by phase 3. No instance program is read in this tag.
- Real-font parity: no real font is checked in and `make test` stays synthetic. A manual check against a license-clean font is allowed during development if its SHA-256 and provenance are recorded; the local gsfonts and urw-base35 files are not used.
- Missing-width fallback: `/Widths`, standard 14 metrics, charstring `hsbw`, then `/MissingWidth`, then 0. If a real font shows Ghostscript using something else, record the measured case first.

## Dependencies

Standard library only. `golang.org/x/image/font/sfnt` and `x/image/vector` are already in `go.mod`; no new module. The decoder is new code in `internal/font`, and `internal/pdf` gains one read path and one outline conversion. `documentation/fonts.md` and `documentation/test.md` carry the policy.

## Not in this plan

- Hinting and hint replacement applied to output; hint operators are parsed and dropped.
- Font writing and subsetting.
- `/CIDFontType0C` and `/CIDFontType0`, multiple master instance selection, Type 3, vertical writing, color and variable fonts, and OCR.
- PostScript `definefont` and `FontDirectory` reading; the PostScript front end keeps the standard 14 policy.
- Editing `plans/v0.0.1/10-deferred.md` or `plans/v0.0.4/00-program.md`. The integrator owns both and this file touches neither.
