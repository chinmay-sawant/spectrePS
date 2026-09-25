# Reference proofs

These proofs run external tools by hand. They are not a build or run-time
dependency, they never run inside `make test` or `make lint`, and a verdict
belongs to the tool and the machine, not to Spectre. The plan row is
`plans/v0.0.4/5-validation.md` phase 11.

## Tool matrix

Measured on 2026-09-26 on the development machine (13th Gen Intel Core i7-13700HX,
24 threads, 7.6 GiB RAM, Go 1.26.4).

| Tool | Version | Path | Role |
| --- | --- | --- | --- |
| Ghostscript | 9.55.0 | `/usr/bin/gs` | Pixel reference for the axis-aligned gate and the rewrite cross-check. |
| veraPDF | 1.30.2 | `verapdf` on PATH, local copy at `./verapdf/verapdf` | PDF/A-4 and PDF/UA-2 proof. |
| qpdf | absent | none | Optional rewrite structure check. |
| pdfcpu | absent | none | Optional rewrite structure check. |
| benchstat | absent | none | Optional benchmark comparison in `make bench-check`. |
| hyperfine | absent | none | Optional CLI timing in `scripts/bench-cli.sh`. |

`make refs-gs-check` resolves `./ghostscript/bin/gs` first and `gs` on PATH
second, and prints a skip when neither exists. `make pdfa-check` and
`make pdfua2-check` resolve `./verapdf/verapdf` first and `verapdf` on PATH
second, and skip when neither exists.

## Ghostscript pixel proofs

Command:

```sh
make refs-gs-check
```

The script builds `bin/spectreps`, renders each program with `spectreps raster
-w 612 -h 792 -r 72` and with `gs -q -dNOPAUSE -dBATCH -dSAFER
-sDEVICE=ppmraw -g612x792 -sOutputFile=out.ppm`, and writes both to the
gitignored `references/`.

Normalization rule: the pixel body starts after the `255` max-value line. gs
writes a comment line the Spectre PPM does not, so the header is dropped on
both sides before any comparison.

Geometry rule: Spectre takes the page size from `-w` and `-h`, and gs takes it
from the `/MediaBox`. A PostScript program with no `setpagedevice` gets the gs
default page, which is A4 on this build, so the PostScript cases pin gs with
`-g612x792` to the Spectre default of 612 by 792 points at 72 dpi.

Measured on 2026-09-26, body of 1,454,112 bytes (612 by 792 pixels of RGB):

| Case | Program | Verdict |
| --- | --- | --- |
| Axis-aligned strokes | `sampledata/validation/refs/line.ps` | Exact match. Required. |
| Axis-aligned fill | `sampledata/validation/refs/rect.ps` | Exact match. Required. |
| Diagonal stroke | `sampledata/validation/refs/diag.ps` | 1,506 differing bytes of 1,454,112. Recorded. |
| Cubic curve | `sampledata/validation/refs/curve.ps` | 5,934 differing bytes of 1,454,112. Recorded. |

The diagonal and curve counts belong to gs 9.55.0 and this machine. They are
recorded, not gated. The axis-aligned gate is exact.

## Rewrite cross-check

Command:

```sh
make refs-gs-check
```

The same script rewrites `sampledata/compress/path.pdf` and
`sampledata/validation/paths/path.pdf` at level 2, renders the source and the
rewrite with gs, normalizes both bodies, and compares:

| File | Verdict |
| --- | --- |
| `compress-path` | 0 differing bytes of 120,000. |
| `paths-path` | 0 differing bytes of 120,000. |

qpdf and pdfcpu are absent on this machine, so the script printed
`qpdf and pdfcpu not installed, skipping the validation step`. The step is
present and runs when either tool is installed.

## veraPDF proofs

Commands:

```sh
make pdfa-check
make pdfua2-check
```

`make pdfa-check` checks the base files with `--flavour 4` and the `4f-` files
with `--flavour 4f`, over `sampledata/pdfa/` and
`sampledata/validation/pdfa/`, excluding `negative/`. On 2026-09-26, veraPDF
1.30.2 reported `compliant="2" nonCompliant="0" failedJobs="0"` for the base
files and `compliant="1" nonCompliant="0" failedJobs="0"` for the 4f file.

`make pdfua2-check` checks `sampledata/pdfua2/` and
`sampledata/validation/tagged/` with `--flavour ua2 --format json`, excluding
`negative/`. On 2026-09-26, veraPDF 1.30.2 reported 7 total jobs, 7 successful,
and 0 failed. The negative folders hold the deliberate failures
(`sampledata/pdfua2/negative/untagged.pdf`,
`sampledata/pdfua2/negative/no-title.pdf`, and the veraPDF fail files under
`sampledata/validation/pdfa/negative/` and
`sampledata/validation/tagged/negative/`).

## Boundaries

- No pixel under `sampledata/validation/` was produced by Ghostscript. The gs
  output stays under the gitignored `references/`.
- The reference checks print a skip and exit 0 when their tool is absent, so
  they are safe on machines without gs or veraPDF.
- The page-size and normalization rules above are the only transformations
  applied before a comparison. No anti-aliasing, gamma, or color transform is
  applied to either side.
