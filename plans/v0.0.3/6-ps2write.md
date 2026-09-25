# v0.0.3 - PDF to PostScript

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** in progress.
> **Estimated effort:** 3 to 4 days for the path phase

---

## Overview

The `ps2write` deferred row wants PostScript output. PDF content bytes cannot be copied into PostScript: `re`, `q`/`Q`, `cm`, `RG`/`rg`, `Do`, and Flate wrapping are not runnable PostScript. Ghostscript reassembles primitives, and `pdf.Paint` already produces the same canonical marks for Spectre.

## Executive summary

A new `internal/psout` recorder implements `graphics.Marker` and emits PostScript operators. A new `spectreps ps -o out.ps in.pdf` command frames pages with a date-free `%!PS-Adobe` header. The proof is a round trip: write PostScript, run it back through `RunPostScript`, and compare pixels with `CompareRaster`. Images and text wait for the interpreter and font machine.

## Phase 1: Recorder

### 1.1 psout.Emit

- [x] `internal/psout` gains a recorder that implements `graphics.Marker` and emits `setrgbcolor` or `setgray`, `setlinewidth`, `m`/`l`, and `S`/`f`/`f*` in points, and an `Emit(ctx, content)` that runs `pdf.Paint` through it. Proof: `go test -count=1 ./internal/psout -run TestEmitPS` exited 0 on 2026-09-25 and checks the operator text for a `re cm S` input.

## Phase 2: Writer and command

### 2.1 psout.Write

- [x] `internal/psout.Write(ctx, pages, opts)` frames pages with `%!PS-Adobe-3.0`, a fixed 612 by 792 box, one `showpage` per page, and no creation date. Two calls return equal bytes. Flate stays off until the interpreter reads `FlateDecode`. Proof: `go test -count=1 ./internal/psout -run TestWritePS` and `TestWritePSStable` exited 0 on 2026-09-25.

### 2.2 Public method and CLI

- [ ] `(*Instance).WritePostScript(ctx, doc, opt)` mirrors `RewritePDF` for nil and canceled contexts and returns `undefined in Tj` on text input. `spectreps ps -o out.ps in.pdf` writes the file at mode `0o600`; a missing `-o` exits 2. Proof: `go test -count=1 ./spectreps -run TestWritePostScript` and `go test -count=1 ./internal/cli -run TestPSCommand`.

## Phase 3: Round trip, docs, and closure

### 3.1 Round trip

- [ ] A one-page PDF with `re`/`f`, `m`/`l`/`S`, a curve, and `q`/`Q`/`cm` becomes PostScript, runs back through `RunPostScript` at 72 dpi, and compares equal under `CompareRaster`. Proof: `go test -count=1 ./spectreps -run TestPostScriptRoundTrip`.

### 3.2 Docs and closure

- [ ] `documentation/cli.md`, `devices.md`, `features.md`, `covered-and-not-covered.md`, and `public-api.md` state the new command, and the deferred row moves to 10.4. Proof: `grep -n 'spectreps ps' documentation/cli.md`.
- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

`pdf.Paint`, the marker seam, `RunPostScript`, and `CompareRaster`. No new module.

## Not in this plan

- Fonts subsetting and embedding, text operators, EPS (`eps2write`), the full DSC comment set, media options, and `MaxInlineImageSize`.
- Inline image emission; that needs `image` in `internal/ps` first, and it is a follow-up row.
