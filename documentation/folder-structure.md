# Folder structure

The module path is `github.com/chinmay-sawant/spectrePS`. The Go package at the repository root is named `spectreps`, matching the binary. The last element of the module path stays `spectrePS`, matching the GitHub repository.

## What is in the tree now

```
AGENTS.md
Makefile
README.md
go.mod
documentation/
plans/v0.0.1/
skills/phase-wise-checklist/SKILLS.md
skills/unslop/SKILL.md
skills/PR/
```

`documentation/` is the prose folder for this repository. `plans/v0.0.1/` is the execution ledger. `skills/` holds agent instructions that already live in this repo.

## What phase 02 adds

```
doc.go
errors.go
instance.go
postscript.go
pdf.go
raster.go
compare.go
api_test.go
cmd/spectreps/main.go
```

`api_test.go` uses `package spectreps_test`.

## What later phases add

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

## Ownership

| Caller | May import |
| --- | --- |
| `cmd/spectreps` | `github.com/chinmay-sawant/spectrePS` |
| `package spectreps` | `internal/...` of this module |
| `package spectreps_test` | the public package only |
| another module | the public package only |

Go rejects an import of `internal/` from outside this module. That is why the exported functions exist in phase 02, before the interpreter does.
