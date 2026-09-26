# Public API

Package `spectreps`. Import path `github.com/chinmay-sawant/spectrePS/spectreps`.

These are the current signatures. Every method below is implemented. `CompareFiles`, `New`, `Close`, and `Version` landed first; the interpreter methods followed in the phases named in `plans/v0.0.1/00-program.md`, `plans/v0.0.2/00-program.md`, `plans/v0.0.3/00-program.md`, and `plans/v0.0.4/00-program.md`.

Signatures may change while the module is below `v1`. Callers in this repo are the CLI and `spectreps_test`.

## Types

```go
func Version() string

type Instance struct { /* unexported */ }

func New() (*Instance, error)
func (in *Instance) Close() error

type RunOptions struct {
    PageWidthPt   float64 // 0 selects 612
    PageHeightPt  float64 // 0 selects 792
    ResolutionDPI int     // 0 selects 72
}

type PageImage struct {
    Width  int
    Height int
    Stride int
    Pixels []byte // RGB8, row 0 is the top, len == Height*Stride, Stride >= Width*3
}

type ImageColor int

const (
    ImageColorRGB ImageColor = iota
    ImageColorGray
    ImageColorCMYK
)

type Document struct { /* unexported */ }

type PDFAMode int

const (
    PDFANone PDFAMode = iota
    PDFA4
    PDFA4F
)

type RewriteOptions struct {
    CompressStreams bool
    Level           int // 0 re-emits the path subset, 1 through 5 pass through
    PDFA            PDFAMode // zero leaves the claim off
    SubsetFonts     bool // off by default, subsets embedded TrueType at levels 1 through 5
    Tag             bool     // generate a PDF/UA-2 structure tree
    Claim           bool     // write pdfuaid after a passing tag preflight
    Title           string   // dc:title of the tagged write
    Lang            string   // catalog /Lang of the tagged write
}

func DefaultRewriteOptions() RewriteOptions // CompressStreams true at level 0

type PostScriptOptions struct{} // no options yet; the page box comes from the document

type CompareResult struct {
    Equal  bool
    Offset int64
    Reason string
}

type Box struct {
    MinX float64
    MinY float64
    MaxX float64
    MaxY float64
}

type Ink struct {
    R float64
    G float64
    B float64
}

type PDFPageSize struct {
    Width  float64
    Height float64
}

type PDFFontInfo struct {
    Name     string
    Embedded bool
}

type PDFInfo struct {
    Version   string
    Pages     int
    PageSizes []PDFPageSize
    Tagged    bool
    Fonts     []PDFFontInfo
    Images    int
}

var ErrNotImplemented = errors.New("spectreps: not implemented")

type JobError struct {
    Op       string
    Msg      string
    Filename string
    Line     int
    Column   int
}
```

`JobError` implements `error`. The text form is one line:

```
Error: /stackunderflow in add at box.ps:3:5
```

Omit ` at file:line:col` when the position is unknown. `Op` is the operator name without a slash. The slash is only in the text form, matching Ghostscript's `/stackunderflow` spelling.

## Methods

```go
func (in *Instance) RunPostScript(ctx context.Context, src []byte, opt RunOptions) ([]PageImage, error)
func (in *Instance) OpenPDF(ctx context.Context, src []byte) (*Document, error)
func (doc *Document) PageCount() int // page leaves, 0 when doc is nil
func (doc *Document) Tagged() bool   // structure tree or /MarkInfo /Marked true, false when doc is nil
func (doc *Document) Info() (PDFInfo, error)
func (in *Instance) RasterizePage(ctx context.Context, doc *Document, pageIndex int, opt RunOptions) (PageImage, error)
func (in *Instance) ExtractText(ctx context.Context, doc *Document, pageIndex int) (string, error)
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error)
func (in *Instance) PreflightUA2(ctx context.Context, doc *Document) error
func (in *Instance) WritePostScript(ctx context.Context, doc *Document, opt PostScriptOptions) ([]byte, error)
func (in *Instance) ImagePDF(ctx context.Context, pages []PageImage, dpi float64) ([]byte, error)
func (in *Instance) ImagePDFColor(ctx context.Context, pages []PageImage, dpi float64, color ImageColor) ([]byte, error)

func CompareFiles(a, b []byte) CompareResult
func CompareRaster(a, b PageImage) CompareResult

func MeasureBox(img PageImage, dpi float64) (Box, bool)
func MeasureInk(img PageImage) Ink
func MeasureInkAmount(img PageImage) Ink
```

