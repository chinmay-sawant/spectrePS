# gs argv mapping

Spectre PS takes a rewritten command line. This file maps the `gs` switches that the current subcommands can already express onto Spectre commands and flags, and names the switches that stay rejected. `spectreps gs` accepts the bounded allowlist of these switches; its grammar is in `documentation/gs-argv-grammar.md`.

The commands and flags below are the current CLI. `-pages` landed in v0.0.2, and the gs mode maps `-dFirstPage` and `-dLastPage` onto it. The full PostScript argv grammar stays out.

## Devices and output suffixes

Ghostscript picks a device with `-sDEVICE=name`. `spectreps raster -format` picks the encoder, the `-o` suffix is the fallback, and `spectreps gs` maps `-sDEVICE` onto the same choice. The other jobs are separate subcommands.

| `gs -sDEVICE=` | Spectre | Notes |
| --- | --- | --- |
| `ppmraw` | `spectreps raster -o page.ppm` | Raw P6 bytes. Any suffix that is not `.png`, `.jpg`, or `.jpeg` falls back to PPM, so `.ppm` is a convention, not a check. |
| `png16m` | `spectreps raster -o page.png` | The same RGB pixmap through `image/png`. PNG bytes are not an equality oracle. |
| `jpeg` | `spectreps raster -o page.jpg` or `.jpeg` | The same pixmap through `image/jpeg` at the `-jpegq` quality, default 75. Lossy. |
| `bbox` | `spectreps bbox` | Writes `%%BoundingBox` and `%%HiResBoundingBox` per page to stdout, with no output file. |
| `inkcov` | `spectreps inkcov` | Writes an RGB occupancy line per page to stdout. It is not the weighted `ink_cov` report. |
| `tiff24nc` | `spectreps raster -o page.tiff` | The same pixmap through `golang.org/x/image/tiff`, Deflate by default. TIFF bytes are not an equality oracle. |
| `pdfimage24` | `spectreps pdfimage -o out.pdf` | One 24-bit RGB image page per input page, with Flate streams. |
| `pdfwrite` | `spectreps rewrite -o out.pdf` | PDF inputs in the reader subset only. Spectre writes its own PDF, and the bytes are not expected to match `pdfwrite`. |

`-dJPEGQ=N` maps to `-jpegq N` on the `jpeg` row. Spectre clamps the quality to 1 through 100 and defaults to 75.

Every other device name is rejected. That includes the grayscale and mono raster devices (`pnggray`, `pngmono`, `jpeggray`, `pgmraw`), alpha and color-space variants (`pngalpha`, `pam`, `pamcmyk32`), the TIFF family beyond `tiff24nc`, the bit devices (`bit`, `bitrgb`, `bitcmyk`), the weighted ink device (`ink_cov`), text devices (`txtwrite`), PostScript writers (`ps2write`, `eps2write`), and the printer devices. `spectreps gs` maps the eight names above; every other `-sDEVICE` value exits 2. The weighted ink command has no device name because Ghostscript has no such device, so `spectreps gs -sDEVICE=ink_cov` exits 2 and `spectreps ink_cov` is the only way to reach it.

> TIFF landed in phase 2 of `plans/v0.0.2/4-quick-wins.md` as `.tif` and `.tiff` output on `raster`. The gs mode maps `-sDEVICE=tiff24nc` onto it.

## Output file

`-sOutputFile=path` maps to `-o path` on `raster`, `pdfimage`, and `rewrite`. On `raster`, the `%d` rule follows `documentation/cli.md`:

- One page and no `%d`: the path is used as given.
- `%d`: the token is replaced with the one-based page number. Every occurrence is replaced.
- More than one page and no `%d`: exit 2.
- Only the two characters `%d` count. A printf form such as `%03d` is not recognized, so a multi-page job with `page-%03d.ppm` exits 2.
- There is no stdout form, and the three that look like one are rejected rather than taken as path text. `-sOutputFile=-` exits 2 because Spectre writes files only, `-sOutputFile=%stdout` exits 2 for the same reason, and `-sOutputFile=%pipe%cat` exits 2 because no pipe opens. `documentation/gs-argv-grammar.md` has the exact message for each.

`pdfimage` and `rewrite` write one file and do not use `%d`.

## Resolution and page size

`-r` maps to `-r`. Both are pixels per inch. Spectre's `-r` is one integer applied to both axes, and 0 or an omitted flag selects 72. Per-axis forms have no equivalent.

`-g<W>x<H>` maps to `-w W -h H` only at `-r 72`. `-g` counts device pixels and `-w` and `-h` count points. At another resolution, set `-w` to `W*72/r` and `-h` to `H*72/r`, because the pixmap is `points * dpi / 72` pixels wide. The defaults are 612 by 792 points and 72 dpi.

`-dFirstPage=N` and `-dLastPage=M` map to the planned `-pages` flag:

- `-dFirstPage=N -dLastPage=M` is `-pages N-M`.
- `-dLastPage=M` alone is `-pages 1-M`.
- `-dFirstPage=N` alone is `-pages N-`, an open range that runs to the last page. This is a deliberate difference from Ghostscript, where `-dFirstPage` alone still needs a last page. Spectre's `-pages` grammar has the open end, so the switch maps onto it.

> Landed. `-pages A-B` is row 1.2 of `plans/v0.0.2/4-quick-wins.md`. Its grammar is 1-based and inclusive, a single `N` selects one page, and an omitted flag selects every page. It applies to `raster`, `bbox`, `inkcov`, `ink_cov`, `pdfimage`, and `compare raster`. A range outside the document returns `rangecheck`. For a PostScript input the run executes every page and the filter applies after it, so a failing page inside the range still fails the command. `spectreps gs` maps `-dFirstPage` and `-dLastPage` onto `-pages`; a token outside its allowlist exits 2.

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

These do not map onto a Spectre subcommand:

| Switch | Reason |
| --- | --- |
| `-c` | It runs PostScript code from the command line. Spectre takes a file path and does not evaluate inline programs. |
| `-f` outside `gs` | `spectreps gs` accepts `-f` as the way to name its input. The other subcommands take the input as a positional argument after the flag list and have no `-f`. |
| `-sDEVICE=<name>` | Any name without a row in the device table above. `spectreps gs` maps the eight allowlisted names and exits 2 on the rest. |
| `-dPDFA`, `-dPDFA=1\|2\|3` | Not in the `gs` allowlist. The rewrite profile is `spectreps rewrite -pdfa 4\|4f`, which claims PDF/A-4 and refuses a known violation instead of keeping the claim. |
| `-dNOPAUSE` beyond batch | There is no interactive mode, so the pause behavior has no equivalent. |
| The rest of the grammar | `-d`, `-s`, `-I`, `-P`, `-Z`, `--`, and every other `gs` token. `spectreps gs` accepts only the allowlist in `documentation/gs-argv-grammar.md` and exits 2 on everything else. |

The binary accepts only the allowlisted switches, through `spectreps gs`, and rejects everything else at the first unknown or rejected switch with exit 2. A switch outside the grammar keeps the exit 2 it has in the mapping table. The rewrite in this file stays the readable form for scripts that do not need a `gs` command line.

## Sources

- `documentation/cli.md` for the commands, flags, output suffixes, and the `%d` rule.
- `documentation/devices.md` for the pixmap, the encoders, `bbox`, and `inkcov`.
- `documentation/covered-and-not-covered.md` for the covered jobs and the deferred list.
- `documentation/ghostscript-baseline.md` for the device names and the SAFER default.
- `plans/v0.0.2/4-quick-wins.md` phase 1 for `-pages`, phase 2 for TIFF.
