# v0.0.4 - PDF coverage

> **Parent:** `plans/v0.0.4/00-program.md` - program ledger
> **Status:** implemented. `tiffsep` is deferred to `plans/v0.0.1/10-deferred.md` 10.2.
> **Estimated effort:** about eight weeks across six phases. Phase 6 is the cut line.

---

## Overview

The v0.0.3 tag paints paths, images, and text. What is left on the PDF side is the Deferred table at `documentation/features.md:71-82` plus the follow-ups at `plans/PR/pr-v0.0.3-deferred-work.md:238-246`. The content interpreter knows `m l c h re S s f f* n q Q cm w RG rg G g` and the text operators. `W`, `W*`, `B`, `B*`, `b`, `b*`, `BX`, `EX`, `gs`, `K`, `k`, `CS`, `cs`, `SC`, `sc`, `SCN`, `scn`, `sh`, `BMC`, `BDC`, and `EMC` return `undefined` with the operator name (`documentation/devices.md:250`). `Do` paints image XObjects only (`internal/pdf/do.go:40`), refuses `/SMask` (`:56-59`), and silently ignores `/Mask` and `/Decode`, which contradicts `documentation/covered-and-not-covered.md:31` and paints wrong pixels. There is no `gs` and no `/ExtGState` lookup (`internal/pdf/content.go:275-292`, `internal/pdf/resource.go:56-75`). Color is one RGB triple (`internal/graphics/state.go:10-17`). The generic stream decoder decodes Flate only; image streams also decode DCT, CCITT, and JPX. A filter chain fails on the first unknown filter (`internal/pdf/filter.go:26-37`). `sfnt` has no encoder, so nothing writes or subsets a font. The writers omit `/Info` on purpose (`documentation/devices.md:99`, `:103`).

`plans/v0.0.3/5-paint-do.md:66-70` records the exclusions this plan picks up: `/ImageMask`, `/SMask`, `/Mask`, `/Decode` arrays, interpolation, and CMYK images. One file holds six phases in dependency order. The row shape follows `plans/v0.0.3/5-paint-do.md`, and the phase shape follows `plans/v0.0.3/9-text-and-fonts.md`. The integrator owns `plans/v0.0.4/00-program.md`, the closure file, and the `plans/v0.0.1/10-deferred.md` moves.

## Executive summary

Phase 1 is the gate for phases 2 and 3: a transparency group and a soft mask are Form XObjects, and both need clip. Form XObjects also widen raster coverage on their own. Phase 2 adds alpha as optional interfaces beside the narrow `graphics.Marker`, so the rewrite recorder keeps refusing effects level 0 cannot write. Phase 3 normalizes color spaces and paints separations to the RGB preview; `tiffsep` is in scope as the last rows of the phase and the first to cut. Phase 4 subsets TrueType, copies OpenType whole, synthesizes `/ToUnicode` and widths, and gates it behind `rewrite -subset-fonts`. Phase 5 decodes LZW, ASCII85, ASCIIHex, and RunLength, applies Flate predictors, and skips content under an off OCG. Phase 6 is read-only `spectreps info` and is the cut line.

## Phase 1: Content graphics

Clip, forms, and `gs` are the gate for phases 2 and 3: a transparency group and a soft mask are Form XObjects (`internal/pdf/do.go:40`), and the state snapshot carries path, CTM, width, RGB, and text state only (`internal/pdf/content.go:37-51`, `:917-949`). The rewrite recorder is a second `graphics.Marker`, and `Emit` refuses a page that painted an image (`internal/pdfout/emit.go:67-71`). Marked content reading is owned by `plans/v0.0.4/3-pdfua2-tags.md` phase 1; this phase consumes parsed content and does not repeat that work.

