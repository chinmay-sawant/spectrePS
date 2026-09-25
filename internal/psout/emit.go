package psout

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const panicNilContext = "psout: nil context"

// Emit runs one PDF content stream through the recorder and returns PostScript
// path operators in 72 dpi points. It is the ps2write shape of pdfout.Emit:
// the coordinates are transformed by the content matrix and written as m and l,
// and curves arrive flattened as line segments because pdf.Paint flattens them.
// A canceled context returns ctx.Err() and a nil slice.
// A nil context panics with "psout: nil context".
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
