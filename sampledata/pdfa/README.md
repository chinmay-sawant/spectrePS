# PDF/A-4 samples

- `path-a4.pdf` is a Spectre write: `spectreps rewrite -pdfa 4 -o sampledata/pdfa/path-a4.pdf sampledata/compress/path.pdf`.
- `compliant-a4.pdf` is copied from the local gopdfsuit project (`sampledata/wasm-js/compliant.pdf`, SHA-256 `475526f82afe9eb6e36f702910c538fc956fbd8bf658e4183efbdceca74e870c`). gopdfsuit is MIT licensed, same author.

Run the proof:

```sh
make pdfa-check
```

The target skips when `verapdf` is not on PATH and excludes `negative/`, which holds deliberate failures.