- [x] `W` and `W*` intersect a clip region applied to later marks, and `q`/`Q` save and restore it. The rewrite recorder does not implement the seam and keeps `undefined in W`. Proof: `go test -count=1 ./internal/pdf -run TestPaintClip`, `TestPaintClipEvenOdd`, and `TestPaintClipRestore`.
- [x] `Do` on a `/Subtype /Form` XObject runs its content with `/Matrix` concatenated, `/BBox` as a clip, form `/Resources` or the page's inherited resources, and a recursion and depth cap that returns `limitcheck`. Proof: `go test -count=1 ./internal/pdf -run TestPaintFormXObject`, `TestPaintFormResources`, and `TestPaintFormRecursion`.
- [x] `BX` skips content through the matching `EX`; an unterminated section is `syntaxerror`. Proof: `go test -count=1 ./internal/pdf -run TestPaintCompatSection`.
- [x] `B`, `B*`, `b`, and `b*` fill and stroke the current path, with the even-odd rule for the starred forms and close for `b` and `b*`. Proof: `go test -count=1 ./internal/pdf -run TestPaintFillStroke`.
- [x] `/ExtGState` resolves from `/Resources` by name and `gs` pushes the known parameters into the graphics state; `q`/`Q` restore them. An unknown name is `undefined in gs`, and an unsupported entry is a named refusal, not a silent skip. Proof: `go test -count=1 ./internal/pdf -run TestPaintExtGStateResource`.
- [x] Decide and document `J`, `j`, `M`, `i`, and `ri`: accept as no-ops with the current capsule stroke and a docs deviation, or refuse by name. Proof: the chosen behavior in `go test -count=1 ./internal/pdf -run TestPaintLineParams`.
- [x] Level 0 stays honest: a page that clips still returns `undefined` with the operator name from `Emit`. Proof: `go test -count=1 ./internal/pdfout -run TestEmitClipUnchanged`.
- [x] Closure: `documentation/devices.md`, `features.md`, `covered-and-not-covered.md`, and `test.md` state the new operators; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 2: Transparency

The painter refuses `/SMask`, ignores `/Mask` and `/Decode`, and refuses `/ImageMask` by accident because the image has no `/BitsPerComponent` (`internal/pdf/image.go:127-139`). `graphics.Marker` is three methods with no alpha, blend, clip, or group seam (`internal/graphics/device.go:8-17`). `Pixmap.DrawImage` samples nearest neighbor and ignores source alpha (`internal/graphics/pixmap.go:113-142`); `DrawGlyph` blends coverage and nothing else. `snapshot` needs a field per new graphics-state parameter and a re-apply on `Q`.

- [x] The pixmap composites fill and stroke marks with a constant alpha, `out = src*a + dst*(1-a)` per channel, rounded. `q`/`Q` restore the alpha, and the rewrite recorder does not implement the seam and keeps refusing. Proof: `go test -count=1 ./internal/graphics -run TestPixmapAlphaBlend` and `go test -count=1 ./internal/pdf -run TestPaintExtGStateAlpha`.
- [x] `gs` carries `/CA` (stroke) and `/ca` (fill) alpha, and an unknown `/ExtGState` name is `undefined in gs`. Proof: `go test -count=1 ./internal/pdf -run TestPaintExtGStateRestore`.
- [x] An image `/SMask` decodes to an alpha plane, honors `/Matte` as a no-op with a documented deviation, and blends the base image through `DrawImage`. `TestPaintDoRejectedSMask` is retired. Proof: `go test -count=1 ./internal/pdf -run TestPaintSMask` and `go test -count=1 ./internal/pdf -run TestPaintSMaskDecode`.
- [x] `/ImageMask true` decodes one bit per sample, applies `/Decode`, and paints the current fill color where the mask is 1. Proof: `go test -count=1 ./internal/pdf -run TestPaintImageMask`.
- [x] A color-key `/Mask [min max ...]` builds coverage from the sample ranges, and a stream `/Mask` becomes a stencil. Proof: `go test -count=1 ./internal/pdf -run TestPaintColorKeyMask`.
- [x] `/Decode` arrays remap samples before color conversion, for gray, RGB, and masks. Proof: `go test -count=1 ./internal/pdf -run TestImageDecodeArray`.
- [x] The separable blend modes paint with the ISO 32000-1 formulas: Normal, Multiply, Screen, Overlay, Darken, Lighten, ColorDodge, ColorBurn, HardLight, SoftLight, Difference, Exclusion. The non-separable modes (Hue, Saturation, Color, Luminosity) are a named refusal. Proof: `go test -count=1 ./internal/pdf -run TestPaintBlendMode` (exact bytes for Multiply and Screen) and `go test -count=1 ./internal/pdf -run TestPaintBlendModeDefault`.
- [x] `/ExtGState /SMask` with `/S /Alpha` and `/S /Luminosity` builds the state soft mask and applies it to later marks; `/TR` carries identity only. Proof: `go test -count=1 ./internal/pdf -run TestPaintSoftMask` and `go test -count=1 ./internal/pdf -run TestPaintSoftMaskLuminosity`.
- [x] A Form XObject with `/Group /S /Transparency` paints into a scratch pixmap and composites once with group alpha, blend, `/CS`, `/I`, and `/K`. The scratch obeys the page pixel and side caps in `documentation/language.md:119-123`. Proof: `go test -count=1 ./internal/pdf -run TestPaintGroupTransparency` and `go test -count=1 ./internal/pdf -run TestPaintGroupIsolation`.
- [x] Closure: `documentation/features.md:11`, `devices.md:91`, `:93`, `covered-and-not-covered.md:31`, `test.md:76`, and `test.md:98` are corrected; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 3: Separations and spot colors

