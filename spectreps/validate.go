package spectreps

// This file exposes the PDF/UA-2 preflight as a public read-only job. The
// result is preflight only, never certification.

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdfa"
)

// PreflightUA2 runs the PDF/UA-2 machine checks on an open document and
// returns a JobError with Op "PDFUA" and the failed rule in Msg. The rules
// are the ones in documentation/devices.md. The checks run only for a caller
// that asks, so an untagged document is not a UA-2 request.
// A nil document returns rangecheck. A canceled context returns ctx.Err().
func (in *Instance) PreflightUA2(ctx context.Context, doc *Document) error {
	if ctx == nil {
		panic("spectreps: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = in
	if doc == nil || doc.file == nil {
		return JobError{Op: "PDFUA", Msg: "rangecheck", Filename: "", Line: 0, Column: 0}
	}
	return asPDFJobError(pdfa.PreflightUA2(ctx, doc.file))
}
