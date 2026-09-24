# Copyright and rewrite

This file records how Spectre PS is allowed to be written, and the product decisions that sit next to that rule. It is a project note. It is not a legal opinion and it is not a patent clearance.

The detailed contracts stay in their own files. This note is the reason those files look the way they do.

| Topic | File |
| --- | --- |
| Ghostscript behavior we measured | `documentation/ghostscript-baseline.md` |
| Jobs Spectre takes and leaves | `documentation/covered-and-not-covered.md` |
| Tests for the jobs Spectre takes | `documentation/test.md` |
| Work ledger | `plans/v0.0.1/00-program.md` |
| Deferred checklist | `plans/v0.0.1/10-deferred.md` |
| MIT text | `LICENSE` |

## What Spectre is

Spectre PS is a Go program for jobs people use Ghostscript for. The command is `spectreps`. The library import path is `github.com/chinmay-sawant/spectrePS`, package name `spectreps`. The CLI and a later importer call the same functions. The module line is `go 1.26.4`. The remote is `https://github.com/chinmay-sawant/spectrePS.git`, branch `master`.

The program reads PostScript and PDF, paints pages to pixels, writes a new PDF, reports interpreter errors, and compares bytes. Printer drivers, PCL, and XPS are out of the ledger.

## Where the behavior comes from

Spectre is an independent implementation of published documents. The inputs are the PostScript language reference, the PDF specification, and Ghostscript's public manual. The Ghostscript install on this machine, GPL Ghostscript 9.55.0 at `/usr/bin/gs`, was used as a behavior reference for device names and wrapper scripts. The current manual used for the feature list is the 10.09.0 set at `https://ghostscript.readthedocs.io/en/latest/`. The product site lists release 10.08.0.

Nobody is reading Ghostscript's C source to produce this program. The repository does not link `libgs` and does not start `gs`. Tests do not call `/usr/bin/gs`. Fixtures are Spectre output.

This is not a clean-room rewrite. A clean room is a formal split: one group studies someone else's program and writes a specification, and a second group that never saw that program writes the new code. Spectre is not doing that. The manuals already exist so other people can build interpreters. Ghostscript itself was built from the published PostScript language.

## Tags

| Tag | What it closes |
| --- | --- |
| 0.0.1 | Public Go API, CLI shell, file byte compare. Interpreter methods return `ErrNotImplemented`. |
| 0.0.2 | PostScript subset, RGB raster to PPM and PNG, pixel compare. |
| 0.0.3 | Open a small PDF, Flate-decode streams, rasterize path-only pages. |
| 0.0.4 | Rewrite a new PDF with Flate-compressed streams and stable bytes. |
| 0.0.5 | `validate`, which stops on the first interpreter error. |

`cmd/spectreps` imports only the public package. Interpreter code lands under `internal/` when the first real file needs it. Go rejects an outside import of `internal/`, so the exported functions have to exist before the interpreter does.

## Ghostscript jobs Spectre takes

Each row is a small slice, not Ghostscript parity.

- Interpret PostScript. The slice is the operator set in `documentation/language.md`, not LanguageLevel 3.
- Open a PDF and rasterize pages. The slice is path operators plus Flate streams, not PDF 1.7 or PDF 2.0.
- Rasterize to an image. Spectre writes PPM and PNG. Ghostscript also writes JPEG, TIFF, BMP, PCX, fax, and PSD.
- Rewrite a PDF as a new file and compress streams with Flate.
- Stop on the first broken-file error, the same idea as `-dPDFSTOPONERROR`.
- A library call and a CLI over that call, the same split as `gsapi` and the `gs` binary.
- Block `file`, `run`, `deletefile`, `renamefile`, and `filenameforall` by default. They return `invalidaccess`. That is the rough idea of SAFER.

Byte compare and pixel compare are Spectre commands. Ghostscript 9.55.0 has no `compare` device, and the 10.09 device manual does not define one. Ghostscript writes images. Something else compares them. Spectre does the compare itself, on pixel buffers or on raw file bytes. PNG bytes are not the equality check, because a PNG is a compressed encoding.

## Ghostscript jobs Spectre leaves

