# Performance

The benchmark suite measures three surfaces: the public `spectreps` package,
the `internal/cli` command layer, and the hot internal packages (`pdf`,
`pdfout`, `graphics`, `ps`). It runs on demand and never inside `make test`.
Output lands in `profiles/`, which is gitignored. The accepted baseline, the
allocation budget, and the findings live here.

The v0.0.4 suite covered six packages and 35 benchmark functions. The v0.0.5
coverage extension adds five packages and 32 more functions, so the tree now has
67 functions producing 86 result rows, because sub-benchmarks expand. The v0.0.4
tables below are the accepted comparison and this file does not re-record them.
The extension is in `Coverage extension`, and every row is in
`documentation/benchmark.md`.

## Machine

| Item | Value |
| --- | --- |
| CPU | 13th Gen Intel Core i7-13700HX, 24 threads |
| Memory | 7.6 GiB |
| OS | linux/amd64 |
| Go | go1.26.4 |
| Run date | 2026-09-26 |
| Load | other work ran on the same box; the load average moved between 2 and 5 |

Timing comes from this machine and this toolchain. A different box will
produce different ns/op values. `B/op` and `allocs/op` are exact allocation
counts and do not depend on the clock.

## Commands

| Command | Writes | What it runs |
| --- | --- | --- |
| `make bench` | `profiles/bench.txt` | `go test -p 1 -run '^$' -bench . -benchmem -count=3` over the eleven benchmark packages |
| `make bench-profile` | `profiles/<pkg>.cpu`, `profiles/<pkg>.mem` | one CPU and one heap profile per package |
| `make bench-check` | `profiles/compare.txt` | a `-count=5` rerun compared against `profiles/bench.txt` with benchstat |
| `bash scripts/bench-cli.sh` | `profiles/cli.txt` | the built binary, one command per row, min and median of 10 runs |
| `go test -run '^$' -bench . -benchmem ./spectreps` | stdout | one package at a time |

`-p 1` serializes the eleven packages so their benchmarks do not share cores.

## Inputs

`sampledata/compress/path.pdf` is a one-page path file with a classic xref and
about 400 horizontal strokes at 0.5 width. The 0.5 width paints no pixel at 72
dpi, so the measure benchmarks use the synthetic program instead.
`sampledata/compress/whatisthis.pdf` carries a Flate xref stream, one object
stream, a text layer, and one DCT image. `sampledata/fixtures/text.pdf` carries
two lines of Helvetica text.

The synthetic program is 2000 horizontal strokes at width 1 on a 200 by 200
point page:

```
0 1 1999 { /y exch def 0 y moveto 200 y lineto 1 setlinewidth stroke } for
```

## Baseline

The `ns/op` column is the median of the three `-count=3` counts. `B/op` and
`allocs/op` matched across the counts in nearly every cell, so they are exact.
The load on the box moved a few ns/op cells by up to 3x between counts; the
median is the recorded value.

### Library, `./spectreps`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkRunPostScript | 2730758 | 701185 | 8069 |
| BenchmarkOpenPDF/classic | 15034 | 23168 | 84 |
| BenchmarkOpenPDF/xref-stream | 1052753 | 1240752 | 3979 |
| BenchmarkRasterizePage | 1411585 | 307110 | 2415 |
| BenchmarkRewriteLevel0 | 697853 | 158982 | 6049 |
| BenchmarkRewriteLevel1 | 1893881 | 3251874 | 1977 |
| BenchmarkRewriteLevel2 | 1785536 | 3253817 | 1979 |
| BenchmarkRewriteLevel3 | 512952881 | 163534504 | 2381 |
| BenchmarkRewriteLevel4 | 376457558 | 107136602 | 2253 |
| BenchmarkRewriteLevel5 | 338656093 | 83133162 | 2250 |
| BenchmarkExtractText | 9129 | 13440 | 52 |
| BenchmarkImagePDF | 308001 | 4565 | 44 |
| BenchmarkMeasureBox | 398711 | 0 | 0 |
| BenchmarkMeasureInk | 55176 | 0 | 0 |
| BenchmarkMeasureInkAmount | 41721 | 0 | 0 |
| BenchmarkCompareRaster | 2067 | 0 | 0 |
| BenchmarkCompareFiles | 155671 | 0 | 0 |
| BenchmarkReuseInstance | 1310224 | 307110 | 2415 |
| BenchmarkPerCallInstance | 1812490 | 330288 | 2499 |

Level 0 rewrites `path.pdf`; levels 1 through 5 rewrite `whatisthis.pdf`,
because level 0 refuses its text layer with `undefined in W`.

### CLI, `./internal/cli`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkCLIVersion | 163 | 16 | 1 |
| BenchmarkCLIRaster/72 | 15169635 | 5932151 | 2545 |
| BenchmarkCLIRaster/300 | 158454140 | 101090491 | 2547 |
| BenchmarkCLIRewrite/level2 | 6289038 | 5744913 | 6753 |
| BenchmarkCLIRewrite/level5 | 377733775 | 85405968 | 6765 |
| BenchmarkCLIPDFImage | 2737650 | 713236 | 8145 |
| BenchmarkCLIText | 39450 | 23568 | 175 |
| BenchmarkCLIGS | 176921 | 118893 | 273 |

`BenchmarkCLIRaster` runs `path.pdf` on the CLI default page, 612 by 792
points, so the 300 dpi cell is 2550 by 3300 pixels, not the 200 by 200 point
geometry the script uses later.

### Layers

`./internal/pdf`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkOpenClassicXRef | 169044 | 238547 | 91 |
| BenchmarkOpenXRefStream | 76009 | 204964 | 129 |
| BenchmarkPaintPage | 1824811 | 311331 | 11986 |

`./internal/pdfout`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkWriteCopy/path | 23722 | 31258 | 83 |
| BenchmarkWriteCopy/image | 2124363 | 3248943 | 1823 |
| BenchmarkLevel5ImageReEncode | 339088826 | 82690973 | 474 |
| BenchmarkScaleImage | 67201393 | 34713835 | 8 |
| BenchmarkEncodeDCT | 6740544 | 66032 | 12 |

`./internal/graphics`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStroke | 90770 | 0 | 0 |
| BenchmarkFill | 11509710 | 192 | 2 |
| BenchmarkDrawImage/1to1 | 116 | 4 | 1 |
| BenchmarkDrawImage/scaled | 578 | 64 | 16 |

`./internal/ps`

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStrokeProgram | 2562173 | 701129 | 8067 |

## Coverage extension

