# Ghostscript baseline

Spectre PS copies Ghostscript's jobs. This file records what that means, from the binary on this machine and from the current manual. It is the source for scope arguments in the plans.

Checked on 2026-09-24.

## Local binary

`/usr/bin/gs` is GPL Ghostscript 9.55.0, dated 2021-09-27. `gs --version` prints `9.55.0`. The Debian package is `ghostscript` `9.55.0~dfsg1-0ubuntu5.13`. The binary loads `libgs.so.9`. Spectre does not load that library.

`gs -h` names these input formats: PostScript, PostScriptLevel1, PostScriptLevel2, PostScriptLevel3, PDF. The default device on this build is `bbox`. Help lists `-dNOPAUSE`, `-q`, `-g<width>x<height>`, `-r<res>`, `-sDEVICE=`, `-dBATCH`, and `-sOutputFile=`. Help does not list `-dSAFER`. The wrapper scripts do.

`gs -q -dNODISPLAY -c "quit"` exits 0. No sample page was rendered for this note.

Companion scripts on `PATH`, and the device each one selects:

| Script | Device or behavior |
| --- | --- |
| `ps2pdf`, `ps2pdf12`, `ps2pdf13`, `ps2pdf14`, `ps2pdfwr` | `pdfwrite`. `ps2pdf14` adds `-dCompatibilityLevel=1.4`. `ps2pdfwr` also passes `-dSAFER`. |
| `pdf2ps` | `ps2write`, with `-dSAFER`. |
| `ps2ps`, `ps2ps2` | `ps2write`. |
| `ps2epsi`, `eps2eps` | `eps2write`. |
| `ps2ascii`, `ps2txt` | `txtwrite`, with `-dSAFER`. |
| `gsnd` | `-dSAFER -dNODISPLAY`. |
| `dvipdf` | `dvips` piped into `pdfwrite`. |
| `pdf2dsc` | index of a PDF, `-dNODISPLAY`. |

`pdfopt` is not installed. `/usr/share/doc/ghostscript/Use.htm` is not installed. `ghostscript-doc` is not installed.

Device names that matter for Spectre, spelled as `gs -h` prints them:

- Raster: `png16m`, `pnggray`, `pngmono`, `pngalpha`, `jpeg`, `jpeggray`, `tiff24nc`, `ppm`, `ppmraw`, `pgmraw`, `pam`, `pamcmyk32`.
- Rewrite: `pdfwrite`, `ps2write`, `eps2write`.
- Analysis: `bbox`, `inkcov`, `ink_cov`, `txtwrite`, `bit`, `bitrgb`, `bitcmyk`.
- PDF bitmap wrappers: `pdfimage8`, `pdfimage24`, `pdfimage32`.

`gs -h` prints `pdfwrite` three times. The device dictionary has one `/pdfwrite` key. There is no `compare` device.

Sample files under `/usr/share/ghostscript/9.55.0/lib` include `PDFA_def.ps`, `PDFX_def.ps`, and `pdf_info.ps`. The header of `PDFA_def.ps` says it is a sample prefix for creating a PDF/A document. Nothing in this audit ran that file.

## Manual, Ghostscript 10.08

The release notes and manual used here are the 10.08.0 set at `https://ghostscript.readthedocs.io/en/gs10.08.0/`. The local binary is older. Where they differ, the table below says so.

Ghostscript interprets PostScript and PDF. GhostPCL interprets PCL and PXL. GhostXPS interprets XPS. GhostPDL is the combined package. The Readme page says the graphics library is shared, and that people sometimes call the whole family Ghostscript. Spectre's ledger is the Ghostscript executable's PostScript and PDF jobs, not PCL or XPS.

Use.html says Ghostscript looks at each file and decides PDF or PostScript. The same switches apply to both, with a few exceptions. PDF needs random access. A PDF on stdin is copied to a temporary file first.

Since 9.50, SAFER is the default. It limits read, write, delete, rename, and some device parameters. `-dNOSAFER` turns that off. Spectre's default is the same idea: page description operators run, filesystem mutation operators do not.

## Raster and rewrite are different jobs

Devices.html describes raster devices as calling the graphics library and storing a bitmap. `-r` is pixels per inch. The usual default is 72. `png16m` is 24-bit RGB. `jpeg` is JFIF. `tiff24nc` is 24-bit RGB. The PNM family includes `ppmraw` and `pgmraw`. Devices.html for 10.08 does not list a device named `pam`. The 9.55.0 help text does. Spectre's raw fixture format is `ppmraw` (P6), because that name exists in both places and the bytes are uncompressed pixels plus a short header.

