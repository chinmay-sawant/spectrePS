package spectreps

// RunOptions selects the page for later raster jobs.
// Zero PageWidthPt selects 612. Zero PageHeightPt selects 792. Zero ResolutionDPI selects 72.
// This tag does not apply those defaults. The fields exist so callers can pass them.
type RunOptions struct {
	PageWidthPt   float64
	PageHeightPt  float64
	ResolutionDPI int
}

// PageImage is RGB8 pixels. Row 0 is the top. len(Pixels) == Height*Stride. Stride >= Width*3.
type PageImage struct {
	Width  int
	Height int
	Stride int
	Pixels []byte
}

// RewriteOptions controls a later PDF rewrite.
// The zero value leaves stream compression off. DefaultRewriteOptions turns it on.
type RewriteOptions struct {
	CompressStreams bool
}

// DefaultRewriteOptions turns stream compression on.
func DefaultRewriteOptions() RewriteOptions {
	return RewriteOptions{CompressStreams: true}
}

// CompareResult is one byte or pixel comparison.
// Equal slices use Offset -1 and an empty Reason.
type CompareResult struct {
	Equal  bool
	Offset int64
	Reason string
}
