# CLI

Binary name `spectreps`, built at `bin/spectreps` by `make build`.

The CLI is a caller of package `spectreps`. Flags exist to fill `RunOptions`, `RewriteOptions`, and file paths. Ghostscript's full switch grammar is not a goal. The bounded mode under `spectreps gs` accepts the allowlisted switches in `documentation/gs-argv-grammar.md`; `documentation/gs-argv-mapping.md` maps a rewritten `gs` job onto a plain Spectre command line.

## Commands

```
spectreps version
spectreps run [options] file.ps
spectreps raster [options] file.ps|file.pdf
spectreps pdfimage [options] file.ps|file.pdf
spectreps bbox [options] file.ps|file.pdf
spectreps inkcov [options] file.ps|file.pdf
spectreps ink_cov [options] file.ps|file.pdf
spectreps rewrite [options] file.pdf
spectreps ps -o path file.pdf
spectreps text [-pages range] file.pdf
spectreps info file.pdf
spectreps validate [options] file.ps|file.pdf
spectreps compare bytes fileA fileB
spectreps compare raster [options] fileA fileB
spectreps gs [switches] file.ps|file.pdf
```

`gs` is the bounded compatibility mode. It accepts only the switches in `documentation/gs-argv-grammar.md` and routes the job to the commands above. Any other switch exits 2 with a message that names it.

`version` prints `0.0.3`. Exit 0.

Shared options for `run`, `raster`, `pdfimage`, `bbox`, `inkcov`, `ink_cov`, and `compare raster`:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-w` | Page width in points | 612 |
| `-h` | Page height in points | 792 |
| `-r` | Pixels per inch | 72 |
| `-o` | Output path | required for `raster` and `pdfimage` |
| `-pages` | Page range, `N` or `A-B`, 1-based inclusive | every page |

`-pages` accepts `A-` to run to the last page and `-B` to start at page 1. A start below 1 or past the last page exits 1 with `Error: /rangecheck in pages`. An end past the last page clamps to the last page. A malformed value exits 2. `run` accepts `-pages` and ignores it, the same way it accepts and ignores `-o`. For PostScript the interpreter runs every page first and the range filters the result, so a failing page outside the range still fails the command. For a PDF only the selected pages are painted.

`raster` adds three flags:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-jpegq` | JPEG quality, clamped to 1 through 100 | 75 |
| `-tiffcompress` | TIFF compression, `none` or `deflate` | `deflate` |
| `-format` | Encoder, `ppm`, `png`, `jpeg`, or `tiff`, and it overrides the `-o` suffix | from the suffix |

A `-format` value outside the four names exits 2. A `spectreps gs` device sets the same choice, so a device wins over the `-o` suffix.

`pdfimage` adds one flag:

| Flag | Meaning | Default |
| --- | --- |
| `-colorspace` | Image stream color space: `rgb`, `gray`, or `cmyk` | `rgb` |

Any other `-colorspace` value exits 2. `rgb` keeps the 24-bit RGB bytes from earlier tags.

`run` and `compare raster` do not accept `-jpegq`, `-tiffcompress`, or `-colorspace`.

`bbox`, `inkcov`, and `ink_cov` write their report to stdout and do not write a file.

`rewrite` options:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-o` | Output PDF path | required |
| `-compress` | Flate content streams at level 0 | true |
| `-level` | Compression level, 0 through 5 | 0 |
| `-pdfa` | PDF/A-4 claim: `4` or `4f` | omitted |

`-level 0` re-emits the path subset and keeps `-compress` as the Flate switch. `-level 1` through `-level 5` use the pass-through writer, so text, fonts, and content Spectre cannot interpret are copied. A level above 0 Flates content streams and ignores `-compress`. The image policy per level is in `documentation/devices.md`. Any other value exits 2. A tagged PDF at `-level 0` exits 1 with `Error: /tagged in RewritePDF`; levels 1 through 5 keep the tags and the source header version.

`-pdfa 4` claims PDF/A-4 base and `-pdfa 4f` claims PDF/A-4f. A claim uses the pass-through writer at the selected level, appends the XMP metadata and the sRGB output intent, and changes the header to `%PDF-2.0` with a binary marker. The command runs the profile preflight first. A known violation exits 1 with one stderr line in the form `Error: /rule in PDFA`, and writes no output file. The rules are the table in `documentation/devices.md`. The claim is a profile preflight, not a certificate.

`ps` options:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-o` | Output PostScript path | required |

`ps` opens a PDF and re-emits each page's path subset as one date-free PostScript program. The header is `%!PS-Adobe-3.0` with a fixed 612 by 792 box. Marks are `setrgbcolor` or `setgray`, `setlinewidth`, `m`/`l`, and `S`/`f`/`f*` in 72 dpi points, and a prolog defines the short names. Text and images are not emitted: a page with `Tj` exits 1 with `Error: /undefined in Tj`. The file is written at mode `0o600`, and bytes are stable across two runs. `documentation/devices.md` has the shape.

`text` opens a PDF and prints the extracted text of the selected pages to stdout. Lines run top to bottom and left to right, each line ends with CRLF, and a font with neither `/ToUnicode` nor a named encoding falls back to the code point. The command accepts `-pages` and no other option. Extraction is compared as text and geometry, not as raster bytes: text pixels never byte-match Ghostscript, because hinting and antialiasing differ. `documentation/devices.md` has the layout.

`info` opens a PDF and prints a read-only summary to stdout. It writes no file and takes no option. The lines are:

```
PDF version: 1.4
Pages: 2
Page 1: 612 x 792
Page 2: 100 x 50
Tagged: false
Fonts:
  Helvetica embedded=false
Images: 1
```

