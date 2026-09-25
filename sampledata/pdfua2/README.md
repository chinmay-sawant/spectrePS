# PDF/UA-2 samples

- `tagged-ua2.pdf` passes Spectre's `PreflightUA2` and passes veraPDF UA-2. `negative/untagged.pdf` fails `ua2-marked` on purpose. Both are written by `go run internal/pdfa/gen_ua2_samples.go`, with no dates, so the bytes are stable.
- `compliant-ua2.pdf` is copied from the local gopdfsuit project (`sampledata/wasm-js/compliant.pdf`, SHA-256 `475526f82afe9eb6e36f702910c538fc956fbd8bf658e4183efbdceca74e870c`). gopdfsuit is MIT licensed, same author.

Run the proof:

```sh
make pdfua2-check
```

The target skips when `verapdf` is not on PATH and excludes `negative/`, which holds deliberate failures.