`pdfimage24` renders to a bitmap and wraps that bitmap in a PDF. That is not `pdfwrite`.

VectorDevices.html says `pdfwrite`, `ps2write`, and `eps2write` reassemble drawing primitives into a new page description. The new file should make the same marks. The insides are a new file. `ps2pdf` is a script that launches Ghostscript with `pdfwrite`. It is not a separate compressor.

Documented `pdfwrite` defaults that Spectre's rewrite phase has to stay honest about:

| Parameter | Documented default |
| --- | --- |
| `CompressPages` | true for `pdfwrite`, false for `ps2write` and `eps2write` |
| `CompressFonts` | true |
| `ColorImageFilter` and `GrayImageFilter` | `/DCTEncode` |
| `MonoImageFilter` | `/CCITTFaxEncode` |
| `DownsampleColorImages`, gray, mono | false, except the screen and ebook presets |
| `UseFlateCompression` | true, treated as always on |

`-dJPEGQ` belongs to the raster `jpeg` device and to `pdfimage*`, not to `pdfwrite`. `pdfwrite` JPEG quality is `QFactor` inside the image dictionaries. The first Spectre rewrite uses Flate on content streams. DCT, CCITT, and downsample wait until an image model exists. See `plans/v0.0.1/10-deferred.md`.

A page that `pdfwrite` cannot keep as vectors, such as live transparency aimed at a PDF level below 1.4, can be rendered to a bitmap and wrapped. That fallback is not the normal path.

## Validate, PDF/A, and compare

Use.html says the PDF interpreter tries to repair broken files and print a warning, so Ghostscript can still be a sanity check. `-dPDFSTOPONERROR` makes it stop instead. `-dPDFSTOPONWARNING` stops on warnings and implies stop on error. `-dPDFINFO` prints an inventory. It is not a conformance test.

VectorDevices.html says Ghostscript can create PDF/A-1b, PDF/A-2b, and PDF/A-3b through `pdfwrite`, a color strategy, and `PDFA_def.ps`. Other PDF/A flavors are not supported. With the default `PDFACompatibilityPolicy` of 0, a feature that breaks PDF/A is included anyway, and the file is not PDF/A compliant even if metadata says otherwise. Creating PDF/A writes a new file. Spectre will not claim a PDF/A certificate. The `validate` command is the stop-on-error sanity check.

No `compare` device exists in `gs -h` or in the 10.08 device manual. Artifex regression practice renders to `ppmraw` or `pbmraw` and hashes those files outside Ghostscript. `bbox` prints a bounding box. `inkcov` prints ink fractions. `txtwrite` extracts text. `-dDetectDuplicateImages` reuses images inside one conversion. None of those compare two inputs.

`pdfwrite` output is not byte-stable across versions or C libraries. Dates, trailer `/ID`, and XMP change the file unless omitted, and some PDF/A and PDF 2.0 modes ignore those omissions. Spectre's own writer has a stricter test: two rewrites of the same input return equal bytes, because dates and random ids are omitted. That test does not mean the bytes match Ghostscript.

## What Spectre takes from this

| Ghostscript job | Spectre command | When |
| --- | --- | --- |
| Interpret a PostScript file and paint marks | `spectreps run` | tag 0.0.2 |
| Rasterize to pixels, then encode an image | `spectreps raster` | tag 0.0.2 |
| Compare two rendered pixmaps, or two files, byte by byte | `spectreps compare` | file bytes in 0.0.1, pixmaps in 0.0.2 |
| Open a PDF and rasterize a page | `spectreps raster` on a PDF | tag 0.0.3 |
| Write a new compressed PDF | `spectreps rewrite` | tag 0.0.4 |
| Stop on the first interpreter error | `spectreps validate` | tag 0.0.5 |

## Sources

- Local `gs -h`, `gs --version`, and the wrapper scripts in `/usr/bin`.
- `https://ghostscript.readthedocs.io/en/gs10.08.0/Readme.html`
- `https://ghostscript.readthedocs.io/en/gs10.08.0/Use.html`
- `https://ghostscript.readthedocs.io/en/gs10.08.0/Devices.html`
- `https://ghostscript.readthedocs.io/en/gs10.08.0/VectorDevices.html`
- `https://ghostscript.readthedocs.io/en/gs10.08.0/Language.html`
- Interpreter API shape: `https://ghostscript.readthedocs.io/en/gs10.01.0/API.html`