`CompareFiles` and `CompareRaster` do not take an `Instance`. `MeasureBox`, `MeasureInk`, and `MeasureInkAmount` do not take one either.

`CompareFiles` rules:

- Equal slices set `Equal` true, `Offset` -1, `Reason` empty.
- The first differing index sets `Equal` false, `Offset` to that index, `Reason` `byte`.
- If one slice is a prefix of the other, `Offset` is the shorter length and `Reason` is `length`.
- Two empty slices are equal.

`CompareRaster` rules:

- Different `Width` or `Height` sets `Equal` false, `Offset` -1, `Reason` `width` or `height`. Width is checked first.
- Same dimensions and different RGB bytes set `Reason` `pixel` and `Offset` to the first byte index in row-major order, ignoring stride padding.
- Stride padding is not compared.

`MeasureBox`, `MeasureInk`, and `MeasureInkAmount` read a finished `PageImage`. They do not paint a second time and they add no operator.

`MeasureBox` rules:

- The box is the union of marked pixels in points, origin at the lower left.
- A pixel marks when any of R, G, or B is not 255. Stride padding is ignored.
- Column `c` spans `c*72/dpi` to `(c+1)*72/dpi`. Row 0 is the top, so pixel row `r` spans `(height-r-1)*72/dpi` to `(height-r)*72/dpi`.
- `dpi` of 0 or less selects 72. No marked pixel returns the zero `Box` and false.

`MeasureInk` rules:

- Each field is the fraction of pixels whose channel byte is not 255. The denominator is `Width * Height`.
- Stride padding is ignored. A zero-size image returns the zero `Ink`.
- The channels are RGB occupancy, not CMYK, and not Ghostscript `ink_cov` amounts. The weighted counterpart is `MeasureInkAmount`.

`MeasureInkAmount` rules:

- Each field is the mean complement of one channel byte: `(255 - c) / 255`, summed over `Width * Height` and divided by the pixel count.
- A white page returns the zero `Ink` and a black page returns `1` on every channel.
- Stride padding is ignored. A zero-size image returns the zero `Ink`.
- The amount is a fraction. The CLI prints it times 100 with five decimals and the `RGB` suffix. The formula and both worked examples are in `documentation/devices.md`.

A cancelled `ctx` returns `ctx.Err()` and no partial success. `nil` context is a programming error and panics. The CLI always passes a real context.

`pageIndex` is zero-based for `RasterizePage` and `ExtractText`. A negative index or an index past the last page returns `rangecheck`.

`ExtractText` returns the text of one page. Lines run top to bottom and left to right, each line ends with CRLF, and a font with neither `/ToUnicode` nor a named encoding falls back to the code point. The text comes from the same glyph sink as the show operators, so a standard 14 font extracts without an outline program. The output is not compared with Ghostscript `txtwrite`: text pixels never byte-match, because hinting and antialiasing differ, so the oracle is text and geometry.

`ImagePDF` writes a new PDF with one 24-bit RGB Flate image per `PageImage`. `dpi` is the resolution the pages were painted at, and zero or less selects 72. The content stream paints `/Im0 Do` and the page resources carry the XObject, so `RasterizePage` of the reopened file matches the source `PageImage` under `CompareRaster`. The output is not `pdfwrite`.

`ImagePDF` calls `ImagePDFColor` with `ImageColorRGB`, so its bytes do not change. `ImageColorGray` writes one 8-bit sample per pixel with `/DeviceGray`. `ImageColorCMYK` writes four 8-bit samples per pixel with `/DeviceCMYK`. Both use `/Filter /FlateDecode`. The conversion formulas and the pure red example are in `documentation/devices.md`.

