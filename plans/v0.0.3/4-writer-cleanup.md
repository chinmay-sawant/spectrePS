# v0.0.3 - Compression writer cleanup

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** partially implemented. 1.1, 2.1, 3.1, and 3.2 landed. 1.2 is open because the guard cannot pass.
> **Estimated effort:** half a day for the skip and the measurement, about a week for the packing phase

---

## Overview

`pdfout.WriteCopy` copies every in-use object, including the source `/Type /XRef` and `/Type /ObjStm` containers, and writes plain objects with a classic xref. On `sampledata/compress/whatisthis.pdf` that grows the file from 596,341 bytes to 614,343 at levels 1 and 2, about 3%.

This is a new row, not one of the eight open deferred rows. It removes a known limitation of v0.0.2.

## Executive summary

Two changes are format-neutral and cheap: skip the dead container objects, and re-measure the samples. Writing a new object stream is a separate phase because it changes the output to PDF 1.5 with an xref stream and needs its own numbering and digest rules.

## Phase 1: Dead containers

### 1.1 Skip XRef and ObjStm objects

- [x] `copyBody` resolves each object and skips it when its `/Type` is `/XRef` or `/ObjStm`, writing a free xref row instead. The `/ID` digest covers only written bodies, and `/Size` stays `ObjectCount+1`. Proof: `go test -count=1 ./internal/pdfout -run TestCopySkipsContainers` exited 0 on 2026-09-25. The test failed against the old `copyBody` with "output copies a dead container".

### 1.2 Re-measure

- [ ] The level 1 and 2 sizes for `whatisthis.pdf` are recorded in the phase row, and `TestRewriteSamples` gains a guard that levels 1 and 2 are not larger than the input. Proof: `go test -count=1 ./internal/cli -run TestRewriteSamples`. Measured on 2026-09-25: the input is 596,341 bytes, levels 1 and 2 were 614,343 before the container skip, 610,034 after it, and 596,491 with the optional packed writer, so the guard cannot pass and is not added. Reason: the source packs 78 non-stream objects, and even the packed writer stays 150 bytes over the input.

## Phase 2: Packing, later

### 2.1 Object stream writer

- [x] `CopyOptions` gains an object-stream mode: non-stream bodies go into a new Flate `/Type /ObjStm`, addressed by a new `/Type /XRef` stream, header `%PDF-1.5`. The `/ID` digest stays SHA-256 over object bodies in object-number order, computed before packing, so bytes stay stable and independent of packing. Proof: `go test -count=1 ./internal/pdfout -run TestWritePackedObjects` and `go test -count=1 ./spectreps -run TestRewriteLevelsStable` exited 0 on 2026-09-25. The mode is opt-in through `CopyOptions.PackObjects`; the levels 1 through 5 path does not select it.

## Phase 3: Docs and closure

### 3.1 Docs

- [x] `documentation/devices.md` and `documentation/test.md` state the container rule and the optional packing, and the v0.0.2 known-limitation notes are closed in the feature map. Proof: `grep -n 'ObjStm' documentation/devices.md` printed the container rule and the packed mode on lines 103 and 105 on 2026-09-25, re-checked at integration, and the feature-map row named "Compression writer container cleanup" is gone.

### 3.2 Closure

- [x] `make lint` and `make test` pass. Outcomes recorded on the day. On 2026-09-25, `make lint` exited 0: `gofmt -l .` printed nothing, `golangci-lint run ./...` exited 0, and `size-check` reported 0 over-limit files. `make test` exited 0, and `go test -count=1 -p 4 ./...` exited 0 with every package `ok`.

## Dependencies

`WriteCopy`, `ObjectValue`, `RawObject`, and the existing digest. No new module.

## Not in this plan

- Changing the default output of level 0. It stays classic PDF 1.4.
- Object-stream support in the reader; it already opens them.