Recorded on 2026-09-26 on the machine above, go1.26.4, from one `make bench`
run over the eleven packages. The plan is `plans/v0.0.5/`. The `ns/op` column
is the median of the three counts and `B/op` and `allocs/op` matched across all
three in every cell. The v0.0.4 tables above are not re-recorded here, and these
numbers are not a comparison against them, because the two sets do not measure
the same work.

**The `ns/op` values below are a first reading, not a baseline.** The `bench-check`
section shows that `make bench-check` compares `-count=3` against `-count=5`,
which is below the six samples benchstat needs before it will compute a
confidence interval, and that the resulting verdict flagged 107 of 323 rows on a
tree where no production file had changed. Treat every timing number in this
section as indicative of magnitude only. The `B/op` and `allocs/op` columns are
exact and are the part to rely on. Deferred row 4.3 of
`plans/v0.0.5/5-baseline-and-budget.md` is the gate for re-capturing at
`-count=10`, and until that row closes no timing number here is a baseline.

### Fill at path complexity

`Pixmap.Fill` (`internal/graphics/pixmap.go:137`) walks every pixel of the page
and calls `inside` (`internal/graphics/pixmap.go:385`) for each one, and
`inside` walks every subpath segment. The cost is `W*H*N`. The v0.0.4 suite
measured it at N of 4 only, on a 612 by 792 page.

| Benchmark | Points | ns/op | B/op | allocs/op |
| --- | --- | --- | --- | --- |
| BenchmarkFillComplexity/4 | 4 | 968770 | 192 | 2 |
| BenchmarkFillComplexity/32 | 32 | 7689271 | 1664 | 2 |
| BenchmarkFillComplexity/128 | 128 | 26492447 | 6272 | 2 |
| BenchmarkFillSubpaths | 128 in 32 subpaths | 21577169 | 11480 | 95 |

The page is 200 by 200, so the 128 point row is 5.1 million inside tests. Going
from 4 to 32 points cost 7.9 times and from 32 to 128 cost 3.4 times, so the cost
is close to linear in N on this page and the constant is the page area. The
allocation count does not move with N, because the only allocation is the
subpath slice. `FillSubpaths` splits the same 128 segments across 32 subpaths
and costs 21.6 ms against 26.5 ms for one subpath, because `cross` rejects a
segment early when the pixel is outside its scanline.

### Clip, glyph, and page

The clip paths had no benchmark. Each clipped mark allocates a `rectSnapshot`
of two fresh byte slices at `internal/graphics/clip.go:152` and walks the same
rectangle again in `restoreOutside` at `internal/graphics/clip.go:171`.

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStrokeClipped | 10995205 | 1900792 | 6 |
| BenchmarkFillClipped | 10318785 | 164255 | 8 |
| BenchmarkCompositeGroupClipped/group-only | 37632 | 0 | 0 |
| BenchmarkCompositeGroupClipped/clipped | 890755 | 164064 | 6 |
| BenchmarkDrawGlyph/full | 29528 | 0 | 0 |
| BenchmarkDrawGlyph/half | 29117 | 0 | 0 |
| BenchmarkDrawGlyph/zero | 9706 | 0 | 0 |
| BenchmarkShowPage | 774935 | 1458197 | 1 |

`CompositeGroupClipped` costs 23.7 times the same composite with no clip, and
the 164 KB is the whole 200 by 200 page, because the snapshot at
`internal/graphics/clip.go:78` covers the page rather than the group bounds.
`DrawGlyph/zero` is a third of the full case, which is the walk with nothing to
write, so the early return at `internal/graphics/pixmap.go:259` is not a
shortcut. `ShowPage` copies 1.46 MB, which is the pixel plane at letter size.

### Output encoders

The v0.0.4 CLI raster benchmark wrote PPM, so no benchmark in the tree had
encoded a PNG, a JPEG, or a TIFF. All four share one painted 200 by 200 page.

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkEncodePPM | 54055 | 246052 | 4 |
| BenchmarkEncodeJPEG | 504297 | 169776 | 11 |
| BenchmarkEncodePNG | 709869 | 1016390 | 34 |
| BenchmarkEncodeTIFF/none | 164556 | 330558 | 27 |
| BenchmarkEncodeTIFF/deflate | 540593 | 981423 | 49 |

`BenchmarkEncodePPM` allocates 246 KB for a 120 KB page, because
`internal/cli/run.go:684` copies the body again to prepend the header. PNG is
the most expensive of the four and allocates 1.0 MB, of which 160 KB is the
`W*H*4` RGBA conversion in `rgbaFromPage`. The TIFF deflate case costs 3.3 times
the uncompressed case for a file 3.0 times larger.

### Image packers

`BenchmarkImagePDF` in `spectreps` covered `ImageColorRGB` only, so the gray and
CMYK branches of `packSamples` (`internal/pdfout/image.go:124`) had never been
timed.

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkWriteImagesColor/rgb | 221533 | 4635 | 43 |
| BenchmarkWriteImagesColor/gray | 202610 | 65268 | 43 |
| BenchmarkWriteImagesColor/cmyk | 640379 | 256786 | 47 |
| BenchmarkEncodeFlateRGB | 14909755 | 3193154 | 532160 |

CMYK costs 2.9 times RGB, which is the per pixel `rgbToCMYK` at
`internal/pdfout/image.go:171`. Gray is 0.9 times RGB because the page is black
on white and the luma of a white pixel is one byte against three.

`EncodeFlateRGB` allocates 532,160 objects for a 842 by 632 image, which is
532,144 pixels, so it is one allocation per pixel. The encoder reads through the
`image.Image` interface at `internal/pdfout/scale.go:47`, and the returned
`color.Color` escapes to the heap on every call. It also writes one zlib row per
scanline at `internal/pdfout/scale.go:53`, so 632 calls reach the writer where
one would do. This is the largest allocation count in the tree by two orders of
magnitude and it was invisible before this extension.

### Flate decode

| Benchmark | ns/op | MB/s | B/op | allocs/op |
| --- | --- | --- | --- | --- |
| BenchmarkDecodeFlate | 1728564 | 606.62 | 5253305 | 27 |

One 1 MiB buffer through `Decode` (`internal/pdf/filter.go:93`) allocates 5.25
MB, which is 5.25 times the output. `readLimited`
(`internal/pdf/filter.go:649`) starts from a 4096 byte buffer and grows by
append, so the output is copied through about 9 reallocations. The 606 MB/s is
the throughput, and it is acceptable. The allocation factor is not, and it is
the same shape as the accepted Flate decode row in `Findings`.

### Font, metadata, and the byte compare

