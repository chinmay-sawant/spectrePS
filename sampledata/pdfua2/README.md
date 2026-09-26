# PDF/UA-2 samples

- `tagged-ua2.pdf` passes Spectre's `PreflightUA2` and passes veraPDF UA-2. `negative/untagged.pdf` fails `ua2-marked` on purpose. Both are written by `go run internal/pdfa/gen_ua2_samples.go`, with no dates, so the bytes are stable.
- `generated/report.pdf` is a Spectre tagged write: a heading, paragraphs, a list, a table, a rule artifact, and an image with `/Alt`. `negative/no-title.pdf` is the refused claim case, which keeps the tree and writes no claim. Both are written by `go run internal/tag/gen_samples.go`, with no dates, so the bytes are stable.
- `compliant-ua2.pdf` is copied from the local gopdfsuit project (`sampledata/wasm-js/compliant.pdf`, SHA-256 `475526f82afe9eb6e36f702910c538fc956fbd8bf658e4183efbdceca74e870c`). gopdfsuit is MIT licensed, same author.

Run the proof:

```sh
make pdfua2-check
```

On 2026-09-26, veraPDF 1.30.2 reported 1727 passed rules and 0 failed rules for each of `generated/report.pdf`, `tagged-ua2.pdf`, and `compliant-ua2.pdf`. The target skips when `verapdf` is not on PATH and excludes `negative/`, which holds deliberate failures.
