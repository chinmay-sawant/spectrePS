## Summary

Add the v0.0.3 work: PDF page ranges, TIFF raster, gray and CMYK image PDF, the bounded `gs` switch map, and PDF compression levels 1 to 5 built on an object pass-through writer. Level 0 keeps the previous rewrite behavior byte for byte. A real-world PDF 1.7 file with text and a DCT image compresses from 596,341 bytes to 85,760 bytes at level 5 with all 8 pages preserved.

---

## Motivation / context

- Plans: `plans/v0.0.3/00-program.md`, `plans/v0.0.3/1-quick-wins.md`, `plans/v0.0.3/2-pdf-compression.md`
- Issues: see **Related issues**

---

## Changes

### Quick wins

- `-pages N|A-B|A-|-B` is accepted by `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster`. `raster` paints every page of a PDF instead of only page 0, and `compare raster` opens PDFs with `OpenPDF` and paints each selected page. Emitted pages number from 1 in a `%d` path, matching Ghostscript.
- `raster` writes `.tif` and `.tiff` through `golang.org/x/image/tiff`, with `-tiffcompress none|deflate` and deflate as the default. This is the first module dependency; the reason is recorded next to the require line. LZW and CCITT are decode-only in that library and are not offered.
- `pdfimage -colorspace rgb|gray|cmyk` and the public `ImagePDFColor`. DeviceGray uses BT.601 luma; DeviceCMYK uses the K-first division. Both formulas are in `documentation/devices.md` with a worked example, and `rgb` stays the default so earlier bytes do not change.
- `documentation/gs-argv-mapping.md` maps the Ghostscript switches the CLI can already express and names the rejected ones.

### Compression

- The PDF content interpreter now composes a six-number matrix for `cm`, saved and restored by `q` and `Q`.
- `internal/pdf` exposes object access and serialization, and `pdfout.WriteCopy` copies every source object it does not rewrite. Text, fonts, annotations, and page boxes survive operators Spectre cannot interpret.
- Image XObjects are discovered and decoded (Flate and DCT), then resampled with CatmullRom and re-encoded as DCT or Flate.
- `rewrite -level 0..5`: level 0 is the previous path-emit writer. Levels 1 to 5 use the pass-through writer. Level 1 Flates uncompressed content streams, level 2 also re-encodes Flate and raw images losslessly, and levels 3 to 5 re-encode images as DCT with longest-side caps of 1754, 1123, and 842 pixels at qualities 80, 60, and 40. Bytes are stable per level, with no `CreationDate` or `ModDate`.

### Tests and ledgers

- Every plan row is checked with a passing proof. New tests cover page ranges, PDF compare, TIFF, image colors, the content matrix, the copy writer, image decode and scale, the levels, and sampledata acceptance.
- `documentation/features.md`, `covered-and-not-covered.md`, `folder-structure.md`, `copyright-and-rewrite.md`, and the root `README.md` are refreshed. `plans/v0.0.1/10-deferred.md` has 10 landed rows and 8 open rows.

---

## Impact

| Area | Impact |
|------|--------|
| **Performance** | Levels 1 to 5 rebuild the file and re-encode images. No measured baseline. |
| **Memory** | Compression decodes one image at a time. The existing raster caps still apply. |
| **Behavior / correctness** | Level 0 is unchanged. Levels 1 to 5 preserve text, fonts, and annotations. CCITT and JPEG2000 streams copy through unchanged. |
| **API / CLI** | New `ImageColor`, `ImagePDFColor`, `RewriteOptions.Level`, `-pages`, `-tiffcompress`, `-colorspace`, and `rewrite -level`. No existing signature changed. |
| **Dependencies** | One new module, `golang.org/x/image` v0.46.0, for TIFF and image scaling. |
| **Binary size / build time** | The command links `image/tiff` and `draw` through the module. `make build` still produces `bin/spectreps`. |

---

## Breaking changes / migration

| Item | Migration |
|------|-----------|
| None | Defaults are unchanged: `rewrite` without `-level` is level 0, `pdfimage` without `-colorspace` is rgb, and every other suffix still falls back to PPM. |

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
go test -count=1 ./internal/cli -run TestRasterPDFPages
go test -count=1 ./internal/cli -run TestPageSelection
go test -count=1 ./internal/cli -run TestCompareRasterPDF
go test -count=1 ./internal/cli -run TestRasterTIFF
go test -count=1 ./internal/cli -run TestPDFImageColor
go test -count=1 ./internal/cli -run TestRewriteLevels
go test -count=1 ./internal/cli -run TestRewriteSamples
go test -count=1 ./spectreps -run TestRewriteLevelsStable
go test -count=1 ./internal/pdfout -run TestCopyObjects
go test -count=1 ./internal/pdf -run TestImageXObject
```

Each command exited 0 on 2026-09-25 on `feature/003-quick-wins-and-compression`.

---

## Screenshots / sample output

Compression levels on `sampledata/compress/whatisthis.pdf`, a PDF 1.7 file with text, a `cm` matrix, and one DCT image of 2480 by 3508 pixels:

| File | Level | Bytes |
|---|---:|---:|
| `whatisthis.pdf` | input | 596,341 |
| `whatisthis_level1.pdf` | 1 | 614,343 |
| `whatisthis_level2.pdf` | 2 | 614,343 |
| `whatisthis_level3.pdf` | 3 | 229,469 |
| `whatisthis_level4.pdf` | 4 | 108,116 |
| `whatisthis_level5.pdf` | 5 | 85,760 |

Levels 1 and 2 are slightly larger than the input because the writer rebuilds the container and the image is already DCT. Levels 3 to 5 more than recover it. All outputs open as valid PDF 1.4 files with 8 pages.

`sampledata/compress/path.pdf` compresses from 8,450 bytes to 667 bytes. Levels 1 to 5 are byte-identical for that file because it has no images.

---

## Related issues

- No GitHub issue exists for this work. The parent ledger is `plans/v0.0.3/00-program.md`.

---

## PR metadata checklist (author)

- [x] Self-assigned (`--assignee @me`)
- [x] Labels applied
- [ ] Related issues filled with real ticket IDs
- [x] Filled body committed under `plans/PR/pr-v0.0.3.md` when process-gated

---

## Follow-ups (out of scope)

- Painting `Do` and reading images into a raster.
- CCITT and JPEG2000 image stream decoding.
- `ink_cov` weighted amounts, which need a named weighting model.
- Text extraction, PDF/A, `ps2write`, the full `gs` grammar, and the printer languages.
- The copy writer copies the source `/Type /XRef` and `/Type /ObjStm` container objects and writes plain objects, which grows already-optimized files at levels 1 and 2 by about 3%. A future row can skip those dead objects and write a new object stream.

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
| `.go` | 30 | 4054 | 139 |
| `.md` | 17 | 627 | 167 |
| `.mod` | 1 | 3 | 0 |
| `.pdf` | 6 | Binary | Binary |
| `.sum` | 1 | 2 | 0 |
| **Total** | **55** | **4686** | **306** |