- Full PostScript LanguageLevel 3, including filters other than Flate, `%pipe%`, and `%disk`.
- Full PDF 1.7 and PDF 2.0, including transparency, optional content, encryption, and passwords.
- Fonts, `show`, text extraction (`txtwrite`, `ps2ascii`), and OCR (`pdfocr`, Tesseract).
- Images inside a PDF, DCT and CCITT compression, downsampling, and JPEG2000.
- Font embedding and subsetting.
- PDF/A-1b, PDF/A-2b, and PDF/A-3b creation.
- PDF/X creation.
- PDF to PostScript (`pdf2ps`, `ps2write`) and EPS rewrite (`eps2write`, `ps2epsi`).
- XPS output (`xpswrite`), DOCX output (`docxwrite`), and PCL-XL output (`pxlmono`, `pxlcolor`).
- A page raster wrapped in a PDF (`pdfimage8`, `pdfimage24`, `pdfimage32`, PCLm).
- Bounding box (`bbox`), ink coverage (`inkcov`), and spot-color separations (`tiffsep`).
- On-screen display.
- Page selection, PDF info (`-dPDFINFO`), linearized PDF, and output encryption.
- Printer devices, duplex, N-up, and PJL.
- GhostPCL (PCL and PXL input), GhostXPS, GhostPDL image inputs, and Ghostscript Office (Word, PowerPoint, Excel).
- The bundled URW fonts.

The maintained short list is `documentation/covered-and-not-covered.md`. Rows that are explicitly deferred, with a reason and a next gate, live only in `plans/v0.0.1/10-deferred.md`.

## PDF/A is a new file

Ghostscript does not grade an existing PDF as PDF/A compliant. PDF/A in Ghostscript means `pdfwrite` paints the pages and writes a second file. The usual shape is `pdfwrite` with `-dPDFA=1`, `-dPDFA=2`, or `-dPDFA=3`, a color strategy, and `PDFA_def.ps` in front of the input. The input file stays where it is. The manual says converting to PDF/A creates a new PDF whose insides are not the original.

Ghostscript can create PDF/A-1b, PDF/A-2b, and PDF/A-3b. With the default `PDFACompatibilityPolicy` of 0, a feature that breaks PDF/A can be kept, and the file can still carry PDF/A metadata. That is creation, and it is not a certificate. The validation Spectre is building is the other job: stop on the first interpreter error. Spectre does not write PDF/A metadata.

Raster and rewrite are also different jobs. Raster devices paint pixels. `pdfwrite` rebuilds a page description and compresses objects inside the new file. Spectre's first compression is Flate on those streams. DCT, CCITT, and downsampling wait until an image model exists.

## Tests

`documentation/test.md` lists one expected result per case for the jobs Spectre takes: the library boundary, the PostScript subset, the y flip, PPM and PNG, pixel compare, PDF open, Flate rewrite, `validate`, and the file-access ban. Tests call package `spectreps` or the `spectreps` binary.

## Copyright

Copyright protects the text of a program, its comments, its fonts, and the wording of a manual. It does not protect the idea of an interpreter, the operator names in the published PostScript specification, or the behavior described in a public manual.

Ghostscript's copyright stays with its authors and with Artifex. The AGPL, and the commercial license next to it, attach when someone copies or links that code. Spectre does not do that. The copyright in Spectre's own Go and docs is Chinmay Sawant's, and the grant to other people is the MIT license in `LICENSE`.

The act that would hand Artifex a copyright claim is pasting Ghostscript source, Ghostscript fonts, or pages of the Ghostscript manual into this tree. Reading the public manual and writing new Go is not that act.

## Trademarks

A trademark protects a name buyers use to tell products apart. It is separate from copyright.

Artifex Software owns the registered mark Ghostscript for interpreter and rasterizer software, US serial 78071366. Adobe owns the registered mark PostScript, US registration 1544284, now held by Adobe Inc.

Spectre can say what it implements. "This program reads the PostScript language" and "these are jobs Ghostscript is used for" describe a published language and another product. The product name, the binary, the module path, and the README title stay Spectre PS. They do not become Ghostscript, and they do not carry the Adobe or Artifex logos. "Official Ghostscript" or "Adobe PostScript" as a brand for this program is the use those companies write letters about.

