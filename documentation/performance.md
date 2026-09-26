# Performance

The benchmark suite measures three surfaces: the public `spectreps` package,
the `internal/cli` command layer, and the hot internal packages (`pdf`,
`pdfout`, `graphics`, `ps`). It runs on demand and never inside `make test`.
Output lands in `profiles/`, which is gitignored. The accepted baseline, the
allocation budget, and the findings live here.

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
| `make bench` | `profiles/bench.txt` | `go test -p 1 -run '^$' -bench . -benchmem -count=3` over the six benchmark packages |
| `make bench-profile` | `profiles/<pkg>.cpu`, `profiles/<pkg>.mem` | one CPU and one heap profile per package |
| `make bench-check` | `profiles/compare.txt` | a `-count=5` rerun compared against `profiles/bench.txt` with benchstat |
| `bash scripts/bench-cli.sh` | `profiles/cli.txt` | the built binary, one command per row, min and median of 10 runs |
| `go test -run '^$' -bench . -benchmem ./spectreps` | stdout | one package at a time |

`-p 1` serializes the six packages so their benchmarks do not share cores.

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
| level 2 rewrite, internal/cli | 701 allocs | exact, collector paused in the test |

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
| benchstat | no | A/B comparison of two benchmark runs | `make bench-check` prints a skip |
| perf | no | not used | nothing |

## bench-check

`make bench-check` runs the suite with `-count=5` and compares
`profiles/bench.txt` with the fresh run through benchstat when it is on PATH.
benchstat is not installed on this machine, so the verdict is:

```
benchstat is not on PATH, skipping the comparison
```

The fresh `-count=5` run passed and its raw rows are in
`profiles/bench-check.txt`. Without benchstat there is no automatic ratio
verdict, so a reader compares the two tables by hand. The allocation columns
track the baseline within a few bytes and, in the Flate and CLI raster cells,
within one allocation per op where a buffer or a pool decision differs.

