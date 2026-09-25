package spectreps_test

import (
	"errors"
	"testing"

	"github.com/chinmay-sawant/spectrePS/spectreps"
)

// TestValidationNilDocument locks the nil Document surface: zero page count,
// no tags, and rangecheck from every document job.
func TestValidationNilDocument(t *testing.T) {
	in := newInst(t)
	var doc *spectreps.Document

	if got := doc.PageCount(); got != 0 {
		t.Fatalf("nil PageCount() = %d, want 0", got)
	}
	if doc.Tagged() {
		t.Fatal("nil Tagged() = true, want false")
	}

	t.Run("RasterizePage", func(t *testing.T) {
		img, err := in.RasterizePage(t.Context(), doc, 0, spectreps.RunOptions{})
		validationRangecheck(t, err, "RasterizePage")
		requireZeroPageImage(t, img)
	})
	t.Run("ExtractText", func(t *testing.T) {
		text, err := in.ExtractText(t.Context(), doc, 0)
		validationRangecheck(t, err, "ExtractText")
		if text != "" {
			t.Fatalf("ExtractText() = %q, want empty", text)
		}
	})
	t.Run("RewritePDF", func(t *testing.T) {
		out, err := in.RewritePDF(t.Context(), doc, spectreps.DefaultRewriteOptions())
		validationRangecheck(t, err, "RewritePDF")
		if out != nil {
			t.Fatalf("RewritePDF() bytes = %#v, want nil", out)
		}
	})
	t.Run("WritePostScript", func(t *testing.T) {
		out, err := in.WritePostScript(t.Context(), doc, spectreps.PostScriptOptions{})
		validationRangecheck(t, err, "WritePostScript")
		if out != nil {
			t.Fatalf("WritePostScript() bytes = %#v, want nil", out)
		}
	})
}

// TestValidationPixelCap locks the measured pixel cap: a letter page at
// 600 dpi is 5100 by 6600, which is under the 40,000,000 cap, and the area
// cap is crossed at 655 dpi.
func TestValidationPixelCap(t *testing.T) {
	in := newInst(t)

	pages, err := in.RunPostScript(t.Context(), []byte("showpage"), spectreps.RunOptions{
		PageWidthPt:   612,
		PageHeightPt:  792,
		ResolutionDPI: 600,
	})
	if err != nil {
		t.Fatalf("letter at 600 dpi error = %v, want nil", err)
	}
	if len(pages) != 1 || pages[0].Width != 5100 || pages[0].Height != 6600 {
		t.Fatalf("letter at 600 dpi = %dx%d pages=%d, want 5100x6600 pages=1",
			pages[0].Width, pages[0].Height, len(pages))
	}

	_, err = in.RunPostScript(t.Context(), []byte("showpage"), spectreps.RunOptions{
		PageWidthPt:   612,
		PageHeightPt:  792,
		ResolutionDPI: 655,
	})
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != "limitcheck" || job.Op != "raster" {
		t.Fatalf("letter at 655 dpi error = %v, want limitcheck in raster", err)
	}
}

func validationRangecheck(t *testing.T, err error, op string) {
	t.Helper()
	var job spectreps.JobError
	if !errors.As(err, &job) || job.Msg != rangecheckMsg || job.Op != op {
		t.Fatalf("error = %v, want rangecheck in %s", err, op)
	}
}