`PDF version` is the header version. `Pages` is the page tree leaf count, and each `Page` line is the resolved `/MediaBox` width and height in points, inherited from the nearest `/Pages` ancestor and defaulting to 612 by 792 when the tree has none. `Tagged` is the reader tagged flag. The `Fonts:` block lists every in-use `/Type /Font` dictionary except CIDFont descendants, sorted by name, and `embedded=true` means a `/FontFile`, `/FontFile2`, or `/FontFile3` program is in the file. A file with no fonts prints `Fonts: none`. `Images` counts the in-use image XObjects. An encrypted trailer exits 1 with `Error: /invalidaccess in Encrypt`, because the reader refuses it. A missing input or a bad flag exits 2, an unreadable path exits 3, and a malformed document exits 1.

`validate` takes one input and writes errors to stderr. It has no output file. `validate` does not run the PDF/UA-2 preflight yet. That preflight is `internal/pdfa.PreflightUA2`, it runs only for a UA-2 request, and its rules are in `documentation/devices.md`. The scope is preserve and preflight, and the claim is preflight only.

`compare bytes` takes two paths and no device flags. `compare raster` rasterizes the selected pages of both inputs with the same options and calls `CompareRaster` on each page pair. A `.pdf` input opens with `OpenPDF` and paints each selected page with `RasterizePage`; any other input uses `RunPostScript`. Different selected page counts print `mismatch length` and exit 1. It does not hash the encoded files.

## Output files

`raster` writes a PPM raw file (P6) unless `-o` ends in `.png`, `.jpg`, `.jpeg`, `.tif`, or `.tiff`. A PDF input paints every selected page with `RasterizePage`; any other input uses `RunPostScript`.

A `.png` output encodes the pixmap with `image/png`. A `.jpg` or `.jpeg` output encodes the same pixmap with `image/jpeg` at the `-jpegq` quality. Every other suffix falls back to PPM. JPEG is lossy, so decoded pixels can differ from the pixmap by a small amount. JPEG file bytes are not an equality oracle. `CompareRaster` and `PageImage` are.

A `.tif` or `.tiff` output encodes the same pixmap with `golang.org/x/image/tiff` as baseline TIFF. `-tiffcompress` picks `none` for no compression or `deflate` for Deflate strips, and the default is `deflate`. That encoder writes those two only, so LZW and the CCITT schemes are not accepted. TIFF file bytes are not an equality oracle either.

If the job produces one page and `-o` has no `%d`, the path is used as given. If the job produces more than one page and `-o` has no `%d`, the command exits 2. `%d` is the one-based number of the emitted page, not the input page, matching the `%d` token Ghostscript documents for `-sOutputFile` and `-dFirstPage`. A range that selects input pages 3 through 5 writes page-1, page-2, and page-3.

`bbox` writes two lines per page to stdout:

```
%%BoundingBox: 0 0 10 10
%%HiResBoundingBox: 0 0 10 10
```

`%%BoundingBox` uses the floor of each minimum and the ceiling of each maximum. `%%HiResBoundingBox` prints the point edges with `strconv.FormatFloat(v, 'f', -1, 64)`. A page with no marked pixel prints `%%BoundingBox: 0 0 0 0` and `%%HiResBoundingBox: 0 0 0 0`. The measured dpi is the resolved `-r`, so 0 selects 72.

`inkcov` writes two lines per page to stdout, page numbers one-based:

```
Page 1
0.25000 0.25000 0.25000 RGB
```

The three fractions are RGB occupancy with five digits after the point. They are not CMYK, and the line does not end in `CMYK OK`.

`ink_cov` writes the same two lines per page with the weighted amount as a percent per channel:

```
Page 1
25.00000 0.00000 0.00000 RGB
```

The formula and both worked examples are in `documentation/devices.md`. The channels are R, G, and B because the pixmap is RGB, so the line still ends in `RGB`. The CLI prints `MeasureInkAmount` times 100.

`rewrite` writes one PDF. The compression levels are in `documentation/devices.md`.

`ps` writes one PostScript program with a date-free `%!PS-Adobe-3.0` header. The marks and the fixed box are in `documentation/devices.md`.

`pdfimage` writes one PDF with one image page per input page. `-colorspace` selects 24-bit RGB, 8-bit DeviceGray, or 32-bit DeviceCMYK image streams, and `rgb` is the default. The input is a PostScript file or a PDF. A PDF input paints every page with `RasterizePage`, and any other input uses `RunPostScript`. A tagged PDF input exits 1 with `Error: /tagged in ImagePDF`, because the image PDF has no tags to keep. The `-o` path is required and does not use `%d`. `-r 0` writes 72 dpi.

## Exit codes

| Code | When |
| --- | --- |
| 0 | Success. `compare` exits 0 when `Equal` is true. |
| 1 | `JobError`, `ErrNotImplemented`, or a compare mismatch. |
| 2 | Usage. Missing file, unknown flag, unknown command, missing `-o`. |
| 3 | A read or write failed before the interpreter ran. |

`bbox`, `inkcov`, and `ink_cov` exit 0 after printing every page, 1 on an interpreter error, and 2 on a missing input or a bad flag.

Mismatch text for compare, one line on stdout:

```
mismatch byte 14
```

or

```
mismatch pixel 120
```

or

```
mismatch width
```

stderr stays empty on a clean mismatch so scripts can diff stdout. Interpreter errors go to stderr and exit 1.

## Phase 02 behavior

`version`, usage errors, and `compare bytes` work. `run`, `raster`, `rewrite`, `validate`, and `compare raster` exit 1 with `spectreps: not implemented` on stderr until their phases land. They still parse flags, so a missing `-o` on `raster` is exit 2 even in phase 02.
