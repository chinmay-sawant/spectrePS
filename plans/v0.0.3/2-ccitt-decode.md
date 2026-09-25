# v0.0.3 - CCITT image streams

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** implemented. Decoder, level integration, and docs landed and all rows are checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** 1 to 2 days

---

## Overview

`DecodeImage` reads Flate and DCT. A `/CCITTFaxDecode` stream returns `undefined` today, and the compression levels copy it through unchanged. `golang.org/x/image/ccitt` is already part of the `x/image` module the project requires, so this needs no new dependency.

## Executive summary

Decode Group 4 and Group 3 streams into `image.Gray`, one byte per pixel, then hand them to the existing image and level paths. Group 3 needs end-of-line markers; a Group 3 stream without them stays unsupported. Mixed mode (`K > 0`) stays unsupported. Never return a blank image on failure.

## Phase 1: Decoder

### 1.1 Parameters and Group 4

- [x] `internal/pdf/image.go` gains a `CCITTFaxDecode` branch before the Flate path. It reads `/DecodeParms`: `K < 0` selects Group 4, `K == 0` with `/EndOfLine true` selects Group 3, and `K > 0` returns `undefined`. `Columns` and `Rows` default to `/Width` and `/Height`. `BlackIs1` maps to `Options.Invert`, and `EncodedByteAlign` maps to `Options.Align`. Decode with `ccitt.DecodeIntoGray` and return `*image.Gray`. Proof: `go test -count=1 ./internal/pdf -run TestImageXObjectCCITT` exited 0 on 2026-09-25.

### 1.2 Group 3 and the reject matrix

- [x] Group 3 with end-of-line markers decodes. `K > 0`, `K == 0` with `/EndOfLine false`, an 8-bit depth, `DeviceRGB`, and a truncated stream each return the package error with name `undefined` or `syntaxerror`. Proof: `go test -count=1 ./internal/pdf -run TestImageXObjectCCITTG3` and `TestImageXObjectCCITTRejects` exited 0 on 2026-09-25.

### 1.3 Size guard

- [x] `Columns * Rows` above the 32 MiB decoded cap returns `limitcheck` before allocation. Proof: `go test -count=1 ./internal/pdf -run TestImageXObjectCCITTLimit` exited 0 on 2026-09-25.

## Phase 2: Rewrite and docs

### 2.1 Level integration

- [x] Levels 2 to 5 treat a CCITT stream like any other decoded image: level 2 re-encodes it as Flate RGB, and levels 3 to 5 as DCT. An undecodable stream still copies through. Proof: `go test -count=1 ./internal/pdfout -run TestLevelCCITTImage` exited 0 on 2026-09-25.

### 2.2 Docs and closure

- [x] `documentation/devices.md`, `features.md`, and `covered-and-not-covered.md` state the decoded filters, and the deferred row moves to 10.4. Proof: `grep -n 'CCITT' documentation/*.md` showed the rows on 2026-09-25. The 10.4 move is owned by the integration session: this worktree's brief freezes `plans/v0.0.1/10-deferred.md`.
- [x] `make lint` and `make test` pass. Outcomes on 2026-09-25: `make lint` exited 0; `gofmt -l .` printed nothing, `golangci-lint run ./...` exited 0, and `size-check` reported 0 over-limit files. `make test` and `go test -count=1 -p 4 ./...` exited 0 with every package passing.

## Dependencies

`DecodeImage`, `internal/pdf/filter.go`, and `x/image/ccitt`, already in the module.

## Not in this plan

- CCITT encoding, TIFF CCITT compression, mixed-mode (`K > 0`) Group 3, damaged-row repair, and `/Decode` array remapping.
- JPEG2000, which is phase 3.
