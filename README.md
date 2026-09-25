# Spectre PS

Spectre PS is a Go library and CLI for the jobs Ghostscript is used for. It reads a PostScript subset and a path-only PDF subset, paints pages to RGB pixels, writes new PDFs, reports interpreter errors, and compares bytes. It does not link or start Ghostscript.

The command is `spectreps`. The library import path is `github.com/chinmay-sawant/spectrePS/spectreps`, package name `spectreps`. The command calls that package through `internal/cli`, and an external importer calls the same functions.

v0.0.1 is the first release: the PostScript subset, the path-only PDF, PPM and PNG raster, Flate rewrite, validate, and byte and pixel compare. v0.0.2 adds `bbox`, `inkcov`, JPEG and TIFF raster, page ranges, gray and CMYK image PDF, and PDF compression levels 1 to 5. `spectreps version` prints `0.0.2`.

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
| `spectreps run` | Execute a PostScript program and paint its pages. |
| `spectreps raster` | Write PPM, PNG, JPEG, or TIFF from the selected pages. |
| `spectreps pdfimage` | Wrap each painted page in a new PDF as one RGB, gray, or CMYK image. |
| `spectreps bbox` | Print the painted box in points. |
| `spectreps inkcov` | Print the RGB mark coverage fractions. |
| `spectreps rewrite` | Write a new PDF, with optional Flate and compression levels 1 to 5. |
| `spectreps validate` | Stop on the first interpreter error. |
| `spectreps compare bytes` | Compare two files byte by byte. |
| `spectreps compare raster` | Rasterize two inputs and compare the pixels. |

`rewrite -level 0` takes a PDF whose content uses the path subset. Levels 1 to 5 use the pass-through writer, so text, fonts, and images survive. The image PDF from `pdfimage` carries `Do`, which Spectre does not interpret yet, so it is not a `rewrite` input at level 0.

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
| [development.md](documentation/development.md) | Make targets and the phase checklist rule. |

The work ledgers are [plans/v0.0.1/00-program.md](plans/v0.0.1/00-program.md) and [plans/v0.0.2/00-program.md](plans/v0.0.2/00-program.md). The v0.0.1 release note is [plans/v0.0.1/PR/release-v0.0.1.md](plans/v0.0.1/PR/release-v0.0.1.md), and deferred work is [plans/v0.0.1/10-deferred.md](plans/v0.0.1/10-deferred.md).

## License

Spectre PS is released under the MIT license. See `LICENSE`. The copyright is Chinmay Sawant's. This project does not link Ghostscript and does not include Ghostscript source.
