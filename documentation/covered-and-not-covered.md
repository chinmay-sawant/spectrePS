# Covered and not covered

This file lists Ghostscript features against the Spectre PS ledger. The manual used here is Ghostscript 10.09.0 at `https://ghostscript.readthedocs.io/en/latest/`. The product site lists release 10.08.0. Ghostscript is the PostScript and PDF interpreter. GhostPCL, GhostXPS, and Ghostscript Office are separate products that share its graphics library.

A covered row is a small slice of that Ghostscript job. It is not a promise of matching Ghostscript output.

Work that is explicitly deferred, with the reason and the next gate, stays in `plans/v0.0.1/10-deferred.md`. This file is the map. That file is the checklist.

## Covered

- Interpret PostScript. Spectre's slice is a small operator set, not LanguageLevel 3. See `documentation/language.md`.
- Open a PDF and rasterize pages. Spectre's slice is path operators, text operators, Flate streams, and image XObjects through `Do`, not PDF 1.7 or PDF 2.0.
- Rasterize to an image. Spectre writes PPM, PNG, JPEG, and TIFF (none or Deflate). Ghostscript also writes BMP, PCX, fax, and PSD.
- Select pages with `-pages`, in the style of `-dFirstPage` and `-dLastPage`. `raster`, `bbox`, `inkcov`, `pdfimage`, and `compare raster` take the flag.
- Report the painted box in points, the RGB mark coverage of a page in the style of `bbox` and `inkcov`, and the weighted RGB ink amount in the style of `ink_cov`. Spectre's channels are RGB because the pixmap is RGB, so neither ink report is a CMYK report.
- Wrap each painted page in a new PDF as one image, in the style of `pdfimage24`, `pdfimage8`, and `pdfimage32`. Spectre uses Flate image streams and the RGB, gray, and CMYK spaces.
- Rewrite a PDF as a new file, compress streams, and re-encode images. Spectre Flates content, re-encodes Flate, raw, and CCITT image streams losslessly at level 2, and DCT-encodes images with a longest-side cap at levels 3 through 5. Levels 3 through 5 also decode JPEG2000 streams and re-encode them as DCT.
- Rewrite a path-only PDF as PostScript, in the style of `pdf2ps` and `ps2write`. Spectre writes the points operators, not glyphs or image data, so text, fonts, and images are not written.
- Paint and extract PDF text, in the style of `txtwrite` and `ps2ascii`. Spectre paints embedded TrueType, OpenType, and Type 1 outlines and the standard 14 advances, and `spectreps text` writes UTF-8 with one CRLF per line and a code-point fallback. Bare CFF, Type 3, vertical writing, color fonts, and variable fonts are out of this tag, and OCR is out.
- Write a PDF/A-4 claim, in the style of `-dPDFA` policy 2. `-pdfa 4` claims the base profile and `-pdfa 4f` claims the embedded-file profile. Spectre appends a static XMP packet and an sRGB output intent, and the profile preflight refuses a known violation instead of keeping the claim. The result is a claim, not a certificate.
- Preserve an existing PDF/UA-2 structure tree through a pass-through rewrite, preflight the machine-checkable subset, and refuse to strip tags in a generated file. Levels 1 through 5 keep `/StructTreeRoot`, MCIDs, `/Alt`, `/ActualText`, and `/Lang`; level 0 and `pdfimage` return `/tagged` instead. The UA-2 preflight reports `Error: /ua2-<rule> in PDFUA`, and a `pdfuaid` claim is written only when the source carried one or an opt-in followed a passing preflight.
- Stop on the first broken-file error, the same idea as `-dPDFSTOPONERROR`.
- A library call and a CLI over that call, the same split as `gsapi` and the `gs` binary.
- Block file write, rename, and delete by default, which is the rough idea of SAFER.

## Not covered

- Full PostScript LanguageLevel 3, including filters other than Flate, `%pipe%`, and `%disk`.
- Full PDF 1.7 and PDF 2.0, including transparency, optional content, encryption, and passwords.
- OCR (`pdfocr`, Tesseract). Font programs beyond the subset in `documentation/fonts.md`: bare CFF, Type 3, vertical writing, color fonts, and variable fonts.
- Images inside a PDF on the reading side beyond `Do` on `/Subtype /Image`. `Do` paints RGB and gray image XObjects. `/SMask`, `/Mask`, `/ImageMask`, and `/Decode` arrays are refused, not approximated. Rewrite decodes CCITT G4 and G3 and JPEG2000 image streams: level 2 re-encodes CCITT losslessly as Flate, and levels 3 through 5 re-encode both as DCT.
- Font embedding and subsetting.
- PDF/A-1, PDF/A-2, PDF/A-3, and PDF/A-4e creation. Spectre writes a PDF/A-4 or 4f claim only. It does not embed missing fonts or convert color, and it produces no certificate.
- PDF/UA-2 conformance checking, certification, tag generation, reading order, and role assignment. Spectre preserves an existing tree and preflights a machine-checkable subset; it says preflight only.
- PDF/X creation.
- EPS rewrite (`eps2write`, `ps2epsi`) and PostScript output for text, fonts, or images.
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
