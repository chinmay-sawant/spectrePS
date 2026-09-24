# v0.0.3 - PDF open and rasterize

> **Parent:** `plans/v0.0.1/00-program.md` - program ledger
> **Status:** implemented. Lint and test passed on 2026-09-24. Tag 0.0.3 is checked.
> **Estimated effort:** 2 weeks

---

## Overview

`OpenPDF` and `RasterizePage` accept the PDF subset in `documentation/devices.md`. Content operators call `internal/graphics`. They do not call the PostScript scanner.

Unsupported text and image operators fail the page. They do not paint a blank page and return nil error.

## Executive summary

Real PDF pages are compressed. Flate decode is part of this phase, not a later optimization. Classic xref comes first. Xref streams are a second row in this same phase because current writers emit them, including Ghostscript after 10.03.

## Phase 6: PDF open and rasterize

### 6.1 Classic xref and Flate

- [x] `internal/pdf` opens a small fixture with a classic xref and a Flate-compressed content stream. `OpenPDF` returns a document whose page count is the page tree length. Proof: `go test -count=1 ./internal/pdf -run TestClassicXref` exited 0 on 2026-09-24.

### 6.2 Xref streams

- [x] A fixture that uses an xref stream and a Flate object stream opens, or returns a `JobError` that names `xref` if that fixture is still unsupported at the end of the row. The row is only checked if the fixture opens and the page count is right. Proof: `go test -count=1 ./internal/pdf -run TestXrefStream` exited 0 on 2026-09-24. The fixture opens and the page count is 1.

### 6.3 Content operators

- [x] Operators `m l c h re S s f f* n q Q w RG rg g G` paint through the graphics device. A one-page path PDF rasterizes to the same pixels as the PostScript program of the same marks, checked with `CompareRaster`. Proof: `go test -count=1 ./spectreps -run TestPDFPathMatchesPS` exited 0 on 2026-09-24.

### 6.4 Rejected constructs

- [x] `Tj`, `Do`, an encrypted trailer, and an unknown filter each return `JobError` with `Op` or `Msg` naming the cause. The error is not `ErrNotImplemented`. Proof: `go test -count=1 ./internal/pdf -run TestPDFReject` exited 0 on 2026-09-24.

### 6.5 Public methods and CLI

- [x] `OpenPDF` and `RasterizePage` no longer return `ErrNotImplemented` for this subset. `spectreps raster -o out.ppm in.pdf` writes the P6 file. A bad page index is `rangecheck` and exit 1. Proof: `go test -count=1 -run 'TestRasterizePage|TestNotImplemented' ./spectreps` and `go test -count=1 ./internal/cli -run TestRasterPDF` exited 0 on 2026-09-24.

### 6.6 Closure

- [x] `make lint` passes. Outcome on 2026-09-24: exit 0. `gofmt -l .` printed nothing. `golangci-lint run ./...` exited 0.
- [x] `make test` passes. Outcome on 2026-09-24: exit 0. Transcript is in `plans/v0.0.1/09-release-records.md` under tag 0.0.3.
- [x] Tag 0.0.3 note added to `plans/v0.0.1/09-release-records.md`.

## Dependencies

Phase 04 pixmap and graphics device. Phase 05 `CompareRaster` is the equality proof in row 6.3. PostScript behavior from phase 03 stays green.