## Patents

A patent covers a method. Following a manual can be exactly the act a patent describes. Writing original Go does not, by itself, answer a patent claim. The public record that was checked in September 2026 says the following.

PostScript. In 1988, Adobe co-founder Charles Geschke told IEEE Spectrum that Adobe had no patents on PostScript, only copyrights and trade secrets. The trade secret was font hinting, which was kept out of the published language. The language itself was published so others could implement it. Spectre's current plan has no fonts, so it is not implementing that unpublished hinting work. Source: IEEE Spectrum, "Inventing Postscript, the Tech That Took the Pain out of Printing."

PDF. Adobe published a royalty-free patent license for implementations of ISO 32000-1, PDF 1.7. The text grants every individual and organization the royalty-free right, under essential claims Adobe owns, to make, have made, use, sell, import, and distribute compliant implementations. A compliant implementation is the portion of a product that reads, writes, modifies, or processes files compliant with that specification. Adobe may revoke the grant if the licensee sues someone else claiming that a compliant implementation infringes an essential claim. Adobe disclaims a warranty that third parties have no patents. Source: Adobe's public patent license, `ISO32000-1PublicPatentLicense.pdf`.

An older Adobe notice listed specific US patents licensed royalty-free for software that produces, consumes, and interprets compliant PDF: 5,634,064, 5,737,599, 5,781,785, 5,819,301, 6,028,583, 6,289,364, and 6,421,460. Patent 5,860,074 was licensed for producing compliant PDF, and the notice excluded software that only consumes or interprets PDF. US 5,634,064 is marked expired on Google Patents, with an anticipated expiration of 12 September 2014. Source: Adobe's patent clarification as reproduced in IETF IPR disclosure 333, and Google Patents for US 5,634,064.

ISO's own text on PDF 2.0 says some elements of the document may be the subject of patent rights, and that ISO does not identify all of them. Details of rights identified during development are listed at `https://www.iso.org/patents`. That list is what patent holders chose to declare. It is not a search of every patent.

Ghostscript. The search did not turn up an Artifex patent that reserves "interpret PostScript," "rasterize a page," or "write a PDF."

The current Spectre slice stays on paths and Flate. It leaves out fonts, JPEG, JPEG2000, LZW, and transparency. Ghostscript's own manual says `pdfwrite` ignores LZW requests. Adding a codec later is a new patent question even when a manual describes that codec.

A letter or a lawsuit can still arrive. An expired patent, or a royalty-free license for a compliant PDF implementation, is why a claim about those particular Adobe patents would be weak. This note does not say every possible patent has been checked.

## What stays out of this tree

- Ghostscript source, `libgs`, and any `os/exec` of `gs`.
- Ghostscript fonts and the URW set shipped with Ghostscript.
- Pasted pages from the Ghostscript manual. Paraphrase the behavior and cite the URL.
- The names Ghostscript and PostScript as the name of this product.
- PDF/A metadata that would look like a conformance claim.

## Sources

- `https://ghostscript.readthedocs.io/en/latest/Readme.html`
- `https://ghostscript.readthedocs.io/en/latest/Use.html`
- `https://ghostscript.readthedocs.io/en/latest/Language.html`
- `https://ghostscript.readthedocs.io/en/latest/Devices.html`
- `https://ghostscript.readthedocs.io/en/latest/VectorDevices.html`
- `https://ghostscript.readthedocs.io/en/latest/API.html`
- `https://ghostscript.com/faq`
- Adobe public patent license for ISO 32000-1: `https://www.adobe.com/pdf/pdfs/ISO32000-1PublicPatentLicense.pdf`
- IETF IPR disclosure 333, Adobe patent clarification: `https://datatracker.ietf.org/ipr/333/`
- US 5,634,064: `https://patents.google.com/patent/US5634064A/en`
- ISO patent declarations: `https://www.iso.org/patents`
- IEEE Spectrum, Geschke on PostScript patents: `https://spectrum.ieee.org/adobe-postscript`
- Ghostscript trademark, Artifex, US serial 78071366
- PostScript trademark, Adobe, US registration 1544284
