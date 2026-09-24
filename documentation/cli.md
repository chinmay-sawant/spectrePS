# CLI

Binary name `spectreps`, built at `bin/spectreps` by `make build`.

The CLI is a caller of package `spectreps`. Flags exist to fill `RunOptions`, `RewriteOptions`, and file paths. Ghostscript's full switch grammar is not a goal of the current tags. A compatibility mode that accepts a `gs` argv can be proposed later in `plans/v0.0.1/10-deferred.md`.

## Commands

```
spectreps version
spectreps run [options] file.ps
spectreps raster [options] file.ps|file.pdf
spectreps pdfimage [options] file.ps|file.pdf
spectreps rewrite [options] file.pdf
spectreps validate [options] file.ps|file.pdf
spectreps compare bytes fileA fileB
spectreps compare raster [options] fileA fileB
```

`version` prints `0.0.1` until the first tag that bumps `Version`, then prints that constant. Exit 0.

Shared options for `run`, `raster`, `pdfimage`, and `compare raster`:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-w` | Page width in points | 612 |
| `-h` | Page height in points | 792 |
| `-r` | Pixels per inch | 72 |
| `-o` | Output path | required for `raster` and `pdfimage` |

`rewrite` options:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-o` | Output PDF path | required |
| `-compress` | Flate content streams | true |

`validate` takes one input and writes errors to stderr. It has no output file.

`compare bytes` takes two paths and no device flags. `compare raster` rasterizes both inputs with the same options, then calls `CompareRaster` on the pixmaps. It does not hash the encoded files.

## Output files

`raster` writes a PPM raw file (P6) unless `-o` ends in `.png`, in which case it writes a PNG from the same pixels.

If the job produces one page and `-o` has no `%d`, the path is used as given. If the job produces more than one page and `-o` has no `%d`, the command exits 2. `%d` is the one-based page number, matching the `%d` token Ghostscript documents for `-sOutputFile`.

`rewrite` writes one PDF.

`pdfimage` writes one PDF with one 24-bit RGB image page per input page. The input is a PostScript file or a PDF. A PDF input paints every page with `RasterizePage`, and any other input uses `RunPostScript`. The `-o` path is required and does not use `%d`. `-r 0` writes 72 dpi.

## Exit codes

| Code | When |
| --- | --- |
| 0 | Success. `compare` exits 0 when `Equal` is true. |
| 1 | `JobError`, `ErrNotImplemented`, or a compare mismatch. |
| 2 | Usage. Missing file, unknown flag, unknown command, missing `-o`. |
| 3 | A read or write failed before the interpreter ran. |

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
