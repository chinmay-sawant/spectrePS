# Benchmarks

Every benchmark in the tree, with the numbers this machine produced. The
narrative reading of these numbers is in `documentation/performance.md`; this
file is the raw record, so a reader can check a claim there against the row
here.

| Item | Value |
| --- | --- |
| Command | `make bench` |
| Underlying | `go test -p 1 -run '^$' -bench . -benchmem -count=3` over eleven packages |
| Capture | `profiles/bench.txt`, 324 lines, 258 benchmark lines |
| CPU | 13th Gen Intel Core i7-13700HX, 24 threads |
| Memory | 7.6 GiB |
| OS | linux/amd64 |
| Go | go1.26.4 |
| Run date | 2026-09-26 |
| Benchmarks | 85 distinct names, 86 rows |

`ns/op` is the median of the three `-count=3` counts. `B/op` and `allocs/op`
matched across all three counts in every row, so they are exact and they do not
depend on the clock. `ns/op` is machine-specific and moved by several percent
between runs on this box, which is why no timing number gates `make test` and
only the allocation ceilings in `Budget` do.

**The `ns/op` column is a first reading, not a baseline.** `make bench-check`
compares `-count=3` against `-count=5`, which is below the six samples benchstat
needs before it computes a confidence interval, and the resulting verdict flagged
107 of 323 rows on a tree where no production file had changed. Read every
timing value below as magnitude only, and rely on `B/op` and `allocs/op`. The
gate for re-capturing at `-count=10` is deferred row 4.3 of
`plans/v0.0.5/5-baseline-and-budget.md`.

`profiles/` is gitignored, so this file is the durable copy. Regenerate it with
`make bench` and re-read `documentation/performance.md` before treating any
number here as current, because the analysis is written by hand and does not
update itself.

`BenchmarkPreflightUA2` appears twice on purpose. One row is the library job
through `spectreps`, the other is the preflight through `internal/pdfa`. They
run the same code over the same sample and the counts agree.

## What the v0.0.5 extension added

The tree had 35 benchmark functions producing 40 result rows before this work
and has 67 producing 86 after. Five packages had no `bench_test.go` at all, and
those five account for 13 of the 32 new functions.

| Package | Functions before | After | Added |
| --- | --- | --- | --- |
| `spectreps` | 18 | 23 | 5 |
| `internal/cli` | 6 | 10 | 4 |
| `internal/graphics` | 3 | 10 | 7 |
| `internal/pdf` | 3 | 4 | 1 |
| `internal/pdfout` | 4 | 6 | 2 |
| `internal/ps` | 1 | 1 | 0 |
| `internal/engine` | 0 | 1 | 1 |
| `internal/font` | 0 | 5 | 5 |
| `internal/pdfa` | 0 | 3 | 3 |
| `internal/psout` | 0 | 2 | 2 |
| `internal/tag` | 0 | 2 | 2 |
| Total | 35 | 67 | 32 |

The functions, by the phase that added them:

| Phase | New functions |
| --- | --- |
| `1-graphics-device.md` | `FillComplexity`, `FillSubpaths`, `StrokeClipped`, `FillClipped`, `CompositeGroupClipped`, `DrawGlyph`, `ShowPage` |
| `2-encoders.md` | `EncodePPM`, `EncodePNG`, `EncodeJPEG`, `EncodeTIFF`, `WriteImagesColor`, `EncodeFlateRGB`, `DecodeFlate` |
| `3-font-and-metadata.md` | `Standard14Lookup`, `EncodingLookup`, `AGLUnicode`, `LoadType1`, `Subset`, `Preflight`, `PreflightUA2`, `XMPPacket`, `CompareBytes` |
| `4-reading-and-writing.md` | `DocumentInfo`, `PreflightUA2`, `WritePostScript`, `RewritePDFTagged`, `RewritePDFA`, `DerivePlan`, `TaggedBuild`, `Emit`, `Write` |

No production file changed. Every function above lives in a `_test.go` file, and
the only non-test edits are the `BENCH_PKGS` list in the `Makefile` and the
documentation.

## spectreps

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkRunPostScript | 2541794 | 753512 | 6072 |
| BenchmarkOpenPDF/classic | 14134 | 23168 | 84 |
| BenchmarkOpenPDF/xref-stream | 1076329 | 1240786 | 3979 |
| BenchmarkRasterizePage | 1203335 | 349787 | 2425 |
| BenchmarkRewriteLevel0 | 614365 | 244810 | 6061 |
| BenchmarkRewriteLevel1 | 1291272 | 3254875 | 1984 |
| BenchmarkRewriteLevel2 | 1403116 | 3256797 | 1986 |
| BenchmarkRewriteLevel3 | 701344185 | 226175652 | 8702229 |
| BenchmarkRewriteLevel4 | 572120346 | 169885716 | 8702227 |
| BenchmarkRewriteLevel5 | 524141094 | 145882044 | 8702221 |
| BenchmarkExtractText | 11211 | 19080 | 60 |
| BenchmarkImagePDF | 228411 | 5341 | 44 |
| BenchmarkMeasureBox | 322434 | 0 | 0 |
| BenchmarkMeasureInk | 50818 | 0 | 0 |
| BenchmarkMeasureInkAmount | 39074 | 0 | 0 |
| BenchmarkCompareRaster | 1647 | 0 | 0 |
| BenchmarkCompareFiles | 143285 | 0 | 0 |
| BenchmarkReuseInstance | 1217084 | 349789 | 2425 |
| BenchmarkPerCallInstance | 1271459 | 372958 | 2509 |
| BenchmarkDocumentInfo | 1359 | 184 | 7 |
| BenchmarkPreflightUA2 | 378477 | 168936 | 4512 |
| BenchmarkWritePostScript | 495039 | 208707 | 5628 |
| BenchmarkRewritePDFTagged | 1505441 | 1440061 | 12094 |
| BenchmarkRewritePDFA | 81110 | 89604 | 147 |

