# gs argv grammar

`spectreps gs` is the bounded compatibility mode for a Ghostscript command line. It is an allowlist, not a Ghostscript clone. A switch is accepted only when it maps onto behavior Spectre already guarantees. A switch whose behavior is always on is accepted and ignored. Anything that would silently change pixels or the output file is rejected with a message that names the switch. The scanner is hand-written, because Go's `flag` package stops at the first positional argument and cannot mix `-sNAME=value` with file names.

`documentation/gs-argv-mapping.md` maps a Ghostscript job onto a Spectre command line. This file is the grammar for the mode that accepts the switches.

## Entry shape

```
spectreps gs [switches] file
spectreps gs [switches] -f file
```

Switches and the input may appear in any order. `-f` names the input and is the way to pass a file whose name starts with `-`. One input is required. A second positional file, a second `-f`, or no input exits 2.

The mode routes the job to the same `raster`, `pdfimage`, `bbox`, `inkcov`, and `rewrite` commands the CLI already has. It does not add a second implementation of any job.

## Exit codes

| Code | When |
| --- | --- |
| 0 | The routed command succeeded. |
| 1 | The interpreter failed, for example `/rangecheck in pages` or `/undefined in show`. |
| 2 | Usage. No `-sDEVICE`, no input, two inputs, a rejected switch, a rejected value, or a missing or malformed `-sOutputFile`. |
| 3 | A read or write failed before the interpreter ran. |

The codes match `documentation/cli.md`.

## Devices

`-sDEVICE=name` selects the command and, for the raster devices, the encoder.

| `-sDEVICE=` | Routes to | Notes |
| --- | --- | --- |
| `ppmraw` | `raster -format ppm` | Raw P6 bytes. |
| `png16m` | `raster -format png` | `image/png`. PNG bytes are not an equality oracle. |
| `jpeg` | `raster -format jpeg` | `image/jpeg` at the `-dJPEGQ` quality. Lossy. |
| `tiff24nc` | `raster -format tiff` | Baseline TIFF through `golang.org/x/image/tiff`, Deflate by default. |
| `bbox` | `bbox` | Writes the bounding box lines to stdout. No output file. |
| `inkcov` | `inkcov` | Writes the RGB occupancy lines to stdout. No output file. |
| `pdfimage24` | `pdfimage` | One 24-bit RGB image page per input page. |
| `pdfwrite` | `rewrite` | PDF input in the reader subset. Spectre writes its own PDF, not Ghostscript's. |

Any other device exits 2:

```
spectreps: -sDEVICE=ps2write is not an accepted device
```

The rejected names include the grayscale and mono raster devices (`pnggray`, `pngmono`, `jpeggray`, `pgmraw`), alpha and color-space variants (`pngalpha`, `pam`, `pamcmyk32`, `tiff32nc`), the bit devices, text devices (`txtwrite`), EPS (`eps2write`), and the printer devices.

`-dJPEGQ=N` is accepted only with `-sDEVICE=jpeg` and maps to `-jpegq N`. Spectre clamps the quality to 1 through 100. On any other device it exits 2 with `spectreps: -dJPEGQ needs -sDEVICE=jpeg`.

`pdfwrite` takes a PDF input in the reader subset. A PostScript input fails at open with exit 1, because `rewrite` opens a PDF and does not run the interpreter.

### raster -format

`raster` gains one flag:

| Flag | Meaning | Default |
| --- | --- | --- |
| `-format` | Encoder: `ppm`, `png`, `jpeg`, or `tiff` | from the `-o` suffix |

The device wins over the `-o` suffix. `spectreps gs -sDEVICE=png16m -sOutputFile=page.ppm` writes PNG bytes to `page.ppm`. Without `-format`, the suffix rule in `documentation/cli.md` holds: `.png`, `.jpg`, `.jpeg`, `.tif`, and `.tiff` select their encoder, and every other suffix falls back to PPM.

