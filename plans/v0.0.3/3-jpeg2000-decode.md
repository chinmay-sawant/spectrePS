# v0.0.3 - JPEG2000 image streams

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** not started.
> **Estimated effort:** one small phase plus the conformance gate

---

## Overview

`/JPXDecode` streams return `undefined` today and copy through the compression levels. The standard library and `golang.org/x/image` have no JPEG2000 decoder. This phase adds one pure-Go module behind a conformance gate.

## Executive summary

The candidate is `github.com/mrjoshuak/go-jpeg2000` v1.5.12, Apache-2.0, no cgo, no transitive requirements. It is young and single-maintainer, so the gate is a fixture made by OpenJPEG that decodes bit-exact, plus a cgo-free build. If the gate fails, the row returns to `[~]` with the reason, and the next gate becomes a Spectre-owned conformance matrix or an `x/image` proposal.

For JPX, `/ColorSpace` is optional and ignored; the codestream carries the color. The branch must run before the existing parameter check and derive color and precision from the decoded image.

## Phase 1: Dependency

### 1.1 Dependency row

- [ ] `go.mod` requires `github.com/mrjoshuak/go-jpeg2000 v1.5.12` with the reason next to it. `go mod tidy` writes `go.sum`, a second run leaves no diff, and `go list -deps` shows no cgo. Proof: `make tidy` twice, then `git diff --exit-code -- go.mod go.sum`, and `CGO_ENABLED=0 go build ./...`.

## Phase 2: Decode

### 2.1 JPX branch

- [ ] `DecodeImage` branches on `/JPXDecode` before the `/BitsPerComponent` and `/ColorSpace` checks, decodes through `jpeg2000.Decode`, and returns the decoded `image.Image`. A decode failure returns `syntaxerror`, never a blank image. The 32 MiB cap still applies. Proof: `go test -count=1 ./internal/pdf -run TestImageXObjectJPX`.

### 2.2 Fixtures and reject matrix

- [ ] One `.j2k` and one `.jp2` fixture made with `opj_compress` are checked in with the command and SHA-256 recorded. Cases: RGB, gray, no `/ColorSpace`, `/Filter [/JPXDecode]`, malformed, and an unsupported profile that copies through. Proof: the same test run.

## Phase 3: Rewrite, docs, and closure

### 3.1 Level integration

- [ ] Levels 2 to 5 decode JPX like DCT. An undecodable stream still copies through. Proof: `go test -count=1 ./internal/pdfout -run TestLevelJPXImage`.

### 3.2 Docs and closure

- [ ] `documentation/devices.md`, `features.md`, `covered-and-not-covered.md`, and the dependency reason state JPX decode. Proof: `grep -n 'JPEG2000' documentation/*.md`.
- [ ] `make lint` and `make test` pass. Outcomes recorded on the day.

## Dependencies

`DecodeImage` and one new pure-Go module.

## Not in this plan

- JPX encoding, JPX in inline images, alpha and ICC handling beyond what the decoder returns, and painting `Do`, which is phase 5.