`imageParams` accepts `DeviceRGB` and `DeviceGray` only (`internal/pdf/image.go:135-137`). The only CMYK code is the output-only `rgbToCMYK` (`internal/pdfout/image.go:170-183`); there is no CMYK to RGB path and no `K`/`k`, `CS`/`cs`, or `SCN`/`scn`. The pass-through writer copies separation color spaces, tint function streams, and `/DeviceN` dictionaries unchanged at levels 1 through 5. The PDF/A preflight refuses a dictionary that names or resolves `DeviceCMYK` through a name, array, or reference, and does not read content streams (`internal/pdfa/preflight.go:180-217`); a `/Separation` or `/DeviceN` value nested inside a page `/ColorSpace` resource still passes. There is no `tiffsep`-style device or separation model (`documentation/covered-and-not-covered.md:39`).

- [x] A color space resolver in `internal/pdf/color.go` returns a component count and a preview RGB conversion for `DeviceRGB`, `DeviceGray`, `DeviceCMYK`, `Indexed`, `ICCBased` through `/Alternate`, and `CalRGB`/`CalGray` treated as RGB and gray with a documented deviation. Proof: `go test -count=1 ./internal/pdf -run TestColorSpaceDeviceCMYK`, `TestColorSpaceIndexed`, and `TestColorSpaceICCBased`.
- [x] Tint transforms evaluate type 2 sampled and type 4 PostScript calculator functions under the existing 32 MiB cap; type 0 and type 3 stay out. Proof: `go test -count=1 ./internal/pdf -run TestTintTransformSampled` and `go test -count=1 ./internal/pdf -run TestTintTransformCalculator`.
- [x] `/Separation` and `/DeviceN` load with their alternate space and tint transform; an unsupported space returns `undefined` with the image or paint operator name. Proof: `go test -count=1 ./internal/pdf -run TestSeparationColor` and `go test -count=1 ./internal/pdf -run TestDeviceNColor`.
- [x] `K`/`k`, `CS`/`cs`, and `SC`/`sc`/`SCN`/`scn` run, the current space and components survive `q`/`Q`, and a mark reaches the RGB pixmap through the preview conversion. Proof: `go test -count=1 ./internal/pdf -run TestCMYKPaint` and `go test -count=1 ./internal/pdf -run TestGenericColorOps`.
- [x] A separation page rasterizes to the RGB preview. Proof: `go test -count=1 ./internal/pdf -run TestSeparationRaster`.
- [~] A separation accumulator behind a new optional device seam records process C, M, Y, K and each named spot ink as its own plane. `tiffsep` is in scope: `spectreps tiffsep -o out.tif in.pdf` writes one grayscale TIFF per ink plus a composite, with deterministic names (`out.Cyan.tif`, `out.Black.tif`, `out.<SpotName>.tif`), reusing `golang.org/x/image/tiff` and the existing page selection. This is the largest row in the phase and the first to cut to v0.0.5 if phase 3 slips. Proof: `go test -count=1 ./internal/pdf -run TestSeparationPlates` and `go test -count=1 ./internal/cli -run TestTiffSep`, `TestTiffSepStable`, and `TestTiffSepCLI`. Cut to `plans/v0.0.1/10-deferred.md` 10.2 as the plan's declared first cut; the reader still resolves Separation and DeviceN to the RGB preview.
- [x] The PDF/A preflight applies `cmyk-without-profile` to a `/Separation` or `/DeviceN` value nested inside a page `/ColorSpace` resource, through a name, array, or reference. Direct dictionaries already refuse. Proof: `go test -count=1 ./internal/pdfa -run TestPreflightSeparation`.
- [x] Closure: freeze the CMYK to RGB rule next to the existing RGB to CMYK rule in `documentation/devices.md`; `features.md`, `covered-and-not-covered.md`, `cli.md`, and `test.md` state the new behavior; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 4: Font embedding and subsetting

