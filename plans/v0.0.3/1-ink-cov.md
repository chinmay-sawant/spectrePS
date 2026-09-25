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

- [ ] `documentation/devices.md` gains a weighted-ink subsection: `amount_c = (1/N) * sum((255 - c_i)/255)`, printed as `amount * 100` with five decimals and the `RGB` suffix. Record the alternatives (luma, total coverage) and why the per-channel complement wins, both worked examples, and the manual-versus-source note. Proof: the doc states the formula and both examples.

## Phase 2: Library and CLI

### 2.1 MeasureInkAmount

- [x] `spectreps.MeasureInkAmount(img PageImage) Ink` returns the mean per-channel complement over `Width * Height`, stride padding ignored, and a zero image returns the zero `Ink`. Proof: `go test -count=1 ./spectreps -run TestMeasureInkAmount` exited 0 on 2026-09-25, and the full `go test -count=1 ./spectreps` exited 0 on the same day.

### 2.2 ink_cov command

- [x] `spectreps ink_cov [-w points] [-h points] [-r dpi] [-pages range] file` prints `Page N` and three five-decimal percentages ending in `RGB`, with the same exit codes as `inkcov`. Proof: `go test -count=1 ./internal/cli -run TestInkCov` exited 0 on 2026-09-25, covering a quarter cyan square (`25.00000 0.00000 0.00000 RGB`), byte-128 gray, a blank page, two PostScript pages, `-pages`, a red PDF, and a missing file.

## Phase 3: Docs and closure

### 3.1 Docs and ledger

- [ ] `documentation/cli.md`, `public-api.md`, `features.md`, `covered-and-not-covered.md`, and `gs-argv-mapping.md` state the new command and function, and the deferred row moves to 10.4 with the phase path. Proof: `grep -n 'ink_cov' documentation/*.md` shows the rows.

### 3.2 Closure

- [ ] `make lint` passes. Outcome recorded on the day.
- [ ] `make test` passes. Outcome recorded on the day.

## Dependencies

`PageImage` and the existing measure and CLI seams. No new module.

## Not in this plan

- Changing `inkcov` occupancy. It stays.
- CMYK rendering and color management, spot separations, and `tiffsep`.
- The `-sDEVICE=ink_cov` grammar; the mapping table stays documentation.
