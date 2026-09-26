# Spectre PS

Spectre PS is a Go library and CLI for the jobs Ghostscript is used for. It reads a PostScript subset and a PDF subset, paints pages to RGB pixels, writes new PDFs and PostScript, reports interpreter errors, extracts text, and compares bytes or pixels. It does not link or start Ghostscript.

The command is `spectreps`. The library import path is `github.com/chinmay-sawant/spectrePS/spectreps`, package name `spectreps`. The command calls that package through `internal/cli`, and an external importer calls the same functions.

v0.0.1 is the first release: the PostScript subset, the path-only PDF, PPM and PNG raster, Flate rewrite, validate, and byte and pixel compare. v0.0.2 adds `bbox`, `inkcov`, JPEG and TIFF raster, page ranges, gray and CMYK image PDF, and PDF compression levels 1 to 5. v0.0.3 adds weighted `ink_cov`, CCITT and JPEG2000 decode, painting `Do`, `spectreps ps`, the `spectreps gs` argv mode, the PDF/A-4 and PDF/UA-2 preflights, and the font and text machine with `spectreps text`. v0.0.4 adds the Type 1 font machine, `spectreps info`, PDF transparency, the extra stream filters and predictors, PDF/UA-2 tag generation, and the checked-in validation corpus. `spectreps version` prints `0.0.4`.

The latest tag is v0.0.4, and that is the version the command reports. Work that v0.0.4 deferred is listed in `plans/v0.0.1/10-deferred.md`.

## Build

```sh
git clone https://github.com/chinmay-sawant/spectrePS.git
cd spectrePS
make build
./bin/spectreps version
```

## Demo

Paint a 100 by 100 square on a 200 by 200 point page, then read the page back.

```sh
printf '0 0 moveto 100 0 lineto 100 100 lineto 0 100 lineto closepath fill\n' > square.ps

# The painted box in points and the RGB mark coverage.
./bin/spectreps bbox -w 200 -h 200 -r 72 square.ps
./bin/spectreps inkcov -w 200 -h 200 -r 72 square.ps

# Encode the same page as PNG or JPEG.
./bin/spectreps raster -o square.png -w 200 -h 200 -r 72 square.ps
./bin/spectreps raster -o square.jpg -jpegq 90 -w 200 -h 200 -r 72 square.ps

# Wrap the page in a new PDF as one 24-bit RGB image.
./bin/spectreps pdfimage -o square.pdf -w 200 -h 200 -r 72 square.ps

# Rasterize two inputs and compare the pixels.
./bin/spectreps compare raster -w 200 -h 200 -r 72 square.ps square.ps
```

The first two commands print:

```
%%BoundingBox: 0 0 100 100
%%HiResBoundingBox: 0 0 100 100
Page 1
0.25000 0.25000 0.25000 RGB
```

## Commands

| Command | Job |
| --- | --- |
| `spectreps version` | Print the version. |
| `spectreps run` | Execute a PostScript program and paint its pages. |
| `spectreps raster` | Write PPM, PNG, JPEG, or TIFF from the selected pages. |
| `spectreps pdfimage` | Wrap each painted page in a new PDF as one RGB, gray, or CMYK image. |
| `spectreps bbox` | Print the painted box in points. |
| `spectreps inkcov` | Print the RGB mark coverage fractions. |
| `spectreps ink_cov` | Print the weighted RGB ink amounts as percentages. |
| `spectreps rewrite` | Write a new PDF, with optional Flate, compression levels 1 to 5, a PDF/A-4 claim, TrueType font subsetting, and a PDF/UA-2 structure tree. |
| `spectreps ps` | Write a path-only PDF as date-free PostScript. |
| `spectreps text` | Print the extracted text of the selected pages. |
| `spectreps info` | Print the document version, page count, page sizes, tagged flag, fonts, and image count. |
| `spectreps gs` | Run an allowlisted `gs` argv against the subcommands above. |
| `spectreps validate` | Stop on the first interpreter error. |
| `spectreps compare bytes` | Compare two files byte by byte. |
| `spectreps compare raster` | Rasterize two inputs and compare the pixels. |

`rewrite -level 0` takes a PDF whose content uses the path subset. A page that paints an image is refused with `undefined in Do` instead of dropping the image. Levels 1 to 5 use the pass-through writer, so text, fonts, and images survive, and Spectre can rasterize the `pdfimage` output through `Do`.

## Documentation

`documentation/` holds the contracts. The index is [documentation/README.md](documentation/README.md).

| Document | Covers |
| --- | --- |
| [cli.md](documentation/cli.md) | Subcommands, flags, output files, exit codes. |
| [public-api.md](documentation/public-api.md) | Exported types and functions. |
| [devices.md](documentation/devices.md) | Raster, compare, rewrite, validate, box and coverage, bitmap PDF. |
| [language.md](documentation/language.md) | PostScript subset, errors, limits, banned operators. |
| [architecture.md](documentation/architecture.md) | Instance, front ends, graphics engine, library boundary. |
| [covered-and-not-covered.md](documentation/covered-and-not-covered.md) | Which Ghostscript jobs Spectre takes. |
| [features.md](documentation/features.md) | The feature inventory: what works now, and what waits. |
| [test.md](documentation/test.md) | The tests each job needs. |
| [fonts.md](documentation/fonts.md) | Font metrics, encodings, and the text scope. |
| [gs-argv-grammar.md](documentation/gs-argv-grammar.md) | The `spectreps gs` allowlist. |
| [gs-argv-mapping.md](documentation/gs-argv-mapping.md) | The `gs` device to `spectreps` command mapping. |
| [performance.md](documentation/performance.md) | Measured baselines, profiles, and the allocation budgets. |
| [reference-proofs.md](documentation/reference-proofs.md) | The Ghostscript and veraPDF cross-checks, run by hand. |
| [development.md](documentation/development.md) | Make targets and the phase checklist rule. |

The work ledgers are [plans/v0.0.1/00-program.md](plans/v0.0.1/00-program.md), [plans/v0.0.2/00-program.md](plans/v0.0.2/00-program.md), [plans/v0.0.3/00-program.md](plans/v0.0.3/00-program.md), and [plans/v0.0.4/00-program.md](plans/v0.0.4/00-program.md). The release notes are [plans/v0.0.1/PR/release-v0.0.1.md](plans/v0.0.1/PR/release-v0.0.1.md), [plans/v0.0.2/PR/release-v0.0.2.md](plans/v0.0.2/PR/release-v0.0.2.md), [plans/v0.0.3/PR/release-v0.0.3.md](plans/v0.0.3/PR/release-v0.0.3.md), and [plans/v0.0.4/PR/release-v0.0.4.md](plans/v0.0.4/PR/release-v0.0.4.md), and deferred work is [plans/v0.0.1/10-deferred.md](plans/v0.0.1/10-deferred.md).

## License

Spectre PS is released under the MIT license. See `LICENSE`. The copyright is Chinmay Sawant's. This project does not link Ghostscript and does not include Ghostscript source.