A value outside the four names exits 2:

```
spectreps: -format wants ppm, png, jpeg, or tiff, got "bmp"
```

## Output

`-sOutputFile=path` maps to `-o path`. `raster` keeps the `%d` rule from `documentation/cli.md`: `%d` is replaced with the one-based emitted page number, every occurrence, and a multi-page job without `%d` exits 2. `pdfimage` and `rewrite` write one file, so no `%` is accepted for those devices.

`bbox` and `inkcov` write their report to stdout and take no output file. `-sOutputFile` on those devices exits 2 with `spectreps: -sOutputFile has no place on -sDEVICE=bbox`.

Rejected forms, each with exit 2:

| Form | Why |
| --- | --- |
| `-sOutputFile=-` | Spectre writes files only. |
| `-sOutputFile=%stdout` | Same. |
| `-sOutputFile=%pipe%cat` | No pipe opens, because `file` returns `invalidaccess`. |
| `page-%03d.ppm` | Only the two characters `%d` count, so a printf width is not a page pattern. |
| any `%` on `pdfimage24` or `pdfwrite` | Those commands write one file per job. |

```
spectreps: -sOutputFile=- is a stream, not a file path
spectreps: -sOutputFile wants only %d in a path, got "page-%03d.ppm"
spectreps: -sOutputFile wants a plain path on -sDEVICE=pdfwrite, got "page-%d.pdf"
```

## Page range

| gs | Spectre |
| --- | --- |
| `-dFirstPage=N -dLastPage=M` | `-pages N-M` |
| `-dFirstPage=N` | `-pages N-` |
| `-dLastPage=M` | `-pages 1-M` |

The `-pages` grammar is 1-based and inclusive, from `documentation/cli.md`. `-pages N-` runs from page N to the last page. An end past the last page clamps to the last page, and a start past the last page exits 1 with `/rangecheck in pages`. `-dFirstPage=N` maps to the open range `N-`, so it runs to the last page rather than selecting one page.

`-dLastPage` before `-dFirstPage` exits 2:

```
spectreps: -dLastPage=1 is before -dFirstPage=2
```

`-sPageList` accepts a comma list of single pages and closed ranges: `2`, `1,2,3`, `1-2,3`, `2-3`. The elements must rise in page order and touch, so the list describes one contiguous `-pages` range. Anything else exits 2 with a message that names the value. The rejected forms are:

| Form | Example |
| --- | --- |
| `even` and `odd` selections | `-sPageList=even` |
| An open range | `-sPageList=1-`, `-sPageList=-3` |
| A reversed range | `-sPageList=3-1` |
| A gap | `-sPageList=1,3` |
| An overlap or a repeat | `-sPageList=1-2,2-3`, `-sPageList=2,2` |
| Page 0 or a non-number | `-sPageList=0`, `-sPageList=x` |
| An argument file | `-sPageList=@pages` |

`-sPageList` cannot be combined with `-dFirstPage` or `-dLastPage`. That exits 2.

`rewrite` cannot select pages, so `-dFirstPage`, `-dLastPage`, and `-sPageList` exit 2 on `pdfwrite`:

```
spectreps: -dFirstPage has no place on -sDEVICE=pdfwrite
```

## Geometry

| gs | Spectre |
| --- | --- |
| `-rN` | `-r N` |
| `-dDEVICEWIDTHPOINTS=N` | `-w N` |
| `-dDEVICEHEIGHTPOINTS=N` | `-h N` |
| `-gWxH` | `-w W -h H`, only when the resolved resolution is 72 |

`-r` takes one integer for both axes and is attached as in `-r300`. A per-axis form such as `-r300x300` exits 2, because Spectre's `-r` is one integer. `-r0` and an omitted `-r` both select 72.

```
spectreps: -r300x300 is not accepted, use -r300
spectreps: -r needs a resolution, e.g. -r300
```

