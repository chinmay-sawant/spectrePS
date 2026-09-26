package spectreps

import (
	"context"
	"errors"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/pdfout"
	"github.com/chinmay-sawant/spectrePS/internal/psout"
)

// taggedMsg is the JobError message for a tagged input a generated writer
// cannot keep. It reads "Error: /tagged in <op>".
const taggedMsg = "tagged"

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

// Tagged reports whether the document carries a structure tree or a
// /MarkInfo /Marked true claim. A nil document reports false.
func (doc *Document) Tagged() bool {
	if doc == nil || doc.file == nil {
		return false
	}
	return doc.file.HasStructTree()
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
// A PDFA mode uses the pass-through writer, runs the profile preflight, and
// appends the PDF/A-4 metadata and output intent. A refused preflight returns
// a JobError with Op "PDFA" and the failed rule in Msg.
func (in *Instance) RewritePDF(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	if !rewriteDocOK(doc) {
		return nil, rewriteJobError("rangecheck")
	}
	if !rewriteRangeOK(opt) {
		return nil, rewriteJobError("rangecheck")
	}
	if opt.Tag {
		return rewriteTaggedRequest(ctx, doc, opt)
	}
	if opt.PDFA != PDFANone {
		return rewritePDFA(ctx, doc.file, opt)
	}
	if opt.Level == 0 && doc.Tagged() {
		return nil, JobError{Op: "RewritePDF", Msg: taggedMsg, Filename: "", Line: 0, Column: 0}
	}
	if opt.Level == 0 {
		return rewriteEmitted(ctx, doc.file, opt.CompressStreams)
	}
	return rewriteLevel(ctx, doc.file, opt)
}

// rewriteDocOK reports whether one document can take a rewrite.
func rewriteDocOK(doc *Document) bool {
	return doc != nil && doc.file != nil
}

// rewriteRangeOK reports whether the option values are in range.
func rewriteRangeOK(opt RewriteOptions) bool {
	if opt.Level < 0 || opt.Level > pdfout.MaxCompressionLevel {
		return false
	}
	return opt.PDFA >= PDFANone && opt.PDFA <= PDFA4F
}

func rewriteEmitted(ctx context.Context, file *pdf.File, compress bool) ([]byte, error) {
	count := file.PageCount()
	pages := make([]pdfout.Page, 0, count)
	for i := range count {
		emitted, err := pdfout.EmitPage(ctx, file, i)
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

// rewriteLevel maps a level above 0 to its stream and image overrides, then
// adds the subset font objects when the caller opted in.
func rewriteLevel(ctx context.Context, file *pdf.File, opt RewriteOptions) ([]byte, error) {
	overrides, err := pdfout.LevelOverrides(ctx, file, opt.Level)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	extra, appended, err := subsetObjects(ctx, file, opt.SubsetFonts, file.ObjectCount()+1)
	if err != nil {
		return nil, err
	}
	mergeOverrides(overrides, extra)
	copyOpt := pdfout.CopyOptions{
		Overrides:       overrides,
		PackObjects:     false,
		AppendObjects:   appended,
		CatalogOverride: nil,
		PDFA:            false,
	}
	out, err := pdfout.WriteCopy(ctx, file, copyOpt)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

// WritePostScript writes a date-free PostScript program from the open document.
// The same path subset as RewritePDF level 0 is re-emitted: setrgbcolor or
// setgray, setlinewidth, m and l, and S, f, or f*. Coordinates are 72 dpi
// points in a fixed 612 by 792 box, with one showpage per page.
// Text and images wait for the font and image machines, so a content operator
// Spectre cannot emit returns undefined with its operator name, the same error
// RewritePDF returns. A text page fails with undefined in Tj.
// A nil document returns rangecheck. A nil context panics and a canceled
// context returns ctx.Err().
func (in *Instance) WritePostScript(
	ctx context.Context,
	doc *Document,
	opt PostScriptOptions,
) ([]byte, error) {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = in
	_ = opt
	if doc == nil || doc.file == nil {
		return nil, JobError{Op: "WritePostScript", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
	}
	return writePostScript(ctx, doc.file)
}

func writePostScript(ctx context.Context, file *pdf.File) ([]byte, error) {
	count := file.PageCount()
	pages := make([]psout.Page, 0, count)
	boxWidth, boxHeight := 0.0, 0.0
	for i := range count {
		content, err := file.Content(i)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		emitted, err := psout.Emit(ctx, content)
		if err != nil {
			return nil, asPDFJobError(err)
		}
		pages = append(pages, psout.Page{Content: emitted})
		if i == 0 {
			boxWidth, boxHeight = pageBox(file, i)
		}
	}
	out, err := psout.Write(ctx, pages, psout.WriteOptions{WidthPt: boxWidth, HeightPt: boxHeight})
	if err != nil {
		return nil, asPDFJobError(err)
	}
	return out, nil
}

// pageBox returns one page's real /MediaBox in points so the PostScript header
// carries the document's own page instead of a fixed letter box. A page whose
// box will not resolve falls back to zero, which psout.Write reads as the 612
// by 792 reader default.
func pageBox(file *pdf.File, index int) (float64, float64) {
	size, err := file.PageSize(index)
	if err != nil {
		return 0, 0
	}
	return size.Width, size.Height
}

func asPDFJobError(err error) error {
	var pdfErr *pdf.Error
	if errors.As(err, &pdfErr) {
		return JobError{Op: pdfErr.Op, Msg: pdfErr.Name, Filename: "", Line: 0, Column: 0}
	}
	return err
}
