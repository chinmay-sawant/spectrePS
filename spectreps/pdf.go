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

// PageCount returns the number of page leaves.
// A nil document reports 0.
func (doc *Document) PageCount() int {
	if doc == nil || doc.file == nil {
		return 0
	}
	return doc.file.PageCount()
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
// Level 0 re-emits the path subset. Levels 1 through 5 use the pass-through
// writer. A level outside 0 through 5 is rangecheck.
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
	if opt.Level < 0 || opt.Level > pdfout.MaxCompressionLevel {
		return nil, JobError{Op: "RewritePDF", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
	}
	if opt.Level == 0 {
		return rewriteEmitted(ctx, doc.file, opt.CompressStreams)
	}
	return rewriteLevel(ctx, doc.file, opt.Level)
}

func rewriteEmitted(ctx context.Context, file *pdf.File, compress bool) ([]byte, error) {
	count := file.PageCount()
	pages := make([]pdfout.Page, 0, count)
	for i := range count {
		content, err := file.Content(i)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		emitted, err := pdfout.Emit(ctx, content)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		pages = append(pages, pdfout.Page{Content: emitted})
	}
	out, err := pdfout.Write(ctx, pages, compress)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

func rewriteLevel(ctx context.Context, file *pdf.File, level int) ([]byte, error) {
	overrides, err := pdfout.LevelOverrides(ctx, file, level)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	out, err := pdfout.WriteCopy(ctx, file, pdfout.CopyOptions{Overrides: overrides})
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