The read side is done for the supported subset: `/Widths`, `/FirstChar`, `/MissingWidth`, `/FontDescriptor`, `/BaseFont`, `/Encoding` with `/Differences`, `/ToUnicode`, and Type0 Identity-H with `/CIDToGIDMap`, `/W`, and `/DW` (`internal/pdf/font.go:57-75`, `:123-155`, `:236-350`). Nothing writes or subsets. `golang.org/x/image/font/sfnt` has no encoder, and the pass-through writer copies source font objects and streams unchanged. `CopyOptions.Overrides` replaces a complete object body by number and `AppendObjects` adds bodies after the highest source number (`internal/pdfout/copy.go:28-41`); the pass-through writer copies `/Resources` and `/Font` untouched, so resource names survive a replaced body. Nothing records which codes a page used per font resource. PDF/A `font-not-embedded` refuses a font with no `/FontFile`, `/FontFile2`, or `/FontFile3`, and the standard 14 and Type 1 `/FontFile` have no outline program in the tree. `TaggedFontOK` is dictionary level only (`internal/pdf/structtree.go:1027-1088`).

- [x] A reader-side collector returns the used character codes per page and per font resource name: sorted, unique, capped, and empty for a page with no text. Proof: `go test -count=1 ./internal/pdf -run TestFontUsedCodes`.
- [x] `internal/font/write.go` parses and rebuilds the TrueType table directory, recomputes table checksums, and emits an sfnt that `sfnt.Parse` accepts. Proof: `go test -count=1 ./internal/font -run TestSubsetTrueTypeTables`.
- [x] The stable-GID `glyf` subset zeroes unused glyphs, follows composite component references, rebuilds `loca`, and keeps `cmap`, `hmtx`, `head`, and `maxp` valid, so content streams, `/Widths`, `/Differences`, `/Encoding`, and `/CIDToGIDMap` stay valid with no page re-encode. Proof: `go test -count=1 ./internal/font -run TestEmbedSubsetTrueType` and `TestEmbedSubsetComposite`.
- [x] The subset tag is six uppercase letters from a digest of the subset bytes, stable across runs, with no dates. Proof: `go test -count=1 ./internal/font -run TestEmbedSubsetTag` and `TestEmbedSubsetStable`.
- [x] `/ToUnicode` synthesis writes `bfchar` and `bfrange` CMaps for simple and Type0 fonts, and the parser at `internal/pdf/cmap.go` reads them back to the same strings. Proof: `go test -count=1 ./internal/pdf -run TestEmbedSubsetToUnicode`.
- [x] `/Widths`, `/FirstChar`, `/LastChar`, `/W`, and `/DW` writing covers a font with and without source width arrays, trimmed to used CIDs for Type0. Proof: `go test -count=1 ./internal/pdf -run TestEmbedSubsetWidths`.
- [x] A Type0 Identity-H subset keeps CIDs and `/CIDToGIDMap`, trims `/W`, and repaints the same glyphs after the round trip. Proof: `go test -count=1 ./internal/pdf -run TestEmbedSubsetCID`.
- [x] An OpenType `/FontFile3 /OpenType` program is copied whole and its dictionary keeps working; CFF subsetting waits. Proof: `go test -count=1 ./internal/pdf -run TestEmbedSubsetOpenType`.
- [x] `RewriteOptions.SubsetFonts` (off by default) and `spectreps rewrite -subset-fonts` apply subsetting on levels 1 through 5, leave level 0 refusing text, and exit 2 on a bad flag. Bytes change only when the caller opts in. Proof: `go test -count=1 ./internal/cli -run TestRewriteSubsetCLI`.
- [x] The PDF/A preflight still refuses a font with no embedded program with or without the option, and a conforming file with `/FontFile2` still passes. Proof: `go test -count=1 ./internal/pdfa -run TestEmbedSubsetPDFA`.
- [x] A tagged input keeps its tree, MCIDs, `/Alt`, `/ActualText`, and `/Lang` through a subsetting rewrite, and the synthesized `/ToUnicode` makes `TaggedFontOK` pass for a simple font with a nonstandard encoding. Proof: `go test -count=1 ./internal/pdfa -run TestEmbedSubsetUA2`.
- [x] Closure: `documentation/fonts.md`, `devices.md`, `features.md`, `covered-and-not-covered.md`, `public-api.md`, and `cli.md` state the option and the refusal policy; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 5: Filters and optional content

