package spectreps

// This file holds the public tagged-write path: one recorder per page, the
// derived reading order and roles, and the structure tree with the UA-2
// preflight. The claim is a generate-and-preflight result, never a
// certification.

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
	"github.com/chinmay-sawant/spectrePS/internal/tag"
)

// tagPDFAMsg is the refusal name for a tag write that also asks for a PDF/A
// profile. The two claims write different catalogs and metadata, open
// decision 10.
const tagPDFAMsg = "unsupported"

// rewriteTaggedRequest refuses the combinations the tagged write cannot
// honour, then builds the tagged file. A tagged input keeps its tree only
// through the pass-through writer, so the tag switch is a named refusal on
// it, open decision 2.
func rewriteTaggedRequest(ctx context.Context, doc *Document, opt RewriteOptions) ([]byte, error) {
	if opt.PDFA != PDFANone {
		return nil, rewriteJobError(tagPDFAMsg)
	}
	if doc.Tagged() {
		return nil, rewriteJobError(taggedMsg)
	}
	return rewriteTagged(ctx, doc.file, opt)
}

// rewriteTagged builds the tagged write. A refusal after the first pass keeps
// the tree and writes no claim, so the bytes come back with the preflight
// error.
func rewriteTagged(ctx context.Context, file *pdf.File, opt RewriteOptions) ([]byte, error) {
	count := file.PageCount()
	recs := make([]*tag.Recorder, count)
	for page := range count {
		rec := tag.NewRecorder()
		if err := paintTagPage(ctx, file, page, rec); err != nil {
			return nil, asPDFJobError(err)
		}
		recs[page] = rec
	}
	plan, err := tag.DerivePlan(recs)
	if err != nil {
		return nil, asPDFJobError(err)
	}
	out, err := tag.Build(ctx, file, recs, plan, tag.Options{
		Title: opt.Title,
		Lang:  opt.Lang,
		Claim: opt.Claim,
	})
	if err != nil {
		return out, asPDFJobError(err)
	}
	return out, nil
}

// paintTagPage records one page at 72 dpi, so one device pixel is one user
// point and the recorder re-emits geometry unchanged.
func paintTagPage(ctx context.Context, file *pdf.File, page int, rec *tag.Recorder) error {
	content, err := file.Content(page)
	if err != nil {
		return err
	}
	res, err := file.PageResources(page)
	if err != nil {
		return err
	}
	return pdf.PaintWith(ctx, content, rec, 1, pdf.PaintOptions{
		Resources:     res,
		Text:          pdf.TextOptions{Fonts: nil, Sink: nil, Runs: rec},
		MarkedContent: rec,
	})
}