Three packages had no benchmark. `internal/font` is 6,820 implementation lines
and `internal/pdfa` is 2,070, and both sit on a text or a rewrite path.

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkStandard14Lookup | 379.6 | 0 | 0 |
| BenchmarkAGLUnicode | 49.76 | 0 | 0 |
| BenchmarkEncodingLookup | 52444 | 0 | 0 |
| BenchmarkLoadType1/parse | 20970 | 32000 | 163 |
| BenchmarkLoadType1/glyph | 497.0 | 648 | 15 |
| BenchmarkSubset/subset | 2354 | 3032 | 27 |
| BenchmarkSubset/tag | 284.5 | 8 | 1 |
| BenchmarkPreflight | 1653991 | 4938 | 12 |
| BenchmarkPreflightUA2 | 371776 | 168936 | 4512 |
| BenchmarkXMPPacket/pdfa4 | 0.1534 | 0 | 0 |
| BenchmarkXMPPacket/ua2 | 2596 | 9142 | 13 |
| BenchmarkCompareBytes/equal | 236164 | 0 | 0 |
| BenchmarkCompareBytes/last-byte | 241644 | 0 | 0 |

Every font lookup is allocation-free, so the standard 14 tables and the AGL map
are not a memory cost. `EncodingLookup` walks 768 code and name pairs in 52
microseconds, which is 68 ns per pair, and the text job pays it per shown code.
`LoadType1/parse` is 42 times the glyph interpret, so a document that shows
every glyph once pays the parse once and the interpret many times.

`BenchmarkPreflight` is 1.65 ms, the most expensive single call in the tree
outside a rewrite, and it is the only PDF/A benchmark that was missing. It walks
every object in `sampledata/pdfa/compliant-a4.pdf`. `PreflightUA2` allocates
4,512 objects for a second content scan at `internal/pdfa/ua2_content.go:27`.
The PDF/A XMP packet is a constant string, which is why it costs 0.15 ns and 0
allocations; the UA-2 packet is built per call and costs 2.6 microseconds.

### Reading order and the tagged write

`DerivePlan` is the only superlinear loop on a public job. `orderFlow` at
`internal/tag/reading.go:897` is `O(n^2)` with a slice shift per removal.

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | --- | --- | --- | --- |
| BenchmarkDerivePlan/50 | 50 | 74667 | 82876 | 706 |
| BenchmarkDerivePlan/200 | 200 | 379228 | 328702 | 2662 |
| BenchmarkDerivePlan/800 | 800 | 2448927 | 1310273 | 10468 |
| BenchmarkTaggedBuild | 20 | 116797 | 95515 | 1350 |
| BenchmarkEmit | 2000 segments | 2323015 | 760994 | 25979 |

Four times the runs cost 5.1 times the time from 50 to 200 and 6.5 times from
200 to 800, so the curve steepens exactly where `orderFlow` starts to dominate.
800 runs is 2.4 ms on one page, which is the worst case for a text-dense page
and the best case for a document that is mostly paths.

`BenchmarkEmit` allocates 25,979 objects for 2,000 path segments, which is 13
per segment, against 5,628 for the whole of `WritePostScript`.

### Library jobs with no benchmark

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkDocumentInfo | 1359 | 184 | 7 |
| BenchmarkPreflightUA2, library | 371776 | 168936 | 4512 |
| BenchmarkWritePostScript | 495039 | 208707 | 5628 |
| BenchmarkRewritePDFTagged | 1505441 | 1440061 | 12094 |
| BenchmarkRewritePDFA | 81110 | 89604 | 147 |

`DocumentInfo` is cheap, so the page tree walk and the font sort are not a cost
worth a fix. `RewritePDFTagged` is 18.6 times `RewritePDFA` on the same input,
and it is the most expensive writer in `RewritePDF`. The difference is the
recorder, `DerivePlan`, the structure build, and the `PreflightUA2` call the
builder makes on its own output, against an XMP packet, an ICC profile, and an
output intent.

### Profiles for the new packages

`make bench-profile` writes one CPU and one heap profile per package, so all
eleven are available. Three carry most of the new cost.

`internal/pdfout` CPU, 15260 ms of samples over 13.14 s:

| Function | Flat | Cumulative |
| --- | --- | --- |
| compress/flate.(*compressor).deflate | 9.04 percent | 16.64 percent |
| x/image/draw.(*kernelScaler).scaleX_RGBA | 7.40 percent | 7.40 percent |
| runtime.memmove | 5.77 percent | 5.77 percent |
| crypto/internal/fips140/sha256.blockSHANI | 5.44 percent | 5.44 percent |

`internal/pdfa` CPU, 5420 ms of samples over 3.96 s:

| Function | Flat | Cumulative |
| --- | --- | --- |
| internal/runtime/maps.(*Iter).Next | 10.15 percent | 13.28 percent |
| pdf.(*File).ObjectCount | 7.38 percent | 22.14 percent |
| strings.(*Replacer).build | 4.06 percent | 11.99 percent |

`ObjectCount` at 22.14 percent cumulative is the whole of the 1.65 ms
`BenchmarkPreflight` figure, and `maps.(*Iter).Next` beside it says why: the
preflight walks the xref map to learn the object count, and that walk is the
cost. `strings.(*Replacer).build` is the UA-2 XMP builder recompiling its
replacer on every call, which is the 2.6 microsecond `BenchmarkXMPPacket/ua2`
row and a fix in one line if anyone wants it.

`internal/tag` CPU, 7360 ms of samples over 5.84 s:

| Function | Flat | Cumulative |
| --- | --- | --- |
| math.archMax | 9.51 percent | 9.51 percent |
| tag.runText | 5.71 percent | 15.22 percent |
| tag.actualTextOf | 3.40 percent | 8.97 percent |

`math.archMax` at 9.51 percent is the geometry work in the reader, and
`runText` plus `actualTextOf` at 24 percent cumulative is where
`BenchmarkDerivePlan` spends its time. The `orderFlow` quadratic is real but it
is not the largest term at 800 runs.

The `internal/pdfout` heap profile is the one that changed a conclusion.
Ranked by allocated objects rather than bytes:

| Function | Share of all objects |
| --- | --- |
| image.(*RGBA).At | 48.52 percent |
| image.(*YCbCr).At | 21.72 percent |
| pdf.(*lexer).parseDict | 6.72 percent |
| pdf.(*lexer).scanName | 3.71 percent |
| pdf.(*lexer).scanWord | 3.57 percent |

`image.(*RGBA).At` and `image.(*YCbCr).At` together are 70.24 percent of every
object the writer package allocates. Both return a `color.Color` interface
value, and the concrete colour struct escapes to the heap on each call, so
every pixel read through the `image.Image` interface costs one allocation. The
`BenchmarkEncodeFlateRGB` row measured the same thing from the other end at
532,160 objects for 532,144 pixels, and the profile says the pattern is not
local to that one function: `ScaleImage` and the level 5 re-encode path read the
same way.

