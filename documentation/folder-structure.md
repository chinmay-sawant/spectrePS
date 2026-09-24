# Folder structure

The module path is `github.com/chinmay-sawant/spectrePS`. The last element stays `spectrePS`, matching the GitHub repository. The public library is package `spectreps` in `spectreps/`. Its import path is `github.com/chinmay-sawant/spectrePS/spectreps`.

The module root holds `go.mod`, the Makefile, the license, and the prose. Library `.go` files live under `spectreps/` and `internal/`.

## What is in the tree now

```
AGENTS.md
Makefile
README.md
go.mod
cmd/spectreps/main.go
internal/cli/
internal/engine/
internal/graphics/
internal/pdf/
internal/pdfout/
internal/ps/
spectreps/
testdata/
documentation/
plans/v0.0.1/
skills/phase-wise-checklist/SKILLS.md
skills/unslop/SKILL.md
skills/PR/
scripts/
```

`documentation/` is the prose folder for this repository. `plans/v0.0.1/` is the execution ledger. `skills/` holds agent instructions that already live in this repo.

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
spectreps/compare.go
spectreps/api_test.go
```

`api_test.go` uses `package spectreps_test`. Another module imports `github.com/chinmay-sawant/spectrePS/spectreps` and nothing under `internal/`.

## Command

`cmd/spectreps/main.go` is the process entry. It calls `internal/cli`. `internal/cli` parses flags, reads files, and calls the public library. It does not import `internal/engine`.

## Private code

`internal/engine` holds the session, file byte compare, and the not-implemented job check. Later phases add their first real file under:

```
internal/ps/
internal/graphics/
internal/raster/
internal/pdf/
internal/pdfout/
testdata/
```

Add a directory when its first `.go` file or fixture is real. Do not add `pkg/`, `api/`, `util/`, or empty placeholder packages.

`testdata/` holds input files and expected PPM bytes. Golden files are written by the test that first locks a case, then checked in. They are not copied from Ghostscript output. Matching Ghostscript byte for byte is not a success criterion.

Package `spectreps` is the caller of `internal/ps`, `internal/graphics`, `internal/raster`, `internal/pdf`, and `internal/pdfout` once those directories exist. `internal/cli` stays on the public library, which is the same boundary an external program has.

## Ownership

| Caller | May import |
| --- | --- |
| `cmd/spectreps` | `github.com/chinmay-sawant/spectrePS/internal/cli` |
| `internal/cli` | `github.com/chinmay-sawant/spectrePS/spectreps` |
| `package spectreps` | `internal/engine`, and later the interpreter packages under `internal/` |
| `package spectreps_test` | the public package only |
| another module | the public package only |

Go rejects an import of `internal/` from outside this module. That is why the exported functions exist before the interpreter does.
