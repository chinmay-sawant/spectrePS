# Spectre PS

Spectre PS is a Go program for the jobs Ghostscript is used for. It reads PostScript and PDF, paints pages to pixels, writes a new PDF, reports interpreter errors, and compares bytes.

The command is `spectreps`. The library import path is `github.com/chinmay-sawant/spectrePS/spectreps`, package name `spectreps`. The command calls that package through `internal/cli`. A later importer calls the same functions. The module does not link the system `gs` library and does not start `gs`.

v0.0.1 is the first release. It includes the PostScript subset, the path-only PDF, PPM and PNG raster, Flate rewrite, validate, and byte and pixel compare. `spectreps version` prints `0.0.1`. The release note is `plans/v0.0.1/PR/release-v0.0.1.md`. Work that is not in this release stays in `plans/v0.0.1/10-deferred.md`.

Contracts are in `documentation/README.md`. The work ledger is `plans/v0.0.1/00-program.md`.

## License

Spectre PS is released under the MIT license. See `LICENSE`. The copyright is Chinmay Sawant's. This project does not link Ghostscript and does not include Ghostscript source.
