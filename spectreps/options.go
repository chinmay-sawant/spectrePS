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
// The zero value leaves stream compression off. DefaultRewriteOptions turns it
// on. Level 0 re-emits the path subset with CompressStreams as the Flate
// switch. Levels 1 through 5 use the pass-through writer, which copies content
// Spectre cannot interpret. A level above 0 ignores CompressStreams.
type RewriteOptions struct {
	CompressStreams bool
	Level           int
}

// DefaultRewriteOptions turns stream compression on at level 0.
func DefaultRewriteOptions() RewriteOptions {
	return RewriteOptions{CompressStreams: true, Level: 0}
}

// PostScriptOptions controls a later PostScript write.
// This tag has one shape: a fixed 612 by 792 box, a date-free header, and no
// stream compression. The type is the seam for media options that come later.
type PostScriptOptions struct{}

// CompareResult is one byte or pixel comparison.
// Equal slices use Offset -1 and an empty Reason.
type CompareResult struct {
	Equal  bool
	Offset int64
	Reason string
}