Ranked by bytes the shape is different, and it is the accepted cost rather than
a new one:

| Function | Share of all bytes |
| --- | --- |
| bytes.growSlice | 42.46 percent |
| bytes.Clone | 31.28 percent |
| x/image/draw.(*kernelScaler).makeTmpBuf | 9.41 percent |
| pdfout.packedCMYK | 3.22 percent |
| pdfout.packedGray | 2.60 percent |

Buffer growth and cloning are 73.74 percent of allocated bytes, which is the
writer assembling whole output files in `bytes.Buffer`, and that is the accepted
cost named in the v0.0.4 findings.

## Application timing

`bash scripts/bench-cli.sh` builds `bin/spectreps`, then runs each command 10
times under GNU time (`/usr/bin/time -f '%e %M'`). hyperfine is not installed
on this machine, so the fallback loop produced these numbers, and the script
wrote the raw rows to `profiles/cli.txt`. GNU time prints hundredths of a
second, so a row of `0.000` means under 10 ms; the startup split below gives
those cells in milliseconds.

| Command | min s | median s | peak RSS (KB) |
| --- | --- | --- | --- |
| `version` | 0.000 | 0.000 | 4416 |
| `raster` path.pdf 72 dpi, default page | 0.000 | 0.005 | 11520 |
| `raster` path.pdf 300 dpi, default page | 0.080 | 0.090 | 104640 |
| `rewrite` whatisthis.pdf level 2 | 0.010 | 0.020 | 11712 |
| `rewrite` whatisthis.pdf level 3 | 0.500 | 0.515 | 167808 |
| `rewrite` whatisthis.pdf level 4 | 0.370 | 0.380 | 112704 |
| `rewrite` whatisthis.pdf level 5 | 0.340 | 0.350 | 89280 |
| `text` text.pdf | 0.000 | 0.000 | 5376 |
| `pdfimage` stroke.ps | 0.000 | 0.000 | 5952 |
| `validate` small.ps | 0.000 | 0.000 | 8448 |
| `validate` path.pdf | 0.000 | 0.000 | 8640 |

The default page for a PDF raster is 612 by 792 points, so the 300 dpi cell is
2550 by 3300 pixels and writes a 25 MB PPM. Level 3 is the slowest rewrite:
the medium cap keeps the longest side at 1754 pixels and encodes at quality
80, while levels 4 and 5 shrink the image further and encode less data.

### Startup split

| Row | Runs | Time | What it measures |
| --- | --- | --- | --- |
| `spectreps version` | 5 | 2.6 to 3.2 ms | process start, flag path, one write |
| in-process `BenchmarkCLIVersion` | 3 counts | 163 ns | the same flag path with no process |
| `spectreps validate small.ps` | 5 | 2.6 to 3.6 ms | process plus one blank page job |
| `spectreps raster path.pdf` 72 dpi | 10 | 5 ms min | process plus one page and one PPM |
| in-process `BenchmarkCLIRaster/72` | 3 counts | 5.5 ms min | one page with no process |

The fixed per-process cost is about 2.6 ms, measured by `version`, and it
recurs in every command. The engine job for a small program adds under 1 ms.
The raster job adds about 2.5 ms over the floor, and the in-process benchmark
lands at the same total, so the raster path carries little cost that only a
process pays.

### Scaling table

Both inputs raster on a 200 by 200 point page with `-w 200 -h 200`, so the
pixel count is `(200 * dpi / 72)` squared and the 1200 dpi cell stays under the
40 million pixel cap. Medians come from 10 runs.

| Input | dpi | Pixels | min s | median s | peak RSS (KB) |
| --- | --- | --- | --- | --- | --- |
| path | 72 | 40000 | 0.000 | 0.000 | 5760 |
| path | 150 | 173889 | 0.000 | 0.000 | 7488 |
| path | 300 | 693889 | 0.010 | 0.010 | 14208 |
| path | 600 | 2778889 | 0.040 | 0.050 | 30528 |
| path | 1200 | 11108889 | 0.170 | 0.195 | 136512 |
| stroke | 72 | 40000 | 0.000 | 0.000 | 5952 |
| stroke | 150 | 173889 | 0.000 | 0.000 | 8064 |
| stroke | 300 | 693889 | 0.010 | 0.020 | 14592 |
| stroke | 600 | 2778889 | 0.050 | 0.050 | 38784 |
| stroke | 1200 | 11108889 | 0.180 | 0.190 | 136704 |

The min column is close to linear. From 300 to 600 dpi the pixel count goes up
4 times and the min time goes up 4 times for path and 5 times for stroke. From
600 to 1200 dpi it goes up 4.25 times and 3.6 times. Peak RSS moves with the
pixel count, from 5.7 MB to 137 MB, because the pixmap, the shown page, and the
PPM body are each one buffer the size of the page.

### The 300 to 600 dpi gap

The plan recorded 81 ms at 300 dpi and 742 ms at 600 dpi on 2026-09-25, 4 times
the pixels and about 9 times the time. That gap does not reproduce on the
fixed tree. On 2026-09-26, `raster` of path.pdf on the same default page
measures 0.08 s at 300 dpi and 0.31 s at 600 dpi, 3.9 times for 4 times the
pixels, and the 200 by 200 point cell above measures 0.01 s to 0.04 s, 4 times.
The attribution: the 2026-09-25 number carried one allocation per stroke for
the `segments` slice, which 5.1 removed, and the page buffer goes from 25 MB at
300 dpi to 101 MB at 600 dpi, past the 30 MB L3, so the white fill, the
ShowPage copy, and the PPM encode become memory-bound. The remaining cost is
linear in pixels. A per-scanline change would live in `internal/graphics` and
is handed to the integrator as 5.6.

### GC summary

`GODEBUG=gctrace=1 bin/spectreps rewrite -level 5 -o profiles/cli/gctrace-out.pdf
sampledata/compress/whatisthis.pdf` produced three GC cycles in a 0.35 s job,
2.4 ms of total pause, and an 81 MB heap peak. That is about 0.7 percent of
the job, so no GC fix is warranted. The raw trace is in `profiles/gctrace.txt`.

## CPU profiles

`make bench-profile` writes one CPU profile per package, and the per-job
profiles come from `-cpuprofile` on a single benchmark. All lists are
`go tool pprof -top -lines -nodecount=20`.

### Application, `profiles/cli.cpu`

The combined profile mixes all eight CLI benchmarks.

