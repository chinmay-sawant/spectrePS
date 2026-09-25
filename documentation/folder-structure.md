# Folder structure

The module path is `github.com/chinmay-sawant/spectrePS`. The last element stays `spectrePS`, matching the GitHub repository. The public library is package `spectreps` in `spectreps/`. Its import path is `github.com/chinmay-sawant/spectrePS/spectreps`.

The module root holds `go.mod`, `go.sum`, the Makefile, the license, and the prose. Library `.go` files live under `spectreps/` and `internal/`. `go.mod` names one third-party requirement, `golang.org/x/image`, for TIFF encoding and image scaling. `go.sum` records its checksum.

## What is in the tree now

```
AGENTS.md
Makefile
README.md
go.mod
go.sum
cmd/spectreps/main.go
internal/cli/
internal/engine/
internal/graphics/
internal/pdf/
internal/pdfout/
internal/ps/
spectreps/
sampledata/
testdata/
documentation/
plans/v0.0.1/
plans/v0.0.2/
plans/v0.0.3/
skills/phase-wise-checklist/SKILLS.md
skills/unslop/SKILL.md
skills/PR/
scripts/
```

`documentation/` is the prose folder for this repository. `plans/v0.0.1/`, `plans/v0.0.2/`, and `plans/v0.0.3/` are the execution ledgers. `sampledata/` holds the PDFs the compression plan measures. `skills/` holds agent instructions that already live in this repo.

## Public library

`spectreps/` exports the signatures in `documentation/public-api.md`.

```
spectreps/doc.go
spectreps/errors.go
spectreps/instance.go
spectreps/options.go
spectreps/postscript.go
spectreps/pdf.go
spectreps/raster.go
spectreps/image.go
spectreps/measure.go
spectreps/compare.go
spectreps/api_test.go
spectreps/compare_test.go
spectreps/image_test.go
spectreps/measure_test.go
spectreps/pdf_test.go
spectreps/raster_test.go
```

The test files use `package spectreps_test`. Another module imports `github.com/chinmay-sawant/spectrePS/spectreps` and nothing under `internal/`.

## Command

`cmd/spectreps/main.go` is the process entry. It calls `internal/cli`. `internal/cli` parses flags, reads files, and calls the public library. It does not import `internal/engine`.

## Private code

`internal/engine` holds the session and file byte compare. `internal/ps` is the PostScript interpreter, `internal/graphics` the device, matrix, and pixmap layer, `internal/pdf` the PDF reader, `internal/pdfout` the PDF writers, and `internal/cli` the command layer. Add a directory when its first `.go` file or fixture is real. Do not add `pkg/`, `api/`, `util/`, or empty placeholder packages.

`testdata/` holds input files and expected PPM bytes. `sampledata/` holds the PDFs the compression plan measures, separate from test fixtures. Golden files are written by the test that first locks a case, then checked in. They are not copied from Ghostscript output. Matching Ghostscript byte for byte is not a success criterion.

Package `spectreps` calls `internal/engine`, `internal/ps`, `internal/graphics`, `internal/pdf`, and `internal/pdfout`. `internal/cli` stays on the public library, which is the same boundary an external program has.

## Ownership

| Caller | May import |
| --- | --- |
| `cmd/spectreps` | `github.com/chinmay-sawant/spectrePS/internal/cli` |
| `internal/cli` | `github.com/chinmay-sawant/spectrePS/spectreps` |
| `package spectreps` | `internal/engine`, and later the interpreter packages under `internal/` |
| `package spectreps_test` | the public package only |
| another module | the public package only |

Go rejects an import of `internal/` from outside this module. That is why the exported functions exist before the interpreter does.
