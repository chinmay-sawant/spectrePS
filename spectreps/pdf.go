package spectreps

import (
	"context"
	"errors"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
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

// RewritePDF writes a new PDF from the open document.
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	if doc == nil || doc.file == nil {
		return nil, JobError{Op: "RewritePDF", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
	}
	count := doc.file.PageCount()
	pages := make([]pdfout.Page, 0, count)
	for i := range count {
		content, err := doc.file.Content(i)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		emitted, err := pdfout.Emit(ctx, content)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		pages = append(pages, pdfout.Page{Content: emitted})
	}
	out, err := pdfout.Write(ctx, pages, opt.CompressStreams)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

func asPDFJobError(err error) error {
	var pdfErr *pdf.Error
	if errors.As(err, &pdfErr) {
		return JobError{Op: pdfErr.Op, Msg: pdfErr.Name, Filename: "", Line: 0, Column: 0}
	}
	return err
}
