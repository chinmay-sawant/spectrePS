# v0.0.3 - JPEG2000 image streams

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** implemented. Every row is checked. Lint and test passed on 2026-09-25.
> **Estimated effort:** one small phase plus the conformance gate

---

## Overview

`/JPXDecode` streams return `undefined` today and copy through the compression levels. The standard library and `golang.org/x/image` have no JPEG2000 decoder. This phase adds one pure-Go module behind a conformance gate.

## Executive summary

The candidate is `github.com/mrjoshuak/go-jpeg2000` v1.5.12, Apache-2.0, no cgo, no transitive requirements. It is young and single-maintainer, so the gate is a fixture made by OpenJPEG that decodes bit-exact, plus a cgo-free build. If the gate fails, the row returns to `[~]` with the reason, and the next gate becomes a Spectre-owned conformance matrix or an `x/image` proposal.

For JPX, `/ColorSpace` is optional and ignored; the codestream carries the color. The branch must run before the existing parameter check and derive color and precision from the decoded image.

## Phase 1: Dependency

### 1.1 Dependency row

- [x] `go.mod` requires `github.com/mrjoshuak/go-jpeg2000 v1.5.12` with the reason next to it. `go mod tidy` writes `go.sum`, a second run leaves no diff, and `go list -deps` shows no cgo. Proof: `make tidy` twice, then `git diff --exit-code -- go.mod go.sum`, and `CGO_ENABLED=0 go build ./...`. 2026-09-25: both tidy runs exited 0, the diff printed nothing and exited 0, and the cgo-free build exited 0. `CGO_ENABLED=1 go list -deps ./...` lists no `runtime/cgo`. The reason comment reads `JPEG2000 image streams decode through a pure-Go decoder; the module uses no cgo.`

## Phase 2: Decode

### 2.1 JPX branch

- [x] `DecodeImage` branches on `/JPXDecode` before the `/BitsPerComponent` and `/ColorSpace` checks, decodes through `jpeg2000.Decode`, and returns the decoded `image.Image`. A decode failure returns `syntaxerror`, never a blank image. The 32 MiB cap still applies. Proof: `go test -count=1 ./internal/pdf -run TestImageXObjectJPX` exited 0 on 2026-09-25. The RGB codestream, gray container, bare dictionary, filter array, malformed, and size-limit subtests all passed.

### 2.2 Fixtures and reject matrix

- [x] One `.j2k` and one `.jp2` fixture are checked in under `internal/pdf/testdata/` with the generation command and SHA-256 recorded in `testdata/README.md`. `opj_compress` is not installed, so Pillow 12.3.0 with OpenJPEG 2.5.4 wrote both. Cases: RGB, gray, no `/ColorSpace`, `/Filter [/JPXDecode]`, malformed, and an undecodable stream that copies through. Proof: the same `TestImageXObjectJPX` run exited 0 on 2026-09-25 for the decode cases, and `go test -count=1 ./internal/pdfout -run TestLevelJPXImage` exited 0 on 2026-09-25 for the copy-through case.

## Phase 3: Rewrite, docs, and closure

### 3.1 Level integration

- [x] Levels 2 to 5 decode JPX like DCT. An undecodable stream still copies through. Proof: `go test -count=1 ./internal/pdfout -run TestLevelJPXImage` exited 0 on 2026-09-25. Level 2 leaves the JPX stream alone like DCT, levels 3 to 5 write DCT and keep the RGB or gray color, and the malformed and truncated streams have no override at any level.

### 3.2 Docs and closure

- [x] `documentation/devices.md`, `features.md`, `covered-and-not-covered.md`, and the dependency reason state JPX decode. Proof: `grep -n 'JPEG2000' documentation/*.md` exited 0 on 2026-09-25. `devices.md` lines 47 and 70 name the decoder and the level policy, `features.md` line 33 names the level policy, and `covered-and-not-covered.md` lines 17 and 27 name the rewrite and reading behavior. `go.mod` carries `JPEG2000 image streams decode through a pure-Go decoder; the module uses no cgo.` above the require.
- [x] `make lint` and `make test` pass. Outcomes recorded on the day. 2026-09-25: `make lint` exited 0, with `golangci-lint run ./...` reporting no findings and `size-check: clean (0 over-limit files).` `make test` exited 0, and every package with tests printed `ok`. `go test -count=1 -p 4 ./...` exited 0 too.

## Dependencies

`DecodeImage` and one new pure-Go module.

## Not in this plan

- JPX encoding, JPX in inline images, alpha and ICC handling beyond what the decoder returns, and painting `Do`, which is phase 5.
