# Fonts

Package `internal/font` holds the advances, encodings, and glyph names the text machine reads. The tables are generated Go source, checked in, and read offline. No cgo, no process, and no host font lookup.

## Metrics source and license

The standard 14 advances come from the Adobe Core 14 AFM files. The generator `internal/font/gen.go` fetches from the mirror at `https://github.com/tecnickcom/tc-font-core14-afms`, pinned to commit `0675784d24b28a55c607cad6b74596ce19ce333c`. That mirror carries the archive Adobe published at `https://www.adobe.com/devnet/font/pdfs/Core14_AFMs.zip`.

The license that accompanies the AFM files reads:

> This file and the 14 PostScript(R) AFM files it accompanies may be used,
> copied, and distributed for any purpose and without charge, with or
> without modification, provided that all copyright notices are retained;
> that the AFM files are not distributed without this file; that all
> modifications to this file or any of the AFM files are prominently noted
> in the modified file(s); and that this paragraph is not modified. Adobe
> Systems has no responsibility or obligation to support the use of the
> AFM files.

`gen.go` is build-ignored, so it never enters the module binary. It fetches over HTTPS, checks a pinned SHA-256 for every file, sorts every key, and writes no timestamp. Running `go run internal/font/gen.go` rebuilds these files:

- `standard14_data.go`, the 14 names and the metrics lookup.
- `widths_data.go` and `codes_data.go`, advances by glyph name and by character code.
- `encodings_data.go` and `agl_data.go`, the tables in the next sections.

The header of `widths_data.go` repeats every Adobe copyright notice found in the AFM files. Two runs write identical bytes, and the checksums in `gen.go` fail the run when the pinned source changes.

## Advances

`font.Width` is an advance in 1/1000 em. `font.Standard14` returns a `*font.Metrics` for one of the 14 names, and `font.Standard14Names` lists them in AFM order. `Metrics.WidthByCode` looks up a character code in the font's own encoding. `Metrics.WidthByName` looks up a glyph name, and a name can have no code while still having an advance, which is what a PDF `/Differences` array needs.

The values are the AFM `WX` numbers. Courier is 600 for every glyph. Helvetica is 667 for `A` and 278 for `space`. Kerning pairs are not read, so the model carries advances only.

## Encodings

`font.Encoding` has three values: `EncodingStandard`, `EncodingWinAnsi`, and `EncodingMacRoman`. `GlyphName` returns the glyph name at a code, `GlyphCode` returns the first code assigned to a name, and `GlyphNames` returns a copy of the 256-entry table. An empty name means the encoding leaves that code undefined.

The tables come from `src/core/encodings.js` in Mozilla pdf.js, pinned to commit `d52fdf411a6e4d338180687456e0df019e28475e`, under the Apache License 2.0. The arrays transcribe the code-to-name table in ISO 32000-1 Annex D.2.

All 256 entries of each table were compared during development against Ghostscript 9.55.0 `Resource/Init/gs_std_e.ps`, `gs_wan_e.ps`, and `gs_mro_e.ps`:

- StandardEncoding and WinAnsiEncoding agree with Ghostscript entry for entry.
- MacRomanEncoding agrees except in 15 slots: `notequal` at 0xAD, `infinity` at 0xB0, `lessequal` at 0xB2, `greaterequal` at 0xB3, `partialdiff` at 0xB6, `summation` at 0xB7, `product` at 0xB8, `pi` at 0xB9, `integral` at 0xBA, `Omega` at 0xBD, `radical` at 0xC3, `approxequal` at 0xC5, `Delta` at 0xC6, `lozenge` at 0xD7, and `apple` at 0xF0. Ghostscript leaves those slots `.notdef` because its substitute fonts have no such glyphs. Spectre follows Annex D.2, which names the Mac OS Roman symbols.

## Glyph names

`font.AGLUnicode` maps a glyph name to its Unicode string from the Adobe Glyph List 2.0, `glyphlist.txt`, merged with the ZapfDingbats list `zapfdingbats.txt`. Both come from `https://github.com/adobe-type-tools/agl-aglfn`, pinned to commit `4036a9ca80a62f64f9de4f7321a9a045ad0ecfd6`, under the BSD-3-Clause style Adobe license in that repository. One name can map to several code points: `dalethatafpatah` is U+05D3 U+05B2.

## Standard 14 painting

Advances, encodings, and extraction work for the standard 14 without an embedded font program. Painting does not.

The policy is `invalidfont`. Spectre ships no substitute outlines for the standard 14 and does not look up host fonts, so painting a glyph from a font with no outline source returns `invalidfont`. That is what Ghostscript returns when it has no substitute font for a non-embedded font. It keeps output independent of the machine and keeps the pixel path free of font programs, which this tag excludes.

A document that needs standard 14 pixels embeds its own font program. A `/FontFile2` stream or an OpenType `/FontFile3` stream is then the outline source, even when `/BaseFont` names a standard 14 font. Phase 3.3 proves this policy.

## Type 1 and CFF scope

Type 1 font programs under `/FontFile` are deferred to a later tag. Phase 3.6 covers charstrings and `seac` and is dropped when this row defers Type 1. Until then a Type 1 font still contributes advances from `/Widths` or from the standard 14 fallback, still extracts through its encoding and ToUnicode, and returns `invalidfont` when painted.

Bare CFF is also out of this ledger: `/FontFile3` with `/Subtype /Type1C` or `/Subtype /CIDFontType0C`. CFF that arrives inside an OpenType wrapper, `/FontFile3` with `/Subtype /OpenType`, parses through `golang.org/x/image/font/sfnt` like `/FontFile2`, so that outline source is in scope.

## Type 0 scope

Identity-H only. The target is a `/Type0` font with `/Encoding /Identity-H`, two-byte character codes, a CIDFontType2 descendant, and a `/CIDToGIDMap`. Advances come from the descendant `/W` array and `/DW`, or from the embedded sfnt program. Phase 2.5 lands the mapping.

Out of scope: predefined CMaps other than Identity-H, embedded CMap streams, Identity-V, vertical metrics with `/W2`, and CIDFontType0 CFF CID fonts.

## Out of scope

- Hinting and hint replacement. Outlines are rasterized as drawn and hint tables are ignored.
- Subsetting and writing fonts.
- Type 3 fonts, vertical writing, color fonts with COLR or CPAL, SVG glyphs, and variable font axes.
- OCR. Extraction reads the codes and ToUnicode that the document carries.
- Pixel parity with Ghostscript on text pages. Hinting and antialiasing differ, so text tests compare shapes and advances.
- ExpertEncoding, MacExpertEncoding, PDFDocEncoding, and the Symbol and ZapfDingbats built-in encodings. The three tables above are the phase 1 set.
- Kerning pairs from the AFM `KP` lines.