| Function | File:line | flat% |
| --- | --- | --- |
| internal/runtime/syscall/linux.Syscall6 | asm_linux_amd64.s:36 | 10.34 |
| runtime.memclrNoHeapPointers | memclr_amd64.s:127 | 5.05 |
| runtime.futex | sys_linux_amd64.s:570 | 2.99 |
| runtime.memmove | memmove_amd64.s:122 | 2.91 |
| runtime.scanObject | mgcmark_greenteagc.go:1250 | 2.76 |
| x/image/draw kernelScaler scaleY_RGBA_Src | draw/impl.go:6148 | 2.14 |
| x/image/draw kernelScaler scaleX_YCbCr420 | draw/impl.go:5955 | 1.91 |
| graphics.(*Pixmap).ShowPage | internal/graphics/pixmap.go:96 | 1.76 |
| compress/flate.(*compressor).reset | deflate.go:617 | 1.53 |
| x/image/draw kernelScaler scaleX_YCbCr420 | draw/impl.go:5982 | 1.15 |
| graphics.NewPixmap | internal/graphics/pixmap.go:45 | 1.00 |
| math.archHypot | hypot_amd64.s:35 | 1.00 |
| graphics.(*Pixmap).strokeSegment | internal/graphics/pixmap.go:237 | 0.92 |
| runtime.madvise | sys_linux_amd64.s:556 | 0.92 |
| graphics.distToSeg | internal/graphics/pixmap.go:278 | 0.84 |
| x/image/draw kernelScaler scaleX_YCbCr420 | draw/impl.go:5981 | 0.84 |
| internal/sync.(*Mutex).Lock | mutex.go:63 | 0.77 |
| x/image/draw kernelScaler scaleX_YCbCr420 | draw/impl.go:5953 | 0.69 |
| runtime.(*unwinder).initAt | traceback.go:216 | 0.69 |
| image/jpeg.(*decoder).reconstructBlock | scan.go:469 | 0.61 |

The shape matches the wall clock: file writes, page-sized buffers, the DCT
scaler, then the stroke kernel. Two targeted profiles sharpen it.

`profiles/cli-raster300.top.txt` for `BenchmarkCLIRaster/300` is Syscall6 21.0
percent, ShowPage 15.5 percent cumulative, memclr 13.9 percent, memmove 18.1
percent across its lines, NewPixmap 7.4 percent cumulative, and strokeSegment
9.0 percent cumulative. One page at 300 dpi is 25 MB, so the buffer copies and
the write dominate the stroke math.

`profiles/cli-rewrite5.top.txt` for `BenchmarkCLIRewrite/level5` is the DCT
scaler: the `kernelScaler` lines sum to about 55 percent flat, jpeg decode is
about 5 percent, and memclr is 4.1 percent.

### Library, per job

Top project functions per job, flat share of the job.

| Job | Top functions | File:line | flat% |
| --- | --- | --- | --- |
| BenchmarkRunPostScript | ps.(*Interp).Lookup | internal/ps/interp.go:171 | 11.69 |
| | ps.(*Interp).Lookup | internal/ps/interp.go:173 | 4.22 |
| | graphics.distToSeg | internal/graphics/pixmap.go:278 | 2.92 |
| BenchmarkOpenPDF | pdf.(*lexer).parseValue | internal/pdf/parse.go:54 | 2.47 |
| | pdf.(*lexer).take | internal/pdf/scan.go:120 | 1.64 |
| | pdf.(*lexer).scanWord | internal/pdf/scan.go:372 | 1.32 |
| BenchmarkRasterizePage | graphics.(*Pixmap).strokeSegment | internal/graphics/pixmap.go:237 | 8.49 |
| | graphics.distToSeg | internal/graphics/pixmap.go:278 | 7.20 |
| | graphics.(*Pixmap).strokeSegment | internal/graphics/pixmap.go:235 | 5.35 |
| BenchmarkRewriteLevel0 | pdf.(*scanner).one | internal/pdf/content.go:319 | 4.28 |
| | pdf.(*scanner).next | internal/pdf/content.go:303 | 4.04 |
| | pdf.(*scanner).scanWord | internal/pdf/content.go:334 | 2.61 |
| BenchmarkRewriteLevel2 | no project function in the top 20 | | |
| BenchmarkRewriteLevel5 | no project function in the top 20 | | |
| BenchmarkExtractText | pdf.(*runner).textDeviceMatrix | internal/pdf/text.go:495 | 4.30 |
| | pdf.(*runner).glyphMatrix | internal/pdf/text.go:488 | 2.15 |
| | pdf.Value.NameEntry | internal/pdf/value.go:130 | 2.15 |
| BenchmarkImagePDF | no project function in the top 20 | | |

Where the areas landed:

- Flate decode and encode: `BenchmarkImagePDF` is `compress/flate` deflate for
  about 47 percent, and `compress/flate.NewWriter` was 24 percent of the
  allocation profile before 5.2.
- DCT decode and encode: `BenchmarkRewriteLevel5` is
  `x/image/draw.(*kernelScaler)` for over 30 percent of flat samples and
  `image/jpeg` decode for about 5 percent.
- Image scale: same profile, `scaleX_YCbCr420` and `scaleY_RGBA_Src`.
- Pixmap stroke and fill: `BenchmarkRasterizePage` and `BenchmarkRunPostScript`,
  with `strokeSegment` and `distToSeg`.
- Glyph coverage: `BenchmarkRasterizePage` at the `DrawGlyph` level is below
  the top 20; glyph coverage is not a hot area for these inputs.
- PDF object parse: `BenchmarkOpenPDF`, `lexer.parseValue`.
- PostScript execution: `BenchmarkRunPostScript`, `Interp.Lookup` at 11.69
  percent. The stroke program names `moveto`, `lineto`, `stroke`, and
  `setlinewidth` 2000 times each, and each name walk starts at the top
  dictionary.
- Text layout: `BenchmarkExtractText`, `textDeviceMatrix`.

## Allocation profile

`make bench-profile` also writes `profiles/spectreps.mem` and
`profiles/cli.mem`. The largest sites with
`pprof -top -nodecount=20 -sample_index=alloc_space`:

`profiles/spectreps.alloc.txt`:

| Site | flat share |
| --- | --- |
| bytes.growSlice | 24.27% |
| bytes.Clone | 21.26% |
| x/image/draw kernelScaler makeTmpBuf | 8.98% |
| pdf.cloneBytes | 7.05% |
| pdf.(*File).loadFont | 5.77% |
| pdf.(*lexer).parseDict | 3.87% |
| pdf.groupLines | 3.66% |
| pdf.(*lexer).parseArray | 3.63% |
| pdf.(*textCollector).Glyph | 3.12% |
| graphics.NewPixmap | 2.01% |
| compress/flate.(*dictDecoder).init | 2.00% |
| graphics.(*Pixmap).ShowPage | 2.00% |