`-g` counts device pixels and `-w` and `-h` count points. They agree only at 72 dpi, which is the default, so `-gWxH` is accepted when the resolved resolution is 72. With `-r144` the same job exits 2. A malformed `-g` also exits 2.

```
spectreps: -g needs -r 72, got -r 144
spectreps: -g wants WxH, got "144"
```

The last width or height assignment wins when `-g` and a `-dDEVICE*POINTS` switch both appear.

`rewrite` has no geometry flags, so `-r`, `-g`, `-dDEVICEWIDTHPOINTS`, and `-dDEVICEHEIGHTPOINTS` exit 2 on `pdfwrite`.

## Accept and ignore

These switches describe behavior Spectre always has. The scanner accepts them and passes nothing to the command.

| Switch | Spectre behavior |
| --- | --- |
| `-dBATCH` | Every command reads the file it was given and exits. |
| `-dNOPAUSE` | Spectre never prompts and never waits for a key. |
| `-q` | stdout carries only the command report, and stderr carries errors. |
| `-dSAFER` | The banned operators return `invalidaccess`; see `documentation/language.md`. |
| `-dFIXEDMEDIA` | The pixmap uses `-w` and `-h`. There is no media list to search. |

`-dNAME=true` and `-dNAME=1` are accepted for the `-d` forms. `-dNAME=false` and `-dNAME=0` would turn off always-on behavior, so they exit 2:

```
spectreps: -dSAFER=false would turn off always-on behavior
```

### Rejected safety switches

`-dNOSAFER` and `-dDELAYSAFER` exit 2. SAFER is always on, and there is no unsafe mode to select.

```
spectreps: -dNOSAFER is rejected, SAFER is always on
```

## Input

One input file is required. Give it as the one positional token or after `-f`. `-c` is rejected because Spectre does not evaluate inline PostScript:

```
spectreps: -c runs inline PostScript, pass a file
```

`-` (stdin), `@file`, and multiple inputs stay out of the mode and exit 2. Inline PostScript, argument files, and stream outputs wait for a later plan.

## The -s and -d value allowlist

Only these names are parsed. Any other `-sNAME` or `-dNAME` exits 2 with `spectreps: -dFoo is not in the gs allowlist`.

| Switch | Maps to |
| --- | --- |
| `-sDEVICE=name` | The device table above. |
| `-sOutputFile=path` | `-o path`. |
| `-sPageList=list` | `-pages range`. |
| `-dFirstPage=N` | `-pages N-`. |
| `-dLastPage=M` | `-pages 1-M`. |
| `-dDEVICEWIDTHPOINTS=N` | `-w N`. |
| `-dDEVICEHEIGHTPOINTS=N` | `-h N`. |
| `-dJPEGQ=N` | `-jpegq N`, `-sDEVICE=jpeg` only. |
| `-dBATCH`, `-dNOPAUSE`, `-dSAFER`, `-dFIXEDMEDIA` | Accept and ignore. |
| `-dNOSAFER`, `-dDELAYSAFER` | Rejected. |

A known name without its value exits 2: `spectreps: -dFirstPage needs =N`.

## Examples

A `ps2pdf` shaped job:

```
spectreps gs -q -dNOPAUSE -dBATCH -sDEVICE=pdfwrite -sOutputFile=out.pdf in.pdf
```

Pages 2 through the end as PNG at 144 dpi:

```
spectreps gs -sDEVICE=png16m -sOutputFile=page-%d.png -dFirstPage=2 -r144 in.ps
```

A one-page bounding box report:

```
spectreps gs -sDEVICE=bbox -dLastPage=1 in.pdf
```

## Sources

- `documentation/cli.md` for the commands, flags, output suffixes, `%d` rule, and exit codes.
- `documentation/gs-argv-mapping.md` for the rewritten command lines behind each device.
- `documentation/devices.md` for the pixmap, the encoders, `bbox`, and `inkcov`.
- `documentation/language.md` for the banned operators behind `-dSAFER`.
