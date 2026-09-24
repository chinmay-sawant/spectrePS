package pdfout

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// Emit interprets content with the phase 06 path subset and returns canonical operators.
// Coordinates stay in user space. The result is not a copy of content.
// A cancelled context returns ctx.Err() and a nil slice.
// A nil context panics with "pdfout: nil context".
// An unsupported operator is returned unchanged from pdf.Paint (*pdf.Error).
// Empty content returns an empty non-nil slice.
func Emit(ctx context.Context, content []byte) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec := newRecorder()
	if err := pdf.Paint(ctx, content, rec, 1); err != nil {
		return nil, err
	}
	return rec.bytes(), nil
}
