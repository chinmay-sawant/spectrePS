package spectreps

import "context"

// Document is an open PDF. Phase 02 never returns one.
type Document struct {
	body []byte
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

// OpenPDF opens a PDF. The parser arrives in a later tag.
func (in *Instance) OpenPDF(ctx context.Context, src []byte) (*Document, error) {
	return nil, stubErr(ctx)
}

// RewritePDF writes a new PDF. The writer arrives in a later tag.
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	return nil, stubErr(ctx)
}