`Document.Info` reads the document summary behind `spectreps info`. It resolves objects and writes nothing. `Version` is the `%PDF-` header version, `Pages` is the page tree leaf count, and `PageSizes` holds one resolved `/MediaBox` per page in points, inherited from the nearest `/Pages` ancestor and defaulting to 612 by 792. `Tagged` is the same flag as `Document.Tagged`. `Fonts` lists every in-use `/Type /Font` dictionary except CIDFont descendants, sorted by name, and `Embedded` is true when the descriptor carries `/FontFile`, `/FontFile2`, or `/FontFile3`, when the font is Type 3, or when every descendant of a Type0 font carries a program. `Images` counts the in-use image XObjects. A nil document returns a `JobError` with `Op` `Info` and `Msg` `rangecheck`; a malformed `/MediaBox` returns `Error: /syntaxerror in Info`. The command's lines are in `documentation/cli.md`.

`DefaultRewriteOptions` turns stream compression on at level 0. The zero `RewriteOptions` leaves it off, so a test can ask for uncompressed streams on purpose. The CLI uses `DefaultRewriteOptions` when no flag is given.

`RewritePDF` at level 0 refuses a tagged document with `Error: /tagged in RewritePDF`, because the path-only writer cannot keep the tree. Levels 1 through 5 keep the tags and the source header version. `Document.Tagged` reads the catalog `/StructTreeRoot` or a true `/MarkInfo /Marked`. The claim for this work is preflight only, never certification.

`RewriteOptions.SubsetFonts` is off by default. At levels 1 through 5 it replaces every embedded `/FontFile2` TrueType font a page showed with a stable-glyph-index subset, appends the new program and a synthesized `/ToUnicode` stream, and rewrites the font dictionary. Glyph indices do not change, so content streams, `/Widths`, `/Differences`, `/Encoding`, and `/CIDToGIDMap` stay valid without a page re-encode. A `/FontFile3 /OpenType` program and a Type 1 `/FontFile` program are copied whole, and a font with no program is copied unchanged, so the PDF/A `font-not-embedded` rule still refuses it whether or not the option ran. Level 0 ignores the option and still refuses text. Two calls with the option on return equal bytes. `documentation/fonts.md` has the scope.

`RewriteOptions.Tag` builds the tagged write: one recorder per page at 72 dpi, a reading order derived from device geometry, and a PDF/UA-2 structure tree. `Claim` writes `pdfuaid:part 2` and `pdfuaid:rev 2024` only after the built bytes pass `pdfa.PreflightUA2`; `Title` and `Lang` fill `dc:title` and the catalog `/Lang`, with the source XMP and catalog as fallbacks. A refusal keeps the tree and writes no claim, so `RewritePDF` returns the bytes with a `JobError` in that case: `ua2-title` when no title exists, and another `ua2-<rule>` when the preflight fails elsewhere. A tagged input returns `Error: /tagged in RewritePDF`, an image with no `/Alt` source returns `Error: /alt in Tag`, and `Tag` with `PDFA` set returns `Error: /unsupported in RewritePDF`. The reading-order thresholds and the whole refusal matrix are in `documentation/devices.md`. The result is generate and preflight, never certification.

`PreflightUA2` runs the PDF/UA-2 machine checks on an open document and returns a `JobError` with `Op` `PDFUA` and the failed rule in `Msg`. It is the request `validate` makes for a tagged input. A nil document returns `rangecheck`. The result is preflight only, never certification.

`RewriteOptions.PDFA` appends a PDF/A-4 claim. `PDFA4` is the base claim and `PDFA4F` is the embedded-file claim. A claim uses the pass-through writer at the selected level, runs the profile preflight, and returns a `JobError` with `Op` `PDFA` and the failed rule in `Msg` when the input carries a known violation. The claim is a profile preflight, not a certificate. The rules and the writer changes are in `documentation/devices.md`.

`WritePostScript` writes one date-free PostScript program from a path-only document. The marks match `RewritePDF` level 0: `setrgbcolor` or `setgray`, `setlinewidth`, `m` and `l`, and `S`, `f`, or `f*` in 72 dpi points. A prolog defines the short names in terms of the long operators, each page ends in `showpage`, and the header carries the first page's resolved `/MediaBox` as its `%%BoundingBox`, floored at the minimum and ceiled at the maximum because DSC wants integers. A page with no resolvable box falls back to the 612 by 792 reader default. Two calls return equal bytes. Text and images are not emitted, so a content operator Spectre cannot emit returns `undefined` with its operator name; a text page returns `undefined in Tj`. A nil document returns `rangecheck`. The zero `PostScriptOptions` is the only supported shape in this tag; media options wait.
