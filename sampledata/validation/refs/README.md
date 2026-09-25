# Reference programs

Four repo-authored PostScript programs for the Phase 11 Ghostscript reference
proofs. `scripts/ref-gs-check.sh` runs a program through Spectre and through
`gs`, normalizes both PPMs to their pixel bodies, and compares:

- `line.ps`: axis-aligned strokes. Exact match is required.
- `rect.ps`: an axis-aligned filled rectangle. Exact match is required.
- `diag.ps`: a diagonal stroke. The pixel difference count is recorded, not
  required to be zero.
- `curve.ps`: a cubic curve. The pixel difference count is recorded.

`line.ps`, `rect.ps`, and `curve.ps` are byte copies of the same programs under
`postscript/`, so the reference check and the corpus paint the same input. The
programs are repo-authored, carry no license notice, and are committed at or
under 1 MiB. The `refs/` rows are in `../manifest.tsv` with source
`repo-authored` and the `paint` expectation.
