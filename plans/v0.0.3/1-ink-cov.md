# v0.0.3 - Weighted ink coverage

> **Parent:** `plans/v0.0.3/00-program.md` - program ledger
> **Status:** not started.
> **Estimated effort:** 1 day

---

## Overview

`inkcov` reports occupancy: the fraction of pixels whose channel byte is not 255. Ghostscript also ships `ink_cov`, which reports a weighted amount. The deferred row's next gate is a written weighting model, so the model is the first row here.

Ghostscript's `ink_cov` renders to CMYK8 and prints `c = dc_pix*100/(total_pix*255)`, a percent, per channel. The 10.08 manual example disagrees with the source; the source is the reference. Spectre's pixmap is RGB, so the three channels are R, G, and B, and the suffix stays `RGB`.

## Executive summary

The weighted amount is the continuous refinement of occupancy: the mean complement of each channel over `Width * Height`. The existing `inkcov` command does not change. A new `spectreps ink_cov` command prints the same three-channel report as percentages, matching the measured Ghostscript device.

## Phase 1: Model

### 1.1 Record the weighted model

- [x] `documentation/devices.md` gains a weighted-ink subsection: `amount_c = (1/N) * sum((255 - c_i)/255)`, printed as `amount * 100` with five decimals and the `RGB` suffix. The subsection records the alternatives (luma, total coverage) and why the per-channel complement wins, both worked examples (quarter cyan and byte-128 gray), and the manual-versus-source note. Proof: `grep -n -e 'amount_c = (1/N)' -e '25.00000 0.00000 0.00000 RGB' -e '49.80392 49.80392 49.80392 RGB' -e 'Manual versus source' documentation/devices.md` exited 0 on 2026-09-25 and printed the formula, both examples, and the note. The hand measurement of Ghostscript 9.55.0 is in the subsection.

## Phase 2: Library and CLI

### 2.1 MeasureInkAmount

- [x] `spectreps.MeasureInkAmount(img PageImage) Ink` returns the mean per-channel complement over `Width * Height`, stride padding ignored, and a zero image returns the zero `Ink`. Proof: `go test -count=1 ./spectreps -run TestMeasureInkAmount` exited 0 on 2026-09-25, and the full `go test -count=1 ./spectreps` exited 0 on the same day.

### 2.2 ink_cov command

- [x] `spectreps ink_cov [-w points] [-h points] [-r dpi] [-pages range] file` prints `Page N` and three five-decimal percentages ending in `RGB`, with the same exit codes as `inkcov`. Proof: `go test -count=1 ./internal/cli -run TestInkCov` exited 0 on 2026-09-25, covering a quarter cyan square (`25.00000 0.00000 0.00000 RGB`), byte-128 gray, a blank page, two PostScript pages, `-pages`, a red PDF, and a missing file.

## Phase 3: Docs and closure

### 3.1 Docs and ledger

- [x] `documentation/cli.md`, `public-api.md`, `features.md`, `covered-and-not-covered.md`, and `gs-argv-mapping.md` state the new command and function. Proof: `grep -n 'ink_cov' documentation/*.md` exited 0 on 2026-09-25 and printed the rows in cli.md, devices.md, features.md, gs-argv-mapping.md, and covered-and-not-covered.md, plus the `MeasureInkAmount` contract in public-api.md. The `plans/v0.0.1/10-deferred.md` move to 10.4 is integrator-owned by the phase brief, so this branch does not touch that file.

### 3.2 Closure

- [ ] `make lint` passes. Outcome recorded on the day.
- [ ] `make test` passes. Outcome recorded on the day.

## Dependencies

`PageImage` and the existing measure and CLI seams. No new module.

## Not in this plan

- Changing `inkcov` occupancy. It stays.
- CMYK rendering and color management, spot separations, and `tiffsep`.
- The `-sDEVICE=ink_cov` grammar; the mapping table stays documentation.