The generic stream decoder decodes Flate only, and every filter in a chain goes through `Decode`, so one known plus one unknown filter fails on the unknown one (`internal/pdf/filter.go:26-37`, `internal/pdf/page.go:272-306`). Predictors above 1 are rejected (`internal/pdf/filter.go:39-58`). Marked-content parsing lands in `plans/v0.0.4/3-pdfua2-tags.md` phase 1; `/OCProperties` is never read. `LZWDecode` is `undefined` and tested as such (`internal/pdf/filter_test.go:43-47`).

- [x] `LZWDecode` decodes in `internal/pdf/lzw.go` with EarlyChange handled and the 32 MiB cap. The PDF/A refusal for LZW stays, because PDF/A still disallows it. Proof: `go test -count=1 ./internal/pdf -run TestLZWDecode`.
- [x] `ASCII85Decode` and `ASCIIHexDecode` decode, including an ASCII85 over Flate pairing. Proof: `go test -count=1 ./internal/pdf -run TestASCII85Decode` and `go test -count=1 ./internal/pdf -run TestASCIIHexDecode`.
- [x] `RunLengthDecode` decodes. Proof: `go test -count=1 ./internal/pdf -run TestRunLengthDecode`.
- [x] Predictors 2 and 10 through 15 apply on Flate for xref, object, content, and image streams, with the 15 versus 12 row selection per row. Proof: `go test -count=1 ./internal/pdf -run TestFlatePredictor` and `go test -count=1 ./internal/pdf -run TestPredictorImage`.
- [x] The existing chain decode is locked by a test: each stage decodes in order, and an unknown stage returns `undefined` with that filter name. Proof: `go test -count=1 ./internal/pdf -run TestFilterChain`.
- [x] Optional content: `/OCProperties /D` is read, content under an `OFF` group is skipped, and `/OC` on XObjects and property dictionaries is honored. Alternate configs, the `/AS` usage map, and the visibility flag stay out. Proof: `go test -count=1 ./internal/pdf -run TestOCGVisibility` and `go test -count=1 ./internal/pdf -run TestOCGXObject`.
- [x] Level 2 re-encodes an LZW image the way it re-encodes CCITT. Proof: `go test -count=1 ./internal/pdfout -run TestLevelLZWImage`.
- [x] Closure: `documentation/devices.md`, `language.md`, `features.md`, `covered-and-not-covered.md`, and `test.md` state the decoders and the OCG rule; the `JBIG2Decode` disagreement between `allowedFilter` and the reader is recorded; `make lint` and `make test` exit 0, recorded in this row. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Phase 6: `spectreps info`

Read-only and the cut line. The writers omit `/Info` on purpose (`documentation/devices.md:99`, `:103`). This phase prints what the reader already knows and writes nothing.

- [x] `spectreps info file.pdf` prints the PDF version, page count and sizes, the tagged flag, an `Encrypt` refusal when the trailer carries `/Encrypt`, fonts with the embedded flag, and image count. Proof: `go test -count=1 ./internal/pdf -run TestPDFInfo`.
- [x] The CLI command maps bad input and a bad flag to the documented exit codes. Proof: `go test -count=1 ./internal/cli -run TestPDFInfoCLI`.
- [x] Closure: `documentation/cli.md`, `features.md`, and `public-api.md` state the command, and `make lint` and `make test` exit 0, recorded in this row. If phases 1 through 5 run long, this phase moves to v0.0.5 whole. Proof: `make lint` and `make test`. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

## Open decisions

