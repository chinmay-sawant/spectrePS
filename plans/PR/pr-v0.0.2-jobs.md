## Summary

Add the three v0.0.2 quick-win jobs: `bbox` and `inkcov` summaries of a painted page, JPEG output from `raster`, and `pdfimage`, which wraps one 24-bit RGB raster per page in a new PDF. The summaries and the JPEG encode a finished `PageImage`; `pdfimage` adds an image writer beside the path writer.

---

## Motivation / context

- Plans: `plans/v0.0.2/00-program.md`, `plans/v0.0.2/1-bbox-inkcov.md`, `plans/v0.0.2/2-jpeg-raster.md`, `plans/v0.0.2/3-pdfimage.md`
- Issues: see **Related issues**

---

## Changes

### Box and ink coverage

- `MeasureBox(PageImage, dpi) (Box, bool)` unions the marked pixel edges in points, origin at the lower left. A pixel marks when any of R, G, or B is not 255. A page with no marked pixel returns false.
- `MeasureInk(PageImage) Ink` returns RGB occupancy fractions over `Width * Height`. It is not a CMYK report and not Ghostscript `ink_cov`.
- `spectreps bbox` prints `%%BoundingBox` and `%%HiResBoundingBox` per page on stdout. `spectreps inkcov` prints `Page N` and three five-decimal fractions ending in `RGB`.
- Both commands accept `-w`, `-h`, and `-r` and rasterize every page of a PDF input, not only page 0.

### JPEG raster

- `raster` encodes `.jpg` and `.jpeg` with `image/jpeg`. PPM stays the fallback for every other suffix, and `.png` is unchanged.
- `-jpegq` defaults to 75 and is clamped to 1 through 100. `run` and `compare raster` do not accept it.
- JPEG file bytes are not an equality oracle. `CompareRaster` and `PageImage` stay the oracle.

### Bitmap PDF

- `internal/pdfout.WriteImages` writes one page per `graphics.Image`, with a `/DeviceRGB`, 8-bit, `/FlateDecode` image XObject and `/MediaBox [0 0 width*72/dpi height*72/dpi]`.
- The content stream paints `q W 0 0 H 0 0 cm /Im0 Do Q`. Spectre's PDF interpreter still returns `undefined` for `Do`, so the tests decode the image stream from the bytes instead of rasterizing the output.
- `ImagePDF` exposes the writer on the public API. `spectreps pdfimage -o out.pdf` paints a PostScript or PDF input and writes the file at mode `0o600`.
- The trailer `/ID` is the SHA-256 of the concatenated Flate image streams, both strings the same. No `/Info`, no `CreationDate`, no `ModDate`. Two calls return equal bytes.
- `RewritePDF` is unchanged and still emits path operators.

### Tests and ledgers

- Eight proof commands across `spectreps`, `internal/cli`, and `internal/pdfout` cover every plan row.
- `documentation/test.md` records the new cases. `documentation/covered-and-not-covered.md` moves the three jobs to covered.
- The phase files are checked, the closure records the merged-tree gate, and the deferred rows for `bbox`, `inkcov`, `pdfimage24`, and JPEG say they landed.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | `bbox` and `inkcov` scan the pixmap once. `pdfimage` Flate-encodes one image per page. No measured baseline. |
| **Memory** | `pdfimage` holds the painted pages and the output file in memory. The existing raster caps still apply. |
| **Behavior / correctness** | `MeasureInk` follows the channel rule: cyan is `1 0 0`, red is `0 1 1`. JPEG is lossy. The image PDF cannot be rasterized by Spectre until `Do` lands. |
| **API / CLI** | New `Box`, `Ink`, `MeasureBox`, `MeasureInk`, and `ImagePDF`; new `bbox`, `inkcov`, and `pdfimage` commands; new `-jpegq` flag. No existing signature changed. |
| **Dependencies** | None. JPEG, SHA-256, and Flate are the standard library. |
| **Binary size / build time** | `image/jpeg` is linked. `make build` still produces `bin/spectreps`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | Existing commands, flags, and signatures are unchanged. |

---

## Test plan

- [x] `make lint`
- [x] `make test`
- [x] `make build`

### Commands

```sh
make lint
make test
make build
go test -count=1 ./spectreps -run TestMeasureBox
go test -count=1 ./spectreps -run TestMeasureInk
go test -count=1 ./internal/cli -run TestBBox
go test -count=1 ./internal/cli -run TestInkcov
go test -count=1 ./internal/cli -run TestRasterJPEG
go test -count=1 ./internal/pdfout -run TestImagePDF
go test -count=1 ./internal/cli -run TestPDFImage
go test -count=1 ./spectreps -run TestImagePDFStable
```

Each command exited 0 on 2026-09-25 on `feature/002-quick-wins-impl` with all three phases merged.

---

## Screenshots / sample output

Program: a 10 by 10 square on a 20 by 20 point page at 72 dpi.

```
$ spectreps bbox -w 20 -h 20 -r 72 sample.ps
%%BoundingBox: 0 0 10 10
%%HiResBoundingBox: 0 0 10 10
$ spectreps inkcov -w 20 -h 20 -r 72 sample.ps
Page 1
0.25000 0.25000 0.25000 RGB
$ spectreps raster -o sample.jpg -w 20 -h 20 -r 72 sample.ps
$ file sample.jpg
sample.jpg: JPEG image data, baseline, precision 8, 20x20, components 3
$ spectreps pdfimage -o sample.pdf -w 20 -h 20 -r 72 sample.ps
$ file sample.pdf
sample.pdf: PDF document, version 1.4, 1 pages
```

---

## Related issues

- No GitHub issue exists for this work. The parent ledger is `plans/v0.0.2/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-v0.0.2-jobs.md` when process-gated

---

## Follow-ups (out of scope)

- TIFF stays deferred: `golang.org/x/image/tiff` is a new module requirement.
- `Do` support and DCT or CCITT image decoding stay deferred.
- Text, PDF/A, PostScript rewrite, `gs` argv, and printer languages stay in `plans/v0.0.1/10-deferred.md`.

---

## Reviewer checklist

- [ ] Behavior matches summary and test plan
- [ ] No unrelated changes in diff
- [ ] Public API / CLI changes documented
- [ ] New rules have fixture coverage when applicable
- [ ] PR has assignee and labels
- [ ] Related issues use correct Closes/Relates keywords
- [ ] No secrets or generated artifacts committed
- [ ] Diff-stat-by-extension table pasted at the bottom

---

## Diff stat by extension

| Extension | Files | Insertions | Deletions |
| --- | ---: | ---: | ---: |
| `.go` | 9 | 1285 | 34 |
| `.md` | 15 | 447 | 32 |
| **Total** | **24** | **1732** | **66** |