`profiles/cli.alloc.txt`:

| Site | flat share |
| --- | --- |
| cli.encodePPM | 18.77% |
| graphics.(*Pixmap).ShowPage | 10.23% |
| graphics.NewPixmap | 10.11% |
| bytes.growSlice | 9.08% |
| bytes.Clone | 8.32% |
| compress/flate.(*dictDecoder).init | 7.18% |
| x/image/draw kernelScaler makeTmpBuf | 4.58% |
| pdf.(*lexer).parseDict | 3.91% |
| os.readFileContents | 3.37% |
| pdf.(*lexer).parseArray | 2.60% |
| pdf.(*File).loadFont | 2.58% |
| pdf.readLimited | 2.56% |

`compress/flate.NewWriter` was 29.81 percent of `profiles/cli.alloc.txt` before
5.2 and is not in the top 20 after it. The remaining Flate sites are decode
(`dictDecoder.init`, `readLimited`) and `NewReader`, which the writer pool does
not touch.

## Instance reuse

`BenchmarkReuseInstance` holds one `New` and one `OpenPDF` and rasterizes a
page per iteration. `BenchmarkPerCallInstance` pays a `New`, an `OpenPDF`, one
page, and a `Close` per iteration.

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| BenchmarkReuseInstance | 1310224 | 307110 | 2415 |
| BenchmarkPerCallInstance | 1812490 | 330288 | 2499 |

The per-call shape costs 502266 ns more per page, about 38 percent, plus
23178 B/op and 84 allocs. That is the context check and engine shim an
embedded consumer pays when it calls `New` and `OpenPDF` for every page
instead of keeping one instance.

## Escape analysis

`go build -gcflags=-m ./spectreps ./internal/...` produced 7560 lines; the
filtered list is in `profiles/escape.txt`. Heap escapes per hot file:

| File | `escapes to heap` | `moved to heap` |
| --- | --- | --- |
| internal/pdf/content.go | 105 | 7 |
| internal/ps/op_type.go | 101 | 0 |
| internal/ps/op_container.go | 66 | 1 |
| internal/ps/op_graphics.go | 64 | 0 |
| internal/ps/interp.go | 57 | 1 |
| internal/pdfout/pack.go | 45 | 2 |
| internal/pdfout/image.go | 39 | 3 |
| internal/pdf/file.go | 39 | 0 |
| internal/pdf/text.go | 35 | 1 |
| internal/pdf/page.go | 33 | 0 |
| internal/pdfout/write.go | 22 | 2 |
| internal/pdfout/emit.go | 11 | 0 |
| internal/graphics/pixmap.go | 12 | 0 |
| internal/pdf/image.go | 13 | 1 |
| internal/pdf/filter.go | 12 | 0 |
| internal/pdf/font.go | 10 | 2 |
| internal/pdfout/stream.go | 1 | 2 |
| internal/pdfout/scale.go | 1 | 1 |

The full-buffer movements are the expected ones. `internal/pdfout/stream.go:49`
and `:69` move the Flate body and the stream body buffer to the heap.
`internal/pdfout/write.go:87` and `:115` move the file buffer and the pages
body. `internal/pdf/image.go:155` moves the jpeg decoder, and
`internal/pdf/font.go:544` and `:565` move the sfnt parse buffers.
`internal/pdf/content.go` moves the path slices and the closure-captured depth
counters. No unexpected whole-page copy showed up outside `ShowPage` and the
PPM encode, both of which are the output contract.

## Findings

One row per hot area. Each row names a phase 5 change or an accepted reason.

| Hot area | Top function, file:line | Share of the job | Linear in | Disposition |
| --- | --- | --- | --- | --- |
| Raster stroke kernel | graphics.(*Pixmap).strokeSegment, internal/graphics/pixmap.go:237, plus distToSeg :278 | 8.5 percent flat and 21.0 percent cumulative in BenchmarkRasterizePage | pixels and stroke width | 5.1 removed the per-stroke slice. The per-pixel `math.Hypot` is accepted: a squared-distance form is a pixel-edge change and not in the phase 5 list |
| Page buffers and PPM encode | graphics.(*Pixmap).ShowPage, internal/graphics/pixmap.go:96, and cli.encodePPM, internal/cli/run.go:634 and :639 | 15.5 percent cumulative and 18.8 percent of allocations in the 300 dpi CLI raster | pixels | Accepted. The pixmap, the shown page, the PPM body, and the file are each page-sized, and the output contract asks for them |
| Flate writer setup | compress/flate.NewWriter | 29.8 percent of the CLI allocation profile before the fix | streams flated | 5.2 landed a writer pool. ImagePDF B/op fell 818204 to 4565, CLIGS 1745909 to 118893 |
| Flate decode | compress/flate.(*dictDecoder).init and pdf.readLimited, internal/pdf/filter.go:60 | 7.2 and 2.6 percent of the CLI allocation profile | compressed bytes | Accepted. A reader pool is possible through the Resetter interface, but the share is below the writer and the reader wraps an io.Reader |
| DCT scale | x/image/draw kernelScaler scaleX_YCbCr420, draw/impl.go:5955 | 30 to 55 percent of levels 3 through 5 | image pixels | Accepted for this cut. The scaler lives in a dependency and the level caps are the product decision, so the integrator decides whether 5.6 covers it |
| JPEG decode | image/jpeg.(*decoder).reconstructBlock, image/jpeg/scan.go:469 | about 5 percent of level 5 | image pixels | Accepted. Decode once per image is the contract |
| PDF content parse | pdf.(*scanner).one, internal/pdf/content.go:319 | 4.3 percent of level 0 | content bytes | Accepted. The scanner is the level 0 job |
| PostScript lookup | ps.(*Interp).Lookup, internal/ps/interp.go:171 | 11.7 percent of BenchmarkRunPostScript | operators | Accepted. Each name walks the dictionary stack from the top; a name cache would change the redefinition-at-execution rule in documentation/language.md |
| Text device matrix | pdf.(*runner).textDeviceMatrix, internal/pdf/text.go:495 | 4.3 percent of BenchmarkExtractText | glyphs | Accepted. Under the phase 5 cut |
| Font parse | pdf.(*File).loadFont, internal/pdf/font.go:80 | 5.8 percent of the spectreps allocation profile | font objects | 5.4 to the integrator: a per-font cache touches internal/pdf/font.go |
| Raster compare | spectreps.CompareRaster, spectreps/compare.go:15 | 100 percent of its own benchmark | pixels | 5.5 landed the tight-stride `bytes.Equal` fast path |
| 300 to 600 dpi | pixmap fill, ShowPage copy, PPM encode | 3.9 times the min time for 4 times the pixels | pixels | Attributed above, and 5.1 removed the allocation part. No new fix row |

