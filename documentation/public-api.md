# Public API

Package `spectreps`. Import path `github.com/chinmay-sawant/spectrePS/spectreps`.

Phase 02 adds these signatures. `CompareFiles`, `New`, `Close`, and `Version` work in that phase. The other methods return `ErrNotImplemented` until the phase named in `plans/v0.0.1/00-program.md`.

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

type Document struct { /* unexported */ }

type RewriteOptions struct {
    CompressStreams bool
}

func DefaultRewriteOptions() RewriteOptions // CompressStreams true

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
func (in *Instance) RasterizePage(ctx context.Context, doc *Document, pageIndex int, opt RunOptions) (PageImage, error)
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error)

func CompareFiles(a, b []byte) CompareResult
func CompareRaster(a, b PageImage) CompareResult

func MeasureBox(img PageImage, dpi float64) (Box, bool)
func MeasureInk(img PageImage) Ink
```

`CompareFiles` and `CompareRaster` do not take an `Instance`. `MeasureBox` and `MeasureInk` do not take one either.

`CompareFiles` rules:

- Equal slices set `Equal` true, `Offset` -1, `Reason` empty.
- The first differing index sets `Equal` false, `Offset` to that index, `Reason` `byte`.
- If one slice is a prefix of the other, `Offset` is the shorter length and `Reason` is `length`.
- Two empty slices are equal.

`CompareRaster` rules:

- Different `Width` or `Height` sets `Equal` false, `Offset` -1, `Reason` `width` or `height`. Width is checked first.
- Same dimensions and different RGB bytes set `Reason` `pixel` and `Offset` to the first byte index in row-major order, ignoring stride padding.
- Stride padding is not compared.

`MeasureBox` and `MeasureInk` read a finished `PageImage`. They do not paint a second time and they add no operator.

`MeasureBox` rules:

- The box is the union of marked pixels in points, origin at the lower left.
- A pixel marks when any of R, G, or B is not 255. Stride padding is ignored.
- Column `c` spans `c*72/dpi` to `(c+1)*72/dpi`. Row 0 is the top, so pixel row `r` spans `(height-r-1)*72/dpi` to `(height-r)*72/dpi`.
- `dpi` of 0 or less selects 72. No marked pixel returns the zero `Box` and false.

`MeasureInk` rules:

- Each field is the fraction of pixels whose channel byte is not 255. The denominator is `Width * Height`.
- Stride padding is ignored. A zero-size image returns the zero `Ink`.
- The channels are RGB occupancy, not CMYK, and not Ghostscript `ink_cov` amounts.

A cancelled `ctx` returns `ctx.Err()` and no partial success. `nil` context is a programming error and panics. The CLI always passes a real context.

`pageIndex` is zero-based. A negative index or an index past the last page returns `rangecheck`.

`DefaultRewriteOptions` turns stream compression on. The zero `RewriteOptions` leaves it off, so a test can ask for uncompressed streams on purpose. The CLI uses `DefaultRewriteOptions`.
