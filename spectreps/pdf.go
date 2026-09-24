package spectreps

import (
	"context"
	"errors"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// Document is an open PDF. file holds the parsed objects.
type Document struct {
	file *pdf.File
}

// OpenPDF opens a PDF and returns its document.
func (in *Instance) OpenPDF(ctx context.Context, src []byte) (*Document, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	opened, err := pdf.Open(ctx, src)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return &Document{file: opened}, nil
}

// RewritePDF writes a new PDF. The writer arrives in a later tag.
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	_ = doc
	_ = opt

	return nil, in.impl.Ready(ctx)
}

func asPDFJobError(err error) error {
	var pdfErr *pdf.Error
	if errors.As(err, &pdfErr) {
		return JobError{Op: pdfErr.Op, Msg: pdfErr.Name, Filename: "", Line: 0, Column: 0}
	}
	return err
}
