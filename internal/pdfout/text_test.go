package pdfout

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/internal/pdf"
)

// TestRewriteTextUndefined proves level 0 still refuses a page that shows
// text. The recorder cannot take glyphs, so EmitPage returns undefined in Tj
// instead of silently outlining the text.
func TestRewriteTextUndefined(t *testing.T) {
	file := textEmitPDF(t)
	got, err := EmitPage(t.Context(), file, 0)
	var job *pdf.Error
	if !errors.As(err, &job) || job.Op != "Tj" || job.Name != errUndefined {
		t.Fatalf("EmitPage() error = %v, want undefined in Tj", err)
	}
	if got != nil {
		t.Fatalf("EmitPage() bytes = %#v, want nil", got)
	}
}

// textEmitPDF is one page whose content shows two characters with a
// standard 14 font.
func textEmitPDF(t *testing.T) *pdf.File {
	t.Helper()
	doc := newFixtureDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R /Contents 4 0 R " +
		"/Resources << /Font << /F1 5 0 R >> >> >>")
	doc.object(string(streamBody([]byte("BT /F1 12 Tf 72 720 Td (Hi) Tj ET"), false)))
	doc.object("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica " +
		"/FirstChar 72 /Widths [722 278] >>")
	return mustOpenPDF(t, doc.classic())
}
