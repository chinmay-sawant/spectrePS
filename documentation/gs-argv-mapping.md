# gs argv mapping

Spectre PS does not accept a `gs` argv. This file maps the `gs` switches that the current subcommands can already express onto Spectre commands and flags, and names the switches that stay rejected.

The commands and flags below are the current CLI. The only planned addition is `-pages`, marked where it appears. The full PostScript argv grammar stays out, per `plans/v0.0.1/10-deferred.md` row 10.2.

## Devices and output suffixes

Ghostscript picks a device with `-sDEVICE=name`. Spectre has no device flag. `spectreps raster` picks the encoder from the `-o` suffix, and the other jobs are separate subcommands.

| `gs -sDEVICE=` | Spectre | Notes |
| --- | --- | --- |
| `ppmraw` | `spectreps raster -o page.ppm` | Raw P6 bytes. Any suffix that is not `.png`, `.jpg`, or `.jpeg` falls back to PPM, so `.ppm` is a convention, not a check. |
| `png16m` | `spectreps raster -o page.png` | The same RGB pixmap through `image/png`. PNG bytes are not an equality oracle. |
| `jpeg` | `spectreps raster -o page.jpg` or `.jpeg` | The same pixmap through `image/jpeg` at the `-jpegq` quality, default 75. Lossy. |
| `bbox` | `spectreps bbox` | Writes `%%BoundingBox` and `%%HiResBoundingBox` per page to stdout, with no output file. |
| `inkcov` | `spectreps inkcov` | Writes an RGB occupancy line per page to stdout. It is not the weighted `ink_cov` report. |
| `pdfimage24` | `spectreps pdfimage -o out.pdf` | One 24-bit RGB image page per input page, with Flate streams. |
| `pdfwrite` | `spectreps rewrite -o out.pdf` | PDF inputs in the reader subset only. Spectre writes its own PDF, and the bytes are not expected to match `pdfwrite`. |

`-dJPEGQ=N` maps to `-jpegq N` on the `jpeg` row. Spectre clamps the quality to 1 through 100 and defaults to 75.

Every other device name is rejected. That includes the grayscale and mono raster devices (`pnggray`, `pngmono`, `jpeggray`, `pgmraw`), alpha and color-space variants (`pngalpha`, `pam`, `pamcmyk32`), the TIFF family (`tiff24nc` and the rest), the bit devices (`bit`, `bitrgb`, `bitcmyk`), text devices (`txtwrite`), PostScript writers (`ps2write`, `eps2write`), and the printer devices. Spectre selects an encoder from the output path, so a device name has no place to go.

> TIFF is planned in phase 2 of `plans/v0.0.3/1-quick-wins.md`. That phase adds `.tif` and `.tiff` output to `raster`. It does not add a `-sDEVICE` flag, and it is not on this branch.

## Output file

`-sOutputFile=path` maps to `-o path` on `raster`, `pdfimage`, and `rewrite`. On `raster`, the `%d` rule follows `documentation/cli.md`:

- One page and no `%d`: the path is used as given.
- `%d`: the token is replaced with the one-based page number. Every occurrence is replaced.
- More than one page and no `%d`: exit 2.
- Only the two characters `%d` count. A printf form such as `%03d` is not recognized, so a multi-page job with `page-%03d.ppm` exits 2.
- No stdout form exists. `%stdout`, `%pipe%`, and a trailing `-` are literal path text, not streams.

`pdfimage` and `rewrite` write one file and do not use `%d`.

## Resolution and page size

`-r` maps to `-r`. Both are pixels per inch. Spectre's `-r` is one integer applied to both axes, and 0 or an omitted flag selects 72. Per-axis forms have no equivalent.

`-g<W>x<H>` maps to `-w W -h H` only at `-r 72`. `-g` counts device pixels and `-w` and `-h` count points. At another resolution, set `-w` to `W*72/r` and `-h` to `H*72/r`, because the pixmap is `points * dpi / 72` pixels wide. The defaults are 612 by 792 points and 72 dpi.

`-dFirstPage=N` and `-dLastPage=M` map to the planned `-pages` flag:

- `-dFirstPage=N -dLastPage=M` is `-pages N-M`.
- `-dLastPage=M` alone is `-pages 1-M`.
- `-dFirstPage=N` alone has no exact match. `-pages N` selects one page, not N through the end.

> Planned, not on this branch. `-pages A-B` is row 1.2 of `plans/v0.0.3/1-quick-wins.md` and lands on a parallel branch. Its grammar is 1-based and inclusive, a single `N` selects one page, and an omitted flag selects every page. It applies to `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster`. A range outside the document returns `rangecheck`. For a PostScript input the run executes every page and the filter applies after it, so a failing page inside the range still fails the command. On this branch no page selection flag exists, so `-dFirstPage` and `-dLastPage` are unknown flags and exit 2.

## Batch, pause, and quiet

Three switches describe behavior that is always on, so no flag exists for any of them:

- `-dBATCH`: every Spectre command reads the file arguments it was given and exits. There is no interactive loop and no input to continue.
- `-dNOPAUSE`: Spectre never prompts and never waits for a key.
- `-q`: stdout carries only the command's report (`version`, `bbox`, `inkcov`, a compare mismatch) and stderr carries errors. There is no banner or progress output.

`-dNOPAUSE` behavior beyond batch has nothing to map. There is no interactive mode for the pause to guard.

## SAFER

`-dSAFER` behavior is always on and there is no flag. The banned operators `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` return `invalidaccess`; see `documentation/language.md`. Pipe paths never open, because `file` fails before it reads the path.

`-dNOSAFER` is rejected. Spectre has no unsafe mode, and `-dDELAYSAFER` has no equivalent either.

## Switches that stay rejected

These do not map and are not planned to map:

| Switch | Reason |
| --- | --- |
| `-c` | It runs PostScript code from the command line. Spectre takes a file path and does not evaluate inline programs. |
| `-f` | It marks the end of options and names the input in `gs`. Spectre takes the input as a positional argument after the subcommand. A leading `-f` is an unknown flag and exits 2. |
| `-sDEVICE=<name>` | Any name without a row in the device table above. Spectre selects the encoder from `-o`, and no device flag exists. |
| `-dPDFA`, `-dPDFA=1|2|3` | Spectre does not create PDF/A and `validate` does not certify it. PDF/A stays deferred in `plans/v0.0.1/10-deferred.md` row 10.2. |
| `-dNOPAUSE` beyond batch | There is no interactive mode, so the pause behavior has no equivalent. |
| The rest of the grammar | `-d`, `-s`, `-I`, `-P`, `-Z`, `--`, and every other `gs` token. A second flag grammar would fork the CLI, so it stays out. |

None of these switches are accepted by the binary. A `gs` command line fails at the first unknown command or flag with exit 2. The mapping in this file describes how to rewrite a `gs` job as a Spectre command. It is not a compatibility mode.

## Sources

- `documentation/cli.md` for the commands, flags, output suffixes, and the `%d` rule.
- `documentation/devices.md` for the pixmap, the encoders, `bbox`, and `inkcov`.
- `documentation/covered-and-not-covered.md` for the covered jobs and the deferred list.
- `documentation/ghostscript-baseline.md` for the device names and the SAFER default.
- `plans/v0.0.3/1-quick-wins.md` phase 1 for `-pages`, phase 2 for TIFF.
