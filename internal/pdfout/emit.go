package pdfout

import (
	"context"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

const (
	opDo         = "Do"
	errUndefined = "undefined"
)

// Emit interprets content with the phase 06 path subset and returns canonical operators.
// Coordinates are transformed by the content matrix, then scaled. The result is not a copy of content.
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
	return emitBytes(rec)
}

// EmitPage interprets one page of an open document with its page resources.
// It is Emit plus the /XObject lookup Do needs.
// A page that paints an image returns undefined in Do, because the rewrite
// recorder cannot write image pixels.
// A cancelled context returns ctx.Err() and a nil slice.
// A nil context panics with "pdfout: nil context".
func EmitPage(ctx context.Context, file *pdf.File, index int) ([]byte, error) {
	if ctx == nil {
		panic(panicNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := file.Content(index)
	if err != nil {
		return nil, err
	}
	res, err := file.PageResources(index)
	if err != nil {
		return nil, err
	}
	rec := newRecorder()
	if err := pdf.PaintWith(ctx, content, rec, 1, pdf.PaintOptions{
		Resources:     res,
		Text:          pdf.TextOptions{Fonts: nil, Sink: nil, Runs: nil},
		MarkedContent: nil,
	}); err != nil {
		return nil, err
	}
	return emitBytes(rec)
}

// emitBytes returns the recorded operators. A recorder that saw an image
// returns undefined in Do, so level 0 does not drop or outline the image.
func emitBytes(rec *recorder) ([]byte, error) {
	if rec.sawImage {
		return nil, pdf.NewError(opDo, errUndefined)
	}
	return rec.bytes(), nil
}