The v0.0.5 extension added these rows. Each is measured and none has a landed
fix, because `plans/v0.0.5/` changes no production code.

| Hot area | Top function, file:line | Measured | Linear in | Disposition |
| --- | --- | --- | --- | --- |
| Fill at path complexity | graphics.inside, internal/graphics/pixmap.go:385, called from Fill :137 | 26.5 ms for 128 points on a 200 by 200 page | page area and path points | Open. The cost is `O(W*H*N)` with no scanline bucketing and no bounds rejection. A fix is a later plan row |
| Clip snapshot | graphics.(*Pixmap).snapshotRect, internal/graphics/clip.go:152, and restoreOutside :171 | 164 KB and 6 allocs per clipped mark, and 23.7 times the cost of the same composite unclipped | snapshot rectangle area | Open. `CompositeGroupClipped` snapshots the whole page where the group bounds would do |
| Flate RGB encode | pdfout.EncodeFlateRGB, internal/pdfout/scale.go:41 | 532,160 allocs/op for 532,144 pixels, 14.9 ms | pixels | Open. One heap allocation per pixel from the `image.Image` interface at :47, and one zlib write per scanline at :53 |
| Flate decode growth | pdf.readLimited, internal/pdf/filter.go:649 | 5.25 MB allocated for a 1 MiB output at 606 MB/s | output bytes | Open. The buffer starts at 4096 bytes and grows by append, so the output is copied about 9 times |
| CMYK packing | pdfout.packedCMYK, internal/pdfout/image.go:157 | 640 us against 222 us for RGB on the same page | pixels | Open. Per pixel float conversion with no packed fast path |
| PPM double copy | cli.encodePPM, internal/cli/run.go:684 | 246 KB for a 120 KB page | page bytes | Open. The body is copied again to prepend the header |
| Reading order | tag.orderFlow, internal/tag/reading.go:897 | 2.4 ms for 800 runs on one page | run count, superlinear | Open. `O(n^2)` with a slice shift per removal |
| PDF/A preflight | pdfa.Preflight, internal/pdfa/preflight.go:73 | 1.65 ms on a compliant A-4 file | object count | Open. The loop visits every object, and it runs inside every PDF/A rewrite |
| UA-2 content scan | pdfa.ua2ContentRule, internal/pdfa/ua2_content.go:27 | 4,512 allocs/op for a second scan of the page bytes | content bytes | Open. A second scanner over bytes the content interpreter already read |

## Budget

The accepted baseline is the `make bench` table above, taken on 2026-09-26.
The allocation ceilings are in `spectreps/alloc_test.go` and
`internal/cli/alloc_test.go`:

| Ceiling | Value | Tolerance |
| --- | --- | --- |
| MeasureBox | 0 allocs | exact |
| MeasureInk | 0 allocs | exact |
| MeasureInkAmount | 0 allocs | exact |
| CompareRaster | 0 allocs | exact |
| CompareFiles | 0 allocs | exact |
| level 2 rewrite, internal/cli | 715 allocs | exact, collector paused in the test |
| DocumentInfo | 7 allocs | exact |
| PreflightUA2 | 4512 allocs | exact |
| WritePostScript | 5628 allocs | exact |
| RewritePDF tagged | 12090 allocs | exact |
| RewritePDF PDF/A-4 | 147 allocs | exact |
| PNG encode, internal/cli | 34 allocs | exact |
| TIFF encode uncompressed, internal/cli | 27 allocs | exact |

The level 2 rewrite ceiling is 715 in `internal/cli/alloc_test.go` and was 701
in the v0.0.4 record. The merged tree added five allocations to that path, the
code comment at `internal/cli/alloc_test.go:18` names them, and this table now
carries the number the test actually asserts.

The five `spectreps` ceilings and the two `internal/cli` ceilings are the v0.0.5
extension. `PreflightUA2`, `WritePostScript`, and `RewritePDF PDF/A-4` match
their `-benchmem` counts. `RewritePDF tagged` does not: the benchmark reads
12,095 and `AllocsPerRun` reads 12,090, because `AllocsPerRun` warms the zlib
writer pool before it measures and a benchmark does not. The ceiling is the
`AllocsPerRun` value, which is the one that gates `make test`.

The guard proof: with the `internal/cli` ceiling lowered by one, to 714,
`go test -count=1 ./internal/cli -run TestPerformanceAllocs` fails with
`level 2 rewrite allocs = 715, want 714`, and the restored ceiling passes. The
same proof ran on the v0.0.5 ceilings. Lowering `allocRewriteTagged` to 12089
made `go test -count=1 ./spectreps -run TestJobAllocs` fail with
`RewritePDF tagged allocs = 12090, want 12089`, and lowering `allocCLIPNG` to 33
made `go test -count=1 ./internal/cli -run TestEncoderAllocs` fail with
`PNG encode allocs = 34, want 33`. Both restored values pass.

The guard proof: with the `internal/cli` ceiling lowered by one, to 700,
`go test -count=1 ./internal/cli -run TestPerformanceAllocs` fails with
`level 2 rewrite allocs = 701, want 700`, and the restored ceiling passes.
The `internal/cli` ceiling pauses the collector so the zlib writer pool stays
populated between the measured runs; without the pause the count moves when a
GC cycle drains the pool.

Timing is machine-specific. A regression on this machine is a signal, and a
green `make bench` on another box proves nothing about this one. Only the
allocation counts above gate `make test`. The accepted wall-clock numbers are
the three baseline tables, the application table, and the scaling table above.

## Accepted fixes

Each fix names its before and after, both from the same tree and the same
benchmark. The B/op and allocs/op columns are exact. The ns/op columns are
medians and carry the load caveat from the baseline.

5.5 `CompareRaster` fast path, `spectreps/compare.go`. When both strides are
tight, compare the packed buffers with `bytes.Equal` and keep the byte loop
for the padded case and for the first differing byte.

| Measurement | Before | After |
| --- | --- | --- |
| BenchmarkCompareRaster ns/op | 85676 | 2067 |
| BenchmarkCompareRaster B/op | 0 | 0 |
| Behavior proof `go test -count=1 ./spectreps -run TestCompareRaster` | passes | |

5.1 `Pixmap.Stroke` without the per-stroke segment slice,
`internal/graphics/pixmap.go`. The pair walk is now direct, so `segments` is
gone. This is the one `internal/graphics` edit, and it is local to `Stroke`.

