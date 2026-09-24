package spectreps

import "context"

// Document is an open PDF. This tag never returns one.
type Document struct{}

// OpenPDF opens a PDF. The parser arrives in a later tag.
func (in *Instance) OpenPDF(ctx context.Context, src []byte) (*Document, error) {
	_ = src

	return nil, in.impl.Ready(ctx)
}

// RewritePDF writes a new PDF. The writer arrives in a later tag.
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	_ = doc
	_ = opt

	return nil, in.impl.Ready(ctx)
}