## CLI

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkCLIVersion | 158.2 | 16 | 1 |
| BenchmarkCLIRaster/72 | 5074591 | 6425703 | 2556 |
| BenchmarkCLIRaster/300 | 75193677 | 109513591 | 2557 |
| BenchmarkCLIRewrite/level2 | 4063261 | 5745679 | 6766 |
| BenchmarkCLIRewrite/level5 | 537481376 | 148046056 | 8706622 |
| BenchmarkCLIPDFImage | 2962398 | 1123858 | 6161 |
| BenchmarkCLIText | 27121 | 29244 | 183 |
| BenchmarkCLIGS | 163148 | 186082 | 306 |
| BenchmarkEncodePPM | 54055 | 246052 | 4 |
| BenchmarkEncodePNG | 709869 | 1016390 | 34 |
| BenchmarkEncodeJPEG | 504297 | 169776 | 11 |
| BenchmarkEncodeTIFF/deflate | 540593 | 981423 | 49 |
| BenchmarkEncodeTIFF/none | 164556 | 330558 | 27 |

## Engine

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkCompareBytes/equal | 236164 | 0 | 0 |
| BenchmarkCompareBytes/last-byte | 241644 | 0 | 0 |

## Font

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStandard14Lookup | 379.6 | 0 | 0 |
| BenchmarkEncodingLookup | 52444 | 0 | 0 |
| BenchmarkAGLUnicode | 49.76 | 0 | 0 |
| BenchmarkLoadType1/parse | 20970 | 32000 | 163 |
| BenchmarkLoadType1/glyph | 497.0 | 648 | 15 |
| BenchmarkSubset/subset | 2354 | 3032 | 27 |
| BenchmarkSubset/tag | 284.5 | 8 | 1 |

## PDF reader

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkOpenClassicXRef | 148010 | 238546 | 91 |
| BenchmarkOpenXRefStream | 63349 | 204963 | 129 |
| BenchmarkPaintPage | 2608446 | 312227 | 11991 |
| BenchmarkDecodeFlate | 1728564 | 5253305 | 27 |

## PDF/A and PDF/UA-2

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkPreflight | 1653991 | 4938 | 12 |
| BenchmarkPreflightUA2 | 406076 | 168930 | 4511 |
| BenchmarkXMPPacket/pdfa4 | 0.1534 | 0 | 0 |
| BenchmarkXMPPacket/ua2 | 2596 | 9142 | 13 |

## PDF writers

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkWriteCopy/path | 19449 | 31261 | 83 |
| BenchmarkWriteCopy/image | 1346286 | 3248528 | 1823 |
| BenchmarkLevel5ImageReEncode | 530257040 | 145437604 | 8700443 |
| BenchmarkScaleImage | 51196776 | 34713829 | 8 |
| BenchmarkEncodeDCT | 5642756 | 66032 | 12 |
| BenchmarkWriteImagesColor/rgb | 221533 | 4635 | 43 |
| BenchmarkWriteImagesColor/gray | 202610 | 65268 | 43 |
| BenchmarkWriteImagesColor/cmyk | 640379 | 256786 | 47 |
| BenchmarkEncodeFlateRGB | 14909755 | 3193154 | 532160 |

## Graphics device

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStroke | 91095 | 0 | 0 |
| BenchmarkFill | 9735132 | 192 | 2 |
| BenchmarkDrawImage/1to1 | 114.8 | 4 | 1 |
| BenchmarkDrawImage/scaled | 711.2 | 64 | 16 |
| BenchmarkFillComplexity/4 | 968770 | 192 | 2 |
| BenchmarkFillComplexity/32 | 7689271 | 1664 | 2 |
| BenchmarkFillComplexity/128 | 26492447 | 6272 | 2 |
| BenchmarkFillSubpaths | 21577169 | 11480 | 95 |
| BenchmarkStrokeClipped | 10995205 | 1900792 | 6 |
| BenchmarkFillClipped | 10318785 | 164255 | 8 |
| BenchmarkCompositeGroupClipped/group-only | 37632 | 0 | 0 |
| BenchmarkCompositeGroupClipped/clipped | 890755 | 164064 | 6 |
| BenchmarkDrawGlyph/full | 29528 | 0 | 0 |
| BenchmarkDrawGlyph/half | 29117 | 0 | 0 |
| BenchmarkDrawGlyph/zero | 9706 | 0 | 0 |
| BenchmarkShowPage | 774935 | 1458197 | 1 |

## PostScript

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStrokeProgram | 2560789 | 753417 | 6070 |

## PostScript writer

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkEmit | 2323015 | 760994 | 25979 |
| BenchmarkWrite | 391634 | 1523990 | 9 |

## Structure tree

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkDerivePlan/50 | 74667 | 82876 | 706 |
| BenchmarkDerivePlan/200 | 379228 | 328702 | 2662 |
| BenchmarkDerivePlan/800 | 2448927 | 1310273 | 10468 |
| BenchmarkTaggedBuild | 116797 | 95515 | 1350 |
