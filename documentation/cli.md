# CLI

Binary name `spectreps`, built at `bin/spectreps` by `make build`.

The CLI is a caller of package `spectreps`. Flags exist to fill `RunOptions`, `RewriteOptions`, and file paths. Ghostscript's full switch grammar is not a goal of the current tags. A compatibility mode that accepts a `gs` argv can be proposed later in `plans/v0.0.1/10-deferred.md`.

## Commands

```
spectreps version
spectreps run [options] file.ps
spectreps raster [options] file.ps|file.pdf
spectreps bbox [options] file.ps|file.pdf
spectreps inkcov [options] file.ps|file.pdf
spectreps rewrite [options] file.pdf
spectreps validate [options] file.ps|file.pdf
spectreps compare bytes fileA fileB
spectreps compare raster [options] fileA fileB
```

`version` prints `0.0.1` until the first tag that bumps `Version`, then prints that constant. Exit 0.

Shared options for `run`, `raster`, `compare raster`, `bbox`, and `inkcov`:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-w` | Page width in points | 612 |
| `-h` | Page height in points | 792 |
| `-r` | Pixels per inch | 72 |
| `-o` | Output path | required for `raster` |

`bbox` and `inkcov` write their report to stdout and do not write a file.

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

`rewrite` writes one PDF.

## Exit codes

| Code | When |
| --- | --- |
| 0 | Success. `compare` exits 0 when `Equal` is true. |
| 1 | `JobError`, `ErrNotImplemented`, or a compare mismatch. |
| 2 | Usage. Missing file, unknown flag, unknown command, missing `-o`. |
| 3 | A read or write failed before the interpreter ran. |

`bbox` and `inkcov` exit 0 after printing every page, 1 on an interpreter error, and 2 on a missing input or a bad flag.

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
