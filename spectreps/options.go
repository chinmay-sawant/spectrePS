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

// PDFAMode selects the PDF/A profile a rewrite claims.
// PDFANone leaves the claim off, and it is the zero value.
type PDFAMode int

const (
	// PDFANone writes no PDF/A claim.
	PDFANone PDFAMode = iota
	// PDFA4 claims PDF/A-4 base. It refuses an input with embedded files.
	PDFA4
	// PDFA4F claims PDF/A-4f. It refuses an input with no embedded files.
	PDFA4F
)

// RewriteOptions controls a later PDF rewrite.
// The zero value leaves stream compression off. DefaultRewriteOptions turns it
// on. Level 0 re-emits the path subset with CompressStreams as the Flate
// switch. Levels 1 through 5 use the pass-through writer, which copies content
// Spectre cannot interpret. A level above 0 ignores CompressStreams.
// PDFA selects a PDF/A-4 claim. A claim uses the pass-through writer, appends
// the XMP metadata and the sRGB output intent, and runs the profile preflight
// first. Level 0 then copies streams unchanged, and levels 1 through 5 still
// use their compression and image policy.
type RewriteOptions struct {
	CompressStreams bool
	Level           int
	PDFA            PDFAMode
}

// DefaultRewriteOptions turns stream compression on at level 0.
func DefaultRewriteOptions() RewriteOptions {
	return RewriteOptions{CompressStreams: true, Level: 0, PDFA: PDFANone}
}

// CompareResult is one byte or pixel comparison.
// Equal slices use Offset -1 and an empty Reason.
type CompareResult struct {
	Equal  bool
	Offset int64
	Reason string
}
