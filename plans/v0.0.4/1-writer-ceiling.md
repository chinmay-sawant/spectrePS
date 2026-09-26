# v0.0.4 - Writer size ceiling

> **Parent:** `plans/v0.0.4/00-program.md` - program ledger
> **Status:** implemented. Rows 1.1 to 1.3 landed, the v0.0.3 row 1.2 is accepted, and `make lint` and `make test` pass.
> **Estimated effort:** half a day

---

## Overview

`plans/v0.0.3/4-writer-cleanup.md` row 1.2 wanted `TestRewriteSamples` to assert that levels 1 and 2 are not larger than the input. That guard cannot pass. `sampledata/compress/whatisthis.pdf` is 596,341 bytes and stores 78 non-stream objects in one Flate `/Type /ObjStm`, so the classic writer expands them: 614,343 bytes before the container skip, 610,034 after it. The optional packed writer reaches 596,491 bytes, 150 over the input, and stays opt-in through `CopyOptions.PackObjects`. The maintainer accepts the classic sizes, so the acceptance is a recorded ceiling, not an input comparison.

The checked-in `sampledata/compress/whatisthis_level1.pdf` and `whatisthis_level2.pdf` are 614,343 bytes each. They landed in `c794c37`, which predates the container skip in `29ac323`, and no test reads them. They stay as release records for v0.0.3; the levels 3 through 5 samples are stale the same way.

## Executive summary

Levels 1 and 2 stay at or under 610,034 bytes on `sampledata/compress/whatisthis.pdf`, the post-skip measurement of 2026-09-25. One assertion in `TestRewriteSamples`, one sentence each in `documentation/devices.md` and `documentation/test.md`, and the closure of row 1.2 carry it. The writer does not change: `CopyOptions.PackObjects` stays opt-in, and level 0 stays classic PDF 1.4.

## Phase 1: Acceptance

### 1.1 Close the v0.0.3 row

- [x] `plans/v0.0.3/4-writer-cleanup.md` row 1.2 is accepted on 2026-09-25 and rewritten as `[x]`: the level 1 and 2 sizes for `whatisthis.pdf` are recorded as a 610,034-byte ceiling, the post-container-skip measurement, and `TestRewriteSamples` asserts both levels stay at or under it. The measured sizes are input 596,341 bytes, 614,343 before the container skip, 610,034 after it, and 596,491 with the optional packed writer; the source packs 78 non-stream objects, so a not-larger-than-input guard cannot pass. The status line becomes `implemented` with 1.2 accepted under this phase file. Proof: `grep -n -e '1.2 is accepted' -e 'Accepted on 2026-09-25' plans/v0.0.3/4-writer-cleanup.md`.

### 1.2 The ceiling assertion

- [x] `internal/cli/run_test.go` gains the ceiling constant after `squarePath` and the boundary check at the end of `checkSampleSizes`, with these exact edits:

  ```go
  // whatisthisLevel12Ceiling is the accepted level 1 and 2 output size for
  // whatisthis.pdf, measured on 2026-09-25 after the container skip. The
  // source packs 78 non-stream objects, so the classic writer expands them;
  // the packed writer reaches 596,491 bytes and stays opt-in.
  const whatisthisLevel12Ceiling = 610034
  ```

  and, before the closing brace of `checkSampleSizes`:

  ```go
  	if hasImage && (sizes[1] > whatisthisLevel12Ceiling || sizes[2] > whatisthisLevel12Ceiling) {
  		t.Fatalf("whatisthis.pdf: level 1 = %d bytes, level 2 = %d bytes, ceiling = %d",
  			sizes[1], sizes[2], whatisthisLevel12Ceiling)
  	}
  ```

  `hasImage` is the existing whatisthis discriminator at the `checkSampleSizes` call. Proof: `go test -count=1 ./internal/cli -run TestRewriteSamples`. Guard proof: with the constant temporarily lowered to 610,033 the same test fails with the ceiling message, then the constant is restored.

### 1.3 Docs

- [x] `documentation/devices.md` appends to the `CopyOptions.PackObjects` paragraph: "Levels 1 and 2 stay at or under 610,034 bytes on `sampledata/compress/whatisthis.pdf`, the recorded ceiling: the source packs 78 non-stream objects, so the classic form is 13,693 bytes over the 596,341-byte input and `PackObjects` stays opt-in." `documentation/test.md` states before the skip sentence: "The level 1 and 2 sizes for `whatisthis.pdf` stay at or under the recorded 610,034-byte ceiling." Proof: `grep -n '610,034' documentation/devices.md documentation/test.md`.

## Phase 2: Closure

### 2.1 Lint

- [x] `make lint` passes. Outcome recorded on the day. Proof: `make lint`; `gofmt -l .` prints nothing, `golangci-lint run ./...` exits 0, and `size-check` reports 0 over-limit files. Lint passes as of 2026-09-26: `make lint` exits 0 with gofmt, golangci-lint, and size-check clean.

### 2.2 Test

- [x] `make test` passes. Outcome recorded on the day. Proof: `make test`.

## Dependencies

`TestRewriteSamples` and `checkSampleSizes` in `internal/cli`, the two docs, and `plans/v0.0.3/4-writer-cleanup.md`. No new module.

## Not in this plan

- Selecting `CopyOptions.PackObjects` for levels 1 and 2. Packing stays opt-in.
- Changing level 0 output. It stays classic PDF 1.4.
- Closing the 150-byte gap between the packed writer and the 596,341-byte input.
- Refreshing the checked-in samples. The level 1 and 2 files (614,343 bytes) and the levels 3 through 5 files were added in `c794c37` and predate the skip commit `29ac323`; they stay as release records.
- Editing `plans/v0.0.1/10-deferred.md` or the v0.0.4 program map. The integrator owns both, and this file touches neither.
