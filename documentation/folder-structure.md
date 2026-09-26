# Folder structure

The module path is `github.com/chinmay-sawant/spectrePS`. The last element stays `spectrePS`, matching the GitHub repository. The public library is package `spectreps` in `spectreps/`. Its import path is `github.com/chinmay-sawant/spectrePS/spectreps`.

The module root holds `go.mod`, `go.sum`, the Makefile, the license, and the prose. Library `.go` files live under `spectreps/` and `internal/`. `go.mod` names two direct third-party requirements, `golang.org/x/image` for TIFF encoding, image scaling, CCITT, and `sfnt` glyph parsing, and `github.com/mrjoshuak/go-jpeg2000` for JPEG2000 decode, plus the indirect `golang.org/x/sys` and `golang.org/x/text`. `go.sum` records their checksums.

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
internal/font/
internal/graphics/
internal/pdf/
internal/pdfa/
internal/pdfout/
internal/ps/
internal/psout/
internal/tag/
internal/truetypesynth/
internal/type1synth/
internal/validation/
spectreps/
sampledata/
documentation/
plans/v0.0.1/
plans/v0.0.2/
plans/v0.0.3/
plans/v0.0.4/
plans/v0.0.5/
plans/PR/
skills/phase-wise-checklist/SKILLS.md
skills/unslop/SKILL.md
skills/PR/
scripts/
```

`verapdf/` and `bin/` also appear on a working machine. Both are gitignored: `bin/` is the built command from `make build`, and `verapdf/` is a local veraPDF install that only `make pdfa-check` and `make pdfua2-check` use.

`documentation/` is the prose folder for this repository. `plans/v0.0.1/` through `plans/v0.0.5/` are the execution ledgers, one folder per phase, and `plans/PR/` holds the pull request text. `plans/v0.0.1/10-deferred.md` is the one list of work that still waits. `sampledata/` holds the fixtures, the scenario PDFs, and the validation corpus. `skills/` holds agent instructions that already live in this repo.

## Public library

`spectreps/` exports the signatures in `documentation/public-api.md`.

```
spectreps/doc.go
spectreps/errors.go
spectreps/instance.go
spectreps/options.go
spectreps/postscript.go
spectreps/pdf.go
spectreps/pdfa.go
spectreps/raster.go
spectreps/image.go
spectreps/measure.go
spectreps/compare.go
spectreps/extract.go
spectreps/info.go
spectreps/subset.go
spectreps/tag.go
spectreps/validate.go
```

Those are the 16 implementation files. The 22 `*_test.go` files beside them all use `package spectreps_test`. Another module imports `github.com/chinmay-sawant/spectrePS/spectreps` and nothing under `internal/`.

## Command

`cmd/spectreps/main.go` is the process entry. It calls `internal/cli`. `internal/cli` parses flags, reads files, and calls the public library. It does not import `internal/engine`.

## Private code

`internal/engine` holds the session and file byte compare. `internal/ps` is the PostScript interpreter, `internal/graphics` the device, matrix, and pixmap layer, `internal/pdf` the PDF reader, `internal/pdfout` the PDF writers, `internal/pdfa` the PDF/A and PDF/UA-2 metadata and preflight, `internal/tag` the structure tree recorder and tagged write, `internal/font` the font metrics, encodings, glyph-name tables, the Type 1 program decoder, and TrueType subsetting, `internal/type1synth` and `internal/truetypesynth` the synthetic font programs the tests embed, `internal/psout` the PDF-to-PostScript writer, `internal/validation` the validation corpus manifest reader and checker, and `internal/cli` the command layer. Add a directory when its first `.go` file or fixture is real. Do not add `pkg/`, `api/`, `util/`, or empty placeholder packages.

`sampledata/` holds the fixtures and samples. Scenario folders (`compress/`, `pdfa/`, `pdfua2/`) hold the PDFs the plans measure. `sampledata/fixtures/` holds the unit-test inputs and expected PPM bytes. Golden files are written by the test that first locks a case, then checked in. They are not copied from Ghostscript output. Matching Ghostscript byte for byte is not a success criterion.

`sampledata/validation/` is the validation corpus. One subfolder per feature area holds real PDF and PostScript files, and every file has a row in `sampledata/validation/manifest.tsv` that records its pinned source, license, SHA-256, feature, and expected verdict. Files at or under 1 MiB are committed. Larger files and whole suites live under the gitignored `sampledata/validation/external/` and are fetched by `go run internal/validation/gen.go -fetch-external`. Every corpus test is named `TestValidation<Area>`, so `go test -count=1 ./... -run TestValidation` runs the group.

Package `spectreps` calls `internal/engine`, `internal/ps`, `internal/graphics`, `internal/pdf`, `internal/pdfout`, `internal/psout`, `internal/pdfa`, and `internal/tag`. It does not call `internal/cli`, `internal/validation`, `internal/font`, `internal/type1synth`, or `internal/truetypesynth`. `internal/cli` stays on the public library, which is the same boundary an external program has.

## Ownership

| Caller | May import |
| --- | --- |
| `cmd/spectreps` | `github.com/chinmay-sawant/spectrePS/internal/cli` |
| `internal/cli` | `github.com/chinmay-sawant/spectrePS/spectreps` |
| `package spectreps` | `internal/engine`, `internal/ps`, `internal/graphics`, `internal/pdf`, `internal/pdfout`, `internal/psout`, `internal/pdfa`, `internal/tag` |
| `package spectreps_test` | the public package only |
| another module | the public package only |

Go rejects an import of `internal/` from outside this module. That is why the exported functions exist before the interpreter does.