| Measurement | Before | After |
| --- | --- | --- |
| BenchmarkStroke B/op | 9728 | 0 |
| BenchmarkStroke allocs/op | 1 | 0 |
| BenchmarkRasterizePage B/op | 345510 | 307110 |
| BenchmarkRasterizePage allocs/op | 2815 | 2415 |
| BenchmarkRunPostScript B/op | 893220 | 701185 |
| BenchmarkRunPostScript allocs/op | 10069 | 8069 |
| internal/pdf BenchmarkPaintPage B/op | 503334 | 311331 |
| internal/pdf BenchmarkPaintPage allocs/op | 13986 | 11986 |
| Behavior proof `go test -count=1 ./internal/graphics ./spectreps -run 'TestPixmap|TestPaint'` | passes | |

5.2 `zlib` writer pool, `internal/pdfout/stream.go` and
`internal/pdfout/scale.go`. `withFlateWriter` reuses one writer per process,
and `EncodeFlateRGB` goes through it. A fresh writer allocates the deflate
window and hash tables, so the pool removes roughly 800 KiB per stream when it
survives a GC cycle. B/op after the fix depends on the GC cadence; the
isolated level 0 run reads 228841 B/op when the pool is drained each cycle,
and the `make bench` context reads 158982.

| Measurement | Before | After |
| --- | --- | --- |
| BenchmarkRewriteLevel0 B/op | 990326 | 158982 |
| BenchmarkRewriteLevel0 allocs/op | 6069 | 6049 |
| BenchmarkImagePDF B/op | 818204 | 4565 |
| BenchmarkImagePDF allocs/op | 63 | 44 |
| BenchmarkCLIGS B/op | 1745909 | 118893 |
| BenchmarkCLIPDFImage B/op | 1713967 | 713236 |
| Behavior proof `go test -count=1 ./internal/pdf ./internal/pdfout -run 'TestDecode|TestWrite|TestLevel'` | passes | |

5.3 recorder bytes without a copy, `internal/pdfout/device.go`. `bytes()`
returns the buffer the recorder owns, and the caller does not write to it
again. `copy.go` was not touched.

| Measurement | Before | After |
| --- | --- | --- |
| BenchmarkRewriteLevel0 B/op, isolated count=5 | 247732 | 228080 |
| BenchmarkRewriteLevel0 allocs/op, isolated count=5 | 6052 | 6051 |
| Behavior proof `go test -count=1 ./internal/pdfout ./spectreps -run 'TestEmit|TestRewriteStable'` | passes | |

The isolated pair ran back to back so the GC cadence stayed the same; the
delta is the emitted content of one page, about 19.6 KiB.

## Tool matrix

| Tool | On this machine | What it proves | When absent |
| --- | --- | --- | --- |
| `/usr/bin/time` | yes | seconds and peak RSS per command | `scripts/bench-cli.sh` stops with a message |
| hyperfine | no | wall-clock distribution per command | the script falls back to GNU time |
| `go tool pprof` | yes | CPU and allocation profiles | profiles cannot be read |
| benchstat | yes | A/B comparison of two benchmark runs | `make bench-check` prints a skip |
| perf | no | not used | nothing |

benchstat is installed at `~/go/bin/benchstat` by
`go install golang.org/x/perf/cmd/benchstat@latest`. `~/go/bin` is on `PATH`
from `~/.zshrc:52`, so `make bench-check` produces a real ratio verdict. The
install is a tool, not a module requirement: `go install` with a version runs
outside the module, and `go.mod` and `go.sum` are byte-identical before and
after. The repository has no CI, so nothing runs this suite automatically and
every number here is reproducible only on this machine.

## bench-check

`make bench-check` runs the suite with `-count=5` and compares
`profiles/bench.txt` with the fresh run through benchstat, writing the verdict
to `profiles/compare.txt`. benchstat is installed, so the comparison runs. This
is the first run on this tree that produced a ratio verdict at all, because
every `make bench-check` before the v0.0.5 extension wrote a skip line.

It also produced a verdict that is not usable, and the reason is the Makefile
rather than the tool. The base capture is `-count=3` and the fresh capture is
`-count=5`, so benchstat gets 3 samples on one side and 5 on the other. Its own
footnote says it needs 6 samples for a confidence interval, and 44 rows came
back as `± ∞` with that footnote. The test it runs is a Mann-Whitney U, and at
n of 3 against 5 the smallest reachable p-value is 0.036, so the p-value
distribution has no room below it.

The measured result on 2026-09-26, on a tree where no non-test Go file had
changed:

| Reading | Value |
| --- | --- |
| Rows compared | 323 |
| Rows benchstat called significant | 107 |
| Of those, rows sitting at exactly p=0.036 | 107 |
| Rows marked `~`, no significant change | 216 |
| Rows reading `all samples are equal` | 22 |
| Per-package `allocs/op` geomean | +0.00 percent in every package |

All 107 significant rows carry the identical floor p-value, so benchstat is not
ranking them, it is reporting that the three base samples and the five new
samples do not overlap on a noisy box. The per-package `sec/op` geomeans ranged
from -2.22 percent to +46.67 percent in a single run with a load average of 1.04
and no competing process, and the sign of the move tracked the size of the
benchmark rather than anything in the code. `Subset/tag` moved +34.21 percent
and `OpenClassicXRef` moved +38.30 percent in the same file, in opposite
directions from their neighbours.

The 22 rows reading `all samples are equal` are the `allocs/op` columns, and
those are the signal. `BenchmarkRewritePDFA` read 147 allocs on both sides
across all eight samples. That is the whole argument for the budget: allocation
counts are exact and reproducible, timing numbers on this box are not, at any
sample count the current targets provide.

Read a verdict in this order.

1. The `allocs/op` and `B/op` columns. They are the only rows a change has to
   move, and they are the only rows that reproduce.
2. The `geomean` row. Near zero means the box was quiet. Several percent off in
   one direction across every package means the run is not comparable to itself.
3. The `sec/op` column last, and never on a single row.

Three things follow, and none of them is a code change.

- Raise the counts before trusting a verdict. `-count=10` on both sides puts
  benchstat above its 6-sample floor and lets the p-value discriminate. That is
  a Makefile row in `plans/v0.0.5/5-baseline-and-budget.md`, not a fix here.
- Do not run lint or a build while a comparison is in flight. An early run on
  this tree shared the box with `golangci-lint` and reported +15.19 percent on
  `EncodePPM` and +14.19 percent on `OpenXRefStream`, which was load.
- Keep timing out of `make test`. The numbers above are the argument, and they
  are stronger than the policy statement was before this section existed.



