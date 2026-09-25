# Covered and not covered

This file lists Ghostscript features against the Spectre PS ledger. The manual used here is Ghostscript 10.09.0 at `https://ghostscript.readthedocs.io/en/latest/`. The product site lists release 10.08.0. Ghostscript is the PostScript and PDF interpreter. GhostPCL, GhostXPS, and Ghostscript Office are separate products that share its graphics library.

A covered row is a small slice of that Ghostscript job. It is not a promise of matching Ghostscript output.

Work that is explicitly deferred, with the reason and the next gate, stays in `plans/v0.0.1/10-deferred.md`. This file is the map. That file is the checklist.

## Covered

- Interpret PostScript. Spectre's slice is a small operator set, not LanguageLevel 3. See `documentation/language.md`.
- Open a PDF and rasterize pages. Spectre's slice is path operators plus Flate streams, not PDF 1.7 or PDF 2.0.
- Rasterize to an image. Spectre writes PPM, PNG, JPEG, and TIFF (none or Deflate). Ghostscript also writes BMP, PCX, fax, and PSD.
- Select pages with `-pages`, in the style of `-dFirstPage` and `-dLastPage`. `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster` take the flag.
- Report the painted box in points and the RGB mark coverage of a page, in the style of `bbox` and `inkcov`. Spectre's coverage is RGB occupancy, not a CMYK report.
- Wrap each painted page in a new PDF as one image, in the style of `pdfimage24`, `pdfimage8`, and `pdfimage32`. Spectre uses Flate image streams and the RGB, gray, and CMYK spaces.
- Rewrite a PDF as a new file, compress streams, and re-encode images. Spectre Flates content, re-encodes Flate and raw image streams losslessly at level 2, and DCT-encodes images with a longest-side cap at levels 3 through 5. Levels 3 through 5 decode JPEG2000 streams and re-encode them as DCT. CCITT streams copy unchanged.
- Stop on the first broken-file error, the same idea as `-dPDFSTOPONERROR`.
- A library call and a CLI over that call, the same split as `gsapi` and the `gs` binary.
- Block file write, rename, and delete by default, which is the rough idea of SAFER.

## Not covered

- Full PostScript LanguageLevel 3, including filters other than Flate, `%pipe%`, and `%disk`.
- Full PDF 1.7 and PDF 2.0, including transparency, optional content, encryption, and passwords.
- Fonts, `show`, text extraction (`txtwrite`, `ps2ascii`), and OCR (`pdfocr`, Tesseract).
- Images inside a PDF on the reading side. `Do` still returns `undefined`, so an image page does not rasterize. Rewrite copies CCITT image streams unchanged; JPEG2000 streams decode and re-encode as DCT at levels 3 through 5.
- Font embedding and subsetting.
- PDF/A-1b, PDF/A-2b, and PDF/A-3b creation.
- PDF/X creation.
- PDF to PostScript (`pdf2ps`, `ps2write`) and EPS rewrite (`eps2write`, `ps2epsi`).
- XPS output (`xpswrite`), DOCX output (`docxwrite`), and PCL-XL output (`pxlmono`, `pxlcolor`).
- PCLm output.
- Spot-color separations (`tiffsep`).
- On-screen display.
- PDF info (`-dPDFINFO`), linearized PDF, and output encryption.
- Printer devices, duplex, N-up, and PJL.
- GhostPCL (PCL and PXL input), GhostXPS, GhostPDL image inputs, and Ghostscript Office (Word, PowerPoint, Excel).
- The bundled URW fonts.

Ghostscript has no file-compare command. Byte compare and pixel compare are Spectre features. They are specified in `documentation/devices.md`.

## Sources

- `https://ghostscript.readthedocs.io/en/latest/Readme.html`
- `https://ghostscript.readthedocs.io/en/latest/Use.html`
- `https://ghostscript.readthedocs.io/en/latest/Language.html`
- `https://ghostscript.readthedocs.io/en/latest/Devices.html`
- `https://ghostscript.readthedocs.io/en/latest/VectorDevices.html`
- `https://ghostscript.readthedocs.io/en/latest/API.html`
- `https://ghostscript.com/faq`