1. `tiffsep`: in scope as the last rows of phase 3, or cut to v0.0.5. This file takes it in scope and marks the row as the first cut.
2. Blend modes: all twelve separable modes in one row, or Normal, Multiply, and Screen first. This file takes all twelve and refuses the non-separable four.
3. Level 0 and clipping: keep refusing `W` in the recorder, or make the recorder emit `W n`. This file keeps the refusal, because the recorder cannot express the clip path yet.
4. Font subsetting surface: `RewriteOptions.SubsetFonts` plus `rewrite -subset-fonts`, or a standalone `spectreps subset` command. This file takes the option and the flag.
5. OpenType CFF: copy `/FontFile3 /OpenType` whole, or attempt a CFF subset in v0.0.4. This file copies whole.
6. PDF/A blend rule: keep refusing `/BM` other than `Normal` after blend modes paint, or align with PDF/A-4, which permits standard blend modes. This file keeps the refusal until a separate decision changes it.
7. `spectreps info`: phase 6 now, or v0.0.5. This file marks it as the cut line.
8. ICC handling: `/Alternate` only, or a pure-Go profile transform dependency. This file takes `/Alternate` only and states the deviation.
9. `J`, `j`, `M`, `i`, `ri`: accept as no-ops with a docs deviation, or keep refusing unknown operators. Phase 1 row decides and records it.
10. Encryption: start its own plan now, or leave it at `plans/v0.0.1/10-deferred.md` with the new gate recorded. This file puts it out and the integrator records the gate.

## Recorded outcomes

The decisions as landed on 2026-09-26: 1 cut `tiffsep` to `plans/v0.0.1/10-deferred.md` 10.2; 2 landed all twelve separable modes and refuses the four non-separable ones by name; 3 keeps the level 0 recorder refusing `W`, `W*`, and `Do`; 4 landed `RewriteOptions.SubsetFonts` plus `rewrite -subset-fonts`; 5 copies `/FontFile3 /OpenType` whole; 6 keeps the PDF/A `/BM` refusal for any mode other than Normal; 7 landed `spectreps info` in phase 6; 8 reads ICC `/Alternate` only and states the deviation; 9 accepts `J`, `j`, `M`, `i`, `ri`, `d`, and `Tr` as no-ops with a documented deviation; 10 keeps encryption out with the gate recorded in `plans/v0.0.1/10-deferred.md` 10.2.

## Dependencies

- Phase 1 uses `DecodeImage`, the page-tree walk, `graphics.Matrix`, and the pass-through writer from v0.0.2 and v0.0.3.
- Phases 2 and 3 depend on phase 1 for Form XObjects, clip, and the state snapshot.
- Phase 4 depends on `golang.org/x/image/font/sfnt` and `CopyOptions.Overrides`, both already in the module.
- Phase 5 is independent and can land in parallel.
- Phase 6 depends on phase 4 for the font report.
- No new module. The closure file and `plans/v0.0.4/00-program.md` belong to the integrator.

## Not in this plan

- Encryption, linearization, and output encryption. Encryption needs key derivation (RC4, AES-128, AES-256), a crypt filter model, and decryption at every string and stream read, so it starts its own plan.
- Full color management: profile transforms, rendering intents, black point compensation, and CMYK ICC. Phase 3 reads `/Alternate` only.
- PDF/UA-2 tag generation is its own plan, `plans/v0.0.4/3-pdfua2-tags.md`. This plan's `/ToUnicode` synthesis and tagged round trip feed that work.
- Type 1 charstrings and `seac` are their own v0.0.4 plan, `plans/v0.0.4/2-type1-fonts.md`. Type 3, vertical writing, color fonts, and variable fonts stay out, per `documentation/fonts.md` and `plans/v0.0.1/10-deferred.md` 10.1.
- Inline images (`BI`/`ID`/`EI`), shading (`sh`), patterns, and dash patterns. Each is a new painting model, not a decode row.
- JBIG2 decode, JPEG2000 encode, PCLm, and printer languages. PCLm is `documentation/covered-and-not-covered.md:38`, the printer gate is `plans/v0.0.1/10-deferred.md` 10.3, and JBIG2 is recorded in this plan's phase 5 closure.
- The non-separable blend modes, alternate OCG configs, the `/AS` usage map, and the OCG visibility flag.
- `/Info` writing. Dates would break the stable-byte promise, so phase 6 reads only.
